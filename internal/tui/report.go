package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/marcoarnulfo/clickup-cli/internal/duration"
	"github.com/marcoarnulfo/clickup-cli/internal/export"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
	"github.com/marcoarnulfo/clickup-cli/internal/service"
)

type reportModel struct {
	r     report.Report
	note  string
	daily []float64 // one entry per day of the range; see report.DailyHours
}

func newReport(r report.Report, note string, daily []float64) reportModel {
	return reportModel{r: r, note: note, daily: daily}
}

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

// dailySeries is the per-day hours of the visible entries over the loaded
// range. It lives on the Model because reportModel has no entries — the report
// is a rendering of an already-aggregated value.
func (m Model) dailySeries() []float64 {
	start, end := m.activeRange()
	return report.DailyHours(m.visibleEntries(), start, end, m.loc)
}

func (m Model) updateReport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := keysFor(m)
	switch {
	case key.Matches(msg, k.Back):
		m = m.pop()
	case key.Matches(msg, k.GroupBy):
		g := nextGroupBy(m.report.GroupBy, m.scope)
		m.rebuildReport(g)
	case key.Matches(msg, k.ChangeRange):
		m = m.pop()
	case key.Matches(msg, k.Reload):
		m = m.replace(screenLoading)
		return m, m.reloadEntriesCmd(screenReport)
	case key.Matches(msg, k.Export):
		m = m.openExport()
	case key.Matches(msg, k.Rates):
		m = m.openRates()
	case key.Matches(msg, k.LogHours):
		m = m.openLog()
	case key.Matches(msg, k.Filters):
		return m.openFilters()
	case key.Matches(msg, k.Budget):
		if !m.openBudgetView() {
			return m, nil
		}
	case key.Matches(msg, k.OpenEntries):
		return m.openEntries()
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
	start, end := m.activeRange()
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

// rebuildReport rebuilds m.report and m.rep for the given grouping, over the
// visible entries and the pinned range. It returns false when the timezone or
// the pricing rules failed to parse, in which case the caller must not overwrite
// the error screen those helpers already routed to. It is the single rebuild
// used by applyReport, by the entries handler and by the grouping and rates
// changes, which each carried their own copy.
func (m *Model) rebuildReport(groupBy string) bool {
	if _, ok := m.locOrErr(); !ok {
		return false
	}
	p, ok := m.pricingOrErr()
	if !ok {
		return false
	}
	start, end := m.activeRange()
	m.report = report.Build(m.visibleEntries(), groupBy, p, start, end, m.loc)
	m.report.Scope = m.scope
	m.rep = newReport(m.report, m.memberFilterNote()+m.filteredNote(), m.dailySeries())
	return true
}

// applyReport rebuilds m.report from the visible entries over the loaded
// range (m.activeRange, #28), keeping the active grouping. It returns false
// when the config's pricing rules failed to parse, in which case
// pricingOrErr has already routed the model to screenError and the caller
// must not overwrite that.
func (m *Model) applyReport() bool {
	g := m.report.GroupBy
	if g == "" {
		g = report.GroupByTotal
	}
	return m.rebuildReport(g)
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
	out := title + "\n\n" + summary + "\n\n"
	if line := rm.sparkView(th, width); line != "" {
		out += line + "\n\n"
	}
	return out + body + "\n" + split
}

// sparkLabel prefixes the sparkline. It leads rather than trails so the chart
// always reads left-to-right from a fixed origin: DailyHours zero-fills the
// whole range and a zero renders as a space (see sparkline.go), so a label
// appended after the chart drifts as many columns as there are idle days at
// the end of the range — visibly so for the current month, viewed partway
// through, which is most of the trailing days. Its width plus a margin is
// what sparkView reserves for the chart.
const sparkLabel = "hours/day "

// sparkView renders the per-day sparkline, or "" when there is nothing worth
// drawing: a range of one day is a single cell, and a report with no buckets
// already says so in its body.
func (rm reportModel) sparkView(th theme, width int) string {
	if len(rm.daily) < 2 || len(rm.r.Buckets) == 0 {
		return ""
	}
	cells := 31 // the longest month, used until the terminal reports its width
	if width > 0 {
		cells = max(1, width-lipgloss.Width(sparkLabel)-2)
	}
	// Trailing idle days render as trailing spaces (see sparkline.go); with
	// the label leading rather than trailing, those spaces would otherwise
	// dangle at the end of the line for no reader to see. The trim runs on
	// the label and chart ASSEMBLED, not on the chart alone: when every day
	// in the range is idle, sparkline itself is all spaces, and trimming only
	// the chart would leave the label's OWN trailing separator space dangling
	// instead ("hours/day" followed by one space and then nothing — the exact
	// defect this trim exists to prevent, just moved one segment to the
	// left). A chart with no ink still renders (see the doc comment above:
	// only "no buckets" and "fewer than two days" skip the line entirely) —
	// zero hours on every day of the range is itself the information, the
	// same way one idle day among several already renders as a blank cell
	// rather than vanishing.
	raw := strings.TrimRight(sparkLabel+sparkline(rm.daily, cells), " ")
	cut := min(len(sparkLabel), len(raw))
	label, chart := raw[:cut], raw[cut:]
	return th.Help.Render(label) + th.Accent.Render(chart)
}
