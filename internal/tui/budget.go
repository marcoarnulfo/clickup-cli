package tui

import (
	"fmt"
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
	return fmt.Sprintf("%s %.0f%%", bar, percentUsed)
}

func (bm budgetModel) view(th theme) string {
	title := th.Title.Render("Budget burn-down")
	if len(bm.lines) == 0 {
		return title + "\n\n" + th.Box.Render("No budgets configured.")
	}
	var rows strings.Builder
	for _, l := range bm.lines {
		rows.WriteString(fmt.Sprintf("%-24s %s  %.2f / %.2f %s (remaining %.2f)\n",
			truncate(l.ListName, 24), renderBudgetBar(th, l.PercentUsed), l.Billed, l.Budget, l.Currency, l.Remaining))
	}
	body := th.Box.Render(strings.TrimRight(rows.String(), "\n"))
	return title + "\n\n" + body
}
