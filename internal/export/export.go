// Package export serializza un report.Report in CSV, JSON o Markdown.
package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// CSV scrive il report come CSV con riga di intestazione e riga totale.
func CSV(w io.Writer, r report.Report) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"label", "hours", "amount", "currency"}); err != nil {
		return err
	}
	num := func(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
	for _, b := range r.Buckets {
		if err := cw.Write([]string{b.Label, num(b.Hours), num(b.Amount), r.Currency}); err != nil {
			return err
		}
	}
	if err := cw.Write([]string{"TOTAL", num(r.TotalHours), num(r.TotalAmount), r.Currency}); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

// jsonReport è la forma serializzata (snake_case) del report.
type jsonReport struct {
	Year        int             `json:"year"`
	Month       int             `json:"month"`
	Scope       string          `json:"scope"`
	GroupBy     string          `json:"group_by"`
	Currency    string          `json:"currency"`
	Rate        float64         `json:"rate"`
	Buckets     []report.Bucket `json:"buckets"`
	TotalHours  float64         `json:"total_hours"`
	TotalAmount float64         `json:"total_amount"`
}

// JSON scrive il report come JSON indentato.
func JSON(w io.Writer, r report.Report) error {
	jr := jsonReport{
		Year: r.Year, Month: int(r.Month), Scope: r.Scope, GroupBy: r.GroupBy,
		Currency: r.Currency, Rate: r.Rate, Buckets: r.Buckets,
		TotalHours: r.TotalHours, TotalAmount: r.TotalAmount,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jr)
}

// Markdown scrive il report come tabella Markdown.
func Markdown(w io.Writer, r report.Report) error {
	fmt.Fprintf(w, "# Report ore %04d-%02d\n\n", r.Year, int(r.Month))
	fmt.Fprintln(w, "| Label | Ore | Importo |")
	fmt.Fprintln(w, "|---|---:|---:|")
	for _, b := range r.Buckets {
		fmt.Fprintf(w, "| %s | %.2f | %.2f %s |\n", b.Label, b.Hours, b.Amount, r.Currency)
	}
	fmt.Fprintf(w, "| **Totale** | **%.2f** | **%.2f %s** |\n", r.TotalHours, r.TotalAmount, r.Currency)
	return nil
}

// ToFile scrive il report nel formato dato sul path indicato.
// Valida il formato PRIMA di creare il file, così un formato ignoto
// non lascia un file vuoto sul disco.
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
		return fmt.Errorf("formato non supportato: %q", format)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return fn(f, r)
}
