package tui

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// Display width and rune count agree only for ASCII, which is why every fixture
// here that matters is not ASCII.
func TestTruncateWidth(t *testing.T) {
	tests := []struct {
		name string
		s    string
		cols int
		want string
	}{
		{"empty", "", 5, ""},
		{"fits", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"cuts ascii", "hello world", 5, "hell…"},
		{"zero cols", "hello", 0, ""},
		{"negative cols", "hello", -3, ""},
		{"one col fits", "x", 1, "x"},
		{"one col cuts", "xy", 1, "…"},
		// A wide cluster is dropped whole rather than split, so the result can
		// land NARROWER than cols. That is what makes the pad in cell() load-bearing.
		{"wide runes cut on the boundary", "日本語のリスト", 2, "…"},
		{"wide runes cut inside", "日本語のリスト", 5, "日本…"},
		{"emoji", "🚀🚀🚀🚀", 3, "🚀…"},
		{"mixed", "ab🚀cd", 5, "ab🚀…"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateWidth(tc.s, tc.cols)
			if got != tc.want {
				t.Errorf("truncateWidth(%q, %d) = %q, want %q", tc.s, tc.cols, got, tc.want)
			}
			if w := lipgloss.Width(got); tc.cols > 0 && w > tc.cols {
				t.Errorf("truncateWidth(%q, %d) rendered %d columns, want at most %d", tc.s, tc.cols, w, tc.cols)
			}
		})
	}
}

// cell must produce EXACTLY cols columns, which is the half that fmt's "%-Ns"
// gets wrong: fmt pads by rune count, so a 7-rune 14-column string padded to
// "%-10s" comes out 17 columns wide.
func TestCellIsExactlyColsWide(t *testing.T) {
	for _, s := range []string{"", "x", "hello", "hello world", "日本語のリスト", "🚀🚀🚀🚀", "ab🚀cd"} {
		for _, cols := range []int{1, 2, 5, 10, 24} {
			got := cell(s, cols)
			if w := lipgloss.Width(got); w != cols {
				t.Errorf("cell(%q, %d) rendered %d columns, want exactly %d (%q)", s, cols, w, cols, got)
			}
		}
	}
}

func TestCellNonPositiveCols(t *testing.T) {
	for _, cols := range []int{0, -1, -12} {
		if got := cell("日本語", cols); got != "" {
			t.Errorf("cell(%q, %d) = %q, want %q", "日本語", cols, got, "")
		}
	}
}

// The migration in Task 2 must be a no-op for ASCII content at cols >= 2: that
// is what keeps every existing golden green, so the wide-rune tests are the only
// thing that distinguishes before from after. cols == 1 is the one documented
// divergence — the old rune cut returned "…" even when the input fit.
func TestCellMatchesTheOldPaddedCutForASCII(t *testing.T) {
	for _, s := range []string{"", "x", "Website", "Website redesign", "Mobile app"} {
		for _, cols := range []int{2, 5, 20, 24, 40} {
			want := fmt.Sprintf("%-*s", cols, oldRuneCutForTest(s, cols))
			if got := cell(s, cols); got != want {
				t.Errorf("cell(%q, %d) = %q, want %q (the pre-migration rendering)", s, cols, got, want)
			}
		}
	}
}

// oldRuneCutForTest is the rune-based cut this tranche deletes, kept in the test
// file only, as the reference the ASCII equivalence is measured against.
func oldRuneCutForTest(s string, n int) string {
	if s == "" {
		return ""
	}
	if n <= 1 {
		return "…"
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// The report footer cuts styled strings, so the cut must survive escapes.
// TestMain pins termenv.Ascii for this package, so a theme style renders without
// escapes: the fixture writes them by hand instead.
func TestTruncateWidthIsANSIAware(t *testing.T) {
	const styled = "\x1b[1mabcdef\x1b[0m"
	got := truncateWidth(styled, 3)
	if want := "\x1b[1mab…\x1b[0m"; got != want {
		t.Errorf("truncateWidth(%q, 3) = %q, want %q", styled, got, want)
	}
	if w := lipgloss.Width(got); w != 3 {
		t.Errorf("truncated styled string rendered %d columns, want 3", w)
	}
}

// A style left OPEN by the caller is not closed by the cut: the escape survives
// and bleeds past it. Nothing in production feeds an unterminated style today
// (help.View closes its own), and this test exists so that a caller who starts
// doing it finds out here rather than on a user's terminal.
func TestTruncateWidthDoesNotCloseAnOpenStyle(t *testing.T) {
	got := truncateWidth("\x1b[31mabcdef", 3)
	if want := "\x1b[31mab…"; got != want {
		t.Errorf("truncateWidth of an unterminated style = %q, want %q", got, want)
	}
}

// The rates list, the entries browser and the budget screen all pair a cut with
// a fixed-width column. With a wide-rune list name the row used to render wider
// than its own column, which is what pushed those screens past the terminal
// edge. The fixture is deliberately CJK: an ASCII name passes against the bug.
func TestFixedWidthRowsHoldTheirColumnWithWideRunes(t *testing.T) {
	const wide = "日本語のリストの名前がとても長い場合"
	if lipgloss.Width(wide) == len([]rune(wide)) {
		t.Fatalf("fixture is not wide: %d runes, %d columns", len([]rune(wide)), lipgloss.Width(wide))
	}
	for _, cols := range []int{20, 24, 30} {
		if w := lipgloss.Width(cell(wide, cols)); w != cols {
			t.Errorf("cell(wide, %d) rendered %d columns, want %d", cols, w, cols)
		}
	}
}
