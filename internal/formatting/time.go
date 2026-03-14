package formatting

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var relativeTimeRe = regexp.MustCompile(`^(\d+)([mhdw])$`)

// ParseTime parses a human-friendly time value into a Unix timestamp (float64).
// It uses the current UTC time for relative calculations.
//
// Accepts:
//
//	Relative: 30m, 3h, 2d, 1w
//	Absolute: 2026-03-01, 2026-03-01T14:00:00, 2026-03-01 14:00:00
//	Raw:      1741234567
func ParseTime(value string) (float64, error) {
	return ParseTimeWithNow(value, time.Now().UTC())
}

// ParseTimeWithNow is like ParseTime but accepts a fixed "now" for testability.
func ParseTimeWithNow(value string, now time.Time) (float64, error) {
	// Try relative time (e.g. 30m, 3h, 2d, 1w).
	if m := relativeTimeRe.FindStringSubmatch(value); m != nil {
		n, _ := strconv.Atoi(m[1])
		unit := m[2]
		var d time.Duration
		switch unit {
		case "m":
			d = time.Duration(n) * time.Minute
		case "h":
			d = time.Duration(n) * time.Hour
		case "d":
			d = time.Duration(n) * 24 * time.Hour
		case "w":
			d = time.Duration(n) * 7 * 24 * time.Hour
		}
		return float64(now.Add(-d).Unix()), nil
	}

	// Try absolute date/datetime formats.
	for _, layout := range []string{
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, value); err == nil {
			return float64(t.UTC().Unix()), nil
		}
	}

	// Try raw unix timestamp.
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f, nil
	}

	return 0, fmt.Errorf("cannot parse time: %q", value)
}
