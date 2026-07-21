package tui

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func TestCurrentRangeDefaultsToMonth(t *testing.T) {
	m := Model{year: 2026, month: time.July, preset: report.PresetThisMonth}
	start, end := m.currentRange()
	ws, we := report.MonthRange(2026, time.July)
	if !start.Equal(ws) || !end.Equal(we) {
		t.Errorf("currentRange = [%s,%s), want month", start, end)
	}
}

func TestCurrentRangeCustomIsInclusive(t *testing.T) {
	from := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	m := Model{preset: report.PresetCustom, customStart: from, customEnd: to}
	start, end := m.currentRange()
	if !start.Equal(from) || !end.Equal(to.AddDate(0, 0, 1)) {
		t.Errorf("custom range = [%s,%s), want [%s, %s+1d)", start, end, from, to)
	}
}

func TestLoadEntriesUsesGivenRange(t *testing.T) {
	var gotStart, gotEnd string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/time_entries") {
			gotStart = r.URL.Query().Get("start_date")
			gotEnd = r.URL.Query().Get("end_date")
			w.Write([]byte(`{"data":[]}`))
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := clickup.New("tok")
	c.BaseURL = srv.URL
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 10)
	if _, ok := loadEntriesCmd(c, "900", start, end, "me", nil)().(entriesMsg); !ok {
		t.Fatal("expected entriesMsg")
	}
	if gotStart != strconv.FormatInt(start.UnixMilli(), 10) || gotEnd != strconv.FormatInt(end.UnixMilli(), 10) {
		t.Errorf("range not forwarded: start=%s end=%s", gotStart, gotEnd)
	}
}

func TestSetupIgnoresKeysWhileLoading(t *testing.T) {
	m := New(config.Config{})
	for _, r := range "tok" {
		u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = u.(Model)
	}
	// Enter starts validation (loading=true)
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if !m.setup.loading {
		t.Fatal("should be loading after Enter")
	}
	before := m.setup.token()
	// subsequent keystrokes must be ignored while loading
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = u.(Model)
	if m.setup.token() != before {
		t.Fatalf("input should be ignored while loading; token %q -> %q", before, m.setup.token())
	}
}

func TestSetupRejectsInvalidRate(t *testing.T) {
	m := New(config.Config{})
	m.setup.step = stepRate
	m.setup.input = newNumberInput("")
	m.setup.input.SetValue("abc")
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if m.setup.step != stepRate {
		t.Fatalf("invalid rate should stay on stepRate, got %v", m.setup.step)
	}
	if m.setup.msg == "" {
		t.Fatal("expected an error message for invalid rate")
	}
}

func TestLoadEntriesTeamWorkspaceNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /team returns a workspace with an id DIFFERENT from the one requested
		w.Write([]byte(`{"teams":[{"id":"OTHER","name":"X","members":[{"user":{"id":1,"username":"a"}}]}]}`))
	}))
	defer srv.Close()
	c := clickup.New("tok")
	c.BaseURL = srv.URL

	jStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	jEnd := jStart.AddDate(0, 1, 0)
	msg := loadEntriesCmd(c, "900", jStart, jEnd, "team", nil)()
	if _, ok := msg.(errMsg); !ok {
		t.Fatalf("team scope with workspace not found should give errMsg, got %T", msg)
	}
}

func TestLoadEntriesTeamHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/team") && strings.Contains(r.URL.Path, "/time_entries"):
			// must filter on team members (assignee set)
			if r.URL.Query().Get("assignee") == "" {
				t.Errorf("team scope: expected assignee parameter to be set")
			}
			w.Write([]byte(`{"data":[{"id":"e1","task":{"id":"t","name":"T"},"task_location":{"list_id":"55"},"user":{"id":7,"username":"x"},"start":"1751360400000","duration":"3600000"}]}`))
		case strings.HasSuffix(r.URL.Path, "/team"):
			// workspace 900 with two members
			w.Write([]byte(`{"teams":[{"id":"900","name":"WS","members":[{"user":{"id":7,"username":"a"}},{"user":{"id":8,"username":"b"}}]}]}`))
		case strings.Contains(r.URL.Path, "/list/"):
			w.Write([]byte(`{"id":"55","name":"Client Z"}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	c := clickup.New("tok")
	c.BaseURL = srv.URL

	jStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	jEnd := jStart.AddDate(0, 1, 0)
	msg := loadEntriesCmd(c, "900", jStart, jEnd, "team", nil)()
	em, ok := msg.(entriesMsg)
	if !ok {
		t.Fatalf("team scope with workspace found should give entriesMsg, got %T", msg)
	}
	if len(em.entries) != 1 || em.entries[0].ListName != "Client Z" {
		t.Fatalf("wrong team entries: %+v", em.entries)
	}
}

func TestLoadEntriesResolvesListNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/time_entries"):
			w.Write([]byte(`{"data":[{"id":"e1","task":{"id":"t","name":"T"},"task_location":{"list_id":"55"},"user":{"id":1,"username":"x"},"start":"1751360400000","duration":"3600000"}]}`))
		case strings.Contains(r.URL.Path, "/list/"):
			w.Write([]byte(`{"id":"55","name":"Client Z"}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	c := clickup.New("t")
	c.BaseURL = srv.URL

	jStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	jEnd := jStart.AddDate(0, 1, 0)
	msg := loadEntriesCmd(c, "900", jStart, jEnd, "me", nil)()
	em, ok := msg.(entriesMsg)
	if !ok {
		t.Fatalf("expected entriesMsg, got %T", msg)
	}
	if len(em.entries) != 1 || em.entries[0].ListName != "Client Z" {
		t.Fatalf("list name not resolved: %+v", em.entries)
	}
}

func TestSelectedAssignees(t *testing.T) {
	m := Model{selectedMembers: map[int]bool{3: true, 1: false, 2: true}}
	got := m.selectedAssignees()
	if len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Errorf("selectedAssignees = %v, want [2 3]", got)
	}
}

