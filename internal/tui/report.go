package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/duration"
	"github.com/marcoarnulfo/clickup-cli/internal/export"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
	"github.com/marcoarnulfo/clickup-cli/internal/service"
)

type reportModel struct {
	r    report.Report
	note string
}

func newReport(r report.Report, note string) reportModel { return reportModel{r: r, note: note} }

// nextGroupBy cycles total -> task -> list -> day -> tag -> [member] -> total.
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
		return report.GroupByTag
	case report.GroupByTag:
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
	k := keysFor(m)
	switch {
	case key.Matches(msg, k.Back):
		m = m.pop()
	case key.Matches(msg, k.GroupBy):
		g := nextGroupBy(m.report.GroupBy, m.scope)
		if _, ok := m.locOrErr(); !ok {
			return m, nil
		}
		start, end := m.currentRange()
		if p, ok := m.pricingOrErr(); ok {
			m.report = report.Build(m.visibleEntries(), g, p, start, end, m.loc)
			m.report.Scope = m.scope
			m.rep = newReport(m.report, m.memberFilterNote()+m.filteredNote())
		}
	case key.Matches(msg, k.ChangeRange):
		m = m.pop()
	case key.Matches(msg, k.Reload):
		m = m.replace(screenLoading)
		return m, m.reloadEntriesCmd(screenReport)
	case key.Matches(msg, k.Export):
		m.export = newExport(m.report)
		m = m.goTo(screenExport)
	case key.Matches(msg, k.Rates):
		m.ratesScreen = newRates(m.entries, m.cfg)
		m = m.goTo(screenRates)
	case key.Matches(msg, k.LogHours):
		m.logScreen = newLog(m.entries, m.cfg)
		m = m.goTo(screenLog)
	case key.Matches(msg, k.Filters):
		missing := m.tasksMissingStatus()
		if len(missing) == 0 {
			m.assignStatuses()
			m.filtersScreen = newFilters(m.entries, m.filterLists, m.filterTags, m.filterStatuses, m.filterBillable)
			m = m.goTo(screenFilters)
			return m, nil
		}
		m.filtersScreen = filtersModel{loadingStatuses: true}
		m = m.goTo(screenFilters)
		if m.demo {
			return m, demoStatusEnrichCmd(m.entries)
		}
		return m, statusEnrichCmd(m.client, missing)
	case key.Matches(msg, k.Budget):
		if !m.openBudgetView() {
			return m, nil
		}
	case key.Matches(msg, k.OpenEntries):
		m = m.openEntries()
		var cmd tea.Cmd
		if m.userID == 0 {
			cmd = m.currentUserCmd() // lazy retry so gating can enable
		}
		return m, cmd
	}
	return m, nil
}

// openBudgetView builds the budget burn-down view (#64) and opens
// screenBudget. It computes billedByList from a report grouped by list
// independently of the currently selected grouping — report.BudgetLines
// needs Bucket.Key to be the listID, which only GroupByList guarantees.
// Budgets/currencies come from the same config the rest of the report uses
// (service.BudgetsFromConfig / the resolved Pricing); no I/O is involved, so
// there's no demo-mode branch to guard. It returns false when the config's
// pricing rules or timezone failed to parse, in which case locOrErr/
// pricingOrErr have already routed the model to screenError.
func (m *Model) openBudgetView() bool {
	if _, ok := m.locOrErr(); !ok {
		return false
	}
	p, ok := m.pricingOrErr()
	if !ok {
		return false
	}
	start, end := m.currentRange()
	perList := report.Build(m.visibleEntries(), report.GroupByList, p, start, end, m.loc)
	billed := billedByListFromBuckets(perList.Buckets, p)
	budgets := service.BudgetsFromConfig(m.cfg)
	lines := report.BudgetLines(billed, budgets, p, listNamesFromBuckets(perList.Buckets))
	m.budgetScreen = newBudget(lines)
	*m = m.goTo(screenBudget)
	return true
}

// billedByListFromBuckets derives report.BudgetLines' billedByList input from
// a report built with report.GroupByList, where Bucket.Key is the listID. A
// bucket may in principle carry more than one currency (Bucket.Amounts is a
// per-currency slice); this picks the amount matching the list's own resolved
// currency (resolved through the shared report.Pricing.CurrencyFor) and never
// sums across currencies (see the task's binding note on budget inputs).
func billedByListFromBuckets(buckets []report.Bucket, p report.Pricing) map[string]float64 {
	out := make(map[string]float64, len(buckets))
	for _, b := range buckets {
		cur := p.CurrencyFor(b.Key)
		for _, a := range b.Amounts {
			if a.Currency == cur {
				out[b.Key] = a.Amount
				break
			}
		}
	}
	return out
}

// listNamesFromBuckets derives report.BudgetLines' listNames input from a
// report built with report.GroupByList: Bucket.Key is the listID, Bucket.Label
// the list name.
func listNamesFromBuckets(buckets []report.Bucket) map[string]string {
	out := make(map[string]string, len(buckets))
	for _, b := range buckets {
		out[b.Key] = b.Label
	}
	return out
}

// applyReport rebuilds m.report from the visible entries over the current
// range, keeping the active grouping. It returns false when the config's
// pricing rules failed to parse, in which case pricingOrErr has already
// routed the model to screenError and the caller must not overwrite that.
func (m *Model) applyReport() bool {
	g := m.report.GroupBy
	if g == "" {
		g = report.GroupByTotal
	}
	if _, ok := m.locOrErr(); !ok {
		return false
	}
	p, ok := m.pricingOrErr()
	if !ok {
		return false
	}
	start, end := m.currentRange()
	m.report = report.Build(m.visibleEntries(), g, p, start, end, m.loc)
	m.report.Scope = m.scope
	m.rep = newReport(m.report, m.memberFilterNote()+m.filteredNote())
	return true
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

// hoursOf renders hours the same way export.SummaryLine does, via
// duration.FormatHours — one hours formatter shared across TUI and export.
func hoursOf(h float64) string {
	return duration.FormatHours(time.Duration(h * float64(time.Hour)))
}

func (rm reportModel) view(th theme, width int) string {
	r := rm.r
	// Timezone is surfaced here (#83): with no configured `timezone` it reads
	// "Local" (time.Local.String()), not a portable IANA name — accepted, see
	// the task's binding amendments. Users wanting a stable zone name across
	// machines should set `timezone` in the config.
	title := th.Title.Render(fmt.Sprintf("Report %s — scope %s%s — grouped by %s — tz %s",
		report.PeriodLabel(r.Start, r.End), r.Scope, rm.note, r.GroupBy, r.Timezone))
	summary := th.Accent.Render(export.SummaryLine(r))

	var body string
	if len(r.Buckets) == 0 {
		body = th.Box.Render("No hours to show.")
	} else {
		body = reportTable(th, r, width)
	}
	// The billable split is a note, not a row of data: inside the table it
	// would take the zebra stripe and be split across columns.
	split := th.Help.Render(fmt.Sprintf("  billable %s · non-billable %s",
		hoursOf(r.BillableHours), hoursOf(r.NonBillableHours)))
	return title + "\n\n" + summary + "\n\n" + body + "\n" + split
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
