package output

import (
	"fmt"
	"math"
	"strings"
)

func FormatBytes(b int64) string {
	if b < 0 {
		return "0 B"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func FormatDuration(seconds int) string {
	if seconds < 0 {
		return "00:00"
	}
	if seconds < 3600 {
		return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
	}
	return fmt.Sprintf("%02d:%02d:%02d", seconds/3600, (seconds%3600)/60, seconds%60)
}

func MaskKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

func TruncateUUID(uuid string) string {
	if len(uuid) <= 8 {
		return uuid
	}
	return uuid[:8]
}

func Pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

func FormatBool(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func SafeString(s string) string {
	return strings.TrimSpace(s)
}

func RoundToStr(v float64, decimals int) string {
	pow := math.Pow(10, float64(decimals))
	return fmt.Sprintf("%.*f", decimals, math.Round(v*pow)/pow)
}
