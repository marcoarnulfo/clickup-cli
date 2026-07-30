package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// The report grid's fixed dimensions. Slack is taken from and given to the
// Item column first: the numeric columns are the reason to have a grid, so
// Item gives up its space before they do. Amount is the one exception, and
// only as a last resort — see reportAmountWidth for why that is safe.
//
// Hours and Billed are not in this block: reportNumWidths measures them from
// the rows instead of reserving a flat width, so a wider value can never push
// the table past the terminal edge the way a fixed reservation did (#138).
const (
	reportMinItemWidth = 12 // below this the labels stop being labels
	reportNatItemWidth = 32 // the fixed width the report had before #66
	// reportTableChrome is the border and padding the table spends on itself:
	// two outer borders (column separators are off) plus two padding columns
	// on each of the four columns.
	reportTableChrome = 2 + 4*2
)

var reportHeaders = []string{"Item", "Hours", "Billed", "Amount"}

// reportRows renders the report as table cells and reports the index at which
// the total rows begin. Bucket rows come first, then either a single TOTAL row
// (one currency) or a TOTAL row with an empty Amount followed by one subtotal
// row per currency. The exception is GroupByTotal with a single bucket: that
// bucket row is suppressed as a duplicate of TOTAL (#137), so TOTAL is row 0
// and there are no bucket rows at all.
//
// Per-bucket Amounts are indicative: PerDay rounding can drift a few cents
// from the subtotals at a fine grouping. CurrencySubtotals is authoritative
// and is never re-derived from the bucket rows.
func reportRows(r report.Report) ([][]string, int) {
	rows := make([][]string, 0, len(r.Buckets)+len(r.CurrencySubtotals)+1)
	// Under GroupByTotal the single bucket IS the totals row: it collects every
	// entry, so its hours, billed hours and Amounts equal the totals and the
	// currency subtotals exactly — not approximately, so there is not even the
	// PerDay rounding caveat that applies at finer groupings. Emitting both put
	// "Total" directly above "TOTAL", differing only in case and color (#137).
	if !(r.GroupBy == report.GroupByTotal && len(r.Buckets) == 1) {
		for _, b := range r.Buckets {
			rows = append(rows, []string{
				b.Label,
				fmt.Sprintf("%.2f", b.Hours),
				fmt.Sprintf("%.2f", b.BilledHours),
				formatAmounts(b.Amounts, r.DefaultCurrency),
			})
		}
	}
	firstTotal := len(rows)

	if len(r.CurrencySubtotals) <= 1 {
		rows = append(rows, []string{
			"TOTAL",
			fmt.Sprintf("%.2f", r.TotalHours),
			fmt.Sprintf("%.2f", r.BilledHours),
			fmt.Sprintf("%.2f %s", r.TotalAmount, r.DefaultCurrency),
		})
		return rows, firstTotal
	}

	rows = append(rows, []string{
		"TOTAL",
		fmt.Sprintf("%.2f", r.TotalHours),
		fmt.Sprintf("%.2f", r.BilledHours),
		"",
	})
	for _, s := range r.CurrencySubtotals {
		rows = append(rows, []string{
			"  subtotal " + s.Currency,
			fmt.Sprintf("%.2f", s.Hours),
			fmt.Sprintf("%.2f", s.BilledHours),
			fmt.Sprintf("%.2f %s", s.Amount, s.Currency),
		})
	}
	return rows, firstTotal
}

// formatAmounts renders a bucket's per-currency amounts on one line, e.g.
// "150.00 EUR + 90.00 USD". An empty slice renders as a zero in fallback.
func formatAmounts(amounts []report.CurrencyAmount, fallback string) string {
	if len(amounts) == 0 {
		return fmt.Sprintf("%.2f %s", 0.0, fallback)
	}
	parts := make([]string, 0, len(amounts))
	for _, a := range amounts {
		parts = append(parts, fmt.Sprintf("%.2f %s", a.Amount, a.Currency))
	}
	return strings.Join(parts, " + ")
}

