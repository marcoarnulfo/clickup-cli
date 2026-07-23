package export

import (
	"testing"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func TestSummaryLine(t *testing.T) {
	r := report.Report{
		Lines:             make([]report.InvoiceLine, 3), // count only matters here
		BilledHours:       3,
		CurrencySubtotals: []report.CurrencySubtotal{{Currency: "EUR", Amount: 200}},
	}
	got := SummaryLine(r)
	want := "3 billing lines · 3.00h · 200.00 EUR"
	if got != want {
		t.Errorf("SummaryLine() = %q, want %q", got, want)
	}
}

// TestSummaryLineEmptyReport pins the shape for a report with no lines and no
// currency subtotals: no panic, a "0 billing lines" count, and no dangling
// " · " separator before an empty amounts list (previously rendered as
// "0 billing lines · 0.00h · ", trailing whitespace and all).
func TestSummaryLineEmptyReport(t *testing.T) {
	got := SummaryLine(report.Report{})
	want := "0 billing lines · 0.00h"
	if got != want {
		t.Errorf("SummaryLine(empty) = %q, want %q", got, want)
	}
}

func TestSummaryLineMultiCurrency(t *testing.T) {
	r := report.Report{
		Lines:       make([]report.InvoiceLine, 4),
		BilledHours: 5,
		CurrencySubtotals: []report.CurrencySubtotal{
			{Currency: "EUR", Amount: 150},
			{Currency: "USD", Amount: 50},
		},
	}
	got := SummaryLine(r)
	want := "4 billing lines · 5.00h · 150.00 EUR, 50.00 USD"
	if got != want {
		t.Errorf("SummaryLine() = %q, want %q", got, want)
	}
}
