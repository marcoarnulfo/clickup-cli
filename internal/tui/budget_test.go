package tui

import (
	"strings"
	"testing"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// #64: the budget view renders one text progress bar per report.BudgetLine.
func TestBudgetViewRendersProgressBar(t *testing.T) {
	lines := []report.BudgetLine{
		{ListID: "list-1", ListName: "Website", Currency: "EUR",
			Budget: 1000, Billed: 600, Remaining: 400, PercentUsed: 60},
	}
	out := newBudget(lines).view()
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
	out := newBudget(nil).view()
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
