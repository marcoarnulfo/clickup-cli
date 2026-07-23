package tui

import (
	"fmt"
	"slices"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/duration"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

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
		}
		// e/x/h wired in Tasks 6–8.
	}
	m.entriesScreen = es
	return m, nil
}

func (m Model) entriesView() string {
	es := m.entriesScreen
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
