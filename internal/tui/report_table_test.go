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

// Under GroupByTotal — the grouping the report loads with — the single bucket IS
// the totals row: same hours, same billed, and the same amounts, because one
// bucket collects every entry. The table used to show "Total" directly above
// "TOTAL", differing only in case and color (#137).
func TestReportRowsSuppressesTheBucketUnderTotalGrouping(t *testing.T) {
	t.Parallel()
	r := report.Report{
		GroupBy:         report.GroupByTotal,
		DefaultCurrency: "EUR",
		Buckets: []report.Bucket{{Label: "Total", Hours: 15.5, BilledHours: 12.5,
			Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 625}}}},
		CurrencySubtotals: []report.CurrencySubtotal{{Currency: "EUR", Hours: 15.5, BilledHours: 12.5, Amount: 625}},
		TotalHours:        15.5, BilledHours: 12.5, TotalAmount: 625,
	}
	rows, firstTotal := reportRows(r)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1 (the TOTAL row alone): %v", len(rows), rows)
	}
	if firstTotal != 0 {
		t.Errorf("firstTotal = %d, want 0 (the TOTAL row is the first row)", firstTotal)
	}
	if rows[0][0] != "TOTAL" {
		t.Errorf("rows[0][0] = %q, want %q", rows[0][0], "TOTAL")
	}
}

// Multi-currency keeps its shape: TOTAL with an empty Amount, then one subtotal
// row per currency. The suppressed bucket carried "A + B", which is the same
// information those subtotal rows carry.
func TestReportRowsSuppressesTheBucketUnderTotalGroupingMultiCurrency(t *testing.T) {
	t.Parallel()
	r := report.Report{
		GroupBy:         report.GroupByTotal,
		DefaultCurrency: "EUR",
		Buckets: []report.Bucket{{Label: "Total", Hours: 18, BilledHours: 17,
			Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 337.5}, {Currency: "USD", Amount: 512.5}}}},
		CurrencySubtotals: []report.CurrencySubtotal{
			{Currency: "EUR", Hours: 7.75, BilledHours: 6.75, Amount: 337.5},
			{Currency: "USD", Hours: 10.25, BilledHours: 10.25, Amount: 512.5},
		},
		TotalHours: 18, BilledHours: 17,
	}
	rows, firstTotal := reportRows(r)
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3 (TOTAL + two subtotals): %v", len(rows), rows)
	}
	if firstTotal != 0 {
		t.Errorf("firstTotal = %d, want 0", firstTotal)
	}
	if rows[0][3] != "" {
		t.Errorf("TOTAL Amount = %q, want empty (the subtotals carry the figures)", rows[0][3])
	}
}

// The suppression targets the degenerate single-bucket case, not GroupByTotal
// in general: today internal/report's aggregation (aggregate.go's groupKeys,
// GroupByTotal's default branch) only ever emits one bucket for that grouping,
// but nothing in this package enforces that invariant, and the guard reads
// r.GroupBy == GroupByTotal alongside len(r.Buckets) == 1 for a reason — if a
// future change to groupKeys ever produced more than one bucket under
// GroupByTotal, dropping every bucket row on GroupBy alone would silently
// discard real data with nothing to fall back on within a currency's
// breakdown. This pins that the narrower, two-part condition is what is
// actually checked, using two synthetic buckets that report.Build itself
// would never produce today.
func TestReportRowsKeepsMultipleBucketsUnderTotalGrouping(t *testing.T) {
	t.Parallel()
	r := report.Report{
		GroupBy:         report.GroupByTotal,
		DefaultCurrency: "EUR",
		Buckets: []report.Bucket{
			{Label: "Total A", Hours: 10, BilledHours: 10,
				Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 500}}},
			{Label: "Total B", Hours: 5, BilledHours: 5,
				Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 250}}},
		},
		CurrencySubtotals: []report.CurrencySubtotal{{Currency: "EUR", Hours: 15, BilledHours: 15, Amount: 750}},
		TotalHours:        15, BilledHours: 15, TotalAmount: 750,
	}
	rows, firstTotal := reportRows(r)
	if len(rows) != 3 || firstTotal != 2 {
		t.Fatalf("got %d rows firstTotal=%d, want 3 rows firstTotal=2 (both buckets survive, then TOTAL): %v",
			len(rows), firstTotal, rows)
	}
	if rows[0][0] != "Total A" || rows[1][0] != "Total B" {
		t.Errorf("bucket rows = %q, %q, want both bucket labels to survive", rows[0][0], rows[1][0])
	}
}

