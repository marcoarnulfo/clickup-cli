package export

import (
	"fmt"
	"strings"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/duration"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// SummaryLine renders a shared one-line human summary of a report: the
// number of billing lines, the total billed hours, and the billed amount per
// currency. Money comes straight from CurrencySubtotals (the authoritative
// totals — see the money-ledger rule on report.Build); currencies are never
// summed together.
func SummaryLine(r report.Report) string {
	hours := duration.FormatHours(time.Duration(r.BilledHours * float64(time.Hour)))

	amounts := make([]string, 0, len(r.CurrencySubtotals))
	for _, cs := range r.CurrencySubtotals {
		amounts = append(amounts, fmt.Sprintf("%.2f %s", cs.Amount, cs.Currency))
	}

	return fmt.Sprintf("%d entries · %s · %s", len(r.Lines), hours, strings.Join(amounts, ", "))
}
