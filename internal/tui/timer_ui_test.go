package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
)

func TestHomeShowsRunningTimerLine(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	m := newTestModel() // helper; see implementer note
	m.now = fixedNow(now)
	m.runningTimer = &clickup.RunningTimer{TaskName: "Fix login bug", Start: now.Add(-90 * time.Minute)}
	out := m.View()
	if !strings.Contains(out, "Fix login bug") || !strings.Contains(out, "01:30:00") {
		t.Errorf("home view missing live timer line:\n%s", out)
	}
}

func TestCFromHomeOpensTimerScreenWithOrigin(t *testing.T) {
	now := time.Now()
	m := newTestModel()
	m.now = fixedNow(now)
	m.screen = screenHome
	m.runningTimer = &clickup.RunningTimer{TaskName: "Fix", Start: now.Add(-time.Minute)}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	mm := m2.(Model)
	if mm.screen != screenLog || mm.logScreen.step != logTimerRunning {
		t.Fatalf("c did not open the timer screen: screen=%v step=%v", mm.screen, mm.logScreen.step)
	}
	if mm.logScreen.origin != screenHome {
		t.Errorf("origin = %v, want screenHome", mm.logScreen.origin)
	}
}

// TestCFromHomeEscReturnsHome exercises the full round trip (spec §11): 'c'
// from Home opens the timer screen, and Esc from there must return to Home
// (not screenReport, the old hardcoded destination).
func TestCFromHomeEscReturnsHome(t *testing.T) {
	now := time.Now()
	m := newTestModel()
	m.now = fixedNow(now)
	m.screen = screenHome
	m.runningTimer = &clickup.RunningTimer{TaskName: "Fix", Start: now.Add(-time.Minute)}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	mm := m2.(Model)
	back, _ := mm.Update(key("esc"))
	if s := back.(Model).screen; s != screenHome {
		t.Errorf("esc from timer screen (origin Home) -> screen = %v, want screenHome", s)
	}
}

func TestTimerStoppedClearsIndicator(t *testing.T) {
	m := newTestModel()
	m.runningTimer = &clickup.RunningTimer{TaskName: "Fix"}
	m.ticking = true
	m2, _ := m.Update(timerStoppedMsg{summary: "45m logged"})
	mm := m2.(Model)
	if mm.runningTimer != nil || mm.ticking {
		t.Errorf("timerStoppedMsg did not clear indicator: %+v ticking=%v", mm.runningTimer, mm.ticking)
	}
}

func TestLogDoneMsgDoesNotClearRunningTimer(t *testing.T) {
	m := newTestModel()
	m.runningTimer = &clickup.RunningTimer{TaskName: "Fix"}
	m2, _ := m.Update(logDoneMsg{summary: "1h logged"})
	if mm := m2.(Model); mm.runningTimer == nil {
		t.Errorf("logDoneMsg (manual create) must not clear the running-timer indicator")
	}
}

// TestRepollFiresOnIntervalTick (spec §11): in real (non-demo) mode, the
// tickCount-th tick that lands on repollTickInterval must re-issue the
// running-timer probe alongside the next tickCmd.
func TestRepollFiresOnIntervalTick(t *testing.T) {
	now := time.Now()
	m := newTestModel()
	m.now = fixedNow(now)
	m.demo = false
	m.runningTimer = &clickup.RunningTimer{TaskName: "Fix", Start: now.Add(-time.Minute)}
	m.ticking = true
	m.tickCount = repollTickInterval - 1

	m2, cmd := m.Update(tickMsg{})
	mm := m2.(Model)
	if mm.tickCount != repollTickInterval {
		t.Fatalf("tickCount = %d, want %d", mm.tickCount, repollTickInterval)
	}
	if cmd == nil {
		t.Fatalf("expected a batched cmd on the repoll tick")
	}
}
