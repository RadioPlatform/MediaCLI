package api

import (
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

var DefaultRetryConfig = RetryConfig{
	MaxRetries:     5,
	InitialBackoff: 1 * time.Second,
	MaxBackoff:     30 * time.Second,
}

type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}

func (RealClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	ae := AsAPIError(err)
	if ae != nil {
		return ae.Code == ErrRateLimited || ae.Code == ErrNetwork
	}
	return true
}

func isRetryableStatusCode(status int) bool {
	return status == 429 || status >= 500
}

func parseRetryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	h := resp.Header.Get("Retry-After")
	if h == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(strings.TrimSpace(h)); err == nil {
		return time.Duration(seconds) * time.Second
	}

	if t, err := time.Parse(time.RFC1123, strings.TrimSpace(h)); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		return d
	}

	return 0
}

func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	j := time.Duration(rand.Int63n(int64(d)))
	return d + j
}

func backoff(attempt int, cfg RetryConfig) time.Duration {
	delay := float64(cfg.InitialBackoff) * math.Pow(2, float64(attempt))
	if delay > float64(cfg.MaxBackoff) {
		delay = float64(cfg.MaxBackoff)
	}
	return jitter(time.Duration(delay))
}
