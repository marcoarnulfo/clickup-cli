package tui

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/duration"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// demoEnabled reports whether demo mode is active (env CLICKUP_DEMO not empty).
// In demo mode the TUI skips setup and uses fake data: no API calls,
// useful for trying the app without an account and for generating the README GIF.
func demoEnabled() bool { return os.Getenv("CLICKUP_DEMO") != "" }

// demoConfig is a fake config for demo mode (no real token). It fills in the
// v1.7 billing fields too (#51,#6,#53,#64) so the billable split,
// per-currency subtotals, member rate and budget burn-down all have
// something real to show; see demoEntries for the fixture they're billed
// against.
func demoConfig() config.Config {
	return config.Config{
		Token:       "DEMO",
		WorkspaceID: "demo",
		Currency:    "EUR",
		Rate:        50,
		Rates:       map[string]float64{"web": 65, "mobile": 45},
		// Pinned (M6): without it the TUI falls back to time.Local, and the
		// rendered report (and the README GIF recorded from it) would differ
		// per machine.
		Timezone: "UTC",
		Billing: config.Billing{
			DefaultCurrency: "EUR",
			// Carol is the senior on the mobile project: a flat per-member
			// rate that overrides the mobile list rate for her entries.
			RatesByMember: map[int]float64{3: 60},
			// Two clients, two invoicing currencies: the Website project
			// bills in EUR, the Mobile app project in USD.
			Currencies: map[string]string{"web": "EUR", "mobile": "USD"},
			// One budget so the burn-down view (#64) has something to show.
			Budgets: map[string]float64{"web": 450},
		},
	}
}

// demoSelfID is the demo "authenticated user": alice. Demo "me" scope filters
// to this id, mirroring the real API's server-side self-filter.
const demoSelfID = 1

// demoEntries returns fake time entries covering the half-open range
// [start, end), spread across a few users so the member selection and the
// per-member grouping are meaningful in demo.
//
// The fixture days are day offsets from start, wrapped modulo the range span
// (#4): anchoring them to a fixed set of month days would leave any range not
// covering those days empty — pressing `w` mid-month showed "No hours to
// show" — and would drop the other month's share of a range straddling two
// months. For a full month (start = the 1st) the offsets still land on days
// 2..10, so the month view is unchanged.
func demoEntries(start, end time.Time) []report.TimeEntry {
	loc := start.Location()
	span := int(end.Sub(start) / (24 * time.Hour))
	if span < 1 {
		span = 1
	}
	day0 := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
	// d is the 1-based day of the fixture within the range: kept as a day
	// number so the month layout reads the same as before.
	at := func(d, h, m int) time.Time {
		return day0.AddDate(0, 0, (d-1)%span).Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
	}
	mk := func(id, taskID, task, listID, list string, uid int, user string, tags []string, status string, start time.Time, dur time.Duration, billable bool) report.TimeEntry {
		return report.TimeEntry{
			ID: id, TaskID: taskID, TaskName: task,
			ListID: listID, ListName: list,
			UserID: uid, UserName: user,
			Tags: tags, Status: status,
			Start: start, Duration: dur,
			Billable: billable,
		}
	}
	return []report.TimeEntry{
		mk("1", "t1", "Landing page redesign", "web", "Website", 1, "alice", []string{"frontend"}, "in progress", at(3, 9, 0), 3*time.Hour+30*time.Minute, true),
		mk("2", "t2", "API integration", "web", "Website", 2, "bob", []string{"backend"}, "in progress", at(3, 14, 0), 2*time.Hour, true),
		mk("3", "t3", "Bugfix checkout", "web", "Website", 1, "alice", []string{"frontend", "qa"}, "done", at(5, 10, 0), 1*time.Hour+15*time.Minute, true),
		mk("4", "t4", "Onboarding screens", "mobile", "Mobile app", 3, "carol", []string{"frontend"}, "in progress", at(6, 9, 30), 4*time.Hour, true),
		mk("5", "t5", "Push notifications", "mobile", "Mobile app", 2, "bob", []string{"backend"}, "done", at(7, 11, 0), 2*time.Hour+45*time.Minute, true),
		mk("6", "t6", "Release QA", "mobile", "Mobile app", 3, "carol", []string{"qa"}, "done", at(10, 15, 0), 1*time.Hour+30*time.Minute, true),
		// Alice also has a foot in the mobile project, billed in USD: her own
		// ("me" scope) report shows both currencies, not just Website's EUR.
		mk("7", "t7", "Cross-platform sync", "mobile", "Mobile app", 1, "alice", []string{"frontend"}, "in progress", at(9, 11, 0), 2*time.Hour, true),
		// Internal, unbilled work: the billable/non-billable split (#51) needs
		// at least one non-billable entry to be visibly non-trivial.
		mk("8", "t8", "Sprint planning", "web", "Website", 1, "alice", []string{"planning"}, "done", at(2, 9, 0), 1*time.Hour, false),
	}
}

