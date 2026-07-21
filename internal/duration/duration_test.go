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
		2*time.Hour + 30*time.Second: "2h", // sub-minute dropped
	}
	for in, want := range cases {
		if got := Format(in); got != want {
			t.Errorf("Format(%v) = %q, want %q", in, got, want)
		}
	}
}
