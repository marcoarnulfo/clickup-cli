package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
	"github.com/marcoarnulfo/clickup-cli/internal/themes"
)

// TestReportKeyLabels pins the exact label set report.go's updateReport
// accepts today (every case label, verbatim), plus q.
func TestReportKeyLabels(t *testing.T) {
	t.Parallel()
	m := Model{screen: screenReport}
	want := []string{"?", "b", "ctrl+c", "ctrl+p", "e", "esc", "f", "g", "m", "n", "p", "q", "r", "s", "v"}
	if got := enabledLabels(keysFor(m)); !slices.Equal(got, want) {
		t.Errorf("report labels = %v, want %v", got, want)
	}
}

// #53: the group cycle grows a "tag" stop between "day" and the team-only
// "member" stop, for both scopes.
func TestNextGroupByCycleIncludesTag(t *testing.T) {
	if got := nextGroupBy(report.GroupByDay, "me"); got != report.GroupByTag {
		t.Errorf("me: day -> %q, want tag", got)
	}
	if got := nextGroupBy(report.GroupByDay, "team"); got != report.GroupByTag {
		t.Errorf("team: day -> %q, want tag", got)
	}
}

func TestNextGroupByTeamIncludesMember(t *testing.T) {
	if got := nextGroupBy(report.GroupByTag, "team"); got != report.GroupByMember {
		t.Errorf("team: tag -> %q, want member", got)
	}
	if got := nextGroupBy(report.GroupByMember, "team"); got != report.GroupByTotal {
		t.Errorf("team: member -> %q, want total", got)
	}
}

func TestNextGroupByMeSkipsMember(t *testing.T) {
	if got := nextGroupBy(report.GroupByTag, "me"); got != report.GroupByTotal {
		t.Errorf("me: tag -> %q, want total", got)
	}
}

