// Package export serializes a report.Report into CSV, JSON, Markdown, HTML,
// or a CSV invoice.
package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// CSV writes the report as CSV: one row per (bucket, per-currency amount),
// plus one TOTAL row per currency subtotal. CurrencySubtotals is the
// authoritative source for the totals — the per-bucket amounts are an
// indicative proportional allocation (see report.Build's caveat) and are
// never summed here to produce a total.
func CSV(w io.Writer, r report.Report) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"label", "hours", "billable_hours", "billed_hours", "amount", "currency"}); err != nil {
		return err
	}
	num := func(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
	for _, b := range r.Buckets {
		for _, a := range bucketAmounts(b, r.DefaultCurrency) {
			row := []string{b.Label, num(b.Hours), num(b.BillableHours), num(b.BilledHours), num(a.Amount), a.Currency}
			if err := cw.Write(row); err != nil {
				return err
			}
		}
	}
	for _, cs := range r.CurrencySubtotals {
		row := []string{"TOTAL", num(cs.Hours), num(cs.BillableHours), num(cs.BilledHours), num(cs.Amount), cs.Currency}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// bucketAmounts returns a bucket's per-currency amounts to render, one row
// each. A bucket whose entries are all non-billable never populates Amounts
// (see currencyAmounts in internal/report/aggregate.go) — without this
// fallback it would vanish entirely from CSV/Markdown, hours included, which
// is wrong for a hours-reporting tool as much as a billing one. In that case
// render a single zero-amount row in the report's DefaultCurrency so the
// bucket's hours stay visible.
func bucketAmounts(b report.Bucket, defaultCurrency string) []report.CurrencyAmount {
	if len(b.Amounts) == 0 {
		return []report.CurrencyAmount{{Currency: defaultCurrency, Amount: 0}}
	}
	return b.Amounts
}

// reportSchemaVersion identifies the shape of the JSON emitted by JSON below.
// It is independent of the config file's schema; bump it only on a breaking
// change to this report-output JSON.
const reportSchemaVersion = 1

// jsonReport is the serialized (snake_case) form of the report.
type jsonReport struct {
	SchemaVersion int    `json:"schema_version"`
	Start         string `json:"start"`
	End           string `json:"end"`
	Scope         string `json:"scope"`
	GroupBy       string `json:"group_by"`
	Timezone      string `json:"timezone"`

	// Currency and Rate are DEPRECATED (A6): kept only so existing scripts
	// parsing this schema keep working. The real model is per-list currencies
	// and rates (report.Pricing); new consumers should use CurrencySubtotals
	// and Lines below instead of these two single-value fields.
	Currency string  `json:"currency"`
	Rate     float64 `json:"rate"`

	Buckets           []report.Bucket           `json:"buckets"`
	Lines             []report.InvoiceLine      `json:"lines"`
	CurrencySubtotals []report.CurrencySubtotal `json:"currency_subtotals"`

	TotalHours       float64 `json:"total_hours"`
	BillableHours    float64 `json:"billable_hours"`
	NonBillableHours float64 `json:"non_billable_hours"`
	BilledHours      float64 `json:"billed_hours"`
	// TotalAmount is 0 unless the report is single-currency; see
	// report.Report's doc comment for the "no cross-currency totals" rule.
	TotalAmount float64 `json:"total_amount"`
}

// JSON writes the report as indented JSON.
func JSON(w io.Writer, r report.Report) error {
	jr := jsonReport{
		SchemaVersion: reportSchemaVersion,
		Start:         r.Start.Format(time.RFC3339), End: r.End.Format(time.RFC3339),
		Scope: r.Scope, GroupBy: r.GroupBy, Timezone: r.Timezone,
		Currency: r.DefaultCurrency, Rate: r.DefaultRate,
		Buckets: r.Buckets, Lines: r.Lines, CurrencySubtotals: r.CurrencySubtotals,
		TotalHours: r.TotalHours, BillableHours: r.BillableHours,
		NonBillableHours: r.NonBillableHours, BilledHours: r.BilledHours,
		TotalAmount: r.TotalAmount,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jr)
}

// Markdown writes the report as a Markdown table: one row per (bucket,
// per-currency amount), plus one bold total row per currency subtotal.
func Markdown(w io.Writer, r report.Report) error {
	fmt.Fprintf(w, "# Hours report %s\n\n", report.PeriodLabel(r.Start, r.End))
	fmt.Fprintln(w, "| Label | Hours | Billable | Billed | Amount | Currency |")
	fmt.Fprintln(w, "|---|---:|---:|---:|---:|---|")
	for _, b := range r.Buckets {
		for _, a := range bucketAmounts(b, r.DefaultCurrency) {
			fmt.Fprintf(w, "| %s | %.2f | %.2f | %.2f | %.2f | %s |\n",
				b.Label, b.Hours, b.BillableHours, b.BilledHours, a.Amount, a.Currency)
		}
	}
	for _, cs := range r.CurrencySubtotals {
		fmt.Fprintf(w, "| **Total** | **%.2f** | **%.2f** | **%.2f** | **%.2f** | **%s** |\n",
			cs.Hours, cs.BillableHours, cs.BilledHours, cs.Amount, cs.Currency)
	}
	return nil
}

// ToFile writes the report in the given format to the given path.
// It validates the format BEFORE creating the file, so an unknown format
// doesn't leave an empty file on disk.
func ToFile(format string, r report.Report, path string) error {
	var fn func(io.Writer, report.Report) error
	switch format {
	case "csv":
		fn = CSV
	case "json":
		fn = JSON
	case "markdown":
		fn = Markdown
	case "html":
		fn = HTML
	case "csv-invoice":
		fn = InvoiceCSV
	default:
		return fmt.Errorf("unsupported format: %q", format)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return fn(f, r)
}
