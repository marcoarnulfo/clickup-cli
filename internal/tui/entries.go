package tui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/duration"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
	"github.com/marcoarnulfo/clickup-cli/internal/service"
)

// entriesReloadedMsg is a successful browser-action reload (delete/edit):
// the fresh entries plus a status line to show inline, without leaving
// screenEntries.
type entriesReloadedMsg struct {
	entries []report.TimeEntry
	status  string
}

// entriesErrMsg is a browser-action error that keeps the user in the entries
// browser (message shown inline) instead of bouncing to screenLog/screenError.
type entriesErrMsg struct{ err error }

// entriesErr routes a browser-action error: auth errors still go global
// (errMsg → re-setup); everything else stays on the browser.
func entriesErr(err error) tea.Msg {
	if errors.Is(err, clickup.ErrUnauthorized) {
		return errMsg{err: err}
	}
	return entriesErrMsg{err: err}
}

type entriesMode int

const (
	entriesList          entriesMode = iota
	entriesConfirmDelete             // Task 6
	entriesEdit                      // Task 7
	entriesHistory                   // Task 8
)

type entriesModel struct {
	entries []report.TimeEntry // sorted snapshot (Start desc), filter-applied
	idx     int
	mode    entriesMode
	msg     string
	msgErr  bool // true → render msg with styleErr; false → styleOK (success)
}

// canEdit reports whether the authenticated user owns the entry. The userID != 0
// guard is required: rawEntry.User.ID defaults to 0 when absent, so a zero
// self-id must never match a zero-owner entry.
func canEdit(e report.TimeEntry, userID int) bool {
	return userID != 0 && e.UserID == userID
}

// sortEntriesByStartDesc returns a NEW slice sorted by Start descending; it never
// mutates the input (which feeds report.Build).
func sortEntriesByStartDesc(in []report.TimeEntry) []report.TimeEntry {
	out := slices.Clone(in)
	slices.SortStableFunc(out, func(a, b report.TimeEntry) int {
		return b.Start.Compare(a.Start)
	})
	return out
}

// openEntries builds the browser from the filter-applied entries. (The lazy
// userID retry, when m.userID == 0, is dispatched by the caller in updateReport.)
func (m Model) openEntries() Model {
	m.entriesScreen = entriesModel{entries: sortEntriesByStartDesc(m.visibleEntries())}
	m.screen = screenEntries
	return m
}

func (m Model) updateEntries(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	es := m.entriesScreen
	switch es.mode {
	case entriesList:
		switch msg.String() {
		case "esc":
			m.screen = screenReport
			return m, nil
		case "up", "k":
			if es.idx > 0 {
				es.idx--
			}
		case "down", "j":
			if es.idx < len(es.entries)-1 {
				es.idx++
			}
		case "x":
			if len(es.entries) > 0 && canEdit(es.entries[es.idx], m.userID) {
				es.mode = entriesConfirmDelete
			}
		}
		// e/h wired in Tasks 7–8.
	case entriesConfirmDelete:
		switch msg.String() {
		case "y", "Y":
			id := es.entries[es.idx].ID
			es.mode = entriesList
			m.entriesScreen = es
			if m.demo { // record the demo deletion on the Model BEFORE building the cmd
				if m.demoDeleted == nil {
					m.demoDeleted = map[string]bool{}
				}
				m.demoDeleted[id] = true
			}
			m.screen = screenLoading
			return m, m.deleteEntryCmd(id)
		default: // any other key cancels
			es.mode = entriesList
		}
	}
	m.entriesScreen = es
	return m, nil
}

// deleteEntryCmd deletes entry id (real API) or records the demo deletion (see
// the entriesConfirmDelete "y" handler, which sets m.demoDeleted before this
// cmd is built) and reloads the browser either way.
func (m Model) deleteEntryCmd(id string) tea.Cmd {
	mm := m // capture by value at dispatch time (range/scope can't change while the browser is open)
	if m.demo {
		return func() tea.Msg { return reloadForBrowser(mm, "Entry deleted.") }
	}
	c := m.client
	teamID := m.cfg.WorkspaceID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := c.DeleteTimeEntry(ctx, teamID, id); err != nil {
			return entriesErr(err)
		}
		return reloadForBrowser(mm, "Entry deleted.")
	}
}

// reloadForBrowser re-reads the entries synchronously (inside a cmd goroutine)
// and wraps them in entriesReloadedMsg. Real mode uses service.LoadEntries;
// demo mode goes through demoEntriesSnapshot so demoDeleted/demoOverrides are
// honored (a plain reload must not resurrect a demo-deleted entry).
func reloadForBrowser(m Model, status string) tea.Msg {
	start, end := m.currentRange()
	assignees := m.reloadAssignees()
	if m.demo {
		return entriesReloadedMsg{entries: m.demoEntriesSnapshot(start, end, assignees), status: status}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	entries, err := service.LoadEntries(ctx, m.client, m.cfg.WorkspaceID, start, end, m.scope, assignees)
	if err != nil {
		return entriesErr(err)
	}
	return entriesReloadedMsg{entries: entries, status: status}
}

func (m Model) entriesView() string {
	es := m.entriesScreen
	if es.mode == entriesConfirmDelete && len(es.entries) > 0 {
		e := es.entries[es.idx]
		return styleTitle.Render("Delete entry") + "\n\n" +
			"Delete " + styleAccent.Render(truncate(e.TaskName, 40)) + " (" +
			duration.FormatHours(e.Duration) + ")?\n\n" +
			styleHelp.Render("y: delete · any other key: cancel")
	}
	b := styleTitle.Render("Entries") + "\n\n"
	if len(es.entries) == 0 {
		b += styleHelp.Render("No entries in the current range.") + "\n"
		b += "\n" + styleHelp.Render("Esc: back to the report")
		return b
	}
	for i, e := range es.entries {
		cursor := "  "
		if i == es.idx {
			cursor = "▸ "
		}
		when := e.Start.In(locOr(m.loc)).Format("02 Jan 15:04")
		bill := " "
		if e.Billable {
			bill = "$"
		}
		owner := "(" + ownerLabel(e, m.userID) + ")"
		line := fmt.Sprintf("%s  %-24s %6s %s  %s",
			when, truncate(e.TaskName, 24), duration.FormatHours(e.Duration), bill, owner)
		if !canEdit(e, m.userID) {
			line += " — read-only"
		}
		if i == es.idx {
			line = styleAccent.Render(line)
		}
		b += cursor + line + "\n"
	}
	b += "\n" + styleHelp.Render("↑/↓ select · e: edit · x: delete · h: history · Esc: back")
	if es.msg != "" {
		style := styleOK
		if es.msgErr {
			style = styleErr
		}
		b += "\n" + style.Render(es.msg)
	}
	return b
}

// ownerLabel is "me" for the authenticated user, else the entry's username.
func ownerLabel(e report.TimeEntry, userID int) string {
	if userID != 0 && e.UserID == userID {
		return "me"
	}
	if e.UserName != "" {
		return e.UserName
	}
	return fmt.Sprintf("user %d", e.UserID)
}

// locOr returns loc, or time.Local when nil (mirrors the Model's loc handling).
func locOr(loc *time.Location) *time.Location {
	if loc == nil {
		return time.Local
	}
	return loc
}
