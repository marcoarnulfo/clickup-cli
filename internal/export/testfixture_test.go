package export

import (
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// buildMultiCurrencyFixture builds a real report.Report via report.Build (not
// a hand-assembled struct literal), so tests that need Lines/Buckets/
// CurrencySubtotals to be mutually consistent exercise the actual domain
// invariants (Task 3) rather than a fixture that could silently drift from
// them. Two lists in two currencies, one entry with a non-2-decimal duration
// (80 minutes), so the fixture also exercises the money-ledger precision rule.
func buildMultiCurrencyFixture() report.Report {
	loc := time.UTC
	start, end := report.MonthRange(2026, time.July, loc)
	entries := []report.TimeEntry{
		{ID: "1", TaskID: "t1", TaskName: "Design", ListID: "l1", ListName: "Client A",
			UserID: 1, UserName: "Alice", Start: time.Date(2026, 7, 1, 9, 0, 0, 0, loc),
			Duration: 2 * time.Hour, Billable: true},
		{ID: "2", TaskID: "t2", TaskName: "Dev", ListID: "l1", ListName: "Client A",
			UserID: 2, UserName: "Bob", Start: time.Date(2026, 7, 2, 9, 0, 0, 0, loc),
			Duration: time.Hour, Billable: true},
		{ID: "3", TaskID: "t3", TaskName: "Review", ListID: "l2", ListName: "Client B",
			UserID: 3, UserName: "Carol", Start: time.Date(2026, 7, 1, 14, 0, 0, 0, loc),
			Duration: 80 * time.Minute, Billable: true},
	}
	p := report.Pricing{
		Rates:           report.Rates{Default: 50, ByList: map[string]float64{"l2": 80}},
		Currencies:      map[string]string{"l2": "USD"},
		DefaultCurrency: "EUR",
	}
	r := report.Build(entries, report.GroupByList, p, start, end, loc)
	r.Scope = "me"
	return r
}
