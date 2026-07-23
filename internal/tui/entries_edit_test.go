package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// keyRunes builds a KeyRunes message for s (a single printable key), separate
// from the special-cased key() helper in log_test.go so 'e'/'q' etc. read as
// plain typed input, not a special key.
func keyRunes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestEditOpensPrefilledForm(t *testing.T) {
	m := newTestModel()
	m.now = fixedNow(time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC))
	own := report.TimeEntry{ID: "e1", TaskName: "Fix", UserID: 1, Billable: true,
		Start: time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC), Duration: 2 * time.Hour}
	m = browserWithEntries(m, own)
	m2, _ := m.Update(keyRunes("e"))
	mm := m2.(Model)
	if mm.entriesScreen.mode != entriesEdit {
		t.Fatalf("e did not open edit form: mode=%v", mm.entriesScreen.mode)
	}
	// prefill: duration field seeded from the current entry.
	if mm.entriesScreen.editDur == "" {
		t.Errorf("duration not prefilled")
	}
}

func TestEditGatedOnOwnership(t *testing.T) {
	m := newTestModel()
	other := report.TimeEntry{ID: "e2", UserID: 2, Start: time.Now()}
	m = browserWithEntries(m, other)
	m2, _ := m.Update(keyRunes("e"))
	if m2.(Model).entriesScreen.mode != entriesList {
		t.Errorf("e on non-owned entry must be a no-op")
	}
}

// TestEditPrefillCarriesDescriptionAndBillable is the CRITICAL-detail #1
// regression guard: without carrying Description through report.TimeEntry, a
// blank editNote would silently wipe the ClickUp description on every save.
func TestEditPrefillCarriesDescriptionAndBillable(t *testing.T) {
	m := newTestModel()
	m.now = fixedNow(time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC))
	own := report.TimeEntry{ID: "e1", TaskName: "Fix", UserID: 1, Billable: false,
		Start: time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC), Duration: 90 * time.Minute,
		Description: "already documented"}
	m = browserWithEntries(m, own)
	m2, _ := m.Update(keyRunes("e"))
	mm := m2.(Model)
	if mm.entriesScreen.editNote != "already documented" {
		t.Errorf("editNote = %q, want the entry's description", mm.entriesScreen.editNote)
	}
	if mm.entriesScreen.editBillable {
		t.Errorf("editBillable = true, want the entry's Billable=false carried through")
	}
	if mm.entriesScreen.editID != "e1" {
		t.Errorf("editID = %q, want e1", mm.entriesScreen.editID)
	}
}

// clearAndType clears the prefilled input (ctrl+u: delete-before-cursor,
// where SetValue parks the cursor) and types s rune by rune, mirroring a user
// replacing a prefilled field.
func clearAndType(m Model, s string) Model {
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m = next.(Model)
	for _, r := range s {
		next, _ = m.Update(keyRunes(string(r)))
		m = next.(Model)
	}
	return m
}

// submitEdit drives the edit form to the final billable step and confirms it
// (Enter = yes), returning the resulting Model and the dispatched cmd.
func submitEdit(m Model, dur, date, timeStr, note string) (Model, tea.Cmd) {
	next, _ := m.Update(keyRunes("e"))
	m = next.(Model)
	m = clearAndType(m, dur)
	next, _ = m.Update(key("enter"))
	m = next.(Model)
	m = clearAndType(m, date)
	next, _ = m.Update(key("enter"))
	m = next.(Model)
	m = clearAndType(m, timeStr)
	next, _ = m.Update(key("enter"))
	m = next.(Model)
	m = clearAndType(m, note)
	next, cmd := m.Update(key("enter")) // note -> billable step
	m = next.(Model)
	next, cmd = m.Update(key("enter")) // billable: Enter = yes
	return next.(Model), cmd
}

// TestDemoEditPersistsAcrossReload is the spec §11 demo-edit invariant: after
// a demo edit, demoOverrides is recorded BEFORE the reload cmd is built, and
// demoEntriesSnapshot (Task 6) must return the edited values on every reload,
// not just the one that triggered the edit.
func TestDemoEditPersistsAcrossReload(t *testing.T) {
	m := Model{demo: true, cfg: demoConfig(), loc: time.UTC, scope: "me", year: 2026, month: time.July, now: fixedNow(time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC))}
	start, end := m.currentRange()
	before := m.demoEntriesSnapshot(start, end, nil)
	if len(before) == 0 {
		t.Fatalf("no demo entries in range, fixture assumption broken")
	}
	own := before[0]
	if own.UserID != demoSelfID {
		t.Fatalf("fixture assumption broken: entry 0 is not owned by demoSelfID")
	}
	m.userID = demoSelfID
	m.screen = screenEntries
	m.entriesScreen = entriesModel{entries: []report.TimeEntry{own}}

	mm, cmd := submitEdit(m, "3h", "2026-07-21", "10:00", "updated note")
	if cmd == nil {
		t.Fatalf("edit did not dispatch a cmd")
	}
	if _, ok := mm.demoOverrides[own.ID]; !ok {
		t.Fatalf("demoOverrides not recorded for %q", own.ID)
	}

	// Every reload — not just the one that triggered the edit — must reflect it.
	start2, end2 := mm.currentRange()
	after := mm.demoEntriesSnapshot(start2, end2, nil)
	var found *report.TimeEntry
	for i := range after {
		if after[i].ID == own.ID {
			found = &after[i]
		}
	}
	if found == nil {
		t.Fatalf("edited entry %q missing from post-edit snapshot", own.ID)
	}
	if found.Duration != 3*time.Hour {
		t.Errorf("Duration = %v, want 3h", found.Duration)
	}
	if found.Description != "updated note" {
		t.Errorf("Description = %q, want %q", found.Description, "updated note")
	}
}

// TestEditNoteAcceptsQ is the spec §11 typability guard: 'q' is a global quit
// key everywhere except a short exception list that already includes
// screenEntries — the edit form's note field must still receive the rune
// instead of quitting the app.
func TestEditNoteAcceptsQ(t *testing.T) {
	m := newTestModel()
	own := report.TimeEntry{ID: "e1", TaskName: "Fix", UserID: 1, Start: time.Now(), Duration: time.Hour}
	m = browserWithEntries(m, own)

	next, _ := m.Update(keyRunes("e"))
	m = next.(Model)
	// advance duration -> date -> time -> note
	next, _ = m.Update(key("enter"))
	m = next.(Model)
	next, _ = m.Update(key("enter"))
	m = next.(Model)
	next, _ = m.Update(key("enter"))
	m = next.(Model)
	if m.entriesScreen.editStep != 3 {
		t.Fatalf("editStep = %d, want 3 (note)", m.entriesScreen.editStep)
	}

	next, _ = m.Update(keyRunes("q"))
	mm := next.(Model)
	if mm.entriesScreen.input.Value() != "q" {
		t.Errorf("q did not reach the note input: value=%q", mm.entriesScreen.input.Value())
	}
	if mm.screen != screenEntries {
		t.Errorf("q quit the app instead of being typed: screen=%v", mm.screen)
	}
}
