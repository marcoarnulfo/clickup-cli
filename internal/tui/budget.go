package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// budgetModel renders the budget burn-down view (#64): one text progress bar
// per report.BudgetLine, most-burned first (BudgetLines' own sort order).
type budgetModel struct {
	lines []report.BudgetLine
}

func newBudget(lines []report.BudgetLine) budgetModel { return budgetModel{lines: lines} }

func (m Model) updateBudget(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := keysFor(m)
	switch {
	case key.Matches(msg, k.Back), key.Matches(msg, k.Budget):
		m = m.pop()
		return m, nil
	}
	return m, nil
}

// budgetBarWidth is the gauge's width in characters.
const budgetBarWidth = 20

// The gauge's glyphs. Blocks rather than '#' and '-': at any font size they
// read as a filled bar instead of as text.
const (
	gaugeFull  = '█'
	gaugeEmpty = '░'
)

// budgetFillStyle is the color of the filled part of a gauge. Over budget is
// the state this screen exists to surface, so it must not look like a healthy
// one. It is a named function because the package goldens run under
// termenv.Ascii, which strips the color: asserting on the style is the only
// way to test the choice.
func budgetFillStyle(th theme, percentUsed float64) lipgloss.Style {
	if percentUsed > 100 {
		return th.Err
	}
	return th.OK
}

// renderBudgetBar renders a percent-used value as a fixed-width gauge, e.g.
// "████████████░░░░░░░░ 60%". The fill is clamped to [0, 100] — a list can run
// over budget, but the bar cannot render past full — while the percentage in
// the label is shown UNCLAMPED, so an over-100% burn stays visible in the
// number. That asymmetry is the whole point: bubbles/progress clamps the
// number too, which is why this screen does not use it.
func renderBudgetBar(th theme, percentUsed float64) string {
	fillPct := min(max(percentUsed, 0), 100)
	filled := int(fillPct / 100 * budgetBarWidth)
	bar := budgetFillStyle(th, percentUsed).Render(strings.Repeat(string(gaugeFull), filled)) +
		th.Help.Render(strings.Repeat(string(gaugeEmpty), budgetBarWidth-filled))
	// The fill truncates (int()), so the label FLOORS to match. Rounding the
	// label instead is what made 99.5% read "100%" over 19 of 20 blocks; and
	// rounding the FILL to match a rounded label would be worse — it would show
	// a full bar from 97.5%, and a full bar means the budget is spent. Above 100
	// the label stays unclamped: that asymmetry is the whole point of this
	// function (see the doc comment).
	return fmt.Sprintf("%s %.0f%%", bar, math.Floor(percentUsed))
}

const (
	// boxChrome is what th.Box spends on itself: a rounded border plus one
	// column of padding on each side. Measured, not assumed:
	// th.Box.Render("12345") renders 9 columns.
	boxChrome = 4
	// budgetNameWidth is the natural list-name column, used before the first
	// WindowSizeMsg — the same "nothing is sized yet" fallback reportItemWidth has.
	budgetNameWidth = 24
	// budgetMinNameWidth is where a list name stops being a name, mirroring
	// reportMinItemWidth's floor on the report table.
	budgetMinNameWidth = 12
)

// budgetFigures renders the money part of a row. remaining is the most redundant
// field on the line — it is Budget minus Billed — so it is what gives way on a
// narrow terminal.
func budgetFigures(l report.BudgetLine, withRemaining bool) string {
	if withRemaining {
		return fmt.Sprintf("%.2f / %.2f %s (remaining %.2f)", l.Billed, l.Budget, l.Currency, l.Remaining)
	}
	return fmt.Sprintf("%.2f / %.2f %s", l.Billed, l.Budget, l.Currency)
}

// budgetLayout resolves the list-name column and whether "(remaining …)" fits.
//
// Everything except the name is MEASURED from the real rows, because none of it
// is fixed-width: the figures grow with the amounts and the percentage grows past
// 100. Reserving a constant for them is exactly how this screen came to render 94
// columns into an 80-column terminal.
//
// One pass, in this order — the circularity (figures depend on the drop, the drop
// depends on the name, the name depends on the figures) is resolved here and not
// left to the caller:
//
//  1. compute with remaining; if the name column clears the floor, done;
//  2. otherwise recompute without remaining, and never put it back;
//  3. if the name is still under the floor, the floor wins and the row
//     overflows. That happens below boxChrome + 1 + budgetBarWidth + 1 + pctW +
//     2 + figures + budgetMinNameWidth — around 63 to 65 columns with realistic
//     amounts. Below it no split saves the row, and pretending otherwise would
//     mean a name column too narrow to read. Accepted and explicit, the same
//     trade-off the report table makes below its own floor.
func budgetLayout(lines []report.BudgetLine, width int) (nameW int, showRemaining bool) {
	if width <= 0 { // before the first WindowSizeMsg nothing is sized against it
		return budgetNameWidth, true
	}
	pctW := 0
	for _, l := range lines {
		pctW = max(pctW, lipgloss.Width(fmt.Sprintf("%.0f%%", math.Floor(l.PercentUsed))))
	}
	widestFigures := func(withRemaining bool) int {
		w := 0
		for _, l := range lines {
			w = max(w, lipgloss.Width(budgetFigures(l, withRemaining)))
		}
		return w
	}
	fixed := 1 + budgetBarWidth + 1 + pctW + 2
	usable := width - boxChrome

	if nameW = usable - fixed - widestFigures(true); nameW >= budgetMinNameWidth {
		return nameW, true
	}
	return max(usable-fixed-widestFigures(false), budgetMinNameWidth), false
}

func (bm budgetModel) view(th theme, width int) string {
	title := th.Title.Render("Budget burn-down")
	if len(bm.lines) == 0 {
		return title + "\n\n" + th.Box.Render("No budgets configured.")
	}
	nameW, showRemaining := budgetLayout(bm.lines, width)
	var rows strings.Builder
	for _, l := range bm.lines {
		rows.WriteString(fmt.Sprintf("%s %s  %s\n",
			cell(l.ListName, nameW), renderBudgetBar(th, l.PercentUsed), budgetFigures(l, showRemaining)))
	}
	body := th.Box.Render(strings.TrimRight(rows.String(), "\n"))
	return title + "\n\n" + body
}