// TestReportCycleGroupByTeamViaUpdate drives the 'g' key through Update() to
// verify the team cycle reaches the tag grouping right after day.
func TestReportCycleGroupByTeamViaUpdate(t *testing.T) {
	m := Model{scope: "team", screen: screenReport, now: time.Now}
	m.report = report.Report{GroupBy: report.GroupByDay}
	m.rep = newReport(m.report, "", nil)
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m = u.(Model)
	if m.report.GroupBy != report.GroupByTag {
		t.Errorf("team g from day -> %q, want tag", m.report.GroupBy)
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
	m.rep = newReport(m.report, "", nil)
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
	out := newReport(r, "", nil).view(testTheme(true), 80)
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
	out := newReport(r, "", nil).view(testTheme(true), 80)
	if strings.Contains(out, "subtotal") {
		t.Errorf("single-currency report should not list subtotals; got:\n%s", out)
	}
	if !strings.Contains(out, "100.00 EUR") {
		t.Errorf("view missing the total amount; got:\n%s", out)
	}
}

// TestReportViewEmptyReportShowsNoHoursAndSplit pins the empty-report shape:
// with no buckets, reportTable is never called (the table would have nothing
// to size against) and "No hours to show." renders instead. The billable
// split line, previously folded inside the box and so absent for an empty
// report, is now always rendered below it — nothing else in the suite exercises
// the zero-bucket path, so this is the only place that would catch either
// regressing.
func TestReportViewEmptyReportShowsNoHoursAndSplit(t *testing.T) {
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	r := report.Report{
		Start: start, End: start.AddDate(0, 1, 0), Scope: "me", GroupBy: report.GroupByTotal,
		DefaultCurrency: "EUR",
	}
	out := newReport(r, "", nil).view(testTheme(true), 80)
	if !strings.Contains(out, "No hours to show.") {
		t.Errorf("empty report view missing \"No hours to show.\"; got:\n%s", out)
	}
	if !strings.Contains(out, "billable 0.00h · non-billable 0.00h") {
		t.Errorf("empty report view missing the billable split line; got:\n%s", out)
	}
}

// TestReportViewShowsSummaryAndBillableSplit drives Update to build a report
// from a mixed-currency fixture and asserts the rendered view carries the
// shared export.SummaryLine ("N billing lines · Xh · amounts"), an explicit
// billable/non-billable split, and per-currency subtotals — all formatted
// with FormatHours/%.2f, never a second summary formatter.
func TestReportViewShowsSummaryAndBillableSplit(t *testing.T) {
	cfg := config.Config{Token: "t", WorkspaceID: "1", Rate: 10, Currency: "EUR"}
	cfg.Billing.Currencies = map[string]string{"eur-list": "EUR", "usd-list": "USD"}
	m := New(cfg, themes.Default())
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
	out := mm.rep.view(testTheme(true), 80)
	for _, want := range []string{
		"2 billing lines · 3.00h · 200.00 EUR, 90.00 USD", // export.SummaryLine (Lines counts only billable units)
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

// #64: billedByListFromBuckets must pick the amount matching the list's own
// currency and never sum across currencies, even if a bucket somehow carries
// more than one (see the binding note on Budget inputs).
func TestBilledByListFromBucketsPicksListCurrencyNotSum(t *testing.T) {
	buckets := []report.Bucket{
		{Key: "list-1", Label: "Website", Amounts: []report.CurrencyAmount{
			{Currency: "EUR", Amount: 100},
			{Currency: "USD", Amount: 40},
		}},
	}
	p := report.Pricing{Currencies: map[string]string{"list-1": "EUR"}, DefaultCurrency: "USD"}
	got := billedByListFromBuckets(buckets, p)
	if got["list-1"] != 100 {
		t.Errorf("billedByList[list-1] = %v, want 100 (EUR only, not summed with USD)", got["list-1"])
	}
}

// #64: pressing 'b' on the Report screen builds a per-list report
// independently of the active grouping and opens the budget burn-down view.
func TestReportBOpensBudgetView(t *testing.T) {
	cfg := config.Config{Token: "t", WorkspaceID: "1", Currency: "EUR", Rate: 50}
	cfg.Billing.Budgets = map[string]float64{"list-1": 100}
	cfg.Billing.Currencies = map[string]string{"list-1": "EUR"}
	m := New(cfg, themes.Default())
	m.screen = screenReport
	m.entries = []report.TimeEntry{
		{ListID: "list-1", ListName: "Website", Start: time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC),
			Duration: 2 * time.Hour, Billable: true},
	}
	m.year, m.month = 2026, time.July

	u, _ := m.updateReport(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	mm := u.(Model)
	if mm.screen != screenBudget {
		t.Fatalf("b should open budget view, got %v (err %v)", mm.screen, mm.err)
	}
	if len(mm.budgetScreen.lines) != 1 {
		t.Fatalf("expected 1 budget line, got %d", len(mm.budgetScreen.lines))
	}
	if mm.budgetScreen.lines[0].ListID != "list-1" {
		t.Errorf("budget line ListID = %q, want list-1", mm.budgetScreen.lines[0].ListID)
	}
	if mm.budgetScreen.lines[0].PercentUsed != 100 {
		t.Errorf("budget line PercentUsed = %v, want 100 (2h @ 50/h == the 100 budget)", mm.budgetScreen.lines[0].PercentUsed)
	}
}

// #59 Task 3 step 3: m, s, r and e have no test that would fail if any of
// them went mute.
func TestReportMReturnsHome(t *testing.T) {
	m := newTestModelOnReport()
	next, _ := m.Update(keyMsg("m"))
	if got := next.(Model).screen; got != screenHome {
		t.Errorf("m from report -> %v, want screenHome", got)
	}
}

func TestReportSReturnsHome(t *testing.T) {
	m := newTestModelOnReport()
	next, _ := m.Update(keyMsg("s"))
	if got := next.(Model).screen; got != screenHome {
		t.Errorf("s from report -> %v, want screenHome", got)
	}
}

func TestReportRReloadsEntries(t *testing.T) {
	m := newTestModelOnReport()
	next, cmd := m.Update(keyMsg("r"))
	if got := next.(Model).screen; got != screenLoading {
		t.Errorf("r from report -> %v, want screenLoading", got)
	}
	if cmd == nil {
		t.Error("r from report should return a reload command")
	}
}

func TestReportEOpensExport(t *testing.T) {
	m := newTestModelOnReport()
	next, _ := m.Update(keyMsg("e"))
	if got := next.(Model).screen; got != screenExport {
		t.Errorf("e from report -> %v, want screenExport", got)
	}
}

// TestReportEscReturnsHome: Report had no esc site for Task 5's classification
// to migrate (nothing to pop from), so it's added here as the tranche's one
// intended behavior change. esc pops to Report's parent, same as m/s.
func TestReportEscReturnsHome(t *testing.T) {
	m := newTestModelOnReport() // Task 5 seeded its nav with [screenHome]
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if got := next.(Model).screen; got != screenHome {
		t.Errorf("esc from report -> %v, want screenHome", got)
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

// TestSparkViewLabelLeadsWithTrailingZeroDays pins the fix for the label
// detaching from the chart: the series ends in three idle days (zero-fills,
// rendered as spaces — see sparkline.go), the shape of the default view (the
// current month, viewed partway through) that goldenDaily() never exercised
// because its own series ends non-zero. Before this fix, the label was
// appended after the chart and the trailing spaces were never trimmed, so it
// drifted as many columns as there were trailing idle days — visibly so in
// the shipped docs/demo.gif. With the label leading and the trailing spaces
// trimmed, it now sits immediately before the first cell every time.
func TestSparkViewLabelLeadsWithTrailingZeroDays(t *testing.T) {
	t.Parallel()
	rm := newReport(report.Report{Buckets: []report.Bucket{{Label: "x"}}}, "",
		[]float64{3.5, 0, 1.25, 4, 0, 0, 0})
	if got, want := rm.sparkView(testTheme(true), 0), "hours/day ▇ ▃█"; got != want {
		t.Errorf("sparkView = %q, want %q", got, want)
	}
}

// TestSparkViewAllZeroSeriesHasNoTrailingWhitespace covers a series with
// buckets but zero hours on every day of the range: sparkline itself renders
// as all spaces (no ink anywhere), and the old fix only trimmed the chart, so
// the label's own trailing separator space ("hours/day ") was left dangling
// with nothing after it — the same defect the trim exists to prevent, just
// moved one segment left. The line still renders (a chart with no ink is
// itself the information, same as one idle day among several rendering as a
// blank cell rather than vanishing); it must simply carry no trailing space.
func TestSparkViewAllZeroSeriesHasNoTrailingWhitespace(t *testing.T) {
	t.Parallel()
	rm := newReport(report.Report{Buckets: []report.Bucket{{Label: "x"}}}, "",
		[]float64{0, 0, 0, 0, 0, 0, 0})
	got := rm.sparkView(testTheme(true), 0)
	if got != strings.TrimRight(got, " ") {
		t.Errorf("sparkView = %q, has trailing whitespace", got)
	}
	if got != "hours/day" {
		t.Errorf("sparkView = %q, want the label alone with no separator space", got)
	}
}

// TestTruncateEmptyStringAlwaysEmpty covers the deliberately empty Amount
// cell in the multi-currency TOTAL row (see reportRows): whatever cols
// shrinks to, there is nothing to mark as cut, so truncateWidth must never
// turn an empty cell into a lone ellipsis.
func TestTruncateEmptyStringAlwaysEmpty(t *testing.T) {
	t.Parallel()
	for _, cols := range []int{0, 1, 5} {
		if got := truncateWidth("", cols); got != "" {
			t.Errorf("truncateWidth(%q, %d) = %q, want %q", "", cols, got, "")
		}
	}
}

// The report table became width-aware in #66 and the budget screen in #136, but
// the header line above them never did: measured, it renders 68 columns on both
// a 40- and a 60-column terminal (the member note is what pushes it past the 54
// it measures without one). Line 1 is the title's own MarginBottom padding and
// overflows with it, which is why the text is truncated BEFORE th.Title.Render
// rather than after — truncating the rendered string would leave that line long.
//
// Measured: every other line of this view already fits at width 40 (summary 37,
// billable note 38, table 39, sparkline 15), so this test can assert on the
// whole view without depending on anything the spec left out of scope.
func TestReportViewHeaderFitsTheTerminal(t *testing.T) {
	t.Parallel()
	th := testTheme(true)
	rm := newReport(goldenReport(), " (2/5 members)", []float64{1, 0, 3, 2, 8})
	for _, width := range []int{40, 60, 80} {
		for i, line := range strings.Split(rm.view(th, width), "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Errorf("width %d: line %d rendered %d columns: %q", width, i, w, line)
			}
		}
	}
}

// Before the first WindowSizeMsg the width is 0 and nothing is sized against
// it: the header keeps its full text, because truncateWidth(s, 0) returns ""
// and would blank it.
func TestReportViewHeaderSurvivesZeroWidth(t *testing.T) {
	t.Parallel()
	th := testTheme(true)
	rm := newReport(goldenReport(), "", nil)
	if !strings.Contains(rm.view(th, 0), "Report ") {
		t.Error("the header vanished at width 0")
	}
}

// TestReportViewHeaderMarginProtectsTruncationOrder ensures truncation happens
// before th.Title.Render, not after. When truncation occurs after rendering,
// ansi.Truncate cuts across the multi-line rendered string (text + MarginBottom),
// corrupting the margin line with ellipsis or wrong spacing. This test verifies
// that line 1 (the margin) is all spaces and matches line 0's width — the only
// outcome when text is truncated before rendering. If truncation is inverted,
// the margin line will be shorter than line 0, and the test fails.
func TestReportViewHeaderMarginProtectsTruncationOrder(t *testing.T) {
	t.Parallel()
	th := testTheme(true)
	rm := newReport(goldenReport(), " (2/5 members)", []float64{1, 0, 3, 2, 8})
	for _, width := range []int{40, 60, 80} {
		lines := strings.Split(rm.view(th, width), "\n")
		if len(lines) < 2 {
			t.Fatalf("width %d: view has fewer than 2 lines", width)
		}
		line0, line1 := lines[0], lines[1]
		w0, w1 := lipgloss.Width(line0), lipgloss.Width(line1)

		// Line 1 must be all spaces (margin produced by th.Title.MarginBottom).
		if line1 != strings.Repeat(" ", w1) {
			t.Errorf("width %d: line 1 is not all spaces: %q", width, line1)
		}

		// Line 1 must match line 0's width (how MarginBottom works).
		if w1 != w0 {
			t.Errorf("width %d: margin line width %d != text line width %d", width, w1, w0)
		}
	}
}
