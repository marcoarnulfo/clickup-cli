package tui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func TestSetupIgnoresKeysWhileLoading(t *testing.T) {
	m := New(config.Config{})
	for _, r := range "tok" {
		u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = u.(Model)
	}
	// Enter avvia la validazione (loading=true)
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if !m.setup.loading {
		t.Fatal("dopo Enter dovrebbe essere in loading")
	}
	before := m.setup.token()
	// digitazioni successive devono essere ignorate finché è in loading
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = u.(Model)
	if m.setup.token() != before {
		t.Fatalf("input ignorato durante loading atteso; token %q -> %q", before, m.setup.token())
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
		t.Fatalf("tariffa non valida deve restare su stepRate, got %v", m.setup.step)
	}
	if m.setup.msg == "" {
		t.Fatal("attesa un messaggio d'errore per tariffa non valida")
	}
}

func TestLoadEntriesTeamWorkspaceNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /team ritorna un workspace con id DIVERSO da quello richiesto
		w.Write([]byte(`{"teams":[{"id":"OTHER","name":"X","members":[{"user":{"id":1,"username":"a"}}]}]}`))
	}))
	defer srv.Close()
	c := clickup.New("tok")
	c.BaseURL = srv.URL

	msg := loadEntriesCmd(c, "900", 2026, time.July, "team")()
	if _, ok := msg.(errMsg); !ok {
		t.Fatalf("scope team con workspace non trovato deve dare errMsg, got %T", msg)
	}
}

func TestLoadEntriesResolvesListNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/time_entries"):
			w.Write([]byte(`{"data":[{"id":"e1","task":{"id":"t","name":"T"},"task_location":{"list_id":"55"},"user":{"id":1,"username":"x"},"start":"1751360400000","duration":"3600000"}]}`))
		case strings.Contains(r.URL.Path, "/list/"):
			w.Write([]byte(`{"id":"55","name":"Cliente Z"}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	c := clickup.New("t")
	c.BaseURL = srv.URL

	msg := loadEntriesCmd(c, "900", 2026, time.July, "me")()
	em, ok := msg.(entriesMsg)
	if !ok {
		t.Fatalf("expected entriesMsg, got %T", msg)
	}
	if len(em.entries) != 1 || em.entries[0].ListName != "Cliente Z" {
		t.Fatalf("list name not resolved: %+v", em.entries)
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
	// digita un carattere nel campo token
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

func TestHomeChangesMonthAndScope(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1"})
	m.year, m.month = 2026, 7
	m.home = newHome(2026, 7)

	// freccia sinistra -> mese precedente
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
	m.report = report.Report{Year: 2026, Month: 7, Currency: "EUR",
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
	m.home = newHome(m.year, m.month)
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
		{ListID: "55", ListName: "Cliente Z", Start: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), Duration: time.Hour},
	}
	u, _ := m.Update(entriesMsg{entries: entries})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = u.(Model)
	if m.screen != screenRates {
		t.Fatalf("p dalla vista report deve aprire screenRates, got %v", m.screen)
	}
	if len(m.ratesScreen.rows) != 1 || m.ratesScreen.rows[0].name != "Cliente Z" {
		t.Fatalf("righe tariffe errate: %+v", m.ratesScreen.rows)
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
	// Enter -> editing riga 0
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if !m.ratesScreen.editing {
		t.Fatal("dovrebbe essere in editing")
	}
	// digita "50"
	for _, r := range "50" {
		u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = u.(Model)
	}
	// Enter conferma il valore
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	// 's' salva e ricalcola
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = u.(Model)
	if m.screen != screenReport {
		t.Fatalf("salvataggio deve tornare al report, got %v", m.screen)
	}
	if m.cfg.Rates["55"] != 50 {
		t.Fatalf("override non salvato: %+v", m.cfg.Rates)
	}
	if m.report.TotalAmount != 100 { // 2h * 50
		t.Fatalf("report non ricalcolato: TotalAmount %v, want 100", m.report.TotalAmount)
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
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // apre editing
	m = u.(Model)
	for _, r := range "99" {
		u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = u.(Model)
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // annulla
	m = u.(Model)
	if m.ratesScreen.editing {
		t.Fatal("Esc deve uscire dall'editing")
	}
	if _, ok := m.ratesScreen.rates["55"]; ok {
		t.Fatalf("Esc non deve aver impostato un override: %+v", m.ratesScreen.rates)
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
	for _, r := range "-5" { // negativo: non valido
		u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = u.(Model)
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if !m.ratesScreen.editing {
		t.Fatal("tariffa non valida deve mantenere l'editing aperto")
	}
	if m.ratesScreen.msg == "" {
		t.Fatal("atteso un messaggio d'errore per tariffa non valida")
	}
	if _, ok := m.ratesScreen.rates["55"]; ok {
		t.Fatal("tariffa non valida non deve creare un override")
	}
}

func TestRatesScreenSaveErrorStaysOnScreen(t *testing.T) {
	f := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", f)            // os.UserConfigDir() deriva da qui (macOS)
	t.Setenv("XDG_CONFIG_HOME", f) // ...o da qui (Linux); un file => MkdirAll fallisce
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
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}) // salva (fallisce)
	m = u.(Model)
	if m.screen != screenRates {
		t.Fatalf("salvataggio fallito deve restare su screenRates, got %v", m.screen)
	}
	if m.ratesScreen.msg == "" {
		t.Fatal("atteso un messaggio d'errore di salvataggio")
	}
}
