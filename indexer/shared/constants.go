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
		"flare":      time.Date(10000, time.December, 1, 0, 0, 0, 0, time.UTC),
		"costwo":     time.Date(2025, time.June, 24, 12, 0, 0, 0, time.UTC),
		"localflare": time.Date(2025, time.May, 15, 14, 0, 0, 0, time.UTC),
		"coston":     time.Date(2025, time.July, 1, 12, 0, 0, 0, time.UTC),
		"songbird":   time.Date(10000, time.December, 1, 0, 0, 0, 0, time.UTC),
		"local":      time.Date(10000, time.December, 1, 0, 0, 0, 0, time.UTC),
	}
)
