package export

import (
	"bytes"
	"encoding/csv"
	"regexp"
	"strconv"
	"testing"
)

// reHTMLSubtotalRow matches one currency-subtotal table row rendered by HTML
// (see the `data-currency` attribute in the template), capturing the currency
// and its rendered amount.
var reHTMLSubtotalRow = regexp.MustCompile(`<tr data-currency="([A-Z]{3})"><td>[^<]*</td><td>[^<]*</td><td>[^<]*</td><td>[^<]*</td><td class="subtotal-amount">([\d.]+)</td></tr>`)

// TestCrossExportLedgerEquality is the hard money-ledger requirement: for the
// same report, summing the InvoiceCSV amount column per currency must equal
// CurrencySubtotals, which must equal what HTML renders as the (authoritative)
// currency-subtotal amounts. None of the three may recompute the money —
// they must all agree because they all read the same already-rounded numbers.
func TestCrossExportLedgerEquality(t *testing.T) {
	r := buildMultiCurrencyFixture()
	if len(r.CurrencySubtotals) < 2 {
		t.Fatalf("fixture sanity: want a multi-currency report, got %d subtotal(s)", len(r.CurrencySubtotals))
	}

	want := map[string]float64{}
	for _, cs := range r.CurrencySubtotals {
		want[cs.Currency] = cs.Amount
	}

	// --- sum the InvoiceCSV amount column per currency ---
	var invBuf bytes.Buffer
	if err := InvoiceCSV(&invBuf, r); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&invBuf).ReadAll()
	if err != nil {
		t.Fatalf("re-parsing invoice CSV: %v", err)
	}
	csvSums := map[string]float64{}
	for _, row := range rows[1:] { // skip header
		amount, err := strconv.ParseFloat(row[7], 64)
		if err != nil {
			t.Fatalf("parsing amount %q: %v", row[7], err)
		}
		currency := row[8]
		csvSums[currency] = round2(csvSums[currency] + amount)
	}
	for cur, wantAmt := range want {
		if csvSums[cur] != wantAmt {
			t.Errorf("InvoiceCSV sum(%s) = %v, want %v (CurrencySubtotals)", cur, csvSums[cur], wantAmt)
		}
	}

	// --- parse the HTML currency-subtotal rows ---
	var htmlBuf bytes.Buffer
	if err := HTML(&htmlBuf, r); err != nil {
		t.Fatal(err)
	}
	matches := reHTMLSubtotalRow.FindAllStringSubmatch(htmlBuf.String(), -1)
	if len(matches) != len(r.CurrencySubtotals) {
		t.Fatalf("HTML has %d currency-subtotal rows, want %d; html:\n%s", len(matches), len(r.CurrencySubtotals), htmlBuf.String())
	}
	htmlAmounts := map[string]float64{}
	for _, m := range matches {
		amount, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			t.Fatalf("parsing HTML amount %q: %v", m[2], err)
		}
		htmlAmounts[m[1]] = amount
	}
	for cur, wantAmt := range want {
		if htmlAmounts[cur] != wantAmt {
			t.Errorf("HTML subtotal(%s) = %v, want %v (CurrencySubtotals)", cur, htmlAmounts[cur], wantAmt)
		}
	}

	// --- the three sources must also agree with each other, not just with want ---
	for cur := range want {
		if csvSums[cur] != htmlAmounts[cur] {
			t.Errorf("ledger mismatch for %s: InvoiceCSV sum = %v, HTML = %v", cur, csvSums[cur], htmlAmounts[cur])
		}
	}
}
