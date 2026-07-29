package fuzzy

import (
	"slices"
	"testing"
)

func TestMatch(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		query   string
		target  string
		wantOK  bool
		wantIdx []int
	}{
		{"empty query matches anything", "", "Export report", true, nil},
		{"empty query matches an empty target", "", "", true, nil},
		{"query longer than target", "exports", "exp", false, nil},
		{"no match at all", "zz", "Export report", false, nil},
		{"exact prefix", "exp", "Export report", true, []int{0, 1, 2}},
		{"query upper, target lower", "EXP", "export report", true, []int{0, 1, 2}},
		{"query lower, target upper", "exp", "EXPORT REPORT", true, []int{0, 1, 2}},
		{"subsequence with gaps", "ert", "Export report", true, []int{0, 4, 5}},
		// Greedy matching returns [0, 5] here: r at 0, then the first t, the one
		// inside "report" — highlighting repor[t]. The word-start bonus is what
		// makes [0, 7] win instead: [r]eport [t]able. This row is the reason
		// Match is a dynamic program and not a left-to-right scan.
		{"prefers a word start over the nearest rune", "rt", "report table", true, []int{0, 7}},
		// idx is in RUNES. With byte offsets this row would be [0, 5].
		{"multibyte target", "cà", "città", true, []int{0, 4}},
		{"whole target", "abc", "abc", true, []int{0, 1, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, idx, ok := Match(tc.query, tc.target)
			if ok != tc.wantOK {
				t.Fatalf("Match(%q, %q) ok = %v, want %v", tc.query, tc.target, ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if !slices.Equal(idx, tc.wantIdx) {
				t.Errorf("Match(%q, %q) idx = %v, want %v", tc.query, tc.target, idx, tc.wantIdx)
			}
		})
	}
}

// TestMatchScoreIsExact pins the arithmetic on the smallest case that exercises
// both bonuses, so the weights cannot drift unnoticed while the orderings in
// TestMatchRanks still happen to hold.
func TestMatchScoreIsExact(t *testing.T) {
	t.Parallel()
	// a at 0: word start (+8), no lead penalty. b at 1: consecutive (+10).
	score, _, ok := Match("ab", "ab")
	if !ok {
		t.Fatal(`Match("ab", "ab") did not match`)
	}
	if score != 18 {
		t.Errorf("score = %d, want 18", score)
	}
}

// TestMatchRanks pins the orderings the palette depends on. These survive a
// tweak to the weights; TestMatchScoreIsExact does not. That split is
// deliberate: the constants are tunable, the orderings are not.
func TestMatchRanks(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		query  string
		better string
		worse  string
	}{
		// "Export", not "Export report": screen actions are labelled
		// capitalize(binding.Help().Desc), and Export's description is "export".
		{"a screen action beats the navigation row", "exp", "Export", "Go to export"},
		{"a prefix beats a match buried inside", "rep", "Report", "Go to prep"},
		{"consecutive beats scattered", "ab", "ab c", "a x b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, _, ok := Match(tc.query, tc.better)
			if !ok {
				t.Fatalf("Match(%q, %q) did not match", tc.query, tc.better)
			}
			w, _, ok := Match(tc.query, tc.worse)
			if !ok {
				t.Fatalf("Match(%q, %q) did not match", tc.query, tc.worse)
			}
			if b <= w {
				t.Errorf("Match(%q, %q) = %d, want more than Match(%q, %q) = %d",
					tc.query, tc.better, b, tc.query, tc.worse, w)
			}
		})
	}
}
