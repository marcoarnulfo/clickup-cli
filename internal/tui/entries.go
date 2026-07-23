package tui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
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

	// edit form (Task 7), modeled on logModel's logForm sequence: Duration →
	// Date → Time → Note → Billable. editStep indexes the field being edited
	// (0=duration 1=date 2=time 3=note 4=billable); input is the shared text
	// field for steps 0-3 (billable is a keypress toggle, no text input).
	editStep     int
	editDur      string
	editDate     string
	editTime     string
	editNote     string
	editBillable bool
	editID       string
	input        textinput.Model

	// history (Task 8): read-only change list opened with 'h', for ANY entry
	// (not ownership-gated — it's read-only).
	historyChanges []clickup.HistoryChange
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

// enterEditForm seeds the edit form from es.entries[es.idx]: editDur from the
// current Duration (a duration.Parse-able string — FormatHours round-trips,
// e.g. "2.00h"), editDate/editTime from Start resolved into loc (now is only a
// fallback for a zero Start), editNote from Description (carried through by
// Step 3a so a blank note never wipes the ClickUp description), editBillable
// from Billable, editID from ID. Mirrors logModel's enterForm.
func enterEditForm(es entriesModel, now time.Time, loc *time.Location) entriesModel {
	e := es.entries[es.idx]
	start := e.Start
	if start.IsZero() {
		start = now
	}
	start = start.In(locOr(loc))
	es.mode = entriesEdit
	es.editStep = 0
	es.editDur = duration.FormatHours(e.Duration)
	es.editDate = start.Format("2006-01-02")
	es.editTime = start.Format("15:04")
	es.editNote = e.Description
	es.editBillable = e.Billable
	es.editID = e.ID
	es.msg = ""
	es.msgErr = false
	es.input = newTextInput("Duration (e.g. 2h30, 1.5h, 90m)")
	es.input.SetValue(es.editDur)
	return es
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
		case "e":
			if len(es.entries) > 0 && canEdit(es.entries[es.idx], m.userID) {
				es = enterEditForm(es, m.now(), m.loc)
			}
		case "h":
			// Read-only, not ownership-gated: history is allowed on ANY entry.
			if len(es.entries) > 0 {
				id := es.entries[es.idx].ID
				m.entriesScreen = es
				m.screen = screenLoading
				return m, m.historyCmd(id)
			}
		}
	case entriesHistory:
		if msg.String() == "esc" {
			es.mode = entriesList
		}
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
	case entriesEdit:
		return m.updateEntriesEdit(es, msg)
	}
	m.entriesScreen = es
	return m, nil
}

// updateEntriesEdit drives the multi-field edit form, mirroring logModel's
// logForm step handling in updateLog: Duration → Date → Time → Note →
// Billable, each step validating on Enter before advancing.
func (m Model) updateEntriesEdit(es entriesModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		es.mode = entriesList
		m.entriesScreen = es
		return m, nil
	}
	if es.editStep == 4 { // billable toggle (keypress, not a text field)
		switch msg.String() {
		case "n", "N":
			es.editBillable = false
		case "y", "Y", "enter":
			es.editBillable = true
		default:
			m.entriesScreen = es
			return m, nil // ignore other keys
		}
		return m.submitEntriesEdit(es)
	}
	if msg.Type == tea.KeyEnter {
		val := es.input.Value()
		switch es.editStep {
		case 0: // duration
			if _, err := duration.Parse(val); err != nil {
				es.msg = "Invalid duration (e.g. 2h30, 1.5h, 90m)"
				es.msgErr = true
				m.entriesScreen = es
				return m, nil
			}
			es.editDur = val
			es.editStep = 1
			es.msg = ""
			es.input = newTextInput("Date (YYYY-MM-DD)")
			es.input.SetValue(es.editDate)
			m.entriesScreen = es
			return m, nil
		case 1: // date
			if val == "" {
				val = es.editDate
			}
			if _, err := time.Parse("2006-01-02", val); err != nil {
				es.msg = "Invalid date (format YYYY-MM-DD)"
				es.msgErr = true
				m.entriesScreen = es
				return m, nil
			}
			es.editDate = val
			es.editStep = 2
			es.msg = ""
			es.input = newTextInput("Time (HH:MM)")
			es.input.SetValue(es.editTime)
			m.entriesScreen = es
			return m, nil
		case 2: // time
			if val == "" {
				val = es.editTime
			}
			if _, err := time.Parse("15:04", val); err != nil {
				es.msg = "Invalid time (format HH:MM)"
				es.msgErr = true
				m.entriesScreen = es
				return m, nil
			}
			es.editTime = val
			es.editStep = 3
			es.msg = ""
			es.input = newTextInput("Note (optional)")
			es.input.SetValue(es.editNote)
			m.entriesScreen = es
			return m, nil
		case 3: // note -> billable step
			es.editNote = es.input.Value()
			es.editStep = 4
			es.msg = ""
			m.entriesScreen = es
			return m, nil
		}
	}
	var cmd tea.Cmd
	es.input, cmd = es.input.Update(msg)
	m.entriesScreen = es
	return m, cmd
}

