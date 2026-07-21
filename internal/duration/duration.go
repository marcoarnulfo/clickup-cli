// Package duration parses durations in human-readable format (2h30, 1.5h, 90m, 45).
// It is pure: no I/O, no external dependencies.
package duration

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	reHM  = regexp.MustCompile(`^(\d+)h(\d+)m?$`)    // 2h30, 2h30m
	reH   = regexp.MustCompile(`^(\d+(?:\.\d+)?)h$`) // 2h, 1.5h
	reM   = regexp.MustCompile(`^(\d+)m$`)           // 90m
	reNum = regexp.MustCompile(`^\d+(?:\.\d+)?$`)    // 45 (bare number = hours)
)

// Parse converts a duration string into a time.Duration.
// Formats: Nh, NhMm/NhM (hours+minutes), N.Nh/N,Nh (decimal hours), Nm (minutes),
// bare number = hours. The comma is accepted like the dot (Italian keyboard).
// Returns an error on unrecognized input or duration <= 0.
func Parse(s string) (time.Duration, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, ",", ".")
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	var d time.Duration
	switch {
	case reHM.MatchString(s):
		m := reHM.FindStringSubmatch(s)
		h, _ := strconv.Atoi(m[1])
		min, _ := strconv.Atoi(m[2])
		d = time.Duration(h)*time.Hour + time.Duration(min)*time.Minute
	case reH.MatchString(s):
		m := reH.FindStringSubmatch(s)
		f, _ := strconv.ParseFloat(m[1], 64)
		d = time.Duration(f * float64(time.Hour))
	case reM.MatchString(s):
		m := reM.FindStringSubmatch(s)
		min, _ := strconv.Atoi(m[1])
		d = time.Duration(min) * time.Minute
	case reNum.MatchString(s):
		f, _ := strconv.ParseFloat(s, 64)
		d = time.Duration(f * float64(time.Hour))
	default:
		return 0, fmt.Errorf("unrecognized duration: %q", s)
	}

	if d <= 0 {
		return 0, fmt.Errorf("duration must be > 0: %q", s)
	}
	return d, nil
}

// Format renders a duration in compact human form: "2h", "1h 30m", "45m".
// Sub-minute remainders are dropped (logged durations are minute-grained).
func Format(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	default:
		return fmt.Sprintf("%dm", m)
	}
}