// reportNumWidths measures the Hours and Billed columns from the rows, HEADERS
// INCLUDED. The headers are not decoration in this arithmetic: lipgloss/table
// sizes a column from every row it is given, the header row among them, so a
// "Billed" header holds 6 columns even when every value is 5. reportNumWidth
// used to reserve a flat 8 for each and nothing enforced it — a wider value
// simply pushed the table past the terminal edge by 2 x (width - 8) (#138).
func reportNumWidths(rows [][]string) (hours, billed int) {
	hours = lipgloss.Width(reportHeaders[1])
	billed = lipgloss.Width(reportHeaders[2])
	for _, row := range rows {
		hours = max(hours, lipgloss.Width(row[1]))
		billed = max(billed, lipgloss.Width(row[2]))
	}
	return hours, billed
}

// reportItemWidth splits width between the numeric columns and the Item
// column. It never stretches Item past the longest label (empty space is not a
// feature) and never shrinks it below reportMinItemWidth — unless the labels
// themselves are shorter than that floor.
//
// Hours and Billed are measured by reportNumWidths, not reserved at a worst
// case, so the table renders exactly as wide as its content needs. This
// function alone cannot promise the whole table fits, though: it assumes
// Amount keeps its natural, untruncated width, and a multi-currency bucket's
// Amount can blow well past whatever is left once Item is already at its
// floor. reportAmountWidth is the other half of that promise: it is what
// actually claws space back from Amount, as a last resort, once Item has none
// left to give.
func reportItemWidth(rows [][]string, width int) int {
	maxLabel := lipgloss.Width(reportHeaders[0])
	amount := lipgloss.Width(reportHeaders[3])
	for _, row := range rows {
		maxLabel = max(maxLabel, lipgloss.Width(row[0]))
		amount = max(amount, lipgloss.Width(row[3]))
	}
	if width <= 0 {
		return min(maxLabel, reportNatItemWidth)
	}
	hours, billed := reportNumWidths(rows)
	floor := min(reportMinItemWidth, maxLabel)
	item := width - reportTableChrome - hours - billed - amount
	return max(floor, min(maxLabel, item))
}

// reportAmountWidth is reportItemWidth's counterpart for the Amount column.
// Item yields first — it is the point of having a grid, so it gives up all
// the slack reportItemWidth is willing to give before Amount loses a single
// column. Only once Item is already at its floor and the row would still
// overflow does this claw space back from Amount, as a last resort.
//
// That resort mostly targets a per-bucket Amount, which is indicative only:
// reportRows' own doc comment already says so — PerDay rounding can drift a
// few cents from the subtotals at a fine grouping, and CurrencySubtotals is
// authoritative. Those authoritative per-currency figures do render as their
// own rows at the bottom of the same table, at full precision — but amountW
// is one column width shared by every row, subtotals included, so a subtotal
// cell truncates exactly like a bucket cell once its natural width no longer
// fits.
//
// Where that line falls is not a fixed column count. Rendering whole needs
// budget — the terminal's width minus chrome, Item and the COMBINED width of
// Hours and Billed (reportNumWidths) — to reach at least the subtotal cell's
// own natural width (computed below), and none of those three quantities is
// reserved at a flat size: all are measured from the rows. Widen any of
// them by a column — more digits in Hours or Billed, or a longer currency
// code in the subtotal itself — and the terminal width the subtotal needs to
// render whole grows by exactly that much; narrow them and it shrinks by the
// same amount. There is no single number to name here that would stay true
// as those widths change. TestReportTableNeverExceedsWidthWithLongLabelAndWideNumbers
// pins one concrete case, with the numbers it needs measured and asserted in
// the test itself. Once Item has hit its reportMinItemWidth floor the whole
// table is already in the degraded regime that floor exists to accept; a
// truncated subtotal in that regime is one more symptom of the same accepted
// trade-off, not a new one.
//
// width <= 0 is the first render, before the terminal has sent its
// WindowSizeMsg: nothing is sized against it yet, so Amount stays at its
// natural width, same as Item. Otherwise this never stretches Amount past
// what the content needs, only ever shrinks it — down to 1, never lower. The
// floor is not about keeping the COLUMN alive: Headers(reportHeaders...)
// passes "Amount" whole, and lipgloss/table sizes a column from every cell it
// is given, the header included, so even at floor 0 the column still renders
// at the header's own width (verified: floor 1 renders that same width, never
// 1 column). What floor 0 actually does is worse: truncateWidth(s, 0)
// collapses every DATA cell to "", so the row shows nothing where a value
// belongs — not even the "…" that would signal a cut. Floor 1 keeps that
// signal, since truncateWidth(s, 1) still renders a one-column ellipsis,
// which is why max(1, ...) below is the floor and not max(0, ...).
func reportAmountWidth(rows [][]string, width, itemW int) int {
	natural := lipgloss.Width(reportHeaders[3])
	for _, row := range rows {
		natural = max(natural, lipgloss.Width(row[3]))
	}
	if width <= 0 {
		return natural
	}
	hours, billed := reportNumWidths(rows)
	budget := width - reportTableChrome - hours - billed - itemW
	return max(1, min(natural, budget))
}

