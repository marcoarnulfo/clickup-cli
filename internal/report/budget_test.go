package report

import (
	"testing"
)

func TestBudgetLines(t *testing.T) {
	billed := map[string]float64{"A": 3000, "B": 500}
	budgets := map[string]float64{"A": 5000, "B": 0} // B has no budget -> skipped
	p := Pricing{Currencies: map[string]string{"A": "EUR"}, DefaultCurrency: "EUR"}
	lines := BudgetLines(billed, budgets, p, map[string]string{"A": "Alpha"})
	if len(lines) != 1 {
		t.Fatalf("want 1 line, got %d", len(lines))
	}
	l := lines[0]
	if l.ListID != "A" || l.Budget != 5000 || l.Billed != 3000 || l.Remaining != 2000 || l.PercentUsed != 60 {
		t.Fatalf("line = %+v", l)
	}
	if l.Currency != "EUR" || l.ListName != "Alpha" {
		t.Fatalf("meta = %+v", l)
	}
}

// TestBudgetLinesNoBilledAmount tests a list that appears in budgets but not in billedByList.
func TestBudgetLinesNoBilledAmount(t *testing.T) {
	billed := map[string]float64{"A": 1000}
	budgets := map[string]float64{"A": 5000, "B": 2000}
	p := Pricing{Currencies: map[string]string{"A": "EUR", "B": "EUR"}, DefaultCurrency: "EUR"}
	lines := BudgetLines(billed, budgets, p, map[string]string{"A": "Alpha", "B": "Beta"})
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d", len(lines))
	}
	// Find line B (0 billed)
	var lineB BudgetLine
	for _, l := range lines {
		if l.ListID == "B" {
			lineB = l
			break
		}
	}
	if lineB.ListID != "B" {
		t.Fatalf("line B not found")
	}
	if lineB.Billed != 0 || lineB.Remaining != 2000 || lineB.PercentUsed != 0 {
		t.Fatalf("line B = %+v, want Billed=0, Remaining=2000, PercentUsed=0", lineB)
	}
}

// TestBudgetLinesSortOrder tests that lines are sorted by PercentUsed descending (most-burned first),
// with ties broken by ListName ascending.
func TestBudgetLinesSortOrder(t *testing.T) {
	billed := map[string]float64{"1": 6000, "2": 5000, "3": 5000}
	budgets := map[string]float64{"1": 10000, "2": 10000, "3": 10000}
	p := Pricing{Currencies: map[string]string{"1": "EUR", "2": "EUR", "3": "EUR"}, DefaultCurrency: "EUR"}
	lines := BudgetLines(
		billed,
		budgets,
		p,
		map[string]string{"1": "Zebra", "2": "Alpha", "3": "Beta"},
	)
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d", len(lines))
	}
	// Expected order: 1 (60%), 2 (50%), 3 (50%)
	// But 2 and 3 have same PercentUsed, so tie broken by ListName: Alpha < Beta
	// So: 1 (60% Zebra), 2 (50% Alpha), 3 (50% Beta)
	if lines[0].ListID != "1" {
		t.Fatalf("lines[0] should be list 1 (60%%), got %s", lines[0].ListID)
	}
	if lines[1].ListID != "2" || lines[1].ListName != "Alpha" {
		t.Fatalf("lines[1] should be list 2 (Alpha, 50%%), got %s (%s)", lines[1].ListID, lines[1].ListName)
	}
	if lines[2].ListID != "3" || lines[2].ListName != "Beta" {
		t.Fatalf("lines[2] should be list 3 (Beta, 50%%), got %s (%s)", lines[2].ListID, lines[2].ListName)
	}
}

// TestBudgetLinesCurrencyResolution pins that BudgetLines resolves currency
// through Pricing.CurrencyFor, so an empty per-list mapping falls back to the
// default currency exactly like the pricing and TUI paths do.
func TestBudgetLinesCurrencyResolution(t *testing.T) {
	p := Pricing{Currencies: map[string]string{"A": "", "B": "USD"}, DefaultCurrency: "EUR"}
	lines := BudgetLines(nil, map[string]float64{"A": 100, "B": 100}, p, map[string]string{"A": "Alpha", "B": "Beta"})
	got := map[string]string{}
	for _, l := range lines {
		got[l.ListID] = l.Currency
	}
	if got["A"] != "EUR" {
		t.Errorf("empty currency mapping = %q, want the default EUR", got["A"])
	}
	if got["B"] != "USD" {
		t.Errorf("list B currency = %q, want USD", got["B"])
	}
}

// TestBudgetLinesNameFallsBackToListID pins that a budgeted list with no hours
// in the period -- the most interesting row, an untouched budget -- is labelled
// by its id instead of rendering as a blank label.
func TestBudgetLinesNameFallsBackToListID(t *testing.T) {
	lines := BudgetLines(nil, map[string]float64{"901": 1000}, Pricing{DefaultCurrency: "EUR"}, nil)
	if len(lines) != 1 {
		t.Fatalf("want 1 line, got %d", len(lines))
	}
	if lines[0].ListName != "901" {
		t.Errorf("ListName = %q, want the list id 901", lines[0].ListName)
	}
}
