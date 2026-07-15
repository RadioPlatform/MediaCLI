package api

import (
	"testing"
	"time"
)

func TestParseRetryAfterSeconds(t *testing.T) {
	resp := testResponse(429)
	resp.Header.Set("Retry-After", "5")
	d := parseRetryAfter(resp)
	if d != 5*time.Second {
		t.Errorf("expected 5s, got %v", d)
	}
}

func TestParseRetryAfterHTTPDate(t *testing.T) {
	resp := testResponse(429)
	resp.Header.Set("Retry-After", time.Now().Add(10*time.Second).Format(time.RFC1123))
	d := parseRetryAfter(resp)
	if d <= 0 || d > 15*time.Second {
		t.Errorf("expected ~10s, got %v", d)
	}
}

func TestParseRetryAfterPastDate(t *testing.T) {
	resp := testResponse(429)
	resp.Header.Set("Retry-After", time.Now().Add(-1*time.Hour).Format(time.RFC1123))
	d := parseRetryAfter(resp)
	if d != 0 {
		t.Errorf("expected 0 for past date, got %v", d)
	}
}

func TestBackoffIncreases(t *testing.T) {
	d1 := backoff(0, DefaultRetryConfig)
	d2 := backoff(1, DefaultRetryConfig)
	d3 := backoff(2, DefaultRetryConfig)

	if d2 <= d1 {
		t.Errorf("backoff should increase: %v <= %v", d2, d1)
	}
	if d3 <= d2 {
		t.Errorf("backoff should increase: %v <= %v", d3, d2)
	}
}

func TestBackoffMax(t *testing.T) {
	for i := 0; i < 20; i++ {
		d := backoff(i, DefaultRetryConfig)
		if d > DefaultRetryConfig.MaxBackoff*2 {
			t.Errorf("backoff %d exceeded max: %v", i, d)
		}
	}
}

func TestRetryableStatusCodes(t *testing.T) {
	tests := []struct {
		code int
		want bool
	}{
		{429, true},
		{500, true},
		{502, true},
		{503, true},
		{504, true},
		{200, false},
		{201, false},
		{400, false},
		{401, false},
		{403, false},
		{404, false},
		{422, false},
	}
	for _, tt := range tests {
		if got := isRetryableStatusCode(tt.code); got != tt.want {
			t.Errorf("isRetryableStatusCode(%d) = %v, want %v", tt.code, got, tt.want)
		}
	}
}