// reportStyleFunc decides every per-cell style: alignment, the header, the
// zebra stripe and the totals.
//
// It is a separate function because a golden cannot see what it does. TestMain
// pins termenv.Ascii, which strips backgrounds and bold, so a broken stripe or
// an uncolored TOTAL leaves every golden byte-identical. The test asserts on
// the Style this returns.
//
// It never calls lipgloss.NewStyle(): that would build on the DEFAULT
// renderer, so plain rows and themed rows would render through different
// renderers. th.Cell is the plain style on the right one.
func reportStyleFunc(th theme, firstTotal int) table.StyleFunc {
	return func(row, col int) lipgloss.Style {
		st := th.Cell
		switch {
		case row == table.HeaderRow:
			st = th.Header
		case row >= firstTotal:
			st = th.OK
			if row == firstTotal {
				st = st.Bold(true) // the TOTAL line, not the per-currency subtotals
			}
		case row%2 == 1:
			st = th.Zebra
		}
		st = st.Padding(0, 1)
		if col > 0 {
			st = st.Align(lipgloss.Right)
		}
		return st
	}
}

// reportTable renders the buckets and the totals as one table sized to width.
// width <= 0 means natural width: the first render, before the terminal has
// sent its WindowSizeMsg.
//
// Three lipgloss/table defaults are wrong for this grid and are all turned
// off: BorderColumn draws separators between columns, Wrap sends a long label
// onto a second line instead of letting truncateWidth cut it, and the zero
// BorderStyle renders the frame through the default renderer.
//
// Sizing is implicit and Table.Width is deliberately NOT called. With
// BorderColumn off, the library's resizer counts only the separators between
// columns, so it believes the frame costs nothing, while String clips the
// result with MaxWidth — the table would render two columns too wide and then
// have its right border sliced off. Pre-formatted numeric cells, labels
// truncated to itemW and Padding(0, 1) from the style function give the exact
// widths this arithmetic assumes.
func reportTable(th theme, r report.Report, width int) string {
	rows, firstTotal := reportRows(r)
	itemW := reportItemWidth(rows, width)
	amountW := reportAmountWidth(rows, width, itemW)
	for i := range rows {
		rows[i][0] = truncateWidth(rows[i][0], itemW)
		rows[i][3] = truncateWidth(rows[i][3], amountW)
	}
	return table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(th.Border).
		BorderColumn(false).
		BorderHeader(true).
		Wrap(false).
		Headers(reportHeaders...).
		Rows(rows...).
		StyleFunc(reportStyleFunc(th, firstTotal)).
		String()
}
