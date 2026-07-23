package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// budgetModel renders the budget burn-down view (#64): one text progress bar
// per report.BudgetLine, most-burned first (BudgetLines' own sort order).
type budgetModel struct {
	lines []report.BudgetLine
}

func newBudget(lines []report.BudgetLine) budgetModel { return budgetModel{lines: lines} }

func (m Model) updateBudget(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "b":
		m.screen = screenReport
	}
	return m, nil
}

// budgetBarWidth is the text progress bar's width in characters, not counting
// the surrounding brackets.
const budgetBarWidth = 20

// renderBudgetBar renders a percent-used value as a fixed-width text bar,
// e.g. "[############--------] 60%". The bar's fill is clamped to [0, 100]
// (a list can run over budget, but the bar itself can't render past full);
// the percentage in the label is shown unclamped so an over-100% burn is
// still visible in the number.
func renderBudgetBar(percentUsed float64) string {
	fillPct := percentUsed
	if fillPct < 0 {
		fillPct = 0
	}
	if fillPct > 100 {
		fillPct = 100
	}
	filled := int(fillPct / 100 * budgetBarWidth)
	bar := strings.Repeat("#", filled) + strings.Repeat("-", budgetBarWidth-filled)
	return fmt.Sprintf("[%s] %.0f%%", bar, percentUsed)
}

func (bm budgetModel) view() string {
	title := styleTitle.Render("Budget burn-down")
	help := styleHelp.Render("Esc/b: back to report · q: quit")
	if len(bm.lines) == 0 {
		return title + "\n\n" + styleBox.Render("No budgets configured.") + "\n\n" + help
	}
	var rows strings.Builder
	for _, l := range bm.lines {
		rows.WriteString(fmt.Sprintf("%-24s %s  %.2f / %.2f %s (remaining %.2f)\n",
			truncate(l.ListName, 24), renderBudgetBar(l.PercentUsed), l.Billed, l.Budget, l.Currency, l.Remaining))
	}
	body := styleBox.Render(strings.TrimRight(rows.String(), "\n"))
	return title + "\n\n" + body + "\n\n" + help
}