func TestLoadEntriesTeamExplicitAssignees(t *testing.T) {
	teamCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/time_entries"):
			if got := r.URL.Query().Get("assignee"); got != "7,9" {
				t.Errorf("assignee = %q, want 7,9", got)
			}
			w.Write([]byte(`{"data":[]}`))
		case strings.HasSuffix(r.URL.Path, "/team"):
			teamCalled = true
			w.Write([]byte(`{"teams":[]}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	c := clickup.New("tok")
	c.BaseURL = srv.URL

	jStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	jEnd := jStart.AddDate(0, 1, 0)
	msg := loadEntriesCmd(c, "900", jStart, jEnd, "team", []int{7, 9})()
	if _, ok := msg.(entriesMsg); !ok {
		t.Fatalf("expected entriesMsg, got %T", msg)
	}
	if teamCalled {
		t.Error("explicit assignees: /team must not be called")
	}
}

func TestReloadEntriesCmdPassesSelectedAssignees(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/time_entries"):
			if got := r.URL.Query().Get("assignee"); got != "5" {
				t.Errorf("assignee = %q, want 5", got)
			}
			w.Write([]byte(`{"data":[]}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	c := clickup.New("tok")
	c.BaseURL = srv.URL
	m := Model{
		client:          c,
		cfg:             config.Config{WorkspaceID: "900"},
		year:            2026,
		month:           time.July,
		scope:           "team",
		selectedMembers: map[int]bool{5: true},
	}
	if _, ok := m.reloadEntriesCmd()().(entriesMsg); !ok {
		t.Fatal("expected entriesMsg from reloadEntriesCmd")
	}
}

func TestReloadEntriesCmdIgnoresSelectionInMeScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/time_entries"):
			if got := r.URL.Query().Get("assignee"); got != "" {
				t.Errorf("me scope must not filter by assignee, got %q", got)
			}
			w.Write([]byte(`{"data":[]}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	c := clickup.New("tok")
	c.BaseURL = srv.URL
	m := Model{
		client:          c,
		cfg:             config.Config{WorkspaceID: "900"},
		year:            2026,
		month:           time.July,
		scope:           "me",
		selectedMembers: map[int]bool{5: true}, // stale from a prior team selection
	}
	if _, ok := m.reloadEntriesCmd()().(entriesMsg); !ok {
		t.Fatal("expected entriesMsg from reloadEntriesCmd")
	}
}

func TestNewStartsInSetupWhenInvalid(t *testing.T) {
	m := New(config.Config{})
	if m.screen != screenSetup {
		t.Fatalf("invalid config should start in setup, got %v", m.screen)
	}
}

func TestNewStartsInHomeWhenValid(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1"})
	if m.screen != screenHome {
		t.Fatalf("valid config should start in home, got %v", m.screen)
	}
}

func TestErrMsgSwitchesToErrorScreen(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1"})
	updated, _ := m.Update(errMsg{err: errTest})
	mm := updated.(Model)
	if mm.screen != screenError {
		t.Fatalf("errMsg should switch to error screen, got %v", mm.screen)
	}
}

func TestEntriesMsgBuildsReportAndShowsReportScreen(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1", Rate: 10, Currency: "EUR"})
	m.year, m.month = 2026, 7
	updated, _ := m.Update(entriesMsg{entries: []report.TimeEntry{}})
	mm := updated.(Model)
	if mm.screen != screenReport {
		t.Fatalf("entriesMsg should switch to report screen, got %v", mm.screen)
	}
}

func TestQuitKey(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1"})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("q should return a quit command")
	}
}

func TestSetupTokenStepAcceptsInput(t *testing.T) {
	m := New(config.Config{})
	// type a character in the token field
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	mm := updated.(Model)
	if mm.setup.token() == "" {
		t.Fatal("token input should capture typed characters")
	}
}

var errTest = &testErr{}

type testErr struct{}

func (*testErr) Error() string { return "boom" }

func TestReportCycleGroupBy(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1", Rate: 10, Currency: "EUR"})
	m.year, m.month = 2026, 7
	updated, _ := m.Update(entriesMsg{entries: []report.TimeEntry{
		{TaskName: "A", ListName: "L", Start: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), Duration: time.Hour},
	}})
	mm := updated.(Model)
	if mm.report.GroupBy != report.GroupByTotal {
		t.Fatalf("initial groupBy should be total, got %q", mm.report.GroupBy)
	}
	// 'g' -> task
	updated2, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	mm2 := updated2.(Model)
	if mm2.report.GroupBy != report.GroupByTask {
		t.Fatalf("after g groupBy should be task, got %q", mm2.report.GroupBy)
	}
}

func TestEntriesMsgMemberGroupingDoesNotLeakIntoMeScope(t *testing.T) {
	// A "me" scope model that inherited a "member" GroupBy from a prior
	// team report must fall back to total: member grouping is team-only.
	m := Model{scope: "me"}
	m.report.GroupBy = report.GroupByMember
	updated, _ := m.Update(entriesMsg{entries: nil})
	mm := updated.(Model)
	if mm.report.GroupBy != report.GroupByTotal {
		t.Fatalf("me scope should reset member grouping to total, got %q", mm.report.GroupBy)
	}
}

func TestHomeChangesMonthAndScope(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1"})
	m.year, m.month = 2026, 7
	m.home = newHome()

	// left arrow -> previous month
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	mm := updated.(Model)
	if mm.month != 6 {
		t.Fatalf("left should go to June, got %v", mm.month)
	}

	// 't' alterna scope
	updated2, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	mm2 := updated2.(Model)
	if mm2.scope != "team" {
		t.Fatalf("t should switch scope to team, got %q", mm2.scope)
	}
}

func TestExportWritesFile(t *testing.T) {
	dir := t.TempDir()
	oldwd, _ := os.Getwd()
	defer os.Chdir(oldwd)
	os.Chdir(dir)

	m := New(config.Config{Token: "t", WorkspaceID: "1", Currency: "EUR"})
	m.year, m.month = 2026, 7
	jStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	m.report = report.Report{Start: jStart, End: jStart.AddDate(0, 1, 0), Currency: "EUR",
		Buckets: []report.Bucket{{Label: "A", Hours: 1, Amount: 0}}, TotalHours: 1}
	m.export = newExport(m.report)
	m.screen = screenExport

	// Enter sul primo formato (csv)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(Model)
	if mm.export.err != nil {
		t.Fatalf("export error: %v", mm.export.err)
	}
	if _, err := os.Stat("clickup-report-2026-07.csv"); err != nil {
		t.Fatalf("expected csv file: %v", err)
	}
}

func TestHomeEnterStartsLoading(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1"})
	m.home = newHome()
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(Model)
	if mm.screen != screenLoading {
		t.Fatalf("enter should switch to loading, got %v", mm.screen)
	}
	if cmd == nil {
		t.Fatal("enter should return a load command")
	}
}

func TestRatesScreenOpensFromReport(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1", Rate: 30, Currency: "EUR"})
	m.year, m.month = 2026, 7
	entries := []report.TimeEntry{
		{ListID: "55", ListName: "Client Z", Start: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), Duration: time.Hour},
	}
	u, _ := m.Update(entriesMsg{entries: entries})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = u.(Model)
	if m.screen != screenRates {
		t.Fatalf("p dalla vista report deve aprire screenRates, got %v", m.screen)
	}
	if len(m.ratesScreen.rows) != 1 || m.ratesScreen.rows[0].name != "Client Z" {
		t.Fatalf("wrong rate rows: %+v", m.ratesScreen.rows)
	}
}

func TestRatesScreenEditSaveRecomputes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("CLICKUP_TOKEN", "")

	m := New(config.Config{Token: "t", WorkspaceID: "1", Rate: 30, Currency: "EUR"})
	m.year, m.month = 2026, 7
	entries := []report.TimeEntry{
		{ListID: "55", ListName: "Z", Start: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), Duration: 2 * time.Hour},
	}
	u, _ := m.Update(entriesMsg{entries: entries})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = u.(Model)
	// Enter -> editing row 0
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if !m.ratesScreen.editing {
		t.Fatal("should be editing")
	}
	// type "50"
	for _, r := range "50" {
		u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = u.(Model)
	}
	// Enter confirms the value
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	// 's' saves and recalculates
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = u.(Model)
	if m.screen != screenReport {
		t.Fatalf("saving should return to the report, got %v", m.screen)
	}
	if m.cfg.Rates["55"] != 50 {
		t.Fatalf("override not saved: %+v", m.cfg.Rates)
	}
	if m.report.TotalAmount != 100 { // 2h * 50
		t.Fatalf("report not recalculated: TotalAmount %v, want 100", m.report.TotalAmount)
	}
}

func TestRatesScreenEscCancelsEdit(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1", Rate: 30, Currency: "EUR"})
	m.year, m.month = 2026, 7
	entries := []report.TimeEntry{
		{ListID: "55", ListName: "Z", Start: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), Duration: time.Hour},
	}
	u, _ := m.Update(entriesMsg{entries: entries})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // opens editing
	m = u.(Model)
	for _, r := range "99" {
		u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = u.(Model)
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // cancel
	m = u.(Model)
	if m.ratesScreen.editing {
		t.Fatal("Esc should exit editing")
	}
	if _, ok := m.ratesScreen.rates["55"]; ok {
		t.Fatalf("Esc should not have set an override: %+v", m.ratesScreen.rates)
	}
}

func TestRatesScreenInvalidRateStaysEditing(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1", Rate: 30, Currency: "EUR"})
	m.year, m.month = 2026, 7
	entries := []report.TimeEntry{
		{ListID: "55", ListName: "Z", Start: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), Duration: time.Hour},
	}
	u, _ := m.Update(entriesMsg{entries: entries})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	// "." is numeric (passes the filter) but isn't a valid float
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'.'}})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if !m.ratesScreen.editing {
		t.Fatal("invalid rate should keep editing open")
	}
	if m.ratesScreen.msg == "" {
		t.Fatal("expected an error message for invalid rate")
	}
	if _, ok := m.ratesScreen.rates["55"]; ok {
		t.Fatal("invalid rate should not create an override")
	}
}

func TestRatesScreenRejectsNonNumericInput(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1", Rate: 30, Currency: "EUR"})
	m.year, m.month = 2026, 7
	entries := []report.TimeEntry{
		{ListID: "55", ListName: "Z", Start: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), Duration: time.Hour},
	}
	u, _ := m.Update(entriesMsg{entries: entries})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // editing
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}) // non-numeric
	m = u.(Model)
	if m.ratesScreen.input.Value() != "" {
		t.Fatalf("non-numeric character should not be accepted, value=%q", m.ratesScreen.input.Value())
	}
}

func TestRatesScreenEscDiscardsAndReturns(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1", Rate: 30, Currency: "EUR"})
	m.year, m.month = 2026, 7
	entries := []report.TimeEntry{
		{ListID: "55", ListName: "Z", Start: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), Duration: time.Hour},
	}
	u, _ := m.Update(entriesMsg{entries: entries})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // editing
	m = u.(Model)
	for _, r := range "50" {
		u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = u.(Model)
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // confirm value in the working copy
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // Esc outside editing = discard
	m = u.(Model)
	if m.screen != screenReport {
		t.Fatalf("Esc should return to the report, got %v", m.screen)
	}
	if _, ok := m.cfg.Rates["55"]; ok {
		t.Fatalf("Esc should not persist override: %+v", m.cfg.Rates)
	}
}

func TestRatesScreenDropsOverrideEqualToDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("CLICKUP_TOKEN", "")

	m := New(config.Config{Token: "t", WorkspaceID: "1", Rate: 30, Currency: "EUR"})
	m.year, m.month = 2026, 7
	entries := []report.TimeEntry{
		{ListID: "55", ListName: "Z", Start: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), Duration: time.Hour},
	}
	u, _ := m.Update(entriesMsg{entries: entries})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // editing
	m = u.(Model)
	for _, r := range "30" { // equal to the default
		u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = u.(Model)
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // confirm 30
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}) // save
	m = u.(Model)
	if _, ok := m.cfg.Rates["55"]; ok {
		t.Fatalf("an override equal to the default should not be saved: %+v", m.cfg.Rates)
	}
}

func TestVisibleEntriesAppliesFilter(t *testing.T) {
	m := Model{
		entries: []report.TimeEntry{
			{ListName: "A", Duration: time.Hour},
			{ListName: "B", Duration: time.Hour},
		},
		filterLists: map[string]bool{"A": true},
	}
	got := m.visibleEntries()
	if len(got) != 1 || got[0].ListName != "A" {
		t.Fatalf("visibleEntries = %+v", got)
	}
	if m.filteredNote() != " · filtered" {
		t.Errorf("filteredNote = %q", m.filteredNote())
	}
	m.filterLists = nil
	if m.filteredNote() != "" {
		t.Errorf("no filter -> empty note, got %q", m.filteredNote())
	}
}

func TestEntriesMsgBuildsFilteredReport(t *testing.T) {
	m := Model{year: 2026, month: time.July, preset: report.PresetThisMonth, filterLists: map[string]bool{"A": true}}
	u, _ := m.Update(entriesMsg{entries: []report.TimeEntry{
		{ListName: "A", Duration: time.Hour, Start: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)},
		{ListName: "B", Duration: 2 * time.Hour, Start: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)},
	}})
	m = u.(Model)
	if m.report.TotalHours != 1 {
		t.Errorf("filtered report total = %v, want 1", m.report.TotalHours)
	}
}

func TestEntriesMsgReappliesCachedStatus(t *testing.T) {
	m := Model{
		year: 2026, month: time.July, preset: report.PresetThisMonth,
		taskStatus:     map[string]string{"t1": "done"},
		filterStatuses: map[string]bool{"done": true},
	}
	u, _ := m.Update(entriesMsg{entries: []report.TimeEntry{
		{TaskID: "t1", ListName: "A", Duration: time.Hour, Start: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)},
	}})
	m = u.(Model)
	if m.report.TotalHours != 1 {
		t.Fatalf("cached status should be re-applied on reload; total = %v, want 1", m.report.TotalHours)
	}
	// The cached status must be assigned BEFORE pruning runs, otherwise pruneFilters
	// would see entries with no status yet and drop the "done" selection.
	if !m.filterStatuses["done"] {
		t.Error("filterStatuses[\"done\"] should survive: assign must happen before prune")
	}
}

func TestMembersMsgPreservesExistingSelection(t *testing.T) {
	m := Model{selectedMembers: map[int]bool{1: true}}
	u, _ := m.Update(membersMsg{members: []clickup.Member{{ID: 1, Username: "a"}, {ID: 2, Username: "b"}}})
	m = u.(Model)
	if !m.selectedMembers[1] {
		t.Error("member 1 should remain selected")
	}
	if m.selectedMembers[2] {
		t.Error("existing selection must be preserved: member 2 should stay deselected, not default to all-selected")
	}
}

func TestStatusesMsgAssignsAndOpens(t *testing.T) {
	m := Model{
		screen:  screenFilters,
		entries: []report.TimeEntry{{TaskID: "t1", ListName: "A"}, {TaskID: "t2", ListName: "A"}},
	}
	u, _ := m.Update(statusesMsg{byTask: map[string]string{"t1": "open", "t2": "done"}})
	m = u.(Model)
	if m.screen != screenFilters {
		t.Fatalf("screen = %v, want filters", m.screen)
	}
	if m.entries[0].Status != "open" || m.entries[1].Status != "done" {
		t.Errorf("statuses not assigned: %+v", m.entries)
	}
	// the Statuses section now has both options
	if len(m.filtersScreen.sections[2].options) != 2 {
		t.Errorf("status options = %v", m.filtersScreen.sections[2].options)
	}
}

func TestReportFEnrichesWhenStatusMissing(t *testing.T) {
	m := Model{screen: screenReport, demo: true, entries: []report.TimeEntry{{TaskID: "t1", ListName: "A"}}}
	u, cmd := m.updateReport(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	m = u.(Model)
	if m.screen != screenFilters || !m.filtersScreen.loadingStatuses {
		t.Fatalf("expected loading-statuses filters screen")
	}
	if cmd == nil {
		t.Fatal("expected a status enrichment command")
	}
	if _, ok := cmd().(statusesMsg); !ok {
		t.Fatal("expected statusesMsg from the enrich command")
	}
}

func TestRatesScreenSaveErrorStaysOnScreen(t *testing.T) {
	f := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", f)            // os.UserConfigDir() derives from here (macOS)
	t.Setenv("XDG_CONFIG_HOME", f) // ...or from here (Linux); a file => MkdirAll fails
	t.Setenv("CLICKUP_TOKEN", "")

	m := New(config.Config{Token: "t", WorkspaceID: "1", Rate: 30, Currency: "EUR"})
	m.year, m.month = 2026, 7
	entries := []report.TimeEntry{
		{ListID: "55", ListName: "Z", Start: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), Duration: time.Hour},
	}
	u, _ := m.Update(entriesMsg{entries: entries})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}) // save (fails)
	m = u.(Model)
	if m.screen != screenRates {
		t.Fatalf("failed save should stay on screenRates, got %v", m.screen)
	}
	if m.ratesScreen.msg == "" {
		t.Fatal("expected a save error message")
	}
}

// Fix 1: a filter selection whose value is absent from the freshly loaded
// entries (e.g. after a range change) must be pruned, so the report doesn't
// silently stay stuck empty with no way to clear it from the Filters UI.
func TestEntriesMsgPrunesStaleFilterSelections(t *testing.T) {
	m := Model{
		year: 2026, month: time.July, preset: report.PresetThisMonth,
		filterLists: map[string]bool{"Gone": true},
	}
	u, _ := m.Update(entriesMsg{entries: []report.TimeEntry{
		{ListName: "A", Duration: time.Hour, Start: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)},
	}})
	m = u.(Model)
	if m.filterLists["Gone"] {
		t.Errorf("stale filter selection %q should have been pruned, got filterLists = %+v", "Gone", m.filterLists)
	}
	if m.report.TotalHours <= 0 {
		t.Errorf("report should not stay stuck empty after pruning stale filters; TotalHours = %v", m.report.TotalHours)
	}
}

// Fix 2: statusEnrichCmd must tolerate a single non-retrievable task (e.g.
// deleted, no permission, rate-limited) instead of discarding everything
// fetched and bricking the Filters screen for the whole session.
func TestStatusEnrichCmdTolerantOfPerTaskFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/task/tgood":
			w.Write([]byte(`{"id":"tgood","status":{"status":"open"}}`))
		case "/task/tbad":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"err":"Task not found","ECODE":"TASK_002"}`))
		}
	}))
	defer srv.Close()
	c := clickup.New("tok")
	c.BaseURL = srv.URL

	msg := statusEnrichCmd(c, []string{"tgood", "tbad"})()
	sm, ok := msg.(statusesMsg)
	if !ok {
		t.Fatalf("a single unreachable task should not abort enrichment; got %T (%+v)", msg, msg)
	}
	if sm.byTask["tgood"] != "open" {
		t.Errorf("byTask[tgood] = %q, want %q", sm.byTask["tgood"], "open")
	}
	if st, ok := sm.byTask["tbad"]; !ok || st != "" {
		t.Errorf("byTask[tbad] = (%q, %v), want (\"\", true): unreachable tasks cache as empty status", st, ok)
	}
}

