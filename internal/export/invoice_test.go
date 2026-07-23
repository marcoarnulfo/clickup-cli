package export

import (
	"bytes"
	"encoding/csv"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func TestInvoiceCSVColumns(t *testing.T) {
	r := buildMultiCurrencyFixture()
	if len(r.Lines) != 3 {
		t.Fatalf("fixture sanity: want 3 lines, got %d", len(r.Lines))
	}

	var b bytes.Buffer
	if err := InvoiceCSV(&b, r); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&b).ReadAll()
	if err != nil {
		t.Fatalf("re-parsing invoice CSV: %v", err)
	}

	wantHeader := []string{"date", "list_id", "client", "user", "description", "qty_hours", "rate", "amount", "currency", "billable"}
	if len(rows) == 0 || !equalRows(rows[0], wantHeader) {
		t.Fatalf("header = %v, want %v", rowsHead(rows), wantHeader)
	}
	if len(rows) != 1+len(r.Lines) {
		t.Fatalf("want one row per billing unit (%d) plus header, got %d rows", len(r.Lines), len(rows))
	}

	// r.Lines is sorted (Date asc, ListName asc, UserName asc): Alice/Client A
	// on 07-01 sorts before Carol/Client B on the same day, which sorts before
	// Bob/Client A on 07-02.
	want := [][]string{
		{"2026-07-01", "l1", "Client A", "Alice", "Design", "2.000000", "50", "100", "EUR", "true"},
		{"2026-07-01", "l2", "Client B", "Carol", "Review", "1.333333", "80", "106.67", "USD", "true"},
		{"2026-07-02", "l1", "Client A", "Bob", "Dev", "1.000000", "50", "50", "EUR", "true"},
	}
	for i, w := range want {
		if !equalRows(rows[i+1], w) {
			t.Errorf("row %d = %v, want %v", i, rows[i+1], w)
		}
	}
}

// TestInvoiceCSVRowReconciles is the binding row-reconciliation test: it uses
// a fixture whose duration is not exact to 2 decimals (20 minutes), and
// asserts that the rendered row still satisfies round2(qty_hours * rate) ==
// amount. Re-rounding qty_hours to 2 decimals before rendering (0.33 instead
// of 0.333333) would break this: round2(0.33 * 200) = 66.00 != 66.67.
func TestInvoiceCSVRowReconciles(t *testing.T) {
	loc := time.UTC
	start, end := report.MonthRange(2026, time.July, loc)
	entries := []report.TimeEntry{
		{ID: "1", TaskID: "t1", TaskName: "Task", ListID: "l1", ListName: "X",
			UserID: 1, UserName: "U", Start: time.Date(2026, 7, 1, 9, 0, 0, 0, loc),
			Duration: 20 * time.Minute, Billable: true},
	}
	p := report.Pricing{Rates: report.Rates{Default: 200}, DefaultCurrency: "EUR"}
	r := report.Build(entries, report.GroupByList, p, start, end, loc)
	if len(r.Lines) != 1 {
		t.Fatalf("fixture sanity: want 1 line, got %d", len(r.Lines))
	}

	var b bytes.Buffer
	if err := InvoiceCSV(&b, r); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&b).ReadAll()
	if err != nil {
		t.Fatalf("re-parsing invoice CSV: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want header + 1 row, got %d rows: %v", len(rows), rows)
	}
	row := rows[1]

	qtyStr, rateStr, amtStr := row[5], row[6], row[7]
	if dec := decimalDigits(qtyStr); dec != 6 {
		t.Fatalf("qty_hours = %q, want exactly 6 decimal digits (got %d)", qtyStr, dec)
	}
	qty, err := strconv.ParseFloat(qtyStr, 64)
	if err != nil {
		t.Fatalf("parsing qty_hours %q: %v", qtyStr, err)
	}
	rate, err := strconv.ParseFloat(rateStr, 64)
	if err != nil {
		t.Fatalf("parsing rate %q: %v", rateStr, err)
	}
	amount, err := strconv.ParseFloat(amtStr, 64)
	if err != nil {
		t.Fatalf("parsing amount %q: %v", amtStr, err)
	}

	if got := round2(qty * rate); got != amount {
		t.Errorf("row does not reconcile: round2(%v * %v) = %v, amount = %v", qty, rate, got, amount)
	}
	if amount != 66.67 {
		t.Fatalf("fixture sanity: amount = %v, want 66.67", amount)
	}
}

// round2 mirrors the report package's private rounding rule; tests live
// outside that package and must not depend on its unexported helpers.
func round2(v float64) float64 { return math.Round(v*100) / 100 }

// decimalDigits returns the number of digits after the decimal point in s.
func decimalDigits(s string) int {
	i := strings.IndexByte(s, '.')
	if i < 0 {
		return 0
	}
	return len(s) - i - 1
}

func equalRows(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func rowsHead(rows [][]string) []string {
	if len(rows) == 0 {
		return nil
	}
	return rows[0]
}
