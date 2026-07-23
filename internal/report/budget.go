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

// BudgetLines computes per-list budget burn-down from billed amounts and budget
// limits, skipping non-positive budgets, and sorts most-burned first (ties by
// ListName). Currency comes from p.CurrencyFor, the one resolver shared with
// the pricing path, so an empty per-list mapping falls back to the default here
// exactly as it does when the money is computed.
//
// A budgeted list with no hours in the period has no entry in listNames (the
// caller derives them from the report's buckets) — it is labelled by its id
// rather than left blank, because an untouched budget is precisely the row a
// user opens this view to see.
func BudgetLines(billedByList, budgets map[string]float64, p Pricing, listNames map[string]string) []BudgetLine {
	var lines []BudgetLine

	for listID, budget := range budgets {
		if budget <= 0 {
			continue
		}
		listName := listNames[listID]
		if listName == "" {
			listName = listID
		}

		billed := billedByList[listID]
		lines = append(lines, BudgetLine{
			ListID:      listID,
			ListName:    listName,
			Currency:    p.CurrencyFor(listID),
			Budget:      budget,
			Billed:      billed,
			Remaining:   budget - billed,
			PercentUsed: round2(billed / budget * 100), // budget > 0 here: no division guard needed
		})
	}

	slices.SortFunc(lines, func(a, b BudgetLine) int {
		if c := cmp.Compare(b.PercentUsed, a.PercentUsed); c != 0 { // descending
			return c
		}
		return cmp.Compare(a.ListName, b.ListName)
	})

	return lines
}
