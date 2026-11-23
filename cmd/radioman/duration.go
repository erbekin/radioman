package main

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var msDurationRe = regexp.MustCompile(`(?i)^(\d+(?:\.\d+)?|\.\d+)\s*([a-z]+)?$`)

// ParseAutoStopDuration parses an ms-style duration string (e.g. "10m", "1.5h",
// "5000", "2 days", "0", "off") into a non-negative time.Duration.
func ParseAutoStopDuration(input string) (time.Duration, error) {
	trimmed := strings.TrimSpace(input)
	switch strings.ToLower(trimmed) {
	case "", "0", "off", "none", "false", "disabled":
		return 0, nil
	}

	m := msDurationRe.FindStringSubmatch(trimmed)
	if m == nil {
		return 0, fmt.Errorf("invalid auto_stop duration %q (examples: 10m, 1.5h, 30s, 0)", input)
	}

	val, err := strconv.ParseFloat(m[1], 64)
	if err != nil || math.IsNaN(val) || math.IsInf(val, 0) || val < 0 {
		return 0, fmt.Errorf("invalid auto_stop duration %q", input)
	}

	unit := strings.ToLower(m[2])
	var mult time.Duration
	switch unit {
	case "", "ms", "msec", "msecs", "millisecond", "milliseconds":
		mult = time.Millisecond
	case "s", "sec", "secs", "second", "seconds":
		mult = time.Second
	case "m", "min", "mins", "minute", "minutes":
		mult = time.Minute
	case "h", "hr", "hrs", "hour", "hours":
		mult = time.Hour
	case "d", "day", "days":
		mult = 24 * time.Hour
	case "w", "week", "weeks":
		mult = 7 * 24 * time.Hour
	case "y", "yr", "yrs", "year", "years":
		mult = 31557600 * time.Second
	default:
		return 0, fmt.Errorf("unknown duration unit %q in %q", m[2], input)
	}

	if val == 0 {
		return 0, nil
	}

	ns := val * float64(mult)
	if ns > float64(math.MaxInt64) {
		return 0, fmt.Errorf("auto_stop duration %q overflows maximum duration", input)
	}

	d := time.Duration(math.Round(ns))
	if d <= 0 {
		return 0, fmt.Errorf("auto_stop duration %q is too small", input)
	}
	return d, nil
}

// NormalizeAutoStopValue validates input and returns its canonical stored string
// ("0" when disabled, otherwise the trimmed input) and parsed duration.
func NormalizeAutoStopValue(input string) (string, time.Duration, error) {
	d, err := ParseAutoStopDuration(input)
	if err != nil {
		return "", 0, err
	}
	if d == 0 {
		return "0", 0, nil
	}
	return strings.TrimSpace(input), d, nil
}

// FormatRemaining formats a remaining duration rounded up to whole seconds
// in a compact human-readable form (e.g. "45s", "9m42s", "10m", "1h23m", "2h").
func FormatRemaining(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	totalSecs := int64(math.Ceil(d.Seconds()))
	if totalSecs <= 0 {
		totalSecs = 1
	}

	hours := totalSecs / 3600
	mins := (totalSecs % 3600) / 60
	secs := totalSecs % 60

	var b strings.Builder
	if hours > 0 {
		fmt.Fprintf(&b, "%dh", hours)
	}
	if mins > 0 {
		fmt.Fprintf(&b, "%dm", mins)
	}
	if secs > 0 {
		fmt.Fprintf(&b, "%ds", secs)
	}
	return b.String()
}
