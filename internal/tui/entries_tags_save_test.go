package tui

import (
	"slices"
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// openTagPicker is defined in entries_tags_test.go (Task 3) and reused here.

func TestNewTagAddsAndSelects(t *testing.T) {
	m := newTestModel()
	m.demo = true
	own := report.TimeEntry{ID: "e1", TaskName: "Fix", UserID: 1, Start: time.Now()}
	m = browserWithEntries(m, own)
	m = openTagPicker(m, []string{"focus"})

	// n enters new-tag mode; type "sprint-7"; enter adds+selects it.
	next, _ := m.Update(keyRunes("n"))
	m = next.(Model)
	if !m.entriesScreen.tagNewMode {
		t.Fatalf("n did not enter new-tag mode")
	}
	for _, r := range "sprint-7" {
		next, _ = m.Update(keyRunes(string(r)))
		m = next.(Model)
	}
	next, _ = m.Update(key("enter"))
	m = next.(Model)
	if m.entriesScreen.tagNewMode {
		t.Errorf("enter did not leave new-tag mode")
	}
	if !m.entriesScreen.tagSel["sprint-7"] {
		t.Errorf("new tag not selected")
	}
	found := false
	for _, s := range m.entriesScreen.tagAll {
		if s == "sprint-7" {
			found = true
		}
	}
	if !found {
		t.Errorf("new tag not added to tagAll")
	}
}

func TestSaveRecordsDemoOverrideAndReloads(t *testing.T) {
	m := newTestModel()
	m.demo = true
	m.loc = time.UTC
	own := report.TimeEntry{ID: "e1", TaskName: "Fix", UserID: 1, Start: time.Now(), EntryTags: []string{"focus"}}
	m = browserWithEntries(m, own)
	m = openTagPicker(m, []string{"focus", "client-A"})

	// toggle client-A on (cursor may be anywhere; set selection directly then save)
	m.entriesScreen.tagSel["client-A"] = true
	next, cmd := m.Update(key("enter")) // save
	mm := next.(Model)
	if cmd == nil {
		t.Fatalf("save did not dispatch a cmd")
	}
	ov, ok := mm.demoOverrides["e1"]
	if !ok {
		t.Fatalf("save did not record a demo override")
	}
	if !slices.Contains(ov.EntryTags, "client-A") || !slices.Contains(ov.EntryTags, "focus") {
		t.Errorf("override EntryTags = %v, want focus+client-A", ov.EntryTags)
	}
}
