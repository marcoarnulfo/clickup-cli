package tui

import (
	"errors"
	"fmt"
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
	m.entries = []report.TimeEntry{{ID: "e1", ListID: "l1", ListName: "List 1", TaskID: "t1", TaskName: "Task 1", Billable: true}}
	start, end := report.MonthRange(m.year, m.month, nil)
	m.report = report.Build(m.entries, report.GroupByTotal, pricingFromConfig(cfg), start, end, nil)
	m.rep = newReport(m.report, "")
	return m
}

func TestReportKeyNOpensLog(t *testing.T) {
	m := newTestModelOnReport()
	next, _ := m.Update(key("n"))
	nm := next.(Model)
	if nm.screen != screenLog {
		t.Fatalf("screen = %v, expected screenLog", nm.screen)
	}
	if nm.logScreen.step != logModeSelect {
		t.Errorf("step = %v, expected logModeSelect", nm.logScreen.step)
	}
}

func TestLogModeSelectTransitions(t *testing.T) {
	m := newTestModelOnReport()
	next, _ := m.Update(key("n"))
	m = next.(Model)

	// 1 = guided -> list pick
	g, _ := m.Update(key("1"))
	if s := g.(Model).logScreen.step; s != logListPick {
		t.Errorf("guided -> step = %v, expected logListPick", s)
	}
	// 2 = ID -> id input
	i, _ := m.Update(key("2"))
	if s := i.(Model).logScreen.step; s != logIDInput {
		t.Errorf("id -> step = %v, expected logIDInput", s)
	}
	// 3 = timer -> timer pick
	tm, _ := m.Update(key("3"))
	if s := tm.(Model).logScreen.step; s != logTimerPick {
		t.Errorf("timer -> step = %v, expected logTimerPick", s)
	}
}

func TestLogEscReturnsToReport(t *testing.T) {
	m := newTestModelOnReport()
	next, _ := m.Update(key("n"))
	m = next.(Model)
	back, _ := m.Update(key("esc"))
	if s := back.(Model).screen; s != screenReport {
		t.Errorf("esc -> screen = %v, expected screenReport", s)
	}
}

func reachForm(t *testing.T) Model {
	t.Helper()
	m := newTestModelOnReport()
	next, _ := m.Update(key("n"))
	m = next.(Model)
	// ID mode -> input -> form
	next, _ = m.Update(key("2"))
	m = next.(Model)
	m.logScreen.input.SetValue("task123")
	next, _ = m.Update(key("enter"))
	return next.(Model)
}

func TestFormInvalidDurationStays(t *testing.T) {
	m := reachForm(t)
	if m.logScreen.step != logForm {
		t.Fatalf("step = %v, expected logForm", m.logScreen.step)
	}
	m.logScreen.input.SetValue("abc") // invalid duration
	next, _ := m.Update(key("enter"))
	nm := next.(Model)
	if nm.logScreen.step != logForm {
		t.Errorf("with invalid duration step = %v, expected to stay in logForm", nm.logScreen.step)
	}
	if nm.logScreen.msg == "" {
		t.Errorf("expected error message for invalid duration")
	}
}

func TestFormValidFlowSubmits(t *testing.T) {
	m := reachForm(t)
	// duration
	m.logScreen.input.SetValue("1h30")
	next, _ := m.Update(key("enter"))
	m = next.(Model)
	if m.logScreen.formField != 1 {
		t.Fatalf("after duration formField = %d, expected 1 (date)", m.logScreen.formField)
	}
	// date (uses the prefilled default)
	next, _ = m.Update(key("enter"))
	m = next.(Model)
	if m.logScreen.formField != 2 {
		t.Fatalf("after date formField = %d, expected 2 (note)", m.logScreen.formField)
	}
	// note -> billable step
	m.logScreen.input.SetValue("work")
	next, _ = m.Update(key("enter"))
	m = next.(Model)
	if m.logScreen.formField != 3 {
		t.Fatalf("after note formField = %d, expected 3 (billable)", m.logScreen.formField)
	}
	// billable: Enter = yes -> submit
	next, cmd := m.Update(key("enter"))
	m = next.(Model)
	if m.screen != screenLoading {
		t.Errorf("after submit screen = %v, expected screenLoading", m.screen)
	}
	if cmd == nil {
		t.Errorf("expected a command (createEntryCmd) after submit")
	}
	if !m.logScreen.billable {
		t.Errorf("Enter on the billable step should keep billable=true")
	}
}

