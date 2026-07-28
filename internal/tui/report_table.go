package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// The report grid's fixed dimensions. Slack is taken from and given to the
// Item column alone: the numeric columns are the reason to have a grid, and an
// amount must never be truncated because a truncated amount hides money.
const (
	reportNumWidth     = 8  // reserved for each of Hours and Billed
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
// row per currency.
//
// Per-bucket Amounts are indicative: PerDay rounding can drift a few cents
// from the subtotals at a fine grouping. CurrencySubtotals is authoritative
// and is never re-derived from the bucket rows.
func reportRows(r report.Report) ([][]string, int) {
	rows := make([][]string, 0, len(r.Buckets)+len(r.CurrencySubtotals)+1)
	for _, b := range r.Buckets {
		rows = append(rows, []string{
			b.Label,
			fmt.Sprintf("%.2f", b.Hours),
			fmt.Sprintf("%.2f", b.BilledHours),
			formatAmounts(b.Amounts, r.DefaultCurrency),
		})
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

// reportItemWidth splits width between the fixed numeric columns and the Item
// column. It never stretches Item past the longest label (empty space is not a
// feature) and never shrinks it below reportMinItemWidth — unless the labels
// themselves are shorter than that floor.
//
// Reserving reportNumWidth for Hours and Billed is a worst case: real values
// are shorter, so the table often renders narrower than the terminal. Narrower
// is fine; wider is the bug this arithmetic exists to prevent.
func reportItemWidth(rows [][]string, headers []string, width int) int {
	maxLabel := lipgloss.Width(headers[0])
	amount := lipgloss.Width(headers[3])
	for _, row := range rows {
		maxLabel = max(maxLabel, lipgloss.Width(row[0]))
		amount = max(amount, lipgloss.Width(row[3]))
	}
	if width <= 0 {
		return min(maxLabel, reportNatItemWidth)
	}
	floor := min(reportMinItemWidth, maxLabel)
	item := width - reportTableChrome - 2*reportNumWidth - amount
	return max(floor, min(maxLabel, item))
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
// onto a second line instead of letting truncate cut it, and the zero
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
	itemW := reportItemWidth(rows, reportHeaders, width)
	for i := range rows {
		rows[i][0] = truncate(rows[i][0], itemW)
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