// filterByUsers keeps only entries whose UserID is in ids. An empty ids slice
// means "no filter" (all users) — matching the "empty selection = all" rule.
func filterByUsers(entries []report.TimeEntry, ids []int) []report.TimeEntry {
	if len(ids) == 0 {
		return entries
	}
	want := make(map[int]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	out := make([]report.TimeEntry, 0, len(entries))
	for _, e := range entries {
		if want[e.UserID] {
			out = append(out, e)
		}
	}
	return out
}

// demoMembers returns the fake workspace members for demo mode.
func demoMembers() []clickup.Member {
	return []clickup.Member{
		{ID: 1, Username: "alice"},
		{ID: 2, Username: "bob"},
		{ID: 3, Username: "carol"},
	}
}

// demoMembersCmd delivers the fake members as membersMsg (no I/O).
func demoMembersCmd() tea.Cmd {
	return func() tea.Msg { return membersMsg{members: demoMembers()} }
}

// demoStatusEnrichCmd returns the demo entries' statuses as a statusesMsg (no I/O).
func demoStatusEnrichCmd(entries []report.TimeEntry) tea.Cmd {
	return func() tea.Msg {
		byTask := make(map[string]string, len(entries))
		for _, e := range entries {
			byTask[e.TaskID] = e.Status
		}
		return statusesMsg{byTask: byTask}
	}
}

// demoEntriesCmd delivers the fake entries as entriesMsg, filtered by the
// selected member ids and clipped to [start, end).
func demoEntriesCmd(start, end time.Time, assignees []int) tea.Cmd {
	return func() tea.Msg {
		entries := filterByUsers(demoEntries(start, end), assignees)
		out := entries[:0]
		for _, e := range entries {
			if !e.Start.Before(start) && e.Start.Before(end) {
				out = append(out, e)
			}
		}
		return entriesMsg{entries: out}
	}
}

// demoSpaces / demoSpaceContents are fake workspace data for demo mode.
func demoSpaces() []clickup.Space {
	return []clickup.Space{{ID: "s-web", Name: "Web"}, {ID: "s-mobile", Name: "Mobile"}}
}

func demoSpaceContents(spaceID string) ([]clickup.Folder, []clickup.List) {
	switch spaceID {
	case "s-web":
		return []clickup.Folder{{ID: "f-site", Name: "Website", Lists: []clickup.List{{ID: "web", Name: "Website"}}}},
			[]clickup.List{{ID: "web-misc", Name: "Misc"}}
	case "s-mobile":
		return []clickup.Folder{{ID: "f-app", Name: "App", Lists: []clickup.List{{ID: "mobile", Name: "Mobile app"}}}}, nil
	default:
		return nil, nil
	}
}

func demoSpacesCmd() tea.Cmd {
	return func() tea.Msg { return spacesMsg{spaces: demoSpaces()} }
}

func demoSpaceContentsCmd(spaceID string) tea.Cmd {
	return func() tea.Msg {
		folders, folderless := demoSpaceContents(spaceID)
		return spaceContentsMsg{spaceID: spaceID, folders: folders, folderless: folderless}
	}
}

// demoTasks returns a small fixed set of fake tasks for demo mode. listID is
// accepted (mirroring the real ListTasks signature) but ignored: the fixture
// set is the same for any list, which is enough to make the picker useful.
func demoTasks(listID string) []clickup.Task {
	return []clickup.Task{
		{ID: "demo-t1", Name: "Fix login bug"},
		{ID: "demo-t2", Name: "Write onboarding docs"},
		{ID: "demo-t3", Name: "Refactor API client"},
	}
}

// demoTasksCmd delivers the fake tasks as taskListMsg (no I/O).
func demoTasksCmd(listID string) tea.Cmd {
	return func() tea.Msg { return taskListMsg{tasks: demoTasks(listID)} }
}

// demoCreateEntryCmd mirrors createEntryCmd's success summary, without ever
// calling the API.
func demoCreateEntryCmd(tid string, dur time.Duration) tea.Cmd {
	return func() tea.Msg {
		return logDoneMsg{summary: fmt.Sprintf("%s logged on %s", duration.Format(dur), tid)}
	}
}

// demoStartTimerCmd returns a fake running timer for tid (no I/O), started
// one minute before now. A non-zero Start is load-bearing: with a zero Start
// the demo timer screen's stopwatch would show "started just now" forever
// instead of ticking (see elapsedLabel).
func demoStartTimerCmd(tid string, now time.Time) tea.Cmd {
	return func() tea.Msg {
		return timerMsg{timer: &clickup.RunningTimer{TaskID: tid, TaskName: tid, Start: now.Add(-1 * time.Minute)}}
	}
}

// demoStopTimerCmd mirrors stopTimerCmd's success summary with a fixed fake
// duration, without ever calling the API.
func demoStopTimerCmd() tea.Cmd {
	return func() tea.Msg {
		return timerStoppedMsg{summary: fmt.Sprintf("timer stopped: %s logged", duration.Format(45*time.Minute))}
	}
}

// demoCurrentTimerCmd reports no timer running, which is the demo default
// (demo mode never has a timer already in flight when the screen opens).
func demoCurrentTimerCmd() tea.Cmd {
	return func() tea.Msg { return timerMsg{timer: nil} }
}

// demoRunningTimerProbeCmd reports a fake running timer for the global Home
// indicator, started 25 minutes before now so the stopwatch is visibly ticking.
// It is SEPARATE from demoCurrentTimerCmd (which returns nil — load-bearing for
// the log timer flow, where nil means "no timer, pick a task").
func demoRunningTimerProbeCmd(now time.Time) tea.Cmd {
	return func() tea.Msg {
		return runningTimerMsg{timer: &clickup.RunningTimer{
			TaskID:   "demo-t1",
			TaskName: "Fix login bug",
			Start:    now.Add(-25 * time.Minute),
		}}
	}
}