func TestFormBillableToggleNo(t *testing.T) {
	m := reachForm(t)
	m.logScreen.input.SetValue("1h")
	next, _ := m.Update(key("enter")) // duration -> date
	m = next.(Model)
	next, _ = m.Update(key("enter")) // date (default) -> note
	m = next.(Model)
	m.logScreen.input.SetValue("x")
	next, _ = m.Update(key("enter")) // note -> billable
	m = next.(Model)
	if m.logScreen.formField != 3 {
		t.Fatalf("expected billable step (formField 3), got %d", m.logScreen.formField)
	}
	next, cmd := m.Update(key("n")) // billable = no -> submit
	m = next.(Model)
	if m.logScreen.billable {
		t.Error("'n' should set billable=false")
	}
	if m.screen != screenLoading || cmd == nil {
		t.Error("'n' should submit (screenLoading + non-nil cmd)")
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
		t.Fatalf("step = %v, expected logForm", nm.logScreen.step)
	}
	if nm.logScreen.taskID != "86abc" {
		t.Errorf("taskID = %q, expected 86abc", nm.logScreen.taskID)
	}
}

func TestIDInputEmptyStays(t *testing.T) {
	m := newTestModelOnReport()
	next, _ := m.Update(key("n"))
	m = next.(Model)
	next, _ = m.Update(key("2")) // ID mode
	m = next.(Model)
	m.logScreen.input.SetValue("") // empty
	next, _ = m.Update(key("enter"))
	nm := next.(Model)
	if nm.logScreen.step != logIDInput {
		t.Errorf("with empty id step = %v, expected to stay in logIDInput", nm.logScreen.step)
	}
	if nm.logScreen.msg == "" {
		t.Errorf("expected error message for empty id")
	}
}

func TestGuidedListPickIssuesCmd(t *testing.T) {
	m := newTestModelOnReport()
	next, _ := m.Update(key("n"))
	m = next.(Model)
	next, _ = m.Update(key("1")) // guided -> listPick
	m = next.(Model)
	if len(m.logScreen.lists) == 0 {
		t.Fatal("no known list (expected List 1 from entries)")
	}
	next, cmd := m.Update(key("enter"))
	m = next.(Model)
	if !m.logScreen.loading {
		t.Errorf("expected loading=true after Enter on the list")
	}
	if cmd == nil {
		t.Errorf("expected listTasksCmd after Enter on the list")
	}
}

func TestGuidedTaskListMsgPopulatesPicker(t *testing.T) {
	m := newTestModelOnReport()
	m.logScreen = newLog(m.entries, m.cfg)
	m.logScreen.step = logListPick
	m.screen = screenLog
	next, _ := m.Update(taskListMsg{tasks: []clickup.Task{{ID: "x1", Name: "One"}, {ID: "x2", Name: "Two"}}})
	nm := next.(Model)
	if nm.logScreen.step != logTaskPick {
		t.Fatalf("step = %v, expected logTaskPick", nm.logScreen.step)
	}
	if len(nm.logScreen.tasks) != 2 {
		t.Errorf("tasks = %d, expected 2", len(nm.logScreen.tasks))
	}
}

func TestGuidedTaskSelectToForm(t *testing.T) {
	m := newTestModelOnReport()
	m.logScreen = newLog(m.entries, m.cfg)
	m.logScreen.step = logTaskPick
	m.logScreen.mode = modeGuided
	m.logScreen.tasks = []clickup.Task{{ID: "x1", Name: "One"}}
	m.screen = screenLog
	next, _ := m.Update(key("enter"))
	nm := next.(Model)
	if nm.logScreen.step != logForm || nm.logScreen.taskID != "x1" {
		t.Errorf("step=%v taskID=%q, expected logForm/x1", nm.logScreen.step, nm.logScreen.taskID)
	}
}

func TestLogDoneMsgShowsConfirm(t *testing.T) {
	m := newTestModelOnReport()
	m.screen = screenLoading
	m.logScreen = newLog(m.entries, m.cfg)
	next, _ := m.Update(logDoneMsg{summary: "1h30 on task123"})
	nm := next.(Model)
	if nm.screen != screenLog || nm.logScreen.step != logDone {
		t.Errorf("screen=%v step=%v, expected screenLog/logDone", nm.screen, nm.logScreen.step)
	}
}

func TestTimerPickRoutes(t *testing.T) {
	m := newTestModelOnReport()
	next, _ := m.Update(key("n"))
	m = next.(Model)
	next, _ = m.Update(key("3")) // timer
	m = next.(Model)
	if m.logScreen.step != logTimerPick {
		t.Fatalf("step = %v, expected logTimerPick", m.logScreen.step)
	}
	// 1 = guided
	g, _ := m.Update(key("1"))
	if s := g.(Model).logScreen.step; s != logListPick {
		t.Errorf("timer/guided -> step = %v, expected logListPick", s)
	}
	// 2 = ID
	i, _ := m.Update(key("2"))
	if s := i.(Model).logScreen.step; s != logIDInput {
		t.Errorf("timer/id -> step = %v, expected logIDInput", s)
	}
}

