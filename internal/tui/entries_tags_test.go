package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func TestTOpensTagPickerGatedOnOwnership(t *testing.T) {
	m := newTestModel()
	own := report.TimeEntry{ID: "e1", TaskName: "Fix", UserID: 1, Start: time.Now(), EntryTags: []string{"focus"}}
	m = browserWithEntries(m, own)
	m2, cmd := m.Update(keyRunes("t"))
	mm := m2.(Model)
	if mm.entriesScreen.mode != entriesTags {
		t.Fatalf("t did not open the tag picker: mode=%v", mm.entriesScreen.mode)
	}
	if !mm.entriesScreen.tagLoading || cmd == nil {
		t.Errorf("t should start the tag fetch (loading + a cmd)")
	}
	// current tags seed the selection
	if !mm.entriesScreen.tagSel["focus"] {
		t.Errorf("current EntryTags did not seed tagSel")
	}
}

func TestTGatedOnOwnership(t *testing.T) {
	m := newTestModel()
	other := report.TimeEntry{ID: "e2", UserID: 2, Start: time.Now()}
	m = browserWithEntries(m, other)
	m2, _ := m.Update(keyRunes("t"))
	if m2.(Model).entriesScreen.mode != entriesList {
		t.Errorf("t on a non-owned entry must be a no-op")
	}
}

// openTagPicker opens the tag picker on the current entry and delivers the
// fetched workspace tags (shared by this file and the Task 4 save tests).
func openTagPicker(m Model, fetched []string) Model {
	next, _ := m.Update(keyRunes("t"))
	m = next.(Model)
	next, _ = m.Update(tagsMsg{tags: fetched})
	return next.(Model)
}

func TestTagsMsgPopulatesAndUnionsCurrent(t *testing.T) {
	m := newTestModel()
	own := report.TimeEntry{ID: "e1", TaskName: "Fix", UserID: 1, Start: time.Now(), EntryTags: []string{"focus", "legacy"}}
	m = browserWithEntries(m, own)
	m = openTagPicker(m, []string{"focus", "client-A"})
	es := m.entriesScreen
	if es.tagLoading {
		t.Errorf("tagsMsg should clear loading")
	}
	// tagAll = fetched ∪ current, deduped: focus, client-A, legacy
	joined := strings.Join(es.tagAll, ",")
	for _, want := range []string{"focus", "client-A", "legacy"} {
		if !strings.Contains(joined, want) {
			t.Errorf("tagAll %q missing %q (should union current tags)", joined, want)
		}
	}
}

func TestSpaceTogglesTag(t *testing.T) {
	m := newTestModel()
	own := report.TimeEntry{ID: "e1", TaskName: "Fix", UserID: 1, Start: time.Now(), EntryTags: []string{"focus"}}
	m = browserWithEntries(m, own)
	m = openTagPicker(m, []string{"focus", "client-A"})
	// cursor on the first tag; space toggles it off. key(" ") delivers a space
	// whose String() == " " (key("space") would send the 5-rune string "space").
	before := m.entriesScreen.tagSel[m.entriesScreen.tagAll[0]]
	m2, _ := m.Update(key(" "))
	after := m2.(Model).entriesScreen.tagSel[m.entriesScreen.tagAll[0]]
	if before == after {
		t.Errorf("space did not toggle the tag under the cursor")
	}
}
