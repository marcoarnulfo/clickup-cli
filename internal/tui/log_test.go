package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func newTestModelOnReport() Model {
	cfg := config.Config{Token: "t", WorkspaceID: "team1", Currency: "EUR", Rate: 40}
	m := New(cfg)
	m.screen = screenReport
	m.entries = []report.TimeEntry{{ListID: "l1", ListName: "Lista 1", TaskID: "t1", TaskName: "Task 1"}}
	m.report = report.Build(m.entries, report.GroupByTotal, ratesFromConfig(cfg), "EUR", m.year, m.month)
	m.rep = newReport(m.report)
	return m
}

func TestReportKeyNOpensLog(t *testing.T) {
	m := newTestModelOnReport()
	next, _ := m.Update(key("n"))
	nm := next.(Model)
	if nm.screen != screenLog {
		t.Fatalf("screen = %v, atteso screenLog", nm.screen)
	}
	if nm.logScreen.step != logModeSelect {
		t.Errorf("step = %v, atteso logModeSelect", nm.logScreen.step)
	}
}

func TestLogModeSelectTransitions(t *testing.T) {
	m := newTestModelOnReport()
	next, _ := m.Update(key("n"))
	m = next.(Model)

	// 1 = guidato -> list pick
	g, _ := m.Update(key("1"))
	if s := g.(Model).logScreen.step; s != logListPick {
		t.Errorf("guidato -> step = %v, atteso logListPick", s)
	}
	// 2 = ID -> id input
	i, _ := m.Update(key("2"))
	if s := i.(Model).logScreen.step; s != logIDInput {
		t.Errorf("id -> step = %v, atteso logIDInput", s)
	}
	// 3 = timer -> timer pick
	tm, _ := m.Update(key("3"))
	if s := tm.(Model).logScreen.step; s != logTimerPick {
		t.Errorf("timer -> step = %v, atteso logTimerPick", s)
	}
}

func TestLogEscReturnsToReport(t *testing.T) {
	m := newTestModelOnReport()
	next, _ := m.Update(key("n"))
	m = next.(Model)
	back, _ := m.Update(key("esc"))
	if s := back.(Model).screen; s != screenReport {
		t.Errorf("esc -> screen = %v, atteso screenReport", s)
	}
}