// Fix 2 (auth exception): a 401 must still surface as errMsg to relaunch setup.
func TestStatusEnrichCmdStillReturnsErrMsgOnUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"err":"Token invalid","ECODE":"OAUTH_025"}`))
	}))
	defer srv.Close()
	c := clickup.New("tok")
	c.BaseURL = srv.URL

	msg := statusEnrichCmd(c, []string{"t1"})()
	em, ok := msg.(errMsg)
	if !ok {
		t.Fatalf("401 should still return errMsg, got %T", msg)
	}
	if !errors.Is(em.err, clickup.ErrUnauthorized) {
		t.Errorf("err = %v, want it to wrap ErrUnauthorized", em.err)
	}
}

// Fix 3: 'q' must not quit while typing a custom date on the range screen —
// it has free-text inputs, so a stray 'q' shouldn't kill the whole session.
func TestQuitKeyDoesNotQuitWhileEditingCustomRange(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1"})
	m.screen = screenRange
	m.rangeScreen = newRange(report.PresetThisMonth)
	m.rangeScreen.editing = true
	m.rangeScreen.field = 0
	m.rangeScreen.fromInput = newTextInput("From (YYYY-MM-DD)")
	m.rangeScreen.fromInput.Focus()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil && reflect.ValueOf(cmd).Pointer() == reflect.ValueOf(tea.Quit).Pointer() {
		t.Fatal("'q' while editing a custom date must not quit the application")
	}
	mm := updated.(Model)
	if !strings.Contains(mm.rangeScreen.fromInput.Value(), "q") {
		t.Fatalf("'q' should have been typed into the From input, got %q", mm.rangeScreen.fromInput.Value())
	}
}

// Fix 1 (staleness guard): spacesMsg arriving while on screenError must cache
// the spaces but NOT teleport to screenListBrowser.
func TestSpacesMsgStaleAfterNavigatingAway(t *testing.T) {
	m := Model{screen: screenError, err: errTest}
	spaces := []clickup.Space{{ID: "s1", Name: "Engineering"}}
	updated, _ := m.Update(spacesMsg{spaces: spaces})
	mm := updated.(Model)

	// cache must be warmed
	if len(mm.browserSpaces) != 1 || mm.browserSpaces[0].ID != "s1" {
		t.Errorf("spacesMsg should warm cache: browserSpaces = %+v, want 1 space with id s1", mm.browserSpaces)
	}

	// but the user should NOT be teleported to screenListBrowser
	if mm.screen != screenError {
		t.Errorf("stale spacesMsg must not change screen: got %v, want error", mm.screen)
	}

	// and the browser screen itself should not be updated
	if mm.browserScreen.loading || len(mm.browserScreen.spaces) != 0 {
		t.Errorf("browser screen should not be touched: loading=%v spaces=%d, want false/0", mm.browserScreen.loading, len(mm.browserScreen.spaces))
	}
}

// Fix 1 (normal path): spacesMsg arriving while on screenListBrowser must
// populate and clear loading.
func TestSpacesMsgWarmsCacheAndUpdatesWhenOnBrowser(t *testing.T) {
	m := Model{screen: screenListBrowser}
	m.browserScreen = listBrowserModel{origin: screenLog, loading: true}
	spaces := []clickup.Space{{ID: "s1", Name: "Engineering"}}
	updated, _ := m.Update(spacesMsg{spaces: spaces})
	mm := updated.(Model)

	// cache must be warmed
	if len(mm.browserSpaces) != 1 || mm.browserSpaces[0].ID != "s1" {
		t.Errorf("spacesMsg should warm cache: browserSpaces = %+v, want 1 space with id s1", mm.browserSpaces)
	}

	// screen must stay on browser
	if mm.screen != screenListBrowser {
		t.Errorf("spacesMsg on browser should stay on browser, got %v", mm.screen)
	}

	// browser screen must be updated
	if mm.browserScreen.loading || len(mm.browserScreen.spaces) != 1 || mm.browserScreen.level != browseSpaces || mm.browserScreen.idx != 0 {
		t.Errorf("browser not updated: loading=%v spaces=%d level=%v idx=%d", mm.browserScreen.loading, len(mm.browserScreen.spaces), mm.browserScreen.level, mm.browserScreen.idx)
	}
}
