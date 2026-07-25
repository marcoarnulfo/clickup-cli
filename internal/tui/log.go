package tui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/duration"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

type logStep int

const (
	logModeSelect logStep = iota
	logTimerPick
	logListPick
	logTaskPick
	logIDInput
	logForm
	logTimerRunning
	logDone
)

type logMode int

const (
	modeGuided logMode = iota
	modeID
	modeTimer
)

// taskListChoice is a known list (report ∪ config) shown in the guided picker.
type taskListChoice struct {
	id   string
	name string
}

type logModel struct {
	step logStep
	mode logMode

	// now is stamped by the root (app.go View()) before each render, so the
	// ticking timer screen never calls time.Now() itself.
	now time.Time

	lists   []taskListChoice
	listIdx int

	tasks   []clickup.Task
	taskIdx int
	loading bool

	taskID string

	input textinput.Model

	// form (fields filled in sequence)
	formField int // 0=duration 1=date 2=note 3=billable
	durStr    string
	dateStr   string
	noteStr   string
	billable  bool

	// timer
	timer *clickup.RunningTimer

	msg string
}

// newLog builds the screen from the known lists (entries ∪ config.Rates),
// in deterministic order for a stable view. Esc/done return to whoever pushed
// screenLog via pop() (Home or Report) — see Model.nav.
func newLog(entries []report.TimeEntry, cfg config.Config) logModel {
	names := map[string]string{}
	var order []string
	remember := func(id, name string) {
		if id == "" {
			return
		}
		if _, ok := names[id]; !ok {
			order = append(order, id)
			names[id] = id
		}
		if name != "" {
			names[id] = name
		}
	}
	for _, e := range entries {
		remember(e.ListID, e.ListName)
	}
	// lists present only in config: deterministic order (ascending id)
	var cfgIDs []string
	for id := range cfg.Rates {
		if _, ok := names[id]; !ok {
			cfgIDs = append(cfgIDs, id)
		}
	}
	slices.Sort(cfgIDs)
	for _, id := range cfgIDs {
		remember(id, "")
	}
	lists := make([]taskListChoice, len(order))
	for i, id := range order {
		lists[i] = taskListChoice{id: id, name: names[id]}
	}
	return logModel{lists: lists}
}

type logDoneMsg struct{ summary string }

// timerStoppedMsg is emitted only by stopping the running timer. It clears the
// global running-timer indicator; the shared logDoneMsg (also from manual
// create) must NOT, or logging an entry while a timer runs would kill the line.
type timerStoppedMsg struct{ summary string }

type taskListMsg struct{ tasks []clickup.Task }

type timerMsg struct{ timer *clickup.RunningTimer }

// logErrMsg is a log-flow error that keeps the user on the log screen (with the
// message shown) instead of bouncing to the global error screen — so a failed
// create/timer call doesn't discard the form. Auth errors are NOT wrapped in it
// (they must still trigger the global re-setup via errMsg).
type logErrMsg struct{ err error }

// logErr routes a log command error: auth errors go to the global handler
// (errMsg → re-setup); everything else stays on the log screen (logErrMsg).
func logErr(err error) tea.Msg {
	if errors.Is(err, clickup.ErrUnauthorized) {
		return errMsg{err: err}
	}
	return logErrMsg{err: err}
}

// startTimerCmd starts the timer and then reads the current state to get the
// authoritative start returned by ClickUp.
func startTimerCmd(c *clickup.Client, teamID, tid, desc string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := c.StartTimer(ctx, teamID, tid, desc); err != nil {
			return logErr(err)
		}
		rt, err := c.CurrentTimer(ctx, teamID)
		if err != nil {
			return logErr(err)
		}
		if rt == nil {
			rt = &clickup.RunningTimer{TaskID: tid, TaskName: tid}
		}
		return timerMsg{timer: rt}
	}
}

// stopTimerCmd stops the timer; the entry is created by ClickUp.
func stopTimerCmd(c *clickup.Client, teamID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		e, err := c.StopTimer(ctx, teamID)
		if err != nil {
			return logErr(err)
		}
		return timerStoppedMsg{summary: fmt.Sprintf("timer stopped: %s logged", duration.Format(e.Duration))}
	}
}

// currentTimerCmd reads the running timer (nil if none).
func currentTimerCmd(c *clickup.Client, teamID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rt, err := c.CurrentTimer(ctx, teamID)
		if err != nil {
			return logErr(err)
		}
		return timerMsg{timer: rt}
	}
}

