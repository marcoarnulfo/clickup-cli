package export

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func sample() report.Report {
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	return report.Report{
		Start: start, End: start.AddDate(0, 1, 0),
		Scope: "me", GroupBy: "list", Timezone: "UTC",
		DefaultCurrency: "EUR", DefaultRate: 50,
		Buckets: []report.Bucket{
			{Label: "Client A", Key: "l1", Hours: 3, BillableHours: 3, BilledHours: 3,
				Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 150}}},
			{Label: "Client B", Key: "l2", Hours: 3, BillableHours: 3, BilledHours: 3,
				Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 150}}},
		},
		CurrencySubtotals: []report.CurrencySubtotal{
			{Currency: "EUR", Hours: 6, BillableHours: 6, BilledHours: 6, Amount: 300},
		},
		TotalHours: 6, BillableHours: 6, BilledHours: 6, TotalAmount: 300,
	}
}

func TestCSVHasHeaderAndRows(t *testing.T) {
	var b bytes.Buffer
	if err := CSV(&b, sample()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.HasPrefix(out, "label,hours,billable_hours,billed_hours,amount,currency\n") {
		t.Fatalf("bad header: %q", out)
	}
	if !strings.Contains(out, "Client A,3,3,3,150,EUR") {
		t.Fatalf("missing row: %q", out)
	}
	if !strings.Contains(out, "TOTAL,6,6,6,300,EUR") {
		t.Fatalf("missing total row: %q", out)
	}
}

func TestJSONRoundTrips(t *testing.T) {
	var b bytes.Buffer
	if err := JSON(&b, sample()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `"total_hours": 6`) {
		t.Fatalf("json missing total_hours: %s", b.String())
	}
	if !strings.Contains(b.String(), `"label": "Client A"`) {
		t.Fatalf("json missing bucket: %s", b.String())
	}
	if !strings.Contains(b.String(), `"start":`) {
		t.Fatalf("json missing start: %s", b.String())
	}
	// A6: the deprecated single-value fields must survive so existing jq
	// scripts keep working, even though the real model is per-list.
	if !strings.Contains(b.String(), `"currency": "EUR"`) || !strings.Contains(b.String(), `"rate": 50`) {
		t.Fatalf("json missing deprecated currency/rate fields: %s", b.String())
	}
	if !strings.Contains(b.String(), `"currency_subtotals"`) {
		t.Fatalf("json missing currency_subtotals: %s", b.String())
	}
}

func TestJSONSchemaVersionAndTimezone(t *testing.T) {
	var b bytes.Buffer
	if err := JSON(&b, sample()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, `"schema_version": 1`) {
		t.Fatalf("json missing schema_version: %s", out)
	}
	if !strings.Contains(out, `"timezone": "UTC"`) {
		t.Fatalf("json missing timezone: %s", out)
	}
}

func TestMarkdownTable(t *testing.T) {
	var b bytes.Buffer
	if err := Markdown(&b, sample()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "| Label | Hours | Billable | Billed | Amount | Currency |") {
		t.Fatalf("missing md header: %q", out)
	}
	if !strings.Contains(out, "# Hours report July 2026") {
		t.Fatalf("missing md period header: %q", out)
	}
	if !strings.Contains(out, "| Client A | 3.00 | 3.00 | 3.00 | 150.00 | EUR |") {
		t.Fatalf("missing md row: %q", out)
	}
	if !strings.Contains(out, "| **Total** | **6.00** | **6.00** | **6.00** | **300.00** | **EUR** |") {
		t.Fatalf("missing md total: %q", out)
	}
}

// sampleWithNonBillableBucket adds a third bucket whose entries are entirely
// non-billable (Amounts stays nil, as report.Build leaves it — see
// currencyAmounts in internal/report/aggregate.go). This is a regression
// fixture: a bucket like this must still render a row (hours only, amount 0
// in DefaultCurrency), not vanish from the export.
func sampleWithNonBillableBucket() report.Report {
	r := sample()
	r.Buckets = append(r.Buckets, report.Bucket{
		Label: "Internal", Key: "l3", Hours: 2, BillableHours: 0, BilledHours: 0, Amounts: nil,
	})
	r.TotalHours += 2
	r.NonBillableHours += 2
	return r
}

func TestCSVIncludesFullyNonBillableBucket(t *testing.T) {
	var b bytes.Buffer
	if err := CSV(&b, sampleWithNonBillableBucket()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "Internal,2,0,0,0,EUR") {
		t.Fatalf("a fully non-billable bucket must still render a row (hours only, amount 0 in DefaultCurrency); got:\n%s", out)
	}
}

func TestMarkdownIncludesFullyNonBillableBucket(t *testing.T) {
	var b bytes.Buffer
	if err := Markdown(&b, sampleWithNonBillableBucket()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "| Internal | 2.00 | 0.00 | 0.00 | 0.00 | EUR |") {
		t.Fatalf("a fully non-billable bucket must still render a row (hours only, amount 0 in DefaultCurrency); got:\n%s", out)
	}
}

func TestCSVEmptyReport(t *testing.T) {
	var b bytes.Buffer
	if err := CSV(&b, report.Report{}); err != nil {
		t.Fatal(err)
	}
	want := "label,hours,billable_hours,billed_hours,amount,currency\n"
	if b.String() != want {
		t.Fatalf("CSV(empty report) = %q, want just the header %q", b.String(), want)
	}
}

func TestMarkdownEmptyReport(t *testing.T) {
	var b bytes.Buffer
	if err := Markdown(&b, report.Report{}); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "| Label | Hours | Billable | Billed | Amount | Currency |") {
		t.Fatalf("missing md header: %q", out)
	}
	if strings.Contains(out, "Total") {
		t.Fatalf("an empty report must not render any total row: %q", out)
	}
}

