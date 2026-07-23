package tui

import (
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
)

func fixedNow(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestElapsedLabel(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	if got := elapsedLabel(time.Time{}, base); got != "" {
		t.Errorf("zero start = %q, want empty", got)
	}
	got := elapsedLabel(base.Add(-90*time.Minute), base)
	if got != "01:30:00  (1.50h)" {
		t.Errorf("elapsed = %q, want 01:30:00  (1.50h)", got)
	}
	if got := elapsedLabel(base.Add(time.Minute), base); got != "00:00:00  (0.00h)" {
		t.Errorf("negative elapsed = %q, want clamped", got)
	}
}

func TestRunningTimerMsgArmsOneTickChain(t *testing.T) {
	m := Model{now: fixedNow(time.Now())}
	rt := &clickup.RunningTimer{TaskID: "t1", TaskName: "Fix", Start: time.Now().Add(-time.Minute)}

	m2, cmd := m.Update(runningTimerMsg{timer: rt})
	mm := m2.(Model)
	if mm.runningTimer == nil || !mm.ticking {
		t.Fatalf("expected runningTimer set and ticking, got %+v ticking=%v", mm.runningTimer, mm.ticking)
	}
	if cmd == nil {
		t.Fatalf("expected a tick cmd on nil->non-nil transition")
	}

	// A second non-nil probe must NOT arm a second chain.
	m3, cmd2 := mm.Update(runningTimerMsg{timer: rt})
	if cmd2 != nil {
		t.Errorf("second probe armed another tick chain (cmd2 != nil)")
	}
	_ = m3

	// nil clears the timer and stops ticking.
	m4, _ := mm.Update(runningTimerMsg{timer: nil})
	if mm4 := m4.(Model); mm4.runningTimer != nil || mm4.ticking {
		t.Errorf("nil probe did not clear timer/ticking: %+v ticking=%v", mm4.runningTimer, mm4.ticking)
	}
}

// TestRunningTimerMsgFailedProbeKeepsIndicator verifies a transient probe
// failure (network blip, timeout, 5xx — e.g. laptop sleep/wake) does not clear
// an already-known running timer nor stop the tick chain: the indicator must
// survive until the next scheduled re-poll succeeds or genuinely finds none.
func TestRunningTimerMsgFailedProbeKeepsIndicator(t *testing.T) {
	m := Model{now: fixedNow(time.Now())}
	rt := &clickup.RunningTimer{TaskID: "t1", TaskName: "Fix", Start: time.Now().Add(-time.Minute)}
	m.runningTimer = rt
	m.ticking = true

	m2, cmd := m.Update(runningTimerMsg{timer: nil, failed: true})
	mm := m2.(Model)
	if mm.runningTimer != rt {
		t.Errorf("failed probe changed runningTimer: got %+v, want unchanged %+v", mm.runningTimer, rt)
	}
	if !mm.ticking {
		t.Errorf("failed probe stopped ticking, expected it to keep the chain alive")
	}
	if cmd != nil {
		t.Errorf("failed probe should not issue a new command (the live chain re-polls on its own)")
	}
}

func TestTickMsgStopsWhenNoTimer(t *testing.T) {
	m := Model{now: fixedNow(time.Now()), ticking: true}
	m2, cmd := m.Update(tickMsg{})
	if mm := m2.(Model); mm.ticking {
		t.Errorf("tick with no runningTimer should stop ticking")
	}
	if cmd != nil {
		t.Errorf("tick with no runningTimer should not re-arm")
	}
}
