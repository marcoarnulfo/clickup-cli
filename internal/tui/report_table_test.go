package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// tableFixture is a two-bucket, single-currency report.
func tableFixture() report.Report {
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	return report.Report{
		Start: start, End: start.AddDate(0, 1, 0),
		Scope: "me", GroupBy: report.GroupByList, DefaultCurrency: "EUR", Timezone: "UTC",
		Buckets: []report.Bucket{
			{Label: "Website", Key: "l1", Hours: 12.5, BillableHours: 12.5, BilledHours: 12.5,
				Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 625}}},
			{Label: "Internal", Key: "l2", Hours: 3, BillableHours: 0, BilledHours: 0,
				Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 0}}},
		},
		CurrencySubtotals: []report.CurrencySubtotal{
			{Currency: "EUR", Hours: 15.5, BillableHours: 12.5, BilledHours: 12.5, Amount: 625},
		},
		TotalHours: 15.5, BillableHours: 12.5, NonBillableHours: 3, BilledHours: 12.5, TotalAmount: 625,
	}
}

// The table must never be wider than the terminal. help's arithmetic taught us
// that "approximately fits" is not a property — this asserts the real thing.
//
// The widths start at 60 because below roughly 48 the Item column hits its
// 12-column floor and the table deliberately overflows instead of shrinking
// into illegibility; TestReportTableFloorsRatherThanVanishing covers that end.
//
// The second label is all double-width runes (🚀). truncate cuts by rune
// count, but reportItemWidth's budget and lipgloss/table's own resizer both
// measure in display columns — without reportTable's second truncation pass
// this label alone renders at up to 2x its rune-counted width and blows the
// terminal budget. An all-ASCII label can never exercise that mismatch.
//
// The threeCurrencies/fiveCurrencies buckets exercise the equivalent mismatch
// on the Amount column: formatAmounts joins every currency onto one line
// ("12345.00 EUR + 6789.00 USD + ..."), which a GroupByTotal report — the
// default first-load grouping — produces from its single bucket for every
// currency in the workspace. Before Amount was allowed to truncate too, 3
// currencies overflowed at width 60 and 5 currencies overflowed even at 100.
func TestReportTableNeverExceedsWidth(t *testing.T) {
	t.Parallel()
	threeCurrencies := []report.CurrencyAmount{
		{Currency: "EUR", Amount: 12345},
		{Currency: "USD", Amount: 6789},
		{Currency: "GBP", Amount: 4321},
	}
	fiveCurrencies := []report.CurrencyAmount{
		{Currency: "EUR", Amount: 12345},
		{Currency: "USD", Amount: 6789},
		{Currency: "GBP", Amount: 4321},
		{Currency: "CHF", Amount: 999},
		{Currency: "JPY", Amount: 50000},
	}
	for _, label := range []string{
		"A very long list name that will not fit anywhere near a narrow terminal",
		strings.Repeat("🚀", 40),
	} {
		r := tableFixture()
		r.Buckets[0].Label = label
		r.Buckets = append(r.Buckets,
			report.Bucket{Label: "Consulting", Key: "l3", Hours: 5, BilledHours: 5, Amounts: threeCurrencies},
			report.Bucket{Label: "Global Ops", Key: "l4", Hours: 2, BilledHours: 2, Amounts: fiveCurrencies},
		)
		for _, width := range []int{60, 80, 100, 120} {
			out := reportTable(testTheme(true), r, width)
			for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
				if w := lipgloss.Width(line); w > width {
					t.Errorf("label %q, width %d: line is %d columns:\n%s", label, width, w, line)
				}
			}
		}
	}
}

// At width 30 the numeric-column arithmetic alone goes negative, and
// reportItemWidth clamps to what the labels actually need instead of
// collapsing to nothing. tableFixture's longest label ("Internal", 8 columns)
// is itself under reportMinItemWidth, so this exercises the "cap wins over
// the floor" branch — the table overflows width 30 rather than truncating a
// short label down to illegibility. TestReportItemWidthGivesSlackToTheLabel
// covers the floor itself, with a label long enough for it to bind.
func TestReportTableFloorsRatherThanVanishing(t *testing.T) {
	t.Parallel()
	out := reportTable(testTheme(true), tableFixture(), 30)
	if !strings.Contains(out, "Website") {
		t.Errorf("the label column collapsed past usefulness at 30 columns:\n%s", out)
	}
}

// The Item column is the one that gives up its space first: the numeric
// columns are the point of having a grid, so Item shrinks before they do.
// Amount only gives up space once Item is already at its floor and the row
// would still overflow (TestReportTableNeverExceedsWidth's multi-currency
// cases), which this fixture's short, single-currency Amounts never trigger.
func TestReportItemWidthGivesSlackToTheLabel(t *testing.T) {
	t.Parallel()
	rows, _ := reportRows(tableFixture())
	rows[0][0] = strings.Repeat("x", 200) // a label longer than any terminal

	narrow := reportItemWidth(rows, 60)
	wide := reportItemWidth(rows, 120)
	if narrow >= wide {
		t.Errorf("item width did not grow with the terminal: 60 -> %d, 120 -> %d", narrow, wide)
	}
	if narrow < 12 {
		t.Errorf("item width %d fell below the 12-column floor", narrow)
	}
	// At width 30 the numeric-column budget alone is negative (see
	// TestReportTableFloorsRatherThanVanishing), so with a label this long the
	// result must be exactly reportMinItemWidth, not some other value that
	// merely happens to sit in [8, 24] — this is the case where the floor
	// itself, not the label length, decides the answer.
	if got := reportItemWidth(rows, 30); got != reportMinItemWidth {
		t.Errorf("reportItemWidth(_, 30) = %d, want the %d-column floor", got, reportMinItemWidth)
	}
}

