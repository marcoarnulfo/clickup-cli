package tui

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// demoEnabled reports whether demo mode is active (env CLICKUP_DEMO not empty).
// In demo mode the TUI skips setup and uses fake data: no API calls,
// useful for trying the app without an account and for generating the README GIF.
func demoEnabled() bool { return os.Getenv("CLICKUP_DEMO") != "" }

// demoConfig is a fake config for demo mode (no real token).
func demoConfig() config.Config {
	return config.Config{
		Token:       "DEMO",
		WorkspaceID: "demo",
		Currency:    "EUR",
		Rate:        50,
		Rates:       map[string]float64{"web": 65, "mobile": 45},
	}
}

// demoEntries returns fake time entries for the given month, spread across a few
// users so the member selection and per-member grouping are meaningful in demo.
func demoEntries(year int, month time.Month) []report.TimeEntry {
	at := func(d, h, m int) time.Time { return time.Date(year, month, d, h, m, 0, 0, time.UTC) }
	mk := func(id, taskID, task, listID, list string, uid int, user string, start time.Time, dur time.Duration) report.TimeEntry {
		return report.TimeEntry{
			ID: id, TaskID: taskID, TaskName: task,
			ListID: listID, ListName: list,
			UserID: uid, UserName: user,
			Start: start, Duration: dur,
		}
	}
	return []report.TimeEntry{
		mk("1", "t1", "Landing page redesign", "web", "Website", 1, "alice", at(3, 9, 0), 3*time.Hour+30*time.Minute),
		mk("2", "t2", "API integration", "web", "Website", 2, "bob", at(3, 14, 0), 2*time.Hour),
		mk("3", "t3", "Bugfix checkout", "web", "Website", 1, "alice", at(5, 10, 0), 1*time.Hour+15*time.Minute),
		mk("4", "t4", "Onboarding screens", "mobile", "Mobile app", 3, "carol", at(6, 9, 30), 4*time.Hour),
		mk("5", "t5", "Push notifications", "mobile", "Mobile app", 2, "bob", at(7, 11, 0), 2*time.Hour+45*time.Minute),
		mk("6", "t6", "Release QA", "mobile", "Mobile app", 3, "carol", at(10, 15, 0), 1*time.Hour+30*time.Minute),
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

// demoEntriesCmd delivers the fake entries as entriesMsg, filtered by the
// selected member ids (empty = all).
func demoEntriesCmd(year int, month time.Month, assignees []int) tea.Cmd {
	return func() tea.Msg {
		return entriesMsg{entries: filterByUsers(demoEntries(year, month), assignees)}
	}
}
