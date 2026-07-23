package export

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// reRemoteAsset matches an href/src attribute pointing outside the document:
// an absolute http(s) URL or a protocol-relative "//host" URL.
var reRemoteAsset = regexp.MustCompile(`(?i)(?:src|href)\s*=\s*["'](https?:)?//`)

func TestHTMLNoRemoteAssets(t *testing.T) {
	var b bytes.Buffer
	if err := HTML(&b, buildMultiCurrencyFixture()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if reRemoteAsset.FindString(out) != "" {
		t.Errorf("HTML references a remote asset: %q", reRemoteAsset.FindString(out))
	}
	if strings.Contains(out, `<link rel="stylesheet"`) {
		t.Error("HTML pulls in an external stylesheet via <link rel=\"stylesheet\">")
	}
	if !strings.Contains(out, "<style>") {
		t.Error("HTML has no inline <style> block")
	}
}

// TestHTMLEscapesUserControlledText proves the template escapes list/task
// names instead of being built by string concatenation: a bucket label
// carrying a script tag must never appear unescaped in the output.
func TestHTMLEscapesUserControlledText(t *testing.T) {
	r := report.Report{
		Start: buildMultiCurrencyFixture().Start, End: buildMultiCurrencyFixture().End,
		Timezone: "UTC", DefaultCurrency: "EUR",
		Buckets: []report.Bucket{
			{Label: `<script>alert(1)</script>`, Key: "l1", Hours: 1, BillableHours: 1, BilledHours: 1,
				Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 50}}},
		},
		CurrencySubtotals: []report.CurrencySubtotal{
			{Currency: "EUR", Hours: 1, BillableHours: 1, BilledHours: 1, Amount: 50},
		},
	}

	var b bytes.Buffer
	if err := HTML(&b, r); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Fatalf("HTML embeds the bucket label unescaped: %s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("HTML does not contain the escaped bucket label: %s", out)
	}
}

// TestHTMLDeclaresAuthoritativeTotals is A7: the doc must say the currency
// subtotals/invoice lines are authoritative, not the (indicative) bucket rows.
func TestHTMLDeclaresAuthoritativeTotals(t *testing.T) {
	var b bytes.Buffer
	if err := HTML(&b, buildMultiCurrencyFixture()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(strings.ToLower(out), "authoritative") {
		t.Errorf("HTML does not call out the authoritative totals (A7); got:\n%s", out)
	}
}

func TestHTMLContainsSummaryAndPeriod(t *testing.T) {
	r := buildMultiCurrencyFixture()
	var b bytes.Buffer
	if err := HTML(&b, r); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, SummaryLine(r)) {
		t.Errorf("HTML does not contain the summary line; got:\n%s", out)
	}
	if !strings.Contains(out, "July 2026") {
		t.Errorf("HTML does not contain the period label; got:\n%s", out)
	}
	if !strings.Contains(out, "UTC") {
		t.Errorf("HTML does not contain the timezone; got:\n%s", out)
	}
}
