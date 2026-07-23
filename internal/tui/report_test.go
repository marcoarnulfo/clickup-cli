package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func TestNextGroupByTeamIncludesMember(t *testing.T) {
	if got := nextGroupBy(report.GroupByDay, "team"); got != report.GroupByMember {
		t.Errorf("team: day -> %q, want member", got)
	}
	if got := nextGroupBy(report.GroupByMember, "team"); got != report.GroupByTotal {
		t.Errorf("team: member -> %q, want total", got)
	}
}

func TestNextGroupByMeSkipsMember(t *testing.T) {
	if got := nextGroupBy(report.GroupByDay, "me"); got != report.GroupByTotal {
		t.Errorf("me: day -> %q, want total", got)
	}
}

// TestReportCycleGroupByTeamViaUpdate drives the 'g' key through Update() to
// verify the team cycle reaches the member grouping.
func TestReportCycleGroupByTeamViaUpdate(t *testing.T) {
	m := Model{scope: "team", screen: screenReport, now: time.Now}
	m.report = report.Report{GroupBy: report.GroupByDay}
	m.rep = newReport(m.report, "")
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m = u.(Model)
	if m.report.GroupBy != report.GroupByMember {
		t.Errorf("team g from day -> %q, want member", m.report.GroupBy)
	}
}

// #57: pressing 'g' with an unparseable billing.rounding.increment must
// route to screenError instead of cycling the grouping with stale/wrong
// pricing.
func TestReportCycleGroupByWithBadRoundingRoutesToErrorScreen(t *testing.T) {
	cfg := config.Config{}
	cfg.Billing.Rounding.Increment = "not-a-duration"
	m := Model{cfg: cfg, scope: "me", screen: screenReport, now: time.Now}
	m.report = report.Report{GroupBy: report.GroupByTotal}
	m.rep = newReport(m.report, "")
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	mm := u.(Model)
	if mm.screen != screenError {
		t.Fatalf("screen = %v, want screenError", mm.screen)
	}
	if mm.err == nil || !strings.Contains(mm.err.Error(), "not-a-duration") {
		t.Fatalf("err = %v, want it to name the offending increment", mm.err)
	}
}

// TestReportViewRendersPerCurrencyAmounts covers the multi-currency view: each
// bucket shows its own amounts and the totals fall back to the authoritative
// per-currency subtotals instead of a single (meaningless) cross-currency sum.
func TestReportViewRendersPerCurrencyAmounts(t *testing.T) {
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	r := report.Report{
		Start: start, End: start.AddDate(0, 1, 0), Scope: "me", GroupBy: report.GroupByList,
		DefaultCurrency: "EUR",
		Buckets: []report.Bucket{
			{Label: "Alpha", Key: "A", Hours: 3, BillableHours: 2, BilledHours: 2,
				Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 200}}},
			{Label: "Beta", Key: "B", Hours: 1, BillableHours: 1, BilledHours: 1,
				Amounts: []report.CurrencyAmount{{Currency: "USD", Amount: 100}}},
		},
		CurrencySubtotals: []report.CurrencySubtotal{
			{Currency: "EUR", Hours: 3, BillableHours: 2, BilledHours: 2, Amount: 200},
			{Currency: "USD", Hours: 1, BillableHours: 1, BilledHours: 1, Amount: 100},
		},
		TotalHours: 4, BillableHours: 3, NonBillableHours: 1, BilledHours: 3,
	}
	out := newReport(r, "").view()
	for _, want := range []string{"200.00 EUR", "100.00 USD", "subtotal EUR", "subtotal USD", "non-billable"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q; got:\n%s", want, out)
		}
	}
}

func TestReportViewSingleCurrencyShowsOneTotal(t *testing.T) {
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	r := report.Report{
		Start: start, End: start.AddDate(0, 1, 0), Scope: "me", GroupBy: report.GroupByTotal,
		DefaultCurrency: "EUR",
		Buckets: []report.Bucket{{Label: "Total", Key: "total", Hours: 2, BillableHours: 2, BilledHours: 2,
			Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 100}}}},
		CurrencySubtotals: []report.CurrencySubtotal{{Currency: "EUR", Hours: 2, BillableHours: 2, BilledHours: 2, Amount: 100}},
		TotalHours:        2, BillableHours: 2, BilledHours: 2, TotalAmount: 100,
	}
	out := newReport(r, "").view()
	if strings.Contains(out, "subtotal") {
		t.Errorf("single-currency report should not list subtotals; got:\n%s", out)
	}
	if !strings.Contains(out, "100.00 EUR") {
		t.Errorf("view missing the total amount; got:\n%s", out)
	}
}

// TestReportViewShowsSummaryAndBillableSplit drives Update to build a report
// from a mixed-currency fixture and asserts the rendered view carries the
// shared export.SummaryLine ("N entries · Xh · amounts"), an explicit
// billable/non-billable split, and per-currency subtotals — all formatted
// with FormatHours/%.2f, never a second summary formatter.
func TestReportViewShowsSummaryAndBillableSplit(t *testing.T) {
	cfg := config.Config{Token: "t", WorkspaceID: "1", Rate: 10, Currency: "EUR"}
	cfg.Billing.Currencies = map[string]string{"eur-list": "EUR", "usd-list": "USD"}
	m := New(cfg)
	m.year, m.month = 2026, 7
	start := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	entries := []report.TimeEntry{
		{ID: "1", ListID: "eur-list", ListName: "Alpha", Start: start,
			Duration: 2 * time.Hour, Billable: true},
		{ID: "2", ListID: "usd-list", ListName: "Beta", Start: start.Add(time.Hour),
			Duration: 1 * time.Hour, Billable: true},
		{ID: "3", ListID: "eur-list", ListName: "Alpha", Start: start.Add(2 * time.Hour),
			Duration: 30 * time.Minute, Billable: false},
	}
	m.cfg.Rates = map[string]float64{"eur-list": 100, "usd-list": 90}
	updated, _ := m.Update(entriesMsg{entries: entries})
	mm := updated.(Model)
	if mm.screen != screenReport {
		t.Fatalf("screen = %v, want screenReport (err: %v)", mm.screen, mm.err)
	}
	// billable: 2h @ 100 EUR/h = 200 EUR, 1h @ 90 USD/h = 90 USD.
	out := mm.rep.view()
	for _, want := range []string{
		"2 entries · 3.00h · 200.00 EUR, 90.00 USD", // export.SummaryLine (Lines counts only billable units)
		"subtotal EUR", "200.00 EUR",
		"subtotal USD", "90.00 USD",
		// The explicit split line, as one contiguous substring: billable 3h
		// (2h EUR + 1h USD) and non-billable 0.5h. A bare "non-billable"
		// check (which "non-billable" itself also satisfies for "billable")
		// would already pass on the old rendering that only ever showed a
		// conditional non-billable line — this pins the new segment.
		"billable 3.00h · non-billable 0.50h",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q; got:\n%s", want, out)
		}
	}
}

func TestMemberFilterNotePartial(t *testing.T) {
	m := Model{
		scope:           "team",
		teamMembers:     make([]clickup.Member, 3), // 3 members total
		selectedMembers: map[int]bool{1: true, 2: true},
	}
	if got := m.memberFilterNote(); got != " (2/3 members)" {
		t.Errorf("memberFilterNote = %q, want ' (2/3 members)'", got)
	}
}