// Every other grouping keeps its bucket rows: the suppression is about the
// degenerate one-bucket case of GroupByTotal, not about totals in general.
func TestReportRowsKeepsBucketsUnderOtherGroupings(t *testing.T) {
	t.Parallel()
	r := report.Report{
		GroupBy:           report.GroupByList,
		DefaultCurrency:   "EUR",
		Buckets:           []report.Bucket{{Label: "Website", Hours: 12.5, BilledHours: 12.5}},
		CurrencySubtotals: []report.CurrencySubtotal{{Currency: "EUR", Hours: 12.5, BilledHours: 12.5, Amount: 625}},
		TotalHours:        12.5, BilledHours: 12.5, TotalAmount: 625,
	}
	rows, firstTotal := reportRows(r)
	if len(rows) != 2 || firstTotal != 1 {
		t.Fatalf("got %d rows firstTotal=%d, want 2 rows firstTotal=1: %v", len(rows), firstTotal, rows)
	}
}

// reportNumWidth reserved 8 columns for each of Hours and Billed but nothing
// enforced it: a wider value pushed the table past the terminal edge by exactly
// 2 x (width - 8). The overflow needs BOTH factors — a long label (so Item is
// budget-bound rather than capped by its own content) and numbers wider than 8.
// A test missing either one passes against the bug: measured, a long label with
// ordinary hours renders 55 columns at width 60.
//
// 40 and 44 are deliberately absent: the fix cannot fit this table in fewer than
// 48 columns (chrome 10 + Item floor 12 + 10 + 10 + Amount header 6), so
// asserting there would fail against correct code.
func TestReportTableNeverExceedsWidthWithLongLabelAndWideNumbers(t *testing.T) {
	th := testTheme(true)
	const longLabel = "Website redesign and content migration backlog"
	rep := reportWithHours(longLabel, 1234567.5)
	for _, width := range []int{60, 80, 100} {
		got := lipgloss.Width(strings.Split(reportTable(th, rep, width), "\n")[0])
		if got > width {
			t.Errorf("width %d: table rendered %d columns", width, got)
		}
	}
}

// The other direction: short labels leave Item capped by its own content, but
// wide numbers still push the table past a narrow terminal. 44 is the width
// where the current code renders 47 (overflowing by 3) and the fix renders 43.
func TestReportTableNeverExceedsWidthWithShortLabelAndWideNumbers(t *testing.T) {
	th := testTheme(true)
	rep := reportWithHours("Website", 1234567.5)
	if got := lipgloss.Width(strings.Split(reportTable(th, rep, 44), "\n")[0]); got > 44 {
		t.Errorf("table rendered %d columns in a 44-column terminal", got)
	}
}

// reportWithHours builds a one-bucket report whose Hours and Billed carry the
// given value, so a test can dial the numeric width independently of the label.
func reportWithHours(label string, hours float64) report.Report {
	return report.Report{
		GroupBy: report.GroupByList, DefaultCurrency: "EUR",
		Buckets: []report.Bucket{{Label: label, Hours: hours, BilledHours: hours,
			Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 625}}}},
		CurrencySubtotals: []report.CurrencySubtotal{{Currency: "EUR", Hours: hours, BilledHours: hours, Amount: 625}},
		TotalHours:        hours, BilledHours: hours, TotalAmount: 625,
	}
}

// Measuring the data only would make the budget assume 10 where the renderer
// uses 11, and the table would overflow by 1 — the same bug in a smaller size.
func TestReportNumWidthsIncludeTheHeaders(t *testing.T) {
	t.Parallel()
	rows := [][]string{{"Website", "1.00", "1.00", "5.00 EUR"}}
	hours, billed := reportNumWidths(rows)
	if hours != lipgloss.Width(reportHeaders[1]) || billed != lipgloss.Width(reportHeaders[2]) {
		t.Errorf("reportNumWidths = (%d, %d), want the header widths (%d, %d)",
			hours, billed, lipgloss.Width(reportHeaders[1]), lipgloss.Width(reportHeaders[2]))
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
