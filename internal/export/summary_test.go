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
	want := "3 entries · 3.00h · 200.00 EUR"
	if got != want {
		t.Errorf("SummaryLine() = %q, want %q", got, want)
	}
}

// TestSummaryLineEmptyReport pins the shape for a report with no lines and no
// currency subtotals: no panic, a "0 entries" count, and a trailing empty
// amounts list rather than a placeholder like "0.00 " with an invented currency.
func TestSummaryLineEmptyReport(t *testing.T) {
	got := SummaryLine(report.Report{})
	want := "0 entries · 0.00h · "
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
	want := "4 entries · 5.00h · 150.00 EUR, 50.00 USD"
	if got != want {
		t.Errorf("SummaryLine() = %q, want %q", got, want)
	}
}
