package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/duration"
)

// runningTimerMsg carries the globally-tracked running timer (nil = none). It is
// distinct from timerMsg (which drives the log timer flow and is dropped unless
// the user is on the log/loading screen); this one must update global state even
// while the user is on Home.
type runningTimerMsg struct{ timer *clickup.RunningTimer }

// userMsg carries the authenticated user's id (0 if unknown).
type userMsg struct{ id int }

// tickMsg fires once per second while a timer is running.
type tickMsg struct{}

// repollTickInterval is how many 1s ticks pass between background re-polls of the
// running timer, so a timer started/stopped in another ClickUp client is
// eventually reflected here. Cheap: one GET per 30s, well within the limiter.
const repollTickInterval = 30

// tickCmd schedules the next 1s tick. Elapsed is derived from m.now() in the
// view, so the tick payload itself is ignored (keeps the clock injectable).
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

// runningTimerProbeCmd reads the current running timer (real or demo).
func (m Model) runningTimerProbeCmd() tea.Cmd {
	if m.demo {
		return demoRunningTimerProbeCmd(m.now())
	}
	c := m.client
	teamID := m.cfg.WorkspaceID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rt, err := c.CurrentTimer(ctx, teamID)
		if err != nil {
			return runningTimerMsg{timer: nil} // silent: a failed probe just means "no indicator"
		}
		return runningTimerMsg{timer: rt}
	}
}

// currentUserCmd resolves the authenticated user id for ownership gating.
func (m Model) currentUserCmd() tea.Cmd {
	if m.demo {
		return func() tea.Msg { return userMsg{id: demoSelfID} }
	}
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		u, err := c.CurrentUser(ctx)
		if err != nil {
			return userMsg{id: 0}
		}
		return userMsg{id: u.ID}
	}
}

// elapsedLabel renders "01:23:45  (1.40h)" for a running timer, or "" when the
// start is unknown (zero) — a zero start would otherwise render a ~490,000h
// stopwatch. Negative elapsed (server clock ahead) clamps via duration.Clock.
func elapsedLabel(start, now time.Time) string {
	if start.IsZero() {
		return ""
	}
	d := now.Sub(start)
	if d < 0 {
		d = 0
	}
	return duration.Clock(d) + "  (" + duration.FormatHours(d) + ")"
}
