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
		"45":    45 * time.Hour, // numero nudo = ore (decisione spec)
		" 2h ":  2 * time.Hour,  // trim
	}
	for in, want := range cases {
		got, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q) errore inatteso: %v", in, err)
		}
		if got != want {
			t.Errorf("Parse(%q) = %v, atteso %v", in, got, want)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	for _, in := range []string{"", "abc", "0", "0h", "-5", "1.5.5h", "h30", "m"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q): atteso errore, non ricevuto", in)
		}
	}
}
