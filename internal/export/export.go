// Package export serializes a report.Report into CSV, JSON, or Markdown.
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

// bucketAmount collapses a bucket's per-currency amounts into a single number.
// It is a stop-gap for the single-currency shape these exporters still assume:
// the multi-currency-aware exports arrive with the invoice/HTML formats.
func bucketAmount(b report.Bucket) float64 {
	var total float64
	for _, a := range b.Amounts {
		total += a.Amount
	}
	return total
}

// CSV writes the report as CSV with a header row and a total row.
func CSV(w io.Writer, r report.Report) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"label", "hours", "amount", "currency"}); err != nil {
		return err
	}
	num := func(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
	for _, b := range r.Buckets {
		if err := cw.Write([]string{b.Label, num(b.Hours), num(bucketAmount(b)), r.DefaultCurrency}); err != nil {
			return err
		}
	}
	if err := cw.Write([]string{"TOTAL", num(r.TotalHours), num(r.TotalAmount), r.DefaultCurrency}); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

// jsonReport is the serialized (snake_case) form of the report.
type jsonReport struct {
	Start   string `json:"start"`
	End     string `json:"end"`
	Scope   string `json:"scope"`
	GroupBy string `json:"group_by"`
	// Currency and Rate are deprecated single-value fields kept so existing
	// scripts parsing this schema keep working; the real model is per-list
	// currencies and rates (see report.Pricing).
	Currency    string          `json:"currency"`
	Rate        float64         `json:"rate"`
	Buckets     []report.Bucket `json:"buckets"`
	TotalHours  float64         `json:"total_hours"`
	TotalAmount float64         `json:"total_amount"`
}

// JSON writes the report as indented JSON.
func JSON(w io.Writer, r report.Report) error {
	jr := jsonReport{
		Start: r.Start.Format(time.RFC3339), End: r.End.Format(time.RFC3339),
		Scope: r.Scope, GroupBy: r.GroupBy,
		Currency: r.DefaultCurrency, Rate: r.DefaultRate, Buckets: r.Buckets,
		TotalHours: r.TotalHours, TotalAmount: r.TotalAmount,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jr)
}

// Markdown writes the report as a Markdown table.
func Markdown(w io.Writer, r report.Report) error {
	fmt.Fprintf(w, "# Hours report %s\n\n", report.PeriodLabel(r.Start, r.End))
	fmt.Fprintln(w, "| Label | Hours | Amount |")
	fmt.Fprintln(w, "|---|---:|---:|")
	for _, b := range r.Buckets {
		fmt.Fprintf(w, "| %s | %.2f | %.2f %s |\n", b.Label, b.Hours, bucketAmount(b), r.DefaultCurrency)
	}
	fmt.Fprintf(w, "| **Total** | **%.2f** | **%.2f %s** |\n", r.TotalHours, r.TotalAmount, r.DefaultCurrency)
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
