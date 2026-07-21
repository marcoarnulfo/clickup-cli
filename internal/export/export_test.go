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
		Scope: "me", GroupBy: "list", Currency: "EUR", Rate: 50,
		Buckets: []report.Bucket{
			{Label: "Client A", Hours: 3, Amount: 150},
			{Label: "Client B", Hours: 3, Amount: 150},
		},
		TotalHours: 6, TotalAmount: 300,
	}
}

func TestCSVHasHeaderAndRows(t *testing.T) {
	var b bytes.Buffer
	if err := CSV(&b, sample()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.HasPrefix(out, "label,hours,amount,currency\n") {
		t.Fatalf("bad header: %q", out)
	}
	if !strings.Contains(out, "Client A,3,150,EUR") {
		t.Fatalf("missing row: %q", out)
	}
	if !strings.Contains(out, "TOTAL,6,300,EUR") {
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
}

func TestMarkdownTable(t *testing.T) {
	var b bytes.Buffer
	if err := Markdown(&b, sample()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "| Label | Hours | Amount |") {
		t.Fatalf("missing md header: %q", out)
	}
	if !strings.Contains(out, "# Hours report July 2026") {
		t.Fatalf("missing md period header: %q", out)
	}
	if !strings.Contains(out, "| Client A | 3.00 | 150.00 EUR |") {
		t.Fatalf("missing md row: %q", out)
	}
	if !strings.Contains(out, "**Total** | **6.00** | **300.00 EUR**") {
		t.Fatalf("missing md total: %q", out)
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
