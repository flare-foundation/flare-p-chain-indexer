package shared

import (
	"time"
)

const (
	ApplicationVersion = "2.2.0"
)

var (
	// Map from network name (HRP) to Durango fork time
	DurangoTimes = map[string]time.Time{
		"flare":      time.Date(2025, time.August, 5, 12, 0, 0, 0, time.UTC),
		"costwo":     time.Date(2025, time.June, 24, 12, 0, 0, 0, time.UTC),
		"localflare": time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
		"coston":     time.Date(2025, time.July, 1, 12, 0, 0, 0, time.UTC),
		"songbird":   time.Date(2025, time.July, 22, 12, 0, 0, 0, time.UTC),
		"local":      time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
)
