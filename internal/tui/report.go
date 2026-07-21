package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

type reportModel struct {
	r    report.Report
	note string
}

func newReport(r report.Report, note string) reportModel { return reportModel{r: r, note: note} }

// nextGroupBy cycles total -> task -> list -> day -> [member] -> total.
// The member grouping is only offered for the team scope.
func nextGroupBy(g, scope string) string {
	switch g {
	case report.GroupByTotal:
		return report.GroupByTask
	case report.GroupByTask:
		return report.GroupByList
	case report.GroupByList:
		return report.GroupByDay
	case report.GroupByDay:
		if scope == "team" {
			return report.GroupByMember
		}
		return report.GroupByTotal
	default: // includes GroupByMember
		return report.GroupByTotal
	}
}

// memberFilterNote returns " (k/n members)" when the team scope has a partial
// member selection, else "".
func (m Model) memberFilterNote() string {
	if m.scope != "team" || len(m.teamMembers) == 0 {
		return ""
	}
	k := len(m.selectedAssignees())
	n := len(m.teamMembers)
	if k == 0 || k == n {
		return ""
	}
	return fmt.Sprintf(" (%d/%d members)", k, n)
}

func (m Model) updateReport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "g":
		g := nextGroupBy(m.report.GroupBy, m.scope)
		m.report = report.Build(m.entries, g, ratesFromConfig(m.cfg), m.cfg.Currency, m.year, m.month)
		m.report.Scope = m.scope
		m.rep = newReport(m.report, m.memberFilterNote())
	case "m", "s":
		m.screen = screenHome
	case "r":
		m.screen = screenLoading
		return m, m.reloadEntriesCmd()
	case "e":
		m.export = newExport(m.report)
		m.screen = screenExport
	case "p":
		m.ratesScreen = newRates(m.entries, m.cfg)
		m.screen = screenRates
	case "n":
		m.logScreen = newLog(m.entries, m.cfg)
		m.screen = screenLog
	}
	return m, nil
}

func (rm reportModel) view() string {
	r := rm.r
	title := styleTitle.Render(fmt.Sprintf("Report %04d-%02d — scope %s%s — grouped by %s",
		r.Year, int(r.Month), r.Scope, rm.note, r.GroupBy))

	header := lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("%-32s %8s %10s %s", "Item", "Hours", "Amount", "Cur"))
	rows := header + "\n"
	for _, b := range r.Buckets {
		rows += fmt.Sprintf("%-32s %8.2f %10.2f %s\n",
			truncate(b.Label, 32), b.Hours, b.Amount, r.Currency)
	}

	total := styleOK.Render(fmt.Sprintf("%-32s %8.2f %10.2f %s",
		"TOTAL", r.TotalHours, r.TotalAmount, r.Currency))

	body := styleBox.Render(rows + total)
	help := styleHelp.Render("g: grouping · e: export · p: rates · n: log hours · m/s: change month/scope · r: reload · q: quit")

	if len(r.Buckets) == 0 {
		body = styleBox.Render("No hours tracked this month.")
	}
	return title + "\n\n" + body + "\n\n" + help
}

// truncate shortens to n runes (not bytes), to avoid breaking UTF-8 characters
// in task names with accents or emoji.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
