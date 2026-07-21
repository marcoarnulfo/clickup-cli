package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

type reportModel struct {
	r report.Report
}

func newReport(r report.Report) reportModel { return reportModel{r: r} }

// nextGroupBy cicla total -> task -> list -> day -> total.
func nextGroupBy(g string) string {
	switch g {
	case report.GroupByTotal:
		return report.GroupByTask
	case report.GroupByTask:
		return report.GroupByList
	case report.GroupByList:
		return report.GroupByDay
	default:
		return report.GroupByTotal
	}
}

func (m Model) updateReport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "g":
		g := nextGroupBy(m.report.GroupBy)
		m.report = report.Build(m.entries, g, ratesFromConfig(m.cfg), m.cfg.Currency, m.year, m.month)
		m.report.Scope = m.scope
		m.rep = newReport(m.report)
	case "m", "s":
		m.screen = screenHome
	case "r":
		m.screen = screenLoading
		return m, loadEntriesCmd(m.client, m.cfg.WorkspaceID, m.year, m.month, m.scope)
	case "e":
		m.export = newExport(m.report)
		m.screen = screenExport
	case "p":
		m.ratesScreen = newRates(m.entries, m.cfg)
		m.screen = screenRates
	}
	return m, nil
}

func (rm reportModel) view() string {
	r := rm.r
	title := styleTitle.Render(fmt.Sprintf("Report %04d-%02d — scope %s — raggruppo per %s",
		r.Year, int(r.Month), r.Scope, r.GroupBy))

	header := lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("%-32s %8s %10s %s", "Voce", "Ore", "Importo", "Val"))
	rows := header + "\n"
	for _, b := range r.Buckets {
		rows += fmt.Sprintf("%-32s %8.2f %10.2f %s\n",
			truncate(b.Label, 32), b.Hours, b.Amount, r.Currency)
	}

	total := styleOK.Render(fmt.Sprintf("%-32s %8.2f %10.2f %s",
		"TOTALE", r.TotalHours, r.TotalAmount, r.Currency))

	body := styleBox.Render(rows + total)
	help := styleHelp.Render("g: raggruppamento · e: esporta · p: tariffe · m/s: cambia mese/scope · r: ricarica · q: esci")

	if len(r.Buckets) == 0 {
		body = styleBox.Render("Nessuna ora tracciata in questo mese.")
	}
	return title + "\n\n" + body + "\n\n" + help
}

// truncate accorcia a n rune (non byte), per non spezzare caratteri UTF-8
// nei nomi di task con accenti o emoji.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
