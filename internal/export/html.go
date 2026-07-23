package export

import (
	"html/template"
	"io"
	"strconv"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// htmlAmount is a pre-formatted currency amount for the template.
type htmlAmount struct {
	Currency string
	Amount   string
}

// htmlSubtotal is one CurrencySubtotal row, pre-formatted for the template.
type htmlSubtotal struct {
	Currency      string
	Hours         string
	BillableHours string
	BilledHours   string
	Amount        string
}

// htmlBucket is one Bucket row, pre-formatted for the template. A bucket may
// carry more than one currency amount (Amounts); those are indicative (see
// report.Build's caveat), never authoritative.
type htmlBucket struct {
	Label         string
	Hours         string
	BillableHours string
	BilledHours   string
	Amounts       []htmlAmount
}

// htmlView is the only thing the template ever sees: plain strings and
// slices, all pre-formatted from the report. No report.Report method or
// field is referenced from inside the template itself.
type htmlView struct {
	Period    string
	Timezone  string
	Summary   string
	Subtotals []htmlSubtotal
	Buckets   []htmlBucket
}

// htmlTemplate is parsed once at package init. html/template auto-escapes
// every {{.}} substitution for its HTML context, so user-controlled text
// (list names, task descriptions, tags) can never inject markup.
var htmlTemplate = template.Must(template.New("report").Parse(htmlTemplateSrc))

const htmlTemplateSrc = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Hours report {{.Period}}</title>
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif; margin: 2rem; color: #1a1a1a; background: #fff; }
  h1 { font-size: 1.4rem; margin-bottom: 0.2rem; }
  .meta { color: #555; margin-bottom: 1rem; }
  .summary { font-size: 1.05rem; margin: 1rem 0; font-weight: 600; }
  table { border-collapse: collapse; width: 100%; margin: 0.5rem 0 1.5rem; }
  caption { text-align: left; font-weight: bold; margin-bottom: 0.3rem; }
  th, td { border: 1px solid #ccc; padding: 0.4rem 0.6rem; text-align: right; }
  th:first-child, td:first-child { text-align: left; }
  .note { font-size: 0.85rem; color: #555; }
  @media (prefers-color-scheme: dark) {
    body { color: #e8e8e8; background: #1e1e1e; }
    th, td { border-color: #444; }
    .meta, .note { color: #aaa; }
  }
</style>
</head>
<body>
<h1>Hours report &mdash; {{.Period}}</h1>
<p class="meta">Timezone: {{.Timezone}}</p>
<p class="summary">{{.Summary}}</p>

<table>
<caption>Currency subtotals (authoritative)</caption>
<tr><th>Currency</th><th>Hours</th><th>Billable</th><th>Billed</th><th>Amount</th></tr>
{{range .Subtotals}}<tr data-currency="{{.Currency}}"><td>{{.Currency}}</td><td>{{.Hours}}</td><td>{{.BillableHours}}</td><td>{{.BilledHours}}</td><td class="subtotal-amount">{{.Amount}}</td></tr>
{{end}}</table>

<table>
<caption>Buckets</caption>
<tr><th>Label</th><th>Hours</th><th>Billable</th><th>Billed</th><th>Amount</th></tr>
{{range .Buckets}}<tr><td>{{.Label}}</td><td>{{.Hours}}</td><td>{{.BillableHours}}</td><td>{{.BilledHours}}</td><td class="bucket-amount">{{range .Amounts}}{{.Amount}} {{.Currency}} {{end}}</td></tr>
{{end}}</table>

<p class="note">Bucket amounts above are an indicative proportional allocation and may drift a few cents. The currency subtotals and invoice lines are the authoritative totals.</p>
</body>
</html>
`

// HTML writes the report as a self-contained HTML document: inline CSS, no
// external stylesheets, fonts, scripts or images. Sections: period +
// timezone, the shared summary line, currency subtotals (authoritative
// totals), and the bucket breakdown (indicative, see report.Build's caveat).
//
// There is no budget section: report.BudgetLines needs inputs
// (billedByList/budgets/listNames) that a report.Report does not carry, so an
// exporter taking only a Report cannot render one.
func HTML(w io.Writer, r report.Report) error {
	fmtHours := func(h float64) string { return strconv.FormatFloat(h, 'f', 2, 64) }
	fmtAmt := func(a float64) string { return strconv.FormatFloat(a, 'f', 2, 64) }

	subtotals := make([]htmlSubtotal, 0, len(r.CurrencySubtotals))
	for _, cs := range r.CurrencySubtotals {
		subtotals = append(subtotals, htmlSubtotal{
			Currency: cs.Currency, Hours: fmtHours(cs.Hours),
			BillableHours: fmtHours(cs.BillableHours), BilledHours: fmtHours(cs.BilledHours),
			Amount: fmtAmt(cs.Amount),
		})
	}

	buckets := make([]htmlBucket, 0, len(r.Buckets))
	for _, b := range r.Buckets {
		amounts := make([]htmlAmount, 0, len(b.Amounts))
		for _, a := range b.Amounts {
			amounts = append(amounts, htmlAmount{Currency: a.Currency, Amount: fmtAmt(a.Amount)})
		}
		buckets = append(buckets, htmlBucket{
			Label: b.Label, Hours: fmtHours(b.Hours),
			BillableHours: fmtHours(b.BillableHours), BilledHours: fmtHours(b.BilledHours),
			Amounts: amounts,
		})
	}

	view := htmlView{
		Period:    report.PeriodLabel(r.Start, r.End),
		Timezone:  r.Timezone,
		Summary:   SummaryLine(r),
		Subtotals: subtotals,
		Buckets:   buckets,
	}
	return htmlTemplate.Execute(w, view)
}
