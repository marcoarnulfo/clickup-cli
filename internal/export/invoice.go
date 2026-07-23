package export

import (
	"encoding/csv"
	"io"
	"strconv"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// InvoiceCSV writes one row per billing unit (report.Report.Lines) — the
// report's authoritative money ledger. Amount is never recomputed here: it is
// exactly the round2(exact billed hours × rate) value report.Build already
// computed for that unit.
//
// qty_hours is rendered at 6 decimals, matching InvoiceLine.Hours' stored
// precision, and must NOT be re-rounded to 2 decimals: that precision is what
// makes round2(qty_hours × rate) == amount hold for every row, including
// units whose billed duration isn't exact to 2 decimals (e.g. 20 minutes).
func InvoiceCSV(w io.Writer, r report.Report) error {
	cw := csv.NewWriter(w)
	header := []string{"date", "list_id", "client", "user", "description", "qty_hours", "rate", "amount", "currency", "billable"}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, l := range r.Lines {
		row := []string{
			l.Date,
			l.ListID,
			l.ListName,
			l.UserName,
			l.Description,
			strconv.FormatFloat(l.Hours, 'f', 6, 64),
			strconv.FormatFloat(l.Rate, 'f', -1, 64),
			strconv.FormatFloat(l.Amount, 'f', -1, 64),
			l.Currency,
			strconv.FormatBool(l.Billable),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
