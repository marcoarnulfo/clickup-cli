package report

import (
	"cmp"
	"slices"
)

// BudgetLine represents a single budget's burn-down status.
type BudgetLine struct {
	ListID      string
	ListName    string
	Currency    string
	Budget      float64 // budget limit in the list's currency
	Billed      float64 // amount billed against the budget
	Remaining   float64 // Budget - Billed
	PercentUsed float64 // percentage of budget used (Billed/Budget*100), rounded to 2 decimals
}

// BudgetLines computes per-list budget burn-down from billed amounts and budget limits.
// It filters to budgets with amount > 0, calculates remaining and percent used,
// and sorts by percent used descending (most-burned first), ties by ListName ascending.
// Currency is resolved per-list from the currencies map, with fallback to defaultCurrency.
func BudgetLines(billedByList, budgets map[string]float64, currencies map[string]string, defaultCurrency string, listNames map[string]string) []BudgetLine {
	var lines []BudgetLine

	// Iterate over budgets with amount > 0
	for listID, budget := range budgets {
		if budget <= 0 {
			continue
		}

		// Get billed amount (default to 0 if not present)
		billed := billedByList[listID]

		// Get currency
		currency, ok := currencies[listID]
		if !ok {
			currency = defaultCurrency
		}

		// Get list name
		listName := listNames[listID]

		// Calculate remaining
		remaining := budget - billed

		// Calculate percent used with guard for Budget==0
		var percentUsed float64
		if budget > 0 {
			percentUsed = round2(billed / budget * 100)
		}

		lines = append(lines, BudgetLine{
			ListID:      listID,
			ListName:    listName,
			Currency:    currency,
			Budget:      budget,
			Billed:      billed,
			Remaining:   remaining,
			PercentUsed: percentUsed,
		})
	}

	// Sort by PercentUsed descending (most-burned first), ties by ListName ascending
	slices.SortFunc(lines, func(a, b BudgetLine) int {
		// Primary: PercentUsed descending (reverse comparison)
		if cmp := cmp.Compare(b.PercentUsed, a.PercentUsed); cmp != 0 {
			return cmp
		}
		// Tie-breaker: ListName ascending
		return cmp.Compare(a.ListName, b.ListName)
	})

	return lines
}