func TestTimerMsgSetsRunning(t *testing.T) {
	m := newTestModelOnReport()
	m.logScreen = newLog(m.entries, m.cfg)
	m.screen = screenLog
	rt := &clickup.RunningTimer{TaskID: "x1", TaskName: "One"}
	next, _ := m.Update(timerMsg{timer: rt})
	nm := next.(Model)
	if nm.logScreen.step != logTimerRunning || nm.logScreen.timer == nil {
		t.Errorf("step=%v timer=%v, expected logTimerRunning with timer", nm.logScreen.step, nm.logScreen.timer)
	}
}

func TestTimerMsgNilNoRunning(t *testing.T) {
	m := newTestModelOnReport()
	m.logScreen = newLog(m.entries, m.cfg)
	m.logScreen.step = logTimerRunning
	m.screen = screenLog
	next, _ := m.Update(timerMsg{timer: nil})
	nm := next.(Model)
	if nm.logScreen.timer != nil {
		t.Errorf("expected timer nil")
	}
}

func TestNewLogIncludesConfigLists(t *testing.T) {
	cfg := config.Config{Token: "t", WorkspaceID: "team1", Currency: "EUR", Rate: 40, Rates: map[string]float64{"111": 60, "222": 30}}
	lg := newLog(nil, cfg)
	got := map[string]bool{}
	for _, l := range lg.lists {
		got[l.id] = true
	}
	if !got["111"] || !got["222"] {
		t.Errorf("missing config lists in picker: %+v", lg.lists)
	}
}

func TestTimerMsgIgnoredWhenAway(t *testing.T) {
	m := newTestModelOnReport()
	m.screen = screenReport
	next, _ := m.Update(timerMsg{timer: &clickup.RunningTimer{TaskID: "x1"}})
	if s := next.(Model).screen; s != screenReport {
		t.Errorf("stale timerMsg changed screen to %v, expected to stay screenReport", s)
	}
}

func TestTimerRunningStopIssuesCmd(t *testing.T) {
	m := newTestModelOnReport()
	m.logScreen = newLog(m.entries, m.cfg)
	m.logScreen.step = logTimerRunning
	m.logScreen.timer = &clickup.RunningTimer{TaskID: "x1"}
	m.screen = screenLog
	next, cmd := m.Update(key("s"))
	m = next.(Model)
	if m.screen != screenLoading {
		t.Errorf("after stop screen = %v, expected screenLoading", m.screen)
	}
	if cmd == nil {
		t.Errorf("expected stopTimerCmd")
	}
}

func TestLogErrKeepsFormOnLogScreen(t *testing.T) {
	m := newTestModelOnReport()
	m.logScreen = newLog(m.entries, m.cfg)
	m.logScreen.step = logForm
	m.logScreen.taskID = "task123"
	m.logScreen.loading = true
	m.screen = screenLoading
	next, _ := m.Update(logErrMsg{err: errors.New("boom")})
	nm := next.(Model)
	if nm.screen != screenLog {
		t.Errorf("screen = %v, want screenLog (form preserved)", nm.screen)
	}
	if nm.logScreen.taskID != "task123" {
		t.Errorf("taskID lost on error: %q", nm.logScreen.taskID)
	}
	if nm.logScreen.loading {
		t.Error("loading should be cleared on error")
	}
	if nm.logScreen.msg == "" {
		t.Error("expected an error message on the log screen")
	}
}

func TestLogErrClassification(t *testing.T) {
	if _, ok := logErr(errors.New("x")).(logErrMsg); !ok {
		t.Error("a non-auth error should become logErrMsg (stay on log screen)")
	}
	authErr := fmt.Errorf("wrap: %w", clickup.ErrUnauthorized)
	if _, ok := logErr(authErr).(errMsg); !ok {
		t.Error("an auth error should become errMsg (global re-setup)")
	}
}

