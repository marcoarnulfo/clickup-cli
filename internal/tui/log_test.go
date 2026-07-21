package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
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

func reachForm(t *testing.T) Model {
	t.Helper()
	m := newTestModelOnReport()
	next, _ := m.Update(key("n"))
	m = next.(Model)
	// modalità ID -> input -> form
	next, _ = m.Update(key("2"))
	m = next.(Model)
	m.logScreen.input.SetValue("task123")
	next, _ = m.Update(key("enter"))
	return next.(Model)
}

func TestFormInvalidDurationStays(t *testing.T) {
	m := reachForm(t)
	if m.logScreen.step != logForm {
		t.Fatalf("step = %v, atteso logForm", m.logScreen.step)
	}
	m.logScreen.input.SetValue("abc") // durata non valida
	next, _ := m.Update(key("enter"))
	nm := next.(Model)
	if nm.logScreen.step != logForm {
		t.Errorf("con durata invalida step = %v, atteso restare in logForm", nm.logScreen.step)
	}
	if nm.logScreen.msg == "" {
		t.Errorf("atteso messaggio d'errore per durata invalida")
	}
}

func TestFormValidFlowSubmits(t *testing.T) {
	m := reachForm(t)
	// durata
	m.logScreen.input.SetValue("1h30")
	next, _ := m.Update(key("enter"))
	m = next.(Model)
	if m.logScreen.formField != 1 {
		t.Fatalf("dopo durata formField = %d, atteso 1 (data)", m.logScreen.formField)
	}
	// data (usa il default precompilato)
	next, _ = m.Update(key("enter"))
	m = next.(Model)
	if m.logScreen.formField != 2 {
		t.Fatalf("dopo data formField = %d, atteso 2 (nota)", m.logScreen.formField)
	}
	// nota -> submit
	m.logScreen.input.SetValue("lavoro")
	next, cmd := m.Update(key("enter"))
	m = next.(Model)
	if m.screen != screenLoading {
		t.Errorf("dopo submit screen = %v, atteso screenLoading", m.screen)
	}
	if cmd == nil {
		t.Errorf("atteso un comando (createEntryCmd) dopo il submit")
	}
}

func TestIDInputToForm(t *testing.T) {
	m := newTestModelOnReport()
	next, _ := m.Update(key("n"))
	m = next.(Model)
	next, _ = m.Update(key("2"))
	m = next.(Model)
	m.logScreen.input.SetValue("https://app.clickup.com/t/86abc")
	next, _ = m.Update(key("enter"))
	nm := next.(Model)
	if nm.logScreen.step != logForm {
		t.Fatalf("step = %v, atteso logForm", nm.logScreen.step)
	}
	if nm.logScreen.taskID != "86abc" {
		t.Errorf("taskID = %q, atteso 86abc", nm.logScreen.taskID)
	}
}

func TestIDInputEmptyStays(t *testing.T) {
	m := newTestModelOnReport()
	next, _ := m.Update(key("n"))
	m = next.(Model)
	next, _ = m.Update(key("2")) // modalità ID
	m = next.(Model)
	m.logScreen.input.SetValue("") // vuoto
	next, _ = m.Update(key("enter"))
	nm := next.(Model)
	if nm.logScreen.step != logIDInput {
		t.Errorf("con id vuoto step = %v, atteso restare in logIDInput", nm.logScreen.step)
	}
	if nm.logScreen.msg == "" {
		t.Errorf("atteso messaggio d'errore per id vuoto")
	}
}

func TestGuidedListPickIssuesCmd(t *testing.T) {
	m := newTestModelOnReport()
	next, _ := m.Update(key("n"))
	m = next.(Model)
	next, _ = m.Update(key("1")) // guidato -> listPick
	m = next.(Model)
	if len(m.logScreen.lists) == 0 {
		t.Fatal("nessuna lista nota (attesa Lista 1 dalle entries)")
	}
	next, cmd := m.Update(key("enter"))
	m = next.(Model)
	if !m.logScreen.loading {
		t.Errorf("atteso loading=true dopo Enter sulla lista")
	}
	if cmd == nil {
		t.Errorf("atteso listTasksCmd dopo Enter sulla lista")
	}
}

func TestGuidedTaskListMsgPopulatesPicker(t *testing.T) {
	m := newTestModelOnReport()
	m.logScreen = newLog(m.entries, m.cfg)
	m.logScreen.step = logListPick
	m.screen = screenLog
	next, _ := m.Update(taskListMsg{tasks: []clickup.Task{{ID: "x1", Name: "Uno"}, {ID: "x2", Name: "Due"}}})
	nm := next.(Model)
	if nm.logScreen.step != logTaskPick {
		t.Fatalf("step = %v, atteso logTaskPick", nm.logScreen.step)
	}
	if len(nm.logScreen.tasks) != 2 {
		t.Errorf("tasks = %d, attesi 2", len(nm.logScreen.tasks))
	}
}

func TestGuidedTaskSelectToForm(t *testing.T) {
	m := newTestModelOnReport()
	m.logScreen = newLog(m.entries, m.cfg)
	m.logScreen.step = logTaskPick
	m.logScreen.mode = modeGuided
	m.logScreen.tasks = []clickup.Task{{ID: "x1", Name: "Uno"}}
	m.screen = screenLog
	next, _ := m.Update(key("enter"))
	nm := next.(Model)
	if nm.logScreen.step != logForm || nm.logScreen.taskID != "x1" {
		t.Errorf("step=%v taskID=%q, atteso logForm/x1", nm.logScreen.step, nm.logScreen.taskID)
	}
}

func TestLogDoneMsgShowsConfirm(t *testing.T) {
	m := newTestModelOnReport()
	m.screen = screenLoading
	m.logScreen = newLog(m.entries, m.cfg)
	next, _ := m.Update(logDoneMsg{summary: "1h30 su task123"})
	nm := next.(Model)
	if nm.screen != screenLog || nm.logScreen.step != logDone {
		t.Errorf("screen=%v step=%v, atteso screenLog/logDone", nm.screen, nm.logScreen.step)
	}
}
