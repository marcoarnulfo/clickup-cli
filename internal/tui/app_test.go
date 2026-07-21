package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

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
