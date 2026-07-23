package duration

import (
	"testing"
	"time"
)

func TestParseValid(t *testing.T) {
	cases := map[string]time.Duration{
		"2h30":  2*time.Hour + 30*time.Minute,
		"2h30m": 2*time.Hour + 30*time.Minute,
		"2h":    2 * time.Hour,
		"1.5h":  90 * time.Minute,
		"1,5h":  90 * time.Minute,
		"90m":   90 * time.Minute,
		"45":    45 * time.Hour,               // bare number = hours (spec decision)
		" 2h ":  2 * time.Hour,                // trim
		"2h90m": 3*time.Hour + 30*time.Minute, // minutes overflow is summed
		"2H":    2 * time.Hour,                // uppercase is lowercased
		"1H30":  1*time.Hour + 30*time.Minute,
	}
	for in, want := range cases {
		got, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q) unexpected error: %v", in, err)
		}
		if got != want {
			t.Errorf("Parse(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	for _, in := range []string{"", "abc", "0", "0h", "-5", "1.5.5h", "h30", "m"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q): expected error, got none", in)
		}
	}
}

func TestFormat(t *testing.T) {
	cases := map[time.Duration]string{
		90 * time.Minute:             "1h 30m",
		2 * time.Hour:                "2h",
		45 * time.Minute:             "45m",
		3*time.Hour + 30*time.Minute: "3h 30m",
		0:                            "0m",
		2*time.Hour + 30*time.Second: "2h",     // sub-minute dropped
		-90 * time.Minute:            "1h 30m", // negative is normalized
	}
	for in, want := range cases {
		if got := Format(in); got != want {
			t.Errorf("Format(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatHours(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{90 * time.Minute, "1.50h"},
		{0, "0.00h"},
		{45 * time.Minute, "0.75h"},
		{2 * time.Hour, "2.00h"},
	}
	for _, c := range cases {
		if got := FormatHours(c.d); got != c.want {
			t.Errorf("FormatHours(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestRound(t *testing.T) {
	q := 15 * time.Minute
	cases := []struct {
		name   string
		d, inc time.Duration
		mode   RoundMode
		want   time.Duration
	}{
		{"off-zero-inc", 7 * time.Minute, 0, RoundNearest, 7 * time.Minute},
		{"nearest-up", 8 * time.Minute, q, RoundNearest, 15 * time.Minute},
		{"nearest-down", 7 * time.Minute, q, RoundNearest, 0},
		{"nearest-exact", 30 * time.Minute, q, RoundNearest, 30 * time.Minute},
		{"up-any", 1 * time.Minute, q, RoundUp, 15 * time.Minute},
		{"up-exact", 30 * time.Minute, q, RoundUp, 30 * time.Minute},
	}
	for _, c := range cases {
		if got := Round(c.d, c.inc, c.mode); got != c.want {
			t.Errorf("%s: Round(%v,%v)=%v want %v", c.name, c.d, c.inc, got, c.want)
		}
	}
}

func TestClock(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "00:00:00"},
		{"seconds only", 45 * time.Second, "00:00:45"},
		{"minutes and seconds", 1*time.Minute + 5*time.Second, "00:01:05"},
		{"hours", 1*time.Hour + 23*time.Minute + 45*time.Second, "01:23:45"},
		{"over 24h not capped", 25*time.Hour + 2*time.Minute, "25:02:00"},
		{"negative clamps to zero", -3 * time.Second, "00:00:00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Clock(tt.d); got != tt.want {
				t.Errorf("Clock(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}
