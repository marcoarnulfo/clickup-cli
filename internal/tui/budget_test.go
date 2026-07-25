package tui

import (
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// #64: the budget view renders one text progress bar per report.BudgetLine.
func TestBudgetViewRendersProgressBar(t *testing.T) {
	lines := []report.BudgetLine{
		{ListID: "list-1", ListName: "Website", Currency: "EUR",
			Budget: 1000, Billed: 600, Remaining: 400, PercentUsed: 60},
	}
	out := newBudget(lines).view(testTheme(true))
	for _, want := range []string{"Website", "60%", "600.00", "1000.00", "EUR"} {
		if !strings.Contains(out, want) {
			t.Errorf("budget view missing %q; got:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "#") || !strings.Contains(out, "-") {
		t.Error("expected a text progress bar built from '#' and '-'")
	}
}

func TestBudgetViewEmptyShowsMessage(t *testing.T) {
	out := newBudget(nil).view(testTheme(true))
	if !strings.Contains(out, "No budgets") {
		t.Errorf("empty budget view should say no budgets are configured; got:\n%s", out)
	}
}

// A list that is over its budget must still be visible in the number even
// though the bar itself caps its fill at 100%.
func TestRenderBudgetBarClampsFillNotPercent(t *testing.T) {
	out := renderBudgetBar(150)
	if !strings.Contains(out, "150%") {
		t.Errorf("renderBudgetBar(150) = %q, want the unclamped 150%% in the label", out)
	}
	full := strings.Repeat("#", budgetBarWidth)
	if !strings.Contains(out, full) {
		t.Errorf("renderBudgetBar(150) = %q, want a fully filled bar", out)
	}
}

// TestBudgetKeyLabels pins the exact label set budget.go's updateBudget
// accepts today (every case label, verbatim), plus q — handled globally in
// app.go, in no case clause of budget.go itself. A dropped label is caught by
// the two transition tests below; this catches an invented one.
func TestBudgetKeyLabels(t *testing.T) {
	t.Parallel()
	m := Model{screen: screenBudget}
	want := []string{"b", "esc", "q"}
	if got := enabledLabels(keysFor(m)); !slices.Equal(got, want) {
		t.Errorf("budget labels = %v, want %v", got, want)
	}
}

// #59 Task 3 step 3: esc and b both close the budget view back to Report —
// neither has a test that would fail if it went mute.
func TestBudgetEscReturnsReport(t *testing.T) {
	m := Model{screen: screenBudget}
	next, _ := m.updateBudget(tea.KeyMsg{Type: tea.KeyEsc})
	if got := next.(Model).screen; got != screenReport {
		t.Errorf("esc from budget -> %v, want screenReport", got)
	}
}

func TestBudgetBReturnsReport(t *testing.T) {
	m := Model{screen: screenBudget}
	next, _ := m.updateBudget(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if got := next.(Model).screen; got != screenReport {
		t.Errorf("b from budget -> %v, want screenReport", got)
	}
}
