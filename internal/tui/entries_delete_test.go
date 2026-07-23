package tui

import (
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// browserWithEntries builds a Model already parked on the entries browser
// (screenEntries) with the given entries, authenticated as userID 1 — the
// shared fixture for the delete-confirm tests below.
func browserWithEntries(m Model, es ...report.TimeEntry) Model {
	m.userID = 1
	m.screen = screenEntries
	m.entriesScreen = entriesModel{entries: es}
	return m
}

func TestDeleteRequiresConfirm(t *testing.T) {
	m := newTestModel()
	own := report.TimeEntry{ID: "e1", TaskName: "Fix", UserID: 1, Start: time.Now()}
	m = browserWithEntries(m, own)

	// x opens the confirm dialog, does not delete yet.
	m2, cmd := m.Update(key("x"))
	mm := m2.(Model)
	if mm.entriesScreen.mode != entriesConfirmDelete {
		t.Fatalf("x did not open confirm dialog: mode=%v", mm.entriesScreen.mode)
	}
	if cmd != nil {
		t.Errorf("x must not delete before confirm")
	}
	// n cancels back to the list.
	m3, _ := mm.Update(key("n"))
	if m3.(Model).entriesScreen.mode != entriesList {
		t.Errorf("n did not cancel")
	}
	// y confirms → a delete cmd is dispatched.
	m4, cmd4 := mm.Update(key("y"))
	if cmd4 == nil {
		t.Errorf("y did not dispatch delete")
	}
	_ = m4
}

func TestDeleteGatedOnOwnership(t *testing.T) {
	m := newTestModel()
	other := report.TimeEntry{ID: "e2", TaskName: "Deploy", UserID: 2, Start: time.Now()}
	m = browserWithEntries(m, other)
	m2, _ := m.Update(key("x"))
	if m2.(Model).entriesScreen.mode != entriesList {
		t.Errorf("x on a non-owned entry must be a no-op (stay in list)")
	}
}

func TestEntriesReloadedStaysInBrowserAndClampsCursor(t *testing.T) {
	m := newTestModel()
	m.screen = screenEntries
	m.entriesScreen = entriesModel{
		entries: []report.TimeEntry{{ID: "a", Start: time.Unix(2, 0)}, {ID: "b", Start: time.Unix(1, 0)}},
		idx:     1,
	}
	// reload with a single remaining entry: must stay on screenEntries, clamp idx.
	m2, _ := m.Update(entriesReloadedMsg{entries: []report.TimeEntry{{ID: "a", Start: time.Unix(2, 0)}}, status: "Entry deleted."})
	mm := m2.(Model)
	if mm.screen != screenEntries {
		t.Errorf("reload ejected the user from the browser: screen=%v", mm.screen)
	}
	if mm.entriesScreen.idx != 0 {
		t.Errorf("cursor not clamped: idx=%d", mm.entriesScreen.idx)
	}
	if mm.entriesScreen.msg != "Entry deleted." {
		t.Errorf("status not shown: %q", mm.entriesScreen.msg)
	}
}

// TestDemoDeleteExcludesEntryFromReload is the spec §11 demo-delete
// invariant: after a demo delete, EVERY reload path (not just the one that
// triggered the delete) must exclude the entry — demoEntriesSnapshot is the
// single source both demoEntriesCmd and the browser reload use.
func TestDemoDeleteExcludesEntryFromReload(t *testing.T) {
	m := Model{demo: true, cfg: demoConfig(), loc: time.UTC, scope: "me", year: 2026, month: time.July, now: time.Now}
	start, end := m.currentRange()
	before := m.demoEntriesSnapshot(start, end, nil)
	if len(before) == 0 {
		t.Fatalf("no demo entries in range, fixture assumption broken")
	}
	deletedID := before[0].ID

	m.demoDeleted = map[string]bool{deletedID: true}
	after := m.demoEntriesSnapshot(start, end, nil)
	for _, e := range after {
		if e.ID == deletedID {
			t.Fatalf("demo-deleted entry %q resurfaced after reload", deletedID)
		}
	}
	if len(after) != len(before)-1 {
		t.Errorf("expected exactly one fewer entry after delete: before=%d after=%d", len(before), len(after))
	}
}
