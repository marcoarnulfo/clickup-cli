package tui

import (
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// #64: the budget view renders one text progress bar per report.BudgetLine.
func TestBudgetViewRendersProgressBar(t *testing.T) {
	lines := []report.BudgetLine{
		{ListID: "list-1", ListName: "Website", Currency: "EUR",
			Budget: 1000, Billed: 600, Remaining: 400, PercentUsed: 60},
	}
	out := newBudget(lines).view(testTheme(true), 80)
	for _, want := range []string{"Website", "60%", "600.00", "1000.00", "EUR"} {
		if !strings.Contains(out, want) {
			t.Errorf("budget view missing %q; got:\n%s", want, out)
		}
	}
	if !strings.ContainsRune(out, gaugeFull) || !strings.ContainsRune(out, gaugeEmpty) {
		t.Errorf("expected a bar built from %q and %q; got:\n%s", gaugeFull, gaugeEmpty, out)
	}
}

func TestBudgetViewEmptyShowsMessage(t *testing.T) {
	out := newBudget(nil).view(testTheme(true), 80)
	if !strings.Contains(out, "No budgets") {
		t.Errorf("empty budget view should say no budgets are configured; got:\n%s", out)
	}
}

// A list over its budget must still be visible in the number even though the
// bar itself caps its fill at 100%. This is exactly what bubbles/progress
// cannot do — it clamps the percentage to 100 before formatting it — and it is
// why this screen keeps its own bar.
func TestRenderBudgetBarClampsFillNotPercent(t *testing.T) {
	t.Parallel()
	out := renderBudgetBar(testTheme(true), 150)
	if !strings.Contains(out, "150%") {
		t.Errorf("renderBudgetBar(150) = %q, want the unclamped 150%% in the label", out)
	}
	if full := strings.Repeat(string(gaugeFull), budgetBarWidth); !strings.Contains(out, full) {
		t.Errorf("renderBudgetBar(150) = %q, want a fully filled bar", out)
	}
}

// Over budget is the state the view exists to surface, so it must not look the
// same as a healthy one. The goldens run under termenv.Ascii, which strips the
// color entirely, so this asserts on the style rather than the output.
func TestBudgetBarColorsOverBudget(t *testing.T) {
	t.Parallel()
	th := paletteTheme(true) // real colors, so foregrounds are comparable
	under := budgetFillStyle(th, 60)
	over := budgetFillStyle(th, 130)
	if under.GetForeground() != th.OK.GetForeground() {
		t.Error("a bar under budget is not drawn in the OK color")
	}
	if over.GetForeground() != th.Err.GetForeground() {
		t.Error("a bar over budget is not drawn in the Err color")
	}
}

// TestBudgetKeyLabels pins the exact label set budget.go's updateBudget
// accepts today (every case label, verbatim), plus q — handled globally in
// app.go, in no case clause of budget.go itself. A dropped label is caught by
// the two transition tests below; this catches an invented one.
func TestBudgetKeyLabels(t *testing.T) {
	t.Parallel()
	m := Model{screen: screenBudget}
	want := []string{"?", "b", "ctrl+c", "ctrl+p", "esc", "q"}
	if got := enabledLabels(keysFor(m)); !slices.Equal(got, want) {
		t.Errorf("budget labels = %v, want %v", got, want)
	}
}

// #59 Task 3 step 3: esc and b both close the budget view back to Report —
// neither has a test that would fail if it went mute.
func TestBudgetEscReturnsReport(t *testing.T) {
	m := Model{screen: screenBudget, nav: []screen{screenReport}}
	next, _ := m.updateBudget(tea.KeyMsg{Type: tea.KeyEsc})
	if got := next.(Model).screen; got != screenReport {
		t.Errorf("esc from budget -> %v, want screenReport", got)
	}
}

func TestBudgetBReturnsReport(t *testing.T) {
	m := Model{screen: screenBudget, nav: []screen{screenReport}}
	next, _ := m.updateBudget(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if got := next.(Model).screen; got != screenReport {
		t.Errorf("b from budget -> %v, want screenReport", got)
	}
}

// The budget line is unbounded: it grows with the figures. Measured before the
// fix, testdata/budget.golden is 94 columns (90 of content + 4 of th.Box), and
// 80 columns is a bare terminal and a split tmux pane (#136).
//
// The widths are the ones this fixture can actually fit: the row needs
// box(4) + 1 + gauge(20) + 1 + pct + 2 + figures + name-floor(12), which is 65
// for these two lines. Asserting at 60 would demand the impossible — below the
// threshold the row overflows by design, and the spec says so.
func TestBudgetViewNeverExceedsWidth(t *testing.T) {
	th := testTheme(true)
	bm := newBudget([]report.BudgetLine{
		{ListName: "Website", Billed: 625, Budget: 1000, Currency: "EUR", Remaining: 375, PercentUsed: 62.5},
		{ListName: "Mobile app", Billed: 1040, Budget: 1000, Currency: "EUR", Remaining: -40, PercentUsed: 104},
	})
	for _, width := range []int{65, 80, 100, 120} {
		for _, line := range strings.Split(bm.view(th, width), "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Errorf("width %d: line rendered %d columns: %q", width, w, line)
			}
		}
	}
}

// Small figures let the row fit a narrow terminal, which is what proves the
// layout is measured rather than reserved: same code, more room, wider name.
func TestBudgetViewFitsNarrowTerminalsWithSmallFigures(t *testing.T) {
	th := testTheme(true)
	bm := newBudget([]report.BudgetLine{
		{ListName: "Website", Billed: 12.5, Budget: 20, Currency: "EUR", Remaining: 7.5, PercentUsed: 62.5},
	})
	for _, width := range []int{60, 80} {
		for _, line := range strings.Split(bm.view(th, width), "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Errorf("width %d: line rendered %d columns: %q", width, w, line)
			}
		}
	}
}