func TestLogBrowseEntryOpensBrowser(t *testing.T) {
	m := Model{screen: screenLog, demo: true}
	m.logScreen = newLog([]report.TimeEntry{{ListID: "a", ListName: "A"}}, config.Config{})
	m.logScreen.step = logListPick
	// move down onto the "Browse all…" row (index len(lists) == 1)
	u, _ := m.updateLog(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = u.(Model)
	if m.logScreen.listIdx != 1 {
		t.Fatalf("listIdx = %d, want 1 (browse row)", m.logScreen.listIdx)
	}
	u, _ = m.updateLog(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if m.screen != screenListBrowser || m.browserScreen.origin != screenLog {
		t.Fatalf("browse row should open the browser for log; screen=%v origin=%v", m.screen, m.browserScreen.origin)
	}
}

// demoLogModel builds a Model with demo mode enabled and a known list, ready
// to drive the log-hours flow without ever touching m.client (which is left
// nil on purpose: any accidental real-API call would panic instead of
// silently succeeding, which is the point of these tests — see #32).
func demoLogModel() Model {
	m := Model{screen: screenLog, demo: true, cfg: demoConfig()}
	m.logScreen = newLog([]report.TimeEntry{{ListID: "a", ListName: "A"}}, m.cfg)
	return m
}

func TestDemoGuidedListPickIssuesDemoTaskListMsg(t *testing.T) {
	m := demoLogModel()
	m.logScreen.step = logListPick
	next, cmd := m.updateLog(key("enter"))
	m = next.(Model)
	if !m.logScreen.loading {
		t.Fatalf("expected loading=true after Enter on the list")
	}
	if cmd == nil {
		t.Fatal("expected a command from list pick in demo mode")
	}
	msg := cmd() // must not hit the network: m.client is nil
	tlm, ok := msg.(taskListMsg)
	if !ok {
		t.Fatalf("expected taskListMsg from the demo tasks cmd, got %T", msg)
	}
	if len(tlm.tasks) == 0 {
		t.Error("expected demo tasks, got 0")
	}

	// Feeding it back through Update must land on the task picker, never
	// bounce to setup/error.
	next2, _ := m.Update(msg)
	m2 := next2.(Model)
	if m2.screen == screenSetup || m2.screen == screenError {
		t.Errorf("demo log flow must never route to setup/error, got screen=%v", m2.screen)
	}
	if m2.logScreen.step != logTaskPick {
		t.Errorf("step = %v, expected logTaskPick", m2.logScreen.step)
	}
}

func TestDemoFormSubmitIssuesLogDoneMsgNoIO(t *testing.T) {
	m := demoLogModel()
	m.logScreen = enterForm(m.logScreen)
	m.logScreen.taskID = "demo-t1"
	m.logScreen.durStr = "1h"
	m.logScreen.formField = 3 // billable step

	next, cmd := m.updateLog(key("enter")) // billable=yes -> submit
	m = next.(Model)
	if m.screen != screenLoading {
		t.Fatalf("screen = %v, expected screenLoading after submit", m.screen)
	}
	if cmd == nil {
		t.Fatal("expected a command on form submit")
	}
	msg := cmd() // must not hit the network: m.client is nil
	ld, ok := msg.(logDoneMsg)
	if !ok {
		t.Fatalf("expected logDoneMsg from the demo create-entry cmd, got %T", msg)
	}
	if ld.summary == "" {
		t.Error("expected a non-empty summary")
	}

	next2, _ := m.Update(msg)
	m2 := next2.(Model)
	if m2.screen == screenSetup || m2.screen == screenError {
		t.Errorf("demo log flow must never route to setup/error, got screen=%v", m2.screen)
	}
	if m2.screen != screenLog || m2.logScreen.step != logDone {
		t.Errorf("screen=%v step=%v, expected screenLog/logDone", m2.screen, m2.logScreen.step)
	}
}

func TestDemoTimerStartAndStopNoIO(t *testing.T) {
	m := demoLogModel()
	m.logScreen.step = logIDInput
	m.logScreen.mode = modeTimer
	m.logScreen.input.SetValue("demo-t1")

	next, cmd := m.updateLog(key("enter"))
	m = next.(Model)
	if cmd == nil {
		t.Fatal("expected a command starting the timer in demo mode")
	}
	msg := cmd() // must not hit the network: m.client is nil
	tm, ok := msg.(timerMsg)
	if !ok || tm.timer == nil {
		t.Fatalf("expected timerMsg with a running timer, got %#v", msg)
	}

	m.logScreen.step = logTimerRunning
	m.logScreen.timer = tm.timer
	next, cmd = m.updateLog(key("s")) // stop
	m = next.(Model)
	if cmd == nil {
		t.Fatal("expected a command stopping the timer in demo mode")
	}
	stopMsg := cmd() // must not hit the network: m.client is nil
	if _, ok := stopMsg.(logDoneMsg); !ok {
		t.Fatalf("expected logDoneMsg from the demo stop-timer cmd, got %T", stopMsg)
	}
}

func TestDemoCurrentTimerCmdReportsNoTimer(t *testing.T) {
	m := demoLogModel()
	cmd := m.timerCurrentCmd()
	msg := cmd() // must not hit the network: m.client is nil
	tm, ok := msg.(timerMsg)
	if !ok || tm.timer != nil {
		t.Fatalf("expected timerMsg{timer:nil} in demo mode, got %#v", msg)
	}
}

func TestListPickDebounceWhileLoading(t *testing.T) {
	m := newTestModelOnReport()
	m.logScreen = newLog(m.entries, m.cfg)
	m.logScreen.step = logListPick
	m.logScreen.loading = true // a listTasksCmd is already in flight
	m.screen = screenLog
	_, cmd := m.Update(key("enter"))
	if cmd != nil {
		t.Error("Enter while loading must not dispatch a second listTasksCmd")
	}
}