// TestMarkdownMultiCurrencyNeverSums pins the Markdown output for a real,
// multi-currency report: one row (and one bold total row) per currency,
// never a combined cross-currency total.
func TestMarkdownMultiCurrencyNeverSums(t *testing.T) {
	r := buildMultiCurrencyFixture()
	var b bytes.Buffer
	if err := Markdown(&b, r); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "| Client A | 3.00 | 3.00 | 3.00 | 150.00 | EUR |") {
		t.Fatalf("missing Client A row: %q", out)
	}
	if !strings.Contains(out, "| Client B | 1.33 | 1.33 | 1.33 | 106.67 | USD |") {
		t.Fatalf("missing Client B row: %q", out)
	}
	if !strings.Contains(out, "| **Total** | **3.00** | **3.00** | **3.00** | **150.00** | **EUR** |") {
		t.Fatalf("missing EUR total row: %q", out)
	}
	if !strings.Contains(out, "| **Total** | **1.33** | **1.33** | **1.33** | **106.67** | **USD** |") {
		t.Fatalf("missing USD total row: %q", out)
	}
	if n := strings.Count(out, "**Total**"); n != 2 {
		t.Fatalf("want exactly 2 total rows (one per currency), never a cross-currency sum; got %d in:\n%s", n, out)
	}
}

func TestToFileUnknownFormat(t *testing.T) {
	if err := ToFile("pdf", sample(), t.TempDir()+"/x"); err == nil {
		t.Fatal("expected error on unknown format")
	}
}

func TestToFileWritesCSV(t *testing.T) {
	p := t.TempDir() + "/out.csv"
	if err := ToFile("csv", sample(), p); err != nil {
		t.Fatal(err)
	}
}

func TestToFileWritesHTML(t *testing.T) {
	p := t.TempDir() + "/out.html"
	if err := ToFile("html", sample(), p); err != nil {
		t.Fatal(err)
	}
}

func TestToFileWritesInvoiceCSV(t *testing.T) {
	p := t.TempDir() + "/out.csv"
	if err := ToFile("csv-invoice", sample(), p); err != nil {
		t.Fatal(err)
	}
}

// TestFormatsMatchesToFile pins Formats() as ToFile's actual source of truth:
// every key it returns must be accepted by ToFile, and nothing else should be.
// Callers (the TUI's export menu) rely on Formats() to stay in lockstep with
// ToFile without hand-duplicating the format list.
func TestFormatsMatchesToFile(t *testing.T) {
	for _, k := range Formats() {
		p := t.TempDir() + "/out"
		if err := ToFile(k, sample(), p); err != nil {
			t.Errorf("Formats() reports %q but ToFile rejected it: %v", k, err)
		}
	}
	if err := ToFile("bogus-format-not-in-formats", sample(), t.TempDir()+"/x"); err == nil {
		t.Error("a format outside Formats() should not be accepted by ToFile")
	}
}