// listTasksCmd loads the tasks of a list in the background.
func listTasksCmd(c *clickup.Client, listID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		tasks, err := c.ListTasks(ctx, listID)
		if err != nil {
			return logErr(err)
		}
		return taskListMsg{tasks: tasks}
	}
}

// createEntryCmd creates the time entry in the background.
func createEntryCmd(c *clickup.Client, teamID, tid string, start time.Time, dur time.Duration, desc string, billable bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := c.CreateTimeEntry(ctx, teamID, tid, start, dur, desc, billable); err != nil {
			return logErr(err)
		}
		return logDoneMsg{summary: fmt.Sprintf("%s logged on %s", duration.Format(dur), tid)}
	}
}

// enterForm initializes the form: duration field, date default = today.
func enterForm(lg logModel) logModel {
	lg.step = logForm
	lg.formField = 0
	lg.durStr = ""
	lg.dateStr = time.Now().Format("2006-01-02")
	lg.noteStr = ""
	lg.billable = true // billing-focused tool: billable by default
	lg.msg = ""
	lg.input = newTextInput("Duration (e.g. 2h30, 1.5h, 90m)")
	return lg
}

// tasksCmd / logCreateCmd / timerStartCmd / timerStopCmd / timerCurrentCmd
// pick the demo or real source for the log-hours flow, mirroring the
// reloadEntriesCmd / spacesCmd pattern: demo mode never touches m.client.
func (m Model) tasksCmd(listID string) tea.Cmd {
	if m.demo {
		return demoTasksCmd(listID)
	}
	return listTasksCmd(m.client, listID)
}

func (m Model) logCreateCmd(tid string, start time.Time, dur time.Duration, desc string, billable bool) tea.Cmd {
	if m.demo {
		return demoCreateEntryCmd(tid, dur)
	}
	return createEntryCmd(m.client, m.cfg.WorkspaceID, tid, start, dur, desc, billable)
}

func (m Model) timerStartCmd(tid, desc string) tea.Cmd {
	if m.demo {
		return demoStartTimerCmd(tid, m.now())
	}
	return startTimerCmd(m.client, m.cfg.WorkspaceID, tid, desc)
}

func (m Model) timerStopCmd() tea.Cmd {
	if m.demo {
		return demoStopTimerCmd()
	}
	return stopTimerCmd(m.client, m.cfg.WorkspaceID)
}

func (m Model) timerCurrentCmd() tea.Cmd {
	if m.demo {
		return demoCurrentTimerCmd(m.runningTimer)
	}
	return currentTimerCmd(m.client, m.cfg.WorkspaceID)
}

