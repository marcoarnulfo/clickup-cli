package tui

import (
	"fmt"
	"strings"

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
		start, end := m.currentRange()
		m.report = report.Build(m.visibleEntries(), g, pricingFromConfig(m.cfg), start, end, nil)
		m.report.Scope = m.scope
		m.rep = newReport(m.report, m.memberFilterNote()+m.filteredNote())
	case "m", "s":
		m.screen = screenHome
	case "r":
		m.screen = screenLoading
		return m, m.reloadEntriesCmd(screenReport)
	case "e":
		m.export = newExport(m.report)
		m.screen = screenExport
	case "p":
		m.ratesScreen = newRates(m.entries, m.cfg)
		m.screen = screenRates
	case "n":
		m.logScreen = newLog(m.entries, m.cfg)
		m.screen = screenLog
	case "f":
		missing := m.tasksMissingStatus()
		if len(missing) == 0 {
			m.assignStatuses()
			m.filtersScreen = newFilters(m.entries, m.filterLists, m.filterTags, m.filterStatuses)
			m.screen = screenFilters
			return m, nil
		}
		m.filtersScreen = filtersModel{loadingStatuses: true}
		m.screen = screenFilters
		if m.demo {
			return m, demoStatusEnrichCmd(m.entries)
		}
		return m, statusEnrichCmd(m.client, missing)
	}
	return m, nil
}

// applyReport rebuilds m.report from the visible entries over the current range,
// keeping the active grouping.
func (m *Model) applyReport() {
	g := m.report.GroupBy
	if g == "" {
		g = report.GroupByTotal
	}
	start, end := m.currentRange()
	m.report = report.Build(m.visibleEntries(), g, pricingFromConfig(m.cfg), start, end, nil)
	m.report.Scope = m.scope
	m.rep = newReport(m.report, m.memberFilterNote()+m.filteredNote())
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

func (rm reportModel) view() string {
	r := rm.r
	title := styleTitle.Render(fmt.Sprintf("Report %s — scope %s%s — grouped by %s",
		report.PeriodLabel(r.Start, r.End), r.Scope, rm.note, r.GroupBy))

	header := lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("%-32s %8s %8s %s", "Item", "Hours", "Billed", "Amount"))
	rows := header + "\n"
	for _, b := range r.Buckets {
		rows += fmt.Sprintf("%-32s %8.2f %8.2f %s\n",
			truncate(b.Label, 32), b.Hours, b.BilledHours, formatAmounts(b.Amounts, r.DefaultCurrency))
	}

	// Totals: one line when the report is single-currency, otherwise a TOTAL
	// hours line plus one authoritative subtotal line per currency (no FX).
	var total string
	if len(r.CurrencySubtotals) <= 1 {
		total = styleOK.Render(fmt.Sprintf("%-32s %8.2f %8.2f %.2f %s",
			"TOTAL", r.TotalHours, r.BilledHours, r.TotalAmount, r.DefaultCurrency))
	} else {
		total = styleOK.Render(fmt.Sprintf("%-32s %8.2f %8.2f", "TOTAL", r.TotalHours, r.BilledHours))
		for _, s := range r.CurrencySubtotals {
			total += "\n" + styleOK.Render(fmt.Sprintf("%-32s %8.2f %8.2f %.2f %s",
				"  subtotal "+s.Currency, s.Hours, s.BilledHours, s.Amount, s.Currency))
		}
	}
	if r.NonBillableHours > 0 {
		total += "\n" + styleHelp.Render(fmt.Sprintf("%-32s %8.2f", "  non-billable", r.NonBillableHours))
	}

	body := styleBox.Render(rows + total)
	help := styleHelp.Render("g: grouping · e: export · p: rates · n: log hours · f: filters · m/s: change range/scope · r: reload · q: quit")

	if len(r.Buckets) == 0 {
		body = styleBox.Render("No hours to show.")
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