// submitEntriesEdit finalizes the edit form: parses the fields, records the
// demo override on the Model BEFORE building the cmd (same pattern as
// delete), and dispatches updateEntryCmd.
func (m Model) submitEntriesEdit(es entriesModel) (tea.Model, tea.Cmd) {
	day, _ := time.Parse("2006-01-02", es.editDate)
	hh, mm2 := 9, 0
	if tm, err := time.Parse("15:04", es.editTime); err == nil {
		hh, mm2 = tm.Hour(), tm.Minute()
	}
	start := time.Date(day.Year(), day.Month(), day.Day(), hh, mm2, 0, 0, locOr(m.loc))
	dur, _ := duration.Parse(es.editDur)
	id := es.editID
	note := es.editNote
	billable := es.editBillable
	base := es.entries[es.idx] // the pre-edit entry (owner/task/etc. preserved)
	es.mode = entriesList
	m.entriesScreen = es
	if m.demo { // record the demo edit on the Model BEFORE building the cmd
		if m.demoOverrides == nil {
			m.demoOverrides = map[string]report.TimeEntry{}
		}
		base.Start, base.Duration, base.Description, base.Billable = start, dur, note, billable
		m.demoOverrides[id] = base
	}
	m.screen = screenLoading
	return m, m.updateEntryCmd(id, start, dur, note, billable)
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

// updateEntryCmd updates entry id (real API) or relies on the demo override
// already recorded on the Model (see submitEntriesEdit, which sets
// m.demoOverrides before this cmd is built) and reloads the browser either
// way. moved/status are computed against the CURRENT range (m.currentRange,
// [start,end) in m.loc) before the goroutine, since the closure captures mm
// by value: an edit that moves the entry outside the visible range still
// succeeds, just with a status line saying so instead of silently vanishing.
func (m Model) updateEntryCmd(id string, start time.Time, dur time.Duration, note string, billable bool) tea.Cmd {
	mm := m
	s, e := m.currentRange()
	moved := start.Before(s) || !start.Before(e)
	status := "Entry saved."
	if moved {
		status = "Entry saved — it moved outside the current range."
	}
	if m.demo { // demoOverrides already recorded in submitEntriesEdit
		return func() tea.Msg { return reloadForBrowser(mm, status) }
	}
	c := m.client
	teamID := m.cfg.WorkspaceID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := c.UpdateTimeEntry(ctx, teamID, id, start, dur, note, billable); err != nil {
			return entriesErr(err)
		}
		return reloadForBrowser(mm, status)
	}
}

// historyCmd fetches entry id's change history (real: clickup.TimeEntryHistory;
// demo: demoHistoryCmd) and returns historyMsg on success. A fetch error routes
// through entriesErr (Task 6) so it shows inline in the browser, never a
// dead-end screenError — history is read-only and not worth losing the
// browser over.
func (m Model) historyCmd(id string) tea.Cmd {
	if m.demo {
		return demoHistoryCmd()
	}
	c := m.client
	teamID := m.cfg.WorkspaceID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		changes, err := c.TimeEntryHistory(ctx, teamID, id)
		if err != nil {
			return entriesErr(err)
		}
		return historyMsg{changes: changes}
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
	if es.mode == entriesEdit {
		return entriesEditView(es)
	}
	if es.mode == entriesHistory {
		return entriesHistoryView(es, m.loc)
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

// entriesEditView renders the multi-field edit form, mirroring logModel's
// logForm rendering (see log.go view()).
func entriesEditView(es entriesModel) string {
	b := styleTitle.Render("Edit entry") + "\n\n"
	if es.editStep == 4 {
		b += "Billable? " + styleAccent.Render("[Y/n]") + "   (Enter = yes)"
	} else {
		labels := []string{"Duration", "Date (YYYY-MM-DD)", "Time (HH:MM)", "Note (optional)"}
		b += labels[es.editStep] + ":\n\n" + es.input.View()
	}
	if es.msg != "" {
		b += "\n" + styleErr.Render(es.msg)
	}
	b += "\n\n" + styleHelp.Render("Esc: cancel")
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