func (m Model) updateLog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lg := m.logScreen
	k := keysFor(m) // reflects lg.step/lg.formField as they stood before this keypress

	// Back returns to whoever pushed the log flow from any non-input step
	// (inputs handle Back locally in logIDInput/logForm).
	if key.Matches(msg, k.Back) && lg.step != logIDInput && lg.step != logForm {
		m = m.pop()
		return m, nil
	}

	switch lg.step {
	case logModeSelect:
		switch {
		case key.Matches(msg, k.PickGuided):
			lg.mode = modeGuided
			lg.step = logListPick
		case key.Matches(msg, k.PickByID):
			lg.mode = modeID
			lg.step = logIDInput
			lg.input = newTextInput("Task ID or URL")
		case key.Matches(msg, k.PickTimer):
			lg.mode = modeTimer
			lg.step = logTimerPick
			m.logScreen = lg
			return m, m.timerCurrentCmd()
		}

	case logTimerPick:
		switch {
		case key.Matches(msg, k.PickGuided):
			lg.step = logListPick
		case key.Matches(msg, k.PickByID):
			lg.step = logIDInput
			lg.input = newTextInput("Task ID or URL")
		}
		m.logScreen = lg
		return m, nil

	case logTimerRunning:
		if key.Matches(msg, k.StopTimer) {
			m.logScreen = lg
			m = m.replace(screenLoading)
			return m, m.timerStopCmd()
		}
		// Esc is caught by the outer guard above and pops back to whoever
		// pushed the log flow; the case that used to live here hardcoded
		// screenReport instead, which was unreachable dead code (the guard
		// always ran first) AND a latent wrong-destination bug (it would
		// ignore an origin of screenHome) — deleted rather than migrated,
		// see #59 Task 4 step 3.
		m.logScreen = lg
		return m, nil

	case logIDInput:
		if key.Matches(msg, k.Back) {
			lg.step = logModeSelect
			lg.msg = ""
			m.logScreen = lg
			return m, nil
		}
		if key.Matches(msg, k.Confirm) {
			id := clickup.ExtractTaskID(lg.input.Value())
			if id == "" {
				lg.msg = "Enter a valid id or URL"
				m.logScreen = lg
				return m, nil
			}
			lg.taskID = id
			lg.msg = ""
			if lg.mode == modeTimer {
				m.logScreen = lg
				m = m.replace(screenLoading)
				return m, m.timerStartCmd(id, "")
			}
			lg = enterForm(lg)
			lg.taskID = id
			m.logScreen = lg
			return m, nil
		}
		var cmd tea.Cmd
		lg.input, cmd = lg.input.Update(msg)
		m.logScreen = lg
		return m, cmd

	case logForm:
		if key.Matches(msg, k.Back) {
			m = m.pop()
			return m, nil
		}
		if lg.formField == 3 { // billable toggle (keypress, not a text field)
			switch {
			case key.Matches(msg, k.No):
				lg.billable = false
			case key.Matches(msg, k.Yes):
				lg.billable = true
			default:
				m.logScreen = lg
				return m, nil // ignore other keys
			}
			dur, _ := duration.Parse(lg.durStr)
			day, _ := time.Parse("2006-01-02", lg.dateStr)
			start := time.Date(day.Year(), day.Month(), day.Day(), 9, 0, 0, 0, time.Local)
			m.logScreen = lg
			m = m.replace(screenLoading)
			return m, m.logCreateCmd(lg.taskID, start, dur, lg.noteStr, lg.billable)
		}
		if key.Matches(msg, k.Confirm) {
			val := lg.input.Value()
			switch lg.formField {
			case 0: // duration
				if _, err := duration.Parse(val); err != nil {
					lg.msg = "Invalid duration (e.g. 2h30, 1.5h, 90m)"
					m.logScreen = lg
					return m, nil
				}
				lg.durStr = val
				lg.formField = 1
				lg.msg = ""
				lg.input = newTextInput("Date (YYYY-MM-DD)")
				lg.input.SetValue(lg.dateStr)
				m.logScreen = lg
				return m, nil
			case 1: // date
				if val == "" {
					val = lg.dateStr
				}
				if _, err := time.Parse("2006-01-02", val); err != nil {
					lg.msg = "Invalid date (format YYYY-MM-DD)"
					m.logScreen = lg
					return m, nil
				}
				lg.dateStr = val
				lg.formField = 2
				lg.msg = ""
				lg.input = newTextInput("Note (optional)")
				m.logScreen = lg
				return m, nil
			case 2: // note -> billable step
				lg.noteStr = lg.input.Value()
				lg.formField = 3
				lg.msg = ""
				m.logScreen = lg
				return m, nil
			}
		}
		var cmd tea.Cmd
		lg.input, cmd = lg.input.Update(msg)
		m.logScreen = lg
		return m, cmd

	case logListPick:
		browseIdx := len(lg.lists) // trailing "Browse all workspace lists…" row
		switch {
		case key.Matches(msg, k.Up):
			if lg.listIdx > 0 {
				lg.listIdx--
			}
		case key.Matches(msg, k.Down):
			if lg.listIdx < browseIdx {
				lg.listIdx++
			}
		case key.Matches(msg, k.Confirm):
			if lg.loading {
				break
			}
			if lg.listIdx == browseIdx {
				m.logScreen = lg
				return m.openListBrowser()
			}
			if len(lg.lists) > 0 {
				lg.loading = true
				m.logScreen = lg
				return m, m.tasksCmd(lg.lists[lg.listIdx].id)
			}
		}
		m.logScreen = lg
		return m, nil

	case logTaskPick:
		switch {
		case key.Matches(msg, k.Up):
			if lg.taskIdx > 0 {
				lg.taskIdx--
			}
		case key.Matches(msg, k.Down):
			if lg.taskIdx < len(lg.tasks)-1 {
				lg.taskIdx++
			}
		case key.Matches(msg, k.Confirm):
			if len(lg.tasks) > 0 {
				t := lg.tasks[lg.taskIdx]
				lg.taskID = t.ID
				if lg.mode == modeTimer {
					m.logScreen = lg
					m = m.replace(screenLoading)
					return m, m.timerStartCmd(t.ID, "")
				}
				id := t.ID
				lg = enterForm(lg)
				lg.taskID = id
				m.logScreen = lg
				return m, nil
			}
		}
		m.logScreen = lg
		return m, nil

	case logDone:
		switch {
		case key.Matches(msg, k.Reload):
			m = m.replace(screenLoading)
			// screenLog isn't a retryableErrMsg-aware origin: falls through to
			// the existing screenError, unchanged behavior (out of scope for #38).
			return m, m.reloadEntriesCmd(screenLog)
		case key.Matches(msg, k.Back), key.Matches(msg, k.Confirm):
			m = m.pop()
			return m, nil
		}
	}

	m.logScreen = lg
	return m, nil
}

