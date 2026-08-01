package tui

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/*.golden files")

// TestMain pins the default renderer's color profile for the whole package.
// Without it a screen golden captured on a color-capable terminal would carry
// ANSI escapes and fail in CI, where the same code renders plain text.
// CLICKUP_DEMO is cleared because New() switches to fixture data when it is
// set, which would make every golden depend on the developer's shell.
func TestMain(m *testing.M) {
	os.Unsetenv("CLICKUP_DEMO")
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

// golden compares got against testdata/<name>.golden, or rewrites the file
// when the suite runs with -update.
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v\nregenerate with: go test ./internal/tui -update", path, err)
	}
	if got != string(want) {
		t.Errorf("%s does not match the golden file.\n--- want ---\n%s\n--- got ---\n%s", path, want, got)
	}
}

// goldenFixedTime is the instant every golden renders at, so any view showing
// a date, a range label or an elapsed timer stays byte-stable.
var goldenFixedTime = time.Date(2026, time.July, 15, 10, 30, 0, 0, time.UTC)

// goldenModel is a Model with a frozen clock and a fixed configuration.
func goldenModel() Model {
	m := testModel(config.Config{Token: "t", WorkspaceID: "team1", Currency: "EUR", Rate: 50})
	m.now = func() time.Time { return goldenFixedTime }
	m.loc = time.UTC
	m.year, m.month = 2026, time.July
	return m
}

// goldenReport is a fixed report: one billable list, one non-billable.
func goldenReport() report.Report {
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	return report.Report{
		Start: start, End: start.AddDate(0, 1, 0),
		Scope: "me", GroupBy: report.GroupByList, DefaultCurrency: "EUR",
		Timezone: "UTC", // reportModel.view prints this; unset renders a dangling "tz "
		Buckets: []report.Bucket{
			{Label: "Website", Key: "l1", Hours: 12.5, BillableHours: 12.5, BilledHours: 12.5,
				Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 625}}},
			{Label: "Internal", Key: "l2", Hours: 3, BillableHours: 0, BilledHours: 0,
				Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 0}}},
		},
		CurrencySubtotals: []report.CurrencySubtotal{
			{Currency: "EUR", Hours: 15.5, BillableHours: 12.5, BilledHours: 12.5, Amount: 625},
		},
		TotalHours: 15.5, BillableHours: 12.5, NonBillableHours: 3,
		BilledHours: 12.5, TotalAmount: 625,
	}
}

// goldenDaily is a fixed per-day series with two idle days, so the golden
// pins both a bar and a gap.
func goldenDaily() []float64 {
	return []float64{3.5, 0, 1.25, 4, 0, 2, 6}
}

// goldenDailyPartialMonth is a 31-day series with 10 days of data followed by
// 21 zero-hours days — the shape of the default view (the current month,
// viewed partway through) that goldenDaily() never depicts, since its own
// 7-value series ends non-zero. This is the exact shape that let the
// "hours/day" label drift 21 columns from the chart before it was moved to a
// prefix (visible in the shipped docs/demo.gif).
func goldenDailyPartialMonth() []float64 {
	days := []float64{3.5, 0, 1.25, 4, 0, 2, 6, 1, 5, 2.5} // 10 days worked
	return append(days, make([]float64, 21)...)            // the rest of July, not yet worked
}

// goldenEntries is the fixed entry set the browser, filters and rates screens
// render from.
func goldenEntries() []report.TimeEntry {
	start := time.Date(2026, time.July, 14, 9, 0, 0, 0, time.UTC)
	return []report.TimeEntry{
		{ID: "e1", ListID: "l1", ListName: "Website", TaskID: "t1", TaskName: "Landing page",
			Start: start, Duration: 90 * time.Minute, UserID: 1, UserName: "Marco", Billable: true},
		{ID: "e2", ListID: "l2", ListName: "Internal", TaskID: "t2", TaskName: "Standup",
			Start: start.Add(2 * time.Hour), Duration: 30 * time.Minute, UserID: 2, UserName: "Ada", Billable: false},
	}
}

func TestGoldenHome(t *testing.T) {
	t.Parallel()
	m := goldenModel()
	golden(t, "home", m.home.view(testTheme(true), "July 2026", "me", "", "", ""))
}

func TestGoldenHomeWithNotices(t *testing.T) {
	t.Parallel()
	m := goldenModel()
	m.home.errText = "request failed: 500"
	// The timer line is the string app.go builds for a running timer; keep it
	// verbatim so the golden depicts something the app actually renders.
	golden(t, "home_notices", m.home.view(testTheme(true), "July 2026", "team", "Members: 2/3", "v1.9.0",
		"⏱  running on Landing page — 00:12:30"))
}

func TestGoldenReport(t *testing.T) {
	t.Parallel()
	// One bucket gets a label long enough that the three widths actually
	// diverge. goldenReport's own labels are 8 columns, so without this all
	// three goldens would be byte-identical and would pin nothing about width.
	// At 60 the label is cut to 24 columns, at 80 to 44, and at 120 it fits
	// whole — which is also the case that proves the column does not stretch
	// past its content.
	r := goldenReport()
	r.Buckets[0].Label = "Website — landing page redesign and checkout hardening"

	for _, tc := range []struct {
		name  string
		width int
	}{{"report_narrow", 60}, {"report", 80}, {"report_wide", 120}} {
		golden(t, tc.name, newReport(r, "", goldenDaily()).view(testTheme(true), tc.width))
	}
}

// A multi-currency report renders a TOTAL row with an empty Amount cell plus
// one subtotal row per currency — the widest shape the total block takes.
func TestGoldenReportMultiCurrency(t *testing.T) {
	t.Parallel()
	r := goldenReport()
	r.Buckets = append(r.Buckets, report.Bucket{
		Label: "Mobile", Key: "l3", Hours: 4, BillableHours: 4, BilledHours: 4,
		Amounts: []report.CurrencyAmount{{Currency: "USD", Amount: 200}},
	})
	r.CurrencySubtotals = []report.CurrencySubtotal{
		{Currency: "EUR", Hours: 15.5, BillableHours: 12.5, BilledHours: 12.5, Amount: 625},
		{Currency: "USD", Hours: 4, BillableHours: 4, BilledHours: 4, Amount: 200},
	}
	// goldenReport's own BillableHours (12.5, from Website only) must grow by
	// Mobile's 4 billable hours too, or the split line below the table
	// contradicts the TOTAL row above it (a 19.5h total next to a 12.5h
	// billable + 3h non-billable split, which only adds up to 15.5h).
	// NonBillableHours is unchanged: Mobile is fully billable, so the 3h from
	// Internal is still the whole non-billable side.
	r.TotalHours, r.BillableHours, r.BilledHours = 19.5, 16.5, 16.5
	golden(t, "report_multicurrency", newReport(r, "", goldenDaily()).view(testTheme(true), 80))
}

// The default view: the current month, viewed partway through, so the daily
// series ends in a run of zero-hours days. goldenDaily() never depicted this
// shape (its own series ends non-zero) — it is exactly the shape that let the
// sparkline's "hours/day" label drift away from the chart before the label
// became a prefix.
func TestGoldenReportPartialMonth(t *testing.T) {
	t.Parallel()
	golden(t, "report_partial_month", newReport(goldenReport(), "", goldenDailyPartialMonth()).view(testTheme(true), 80))
}

// The Total grouping is what the report loads with, so it is the first table a
// user (and the README GIF) ever sees. Two goldens: one currency, and the
// multi-currency shape where TOTAL's Amount is empty and the subtotals carry
// the figures (#137).
func TestGoldenReportTotalGrouping(t *testing.T) {
	t.Parallel()
	r := report.Report{
		GroupBy:         report.GroupByTotal,
		DefaultCurrency: "EUR",
		Buckets: []report.Bucket{{Label: "Total", Hours: 15.5, BilledHours: 12.5,
			Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 625}}}},
		CurrencySubtotals: []report.CurrencySubtotal{{Currency: "EUR", Hours: 15.5, BilledHours: 12.5, Amount: 625}},
		TotalHours:        15.5, BilledHours: 12.5, TotalAmount: 625,
	}
	golden(t, "report_total", reportTable(testTheme(true), r, 80))
}

func TestGoldenReportTotalGroupingMultiCurrency(t *testing.T) {
	t.Parallel()
	r := report.Report{
		GroupBy:         report.GroupByTotal,
		DefaultCurrency: "EUR",
		Buckets: []report.Bucket{{Label: "Total", Hours: 18, BilledHours: 17,
			Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 337.5}, {Currency: "USD", Amount: 512.5}}}},
		CurrencySubtotals: []report.CurrencySubtotal{
			{Currency: "EUR", Hours: 7.75, BilledHours: 6.75, Amount: 337.5},
			{Currency: "USD", Hours: 10.25, BilledHours: 10.25, Amount: 512.5},
		},
		TotalHours: 18, BilledHours: 17,
	}
	golden(t, "report_total_multicurrency", reportTable(testTheme(true), r, 80))
}

func TestGoldenExport(t *testing.T) {
	t.Parallel()
	golden(t, "export", newExport(goldenReport()).view(testTheme(true)))
}

func TestGoldenBudget(t *testing.T) {
	t.Parallel()
	lines := []report.BudgetLine{
		{ListID: "l1", ListName: "Website", Currency: "EUR",
			Budget: 1000, Billed: 625, Remaining: 375, PercentUsed: 62.5},
	}
	golden(t, "budget", newBudget(lines).view(testTheme(true), 80))
}

func TestGoldenBudgetEmpty(t *testing.T) {
	t.Parallel()
	golden(t, "budget_empty", newBudget(nil).view(testTheme(true), 80))
}

func TestGoldenMembers(t *testing.T) {
	t.Parallel()
	members := []clickup.Member{{ID: 1, Username: "Marco"}, {ID: 2, Username: "Ada"}}
	golden(t, "members", newMembers(members, map[int]bool{1: true}).view(testTheme(true)))
}

func TestGoldenRange(t *testing.T) {
	t.Parallel()
	golden(t, "range", newRange(report.PresetThisMonth).view(testTheme(true)))
}

func TestGoldenFilters(t *testing.T) {
	t.Parallel()
	golden(t, "filters", newFilters(goldenEntries(), nil, nil, nil, nil).view(testTheme(true), 24))
}

// TestGoldenFiltersScrolled pins the windowed view a short terminal actually
// renders: 40 lists is far more than fits in 12 rows, so the golden should cut
// off partway through the Lists section with the cursor still inside it (#28).
func TestGoldenFiltersScrolled(t *testing.T) {
	t.Parallel()
	golden(t, "filters_scrolled", newFilters(filtersFixture(40), nil, nil, nil, nil).view(testTheme(true), 12))
}

func TestGoldenSetup(t *testing.T) {
	t.Parallel()
	golden(t, "setup", newSetup().view(testTheme(true)))
}

func TestGoldenListBrowser(t *testing.T) {
	t.Parallel()
	bs := listBrowserModel{
		spaces: []clickup.Space{{ID: "s1", Name: "Clients"}, {ID: "s2", Name: "Internal"}},
	}
	golden(t, "listbrowser", bs.view(testTheme(true)))
}

func TestGoldenHistory(t *testing.T) {
	t.Parallel()
	es := entriesModel{historyChanges: []clickup.HistoryChange{
		{Field: "duration", Before: "1h", After: "1h30m",
			Date: time.Date(2026, time.July, 14, 11, 0, 0, 0, time.UTC), User: "Marco"},
		{Field: "billable", Before: "false", After: "true",
			Date: time.Date(2026, time.July, 14, 11, 5, 0, 0, time.UTC), User: "Marco"},
	}}
	golden(t, "history", entriesHistoryView(testTheme(true), es, time.UTC))
}

func TestGoldenHistoryEmpty(t *testing.T) {
	t.Parallel()
	golden(t, "history_empty", entriesHistoryView(testTheme(true), entriesModel{}, time.UTC))
}

func TestGoldenRatesTabs(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Token: "t", WorkspaceID: "team1", Currency: "EUR", Rate: 50}
	for _, tc := range []struct {
		name string
		sec  ratesSection
	}{
		{"rates_lists", secLists},
		{"rates_members", secMembers},
		{"rates_overrides", secOverrides},
		{"rates_rules", secRules},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt := newRates(goldenEntries(), cfg)
			rt.sec = tc.sec
			golden(t, tc.name, rt.view(testTheme(true), KeyTable{}))
		})
	}
}

func TestGoldenLog(t *testing.T) {
	t.Parallel()
	lg := newLog(goldenEntries(), config.Config{Token: "t", WorkspaceID: "team1"})
	lg.now = goldenFixedTime
	golden(t, "log", lg.view(testTheme(true), KeyTable{}))
}

// The entries browser has no constructor: it is built inline when 'v' is
// pressed on the report screen, so this drives the real key path.
func TestGoldenEntriesBrowser(t *testing.T) {
	t.Parallel()
	m := goldenModel()
	m.userID = 1
	m.entries = goldenEntries()
	m.screen = screenReport
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	mm := next.(Model)
	if mm.screen != screenEntries {
		t.Fatalf("v did not open the entries browser: screen=%v", mm.screen)
	}
	golden(t, "entries", mm.View())
}

func TestGoldenLogForm(t *testing.T) {
	t.Parallel()
	lg := newLog(goldenEntries(), config.Config{Token: "t", WorkspaceID: "team1"})
	lg.now = goldenFixedTime
	lg = enterForm(lg, goldenFixedTime) // sets step AND initializes the text inputs
	// Production always sets taskID before the form is reachable (both the
	// guided picker and the ID-input step assign it), so capturing it empty
	// depicted a state the app never shows — and left the "Task:" row
	// unprotected.
	lg.taskID = "86abc"
	golden(t, "log_form", lg.view(testTheme(true), KeyTable{}))
}

func TestGoldenEntriesEdit(t *testing.T) {
	t.Parallel()
	m := goldenModel()
	m.userID = 1
	m.entries = goldenEntries()
	m.screen = screenReport
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // to the own entry
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	mm := next.(Model)
	if mm.entriesScreen.mode != entriesEdit {
		t.Fatalf("did not reach the edit form: mode=%v", mm.entriesScreen.mode)
	}
	golden(t, "entries_edit", mm.View())
}

func TestGoldenEntriesConfirmDelete(t *testing.T) {
	t.Parallel()
	m := goldenModel()
	m.userID = 1
	m.entries = goldenEntries()
	m.screen = screenReport
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	mm := next.(Model)
	if mm.entriesScreen.mode != entriesConfirmDelete {
		t.Fatalf("did not reach confirm-delete: mode=%v", mm.entriesScreen.mode)
	}
	golden(t, "entries_confirm_delete", mm.View())
}

func TestGoldenError(t *testing.T) {
	t.Parallel()
	m := goldenModel()
	m.screen = screenError
	m.err = errors.New("request failed: 500 Internal Server Error")
	golden(t, "error", m.View())
}

func TestGoldenLoading(t *testing.T) {
	t.Parallel()
	m := goldenModel()
	m.screen = screenLoading
	golden(t, "loading", m.View())
}

// goldenPaletteModel is a report screen with the palette open at a fixed size,
// so the box's position and contents are byte-stable.
func goldenPaletteModel() Model {
	m := goldenModel()
	m.theme = testTheme(true)
	m.entries = goldenEntries()
	m.report = goldenReport()
	m.rep = newReport(m.report, "", goldenDaily())
	m.screen = screenReport
	m.nav = []screen{screenHome}
	m.width, m.height = 90, 30
	return m.openPalette()
}

func TestGoldenPalette(t *testing.T) {
	t.Parallel()
	golden(t, "palette_report", goldenPaletteModel().View())
}

func TestGoldenPaletteFiltered(t *testing.T) {
	t.Parallel()
	m := goldenPaletteModel()
	m.palette.query = "exp"
	m = m.refreshPalette()
	golden(t, "palette_filtered", m.View())
}

func TestGoldenPaletteNoMatch(t *testing.T) {
	t.Parallel()
	m := goldenPaletteModel()
	m.palette.query = "zzzzzz"
	m = m.refreshPalette()
	golden(t, "palette_no_match", m.View())
}

// A terminal narrower than the preferred box, to pin the floor and the centering.
func TestGoldenPaletteNarrow(t *testing.T) {
	t.Parallel()
	m := goldenPaletteModel()
	m.width = 40
	golden(t, "palette_narrow", m.View())
}