// A wide-rune list name must not push the row past its column: the name column
// is the one that absorbs the slack, so it is also the one that used to
// misalign. An ASCII fixture passes against the bug.
func TestBudgetViewHoldsItsWidthWithWideRunes(t *testing.T) {
	th := testTheme(true)
	bm := newBudget([]report.BudgetLine{
		{ListName: strings.Repeat("🚀", 24), Billed: 625, Budget: 1000, Currency: "EUR", Remaining: 375, PercentUsed: 62.5},
	})
	for _, line := range strings.Split(bm.view(th, 80), "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("line rendered %d columns: %q", w, line)
		}
	}
}

// "(remaining X)" is the most redundant field on the row (it is budget minus
// billed), so it is what gives way when the name column would fall below the
// floor. One pass: compute with it, and if the name is too narrow, recompute
// without it and never put it back. Measured for this fixture at width 80: with
// remaining the name would get 10 columns, under the floor of 12, so remaining
// goes and the name gets 29.
func TestBudgetLayoutDropsRemainingBeforeStarvingTheName(t *testing.T) {
	lines := []report.BudgetLine{
		{ListName: "Website", Billed: 625, Budget: 1000, Currency: "EUR", Remaining: 375, PercentUsed: 62.5},
	}
	nameW, showRemaining := budgetLayout(lines, 80)
	if showRemaining {
		t.Errorf("at width 80 remaining should have been dropped; name column is %d", nameW)
	}
	if nameW != 29 {
		t.Errorf("nameW = %d, want 29", nameW)
	}
	// A roomy terminal keeps remaining AND a wider name.
	wideName, wideRem := budgetLayout(lines, 100)
	if !wideRem {
		t.Errorf("at width 100 remaining should fit; name column is %d", wideName)
	}
}

// Before the first WindowSizeMsg the width is 0 and nothing is sized against it:
// the screen keeps its natural layout, the same fallback reportItemWidth has.
func TestBudgetLayoutFallsBackBeforeTheFirstWindowSize(t *testing.T) {
	nameW, showRemaining := budgetLayout([]report.BudgetLine{{ListName: "Website"}}, 0)
	if nameW != budgetNameWidth || !showRemaining {
		t.Errorf("budgetLayout(width=0) = (%d, %v), want (%d, true)", nameW, showRemaining, budgetNameWidth)
	}
}