func (lg logModel) view(th theme) string {
	b := th.Title.Render("Log hours") + "\n\n"
	switch lg.step {
	case logModeSelect:
		b += "Choose the mode:\n\n"
		b += "  " + th.Accent.Render("1") + ") Guided — pick list and task\n"
		b += "  " + th.Accent.Render("2") + ") Task ID/URL — straight to the form\n"
		b += "  " + th.Accent.Render("3") + ") Timer — start/stop stopwatch\n"
	case logTimerPick:
		b += "Timer — how do you pick the task?\n\n"
		b += "  " + th.Accent.Render("1") + ") Guided (list → task)\n"
		b += "  " + th.Accent.Render("2") + ") Task ID/URL\n"
	case logTimerRunning:
		if lg.timer == nil {
			b += th.Help.Render("No timer running.") + "\n"
			b += "\n" + th.Help.Render("Esc: back")
			break
		}
		b += "⏱  Timer running on: " + th.Accent.Render(lg.timer.TaskName) + "\n"
		if label := elapsedLabel(lg.timer.Start, lg.now); label != "" {
			b += th.Accent.Render(label) + "\n"
		} else {
			b += th.Help.Render("started just now") + "\n"
		}
		b += "\n" + th.Help.Render("s: stop and record · Esc: back")
	case logListPick:
		if lg.loading {
			b += th.Help.Render("Loading tasks…") + "\n\n"
		}
		b += "Choose the list:\n\n"
		for i, l := range lg.lists {
			cursor := "  "
			line := l.name
			if i == lg.listIdx {
				cursor = "▸ "
				line = th.Accent.Render(line)
			}
			b += cursor + line + "\n"
		}
		browseLine := "🔍 Browse all workspace lists…"
		if lg.listIdx == len(lg.lists) {
			b += "▸ " + th.Accent.Render(browseLine) + "\n"
		} else {
			b += "  " + browseLine + "\n"
		}
		b += "\n" + th.Help.Render("↑/↓ select · Enter: open tasks / browse")
	case logTaskPick:
		b += "Choose the task:\n\n"
		for i, tk := range lg.tasks {
			cursor := "  "
			line := truncate(tk.Name, 40)
			if i == lg.taskIdx {
				cursor = "▸ "
				line = th.Accent.Render(line)
			}
			b += cursor + line + "\n"
		}
		if len(lg.tasks) == 0 {
			b += th.Help.Render("No tasks in the list.") + "\n"
		}
		b += "\n" + th.Help.Render("↑/↓ select · Enter: continue")
	case logIDInput:
		b += "Task ID or URL:\n\n" + lg.input.View()
	case logForm:
		b += "Task: " + th.Accent.Render(lg.taskID) + "\n\n"
		if lg.formField == 3 {
			b += "Billable? " + th.Accent.Render("[Y/n]") + "   (Enter = yes)"
		} else {
			labels := []string{"Duration", "Date", "Note (optional)"}
			b += labels[lg.formField] + ":\n\n" + lg.input.View()
		}
	case logDone:
		b += th.OK.Render("✓ Hours logged.") + "\n\n"
		if lg.msg != "" {
			b += th.OK.Render(lg.msg) + "\n\n"
		}
		b += th.Help.Render("r: reload the report · Esc: back")
	default:
		b += th.Help.Render("…")
	}
	if lg.msg != "" && lg.step != logDone {
		b += "\n" + th.Err.Render(lg.msg)
	}
	b += "\n\n" + th.Help.Render("Esc: cancel · Ctrl+C: quit")
	return b
}