// width <= 0 is the first render, before the terminal has sent WindowSizeMsg.
// It must reproduce the fixed 32-column layout the report had before #66.
func TestReportItemWidthZeroIsNaturalWidth(t *testing.T) {
	t.Parallel()
	rows, _ := reportRows(tableFixture())
	rows[0][0] = strings.Repeat("x", 200)
	if got := reportItemWidth(rows, 0); got != 32 {
		t.Errorf("natural item width = %d, want 32", got)
	}
}

// A long label must render as one row, never spread across several lines.
// This does not actually exercise Wrap(false): reportTable never calls
// Table.Width, so every column is sized to exactly its own content's width
// (see resizing.go) and lipgloss/table has no spare width to wrap a cell
// into regardless of the Wrap setting — verified empirically, the rendered
// output is byte-identical with and without Wrap(false). Wrap(false) is kept
// as insurance against a future change that starts calling Width(); what this
// test actually pins is the real invariant: one bucket is one row.
func TestReportTableTruncatesRatherThanWraps(t *testing.T) {
	t.Parallel()
	r := tableFixture()
	r.Buckets[0].Label = strings.Repeat("long ", 40)
	out := reportTable(testTheme(true), r, 60)
	// top border, header, header separator, 2 buckets, TOTAL, bottom border.
	if n := len(strings.Split(strings.TrimRight(out, "\n"), "\n")); n != 7 {
		t.Errorf("table has %d lines, want 7 — a cell wrapped instead of truncating:\n%s", n, out)
	}
}

// The multi-currency total block: a TOTAL row with an empty Amount cell,
// followed by one subtotal row per currency.
func TestReportRowsMultiCurrencyTotals(t *testing.T) {
	t.Parallel()
	r := tableFixture()
	r.CurrencySubtotals = []report.CurrencySubtotal{
		{Currency: "EUR", Hours: 12.5, BilledHours: 12.5, Amount: 625},
		{Currency: "USD", Hours: 3, BilledHours: 3, Amount: 150},
	}
	rows, firstTotal := reportRows(r)
	if firstTotal != 2 {
		t.Fatalf("firstTotal = %d, want 2 (the two buckets come first)", firstTotal)
	}
	if len(rows) != 5 {
		t.Fatalf("got %d rows, want 5 (2 buckets + TOTAL + 2 subtotals)", len(rows))
	}
	if rows[2][0] != "TOTAL" || rows[2][3] != "" {
		t.Errorf("TOTAL row = %q, want an empty Amount cell when the report is multi-currency", rows[2])
	}
	if !strings.Contains(rows[3][0], "subtotal EUR") || !strings.Contains(rows[4][0], "subtotal USD") {
		t.Errorf("subtotal rows = %q, %q", rows[3][0], rows[4][0])
	}
}

// TestReportStyleFunc is the ONLY check on the grid's colors. The package
// goldens run under termenv.Ascii, which strips backgrounds and bold, so a
// broken zebra or an uncolored TOTAL would leave every golden byte-identical.
// It therefore asserts on the Style the function returns, not on rendered text.
func TestReportStyleFunc(t *testing.T) {
	t.Parallel()
	th := paletteTheme(true) // real colors, so backgrounds are comparable
	const firstTotal = 2
	f := reportStyleFunc(th, firstTotal)

	if got := f(table.HeaderRow, 0); !got.GetBold() {
		t.Error("header row is not bold")
	}
	if f(0, 0).GetBackground() == th.Zebra.GetBackground() {
		t.Error("row 0 is striped; zebra must start on the second row")
	}
	if f(1, 0).GetBackground() != th.Zebra.GetBackground() {
		t.Error("row 1 is not striped")
	}
	if f(firstTotal, 0).GetBackground() == th.Zebra.GetBackground() {
		t.Error("the TOTAL row is striped; totals are not part of the zebra sequence")
	}
	if f(firstTotal, 0).GetForeground() != th.OK.GetForeground() {
		t.Error("the TOTAL row does not carry the OK color")
	}
	if !f(firstTotal, 0).GetBold() {
		t.Error("the TOTAL row is not bold")
	}
	if f(firstTotal+1, 0).GetBold() {
		t.Error("a per-currency subtotal row is bold; only the TOTAL row should be")
	}
	if f(0, 0).GetAlign() != lipgloss.Left {
		t.Error("the Item column is not left-aligned")
	}
	for col := 1; col <= 3; col++ {
		if f(0, col).GetAlign() != lipgloss.Right {
			t.Errorf("column %d is not right-aligned", col)
		}
	}
}
