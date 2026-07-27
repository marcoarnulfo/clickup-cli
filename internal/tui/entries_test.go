package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func TestCanEdit(t *testing.T) {
	e := report.TimeEntry{UserID: 5}
	if canEdit(e, 0) {
		t.Error("userID 0 must gate everything read-only")
	}
	if canEdit(report.TimeEntry{UserID: 0}, 0) {
		t.Error("zero-owner vs zero-self must not match")
	}
	if !canEdit(e, 5) {
		t.Error("owner should be editable")
	}
	if canEdit(e, 6) {
		t.Error("non-owner should be read-only")
	}
}

func TestSortEntriesByStartDesc(t *testing.T) {
	a := report.TimeEntry{ID: "a", Start: time.Unix(100, 0)}
	b := report.TimeEntry{ID: "b", Start: time.Unix(200, 0)}
	got := sortEntriesByStartDesc([]report.TimeEntry{a, b})
	if got[0].ID != "b" || got[1].ID != "a" {
		t.Errorf("order = %v, want b,a", []string{got[0].ID, got[1].ID})
	}
}

func TestVOpensEntriesBrowser(t *testing.T) {
	m := newTestModel()
	m.screen = screenReport
	m.userID = 1
	m.entries = []report.TimeEntry{{ID: "e1", TaskName: "Fix", Start: time.Now(), Duration: time.Hour, UserID: 1, Billable: true}}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	mm := m2.(Model)
	if mm.screen != screenEntries {
		t.Fatalf("v did not open entries browser: screen=%v", mm.screen)
	}
	if !strings.Contains(mm.View(), "Fix") {
		t.Errorf("browser missing the entry:\n%s", mm.View())
	}
}

// ---------------------------------------------------------- keymap parity --

// entriesFixture builds a Model on screenEntries with one entry, owned by
// userID (or by someone else when owned is false).
func entriesFixture(owned bool) Model {
	owner := 1
	if !owned {
		owner = 99
	}
	m := newTestModel()
	m.screen = screenEntries
	m.nav = []screen{screenReport} // the entries browser is only ever reached from Report
	m.userID = 1
	m.entriesScreen = entriesModel{entries: []report.TimeEntry{{ID: "e1", UserID: owner, TaskName: "X"}}}
	return m
}

// TestEntriesListEscReturnsToReport is #59 Task 4 step 4's required
// transition test: entries-list's Back must still route to screenReport now
// that it is dispatched via key.Matches(msg, k.Back) instead of
// msg.String() == "esc".
func TestEntriesListEscReturnsToReport(t *testing.T) {
	t.Parallel()
	m := entriesFixture(true)
	next, _ := m.updateEntries(keyMsg("esc"))
	nm := next.(Model)
	if nm.screen != screenReport {
		t.Errorf("esc at entriesList should return to screenReport, got %v", nm.screen)
	}
}

// TestEntriesListKeyLabels pins the label set entries.go's updateEntries
// accepts in entriesList mode today, with an owned entry selected (so
// Delete/Edit/Tags are enabled — see TestEntriesEditKeyMatchesOwnershipGuard
// for the ownership gate and TestEntriesHistoryKeyIsNotOwnershipGated for why
// History isn't included in that gate).
func TestEntriesListKeyLabels(t *testing.T) {
	t.Parallel()
	m := entriesFixture(true)
	want := []string{"?", "ctrl+c", "down", "e", "esc", "h", "j", "k", "t", "up", "x"}
	if got := enabledLabels(keysFor(m)); !slices.Equal(got, want) {
		t.Errorf("entries list-mode labels = %v, want %v", got, want)
	}
}

// TestEntriesListEmptyKeyLabels pins the label set with no entries loaded:
// Delete/Edit/Tags/History all require at least one row (they index
// es.entries[es.idx]), so only navigation and Back survive.
func TestEntriesListEmptyKeyLabels(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	m.screen = screenEntries
	m.entriesScreen = entriesModel{}
	want := []string{"?", "ctrl+c", "down", "esc", "j", "k", "up"}
	if got := enabledLabels(keysFor(m)); !slices.Equal(got, want) {
		t.Errorf("entries empty-list labels = %v, want %v", got, want)
	}
}

// TestEntriesEditKeyMatchesOwnershipGuard is #59 Task 4 step 4's required
// enablement⇔guard test: Edit's (and, by the same gate, Delete's/Tags') keymap
// enablement must never disagree with canEdit, the guard entries.go's
// handlers used to check inline.
func TestEntriesEditKeyMatchesOwnershipGuard(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		userID int
		want   bool
	}{
		{"own entry", 1, true},
		{"someone else's entry", 99, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel()
			m.screen = screenEntries
			m.userID = tc.userID
			m.entries = []report.TimeEntry{{ID: "e1", UserID: 1, TaskName: "X"}}
			m.entriesScreen = entriesModel{entries: m.entries}
			if got := keysFor(m).Edit.Enabled(); got != tc.want {
				t.Errorf("Edit binding enabled = %v, want %v", got, tc.want)
			}
			if got := canEdit(m.entries[0], m.userID); got != tc.want {
				t.Errorf("canEdit = %v, want %v — keymap and guard disagree", got, tc.want)
			}
		})
	}
}

// TestEntriesHistoryKeyIsNotOwnershipGated is #59 Task 4 step 4's required
// enablement⇔guard test for 'h': unlike Delete/Edit/Tags, History is
// read-only and allowed on ANY entry (entries.go:162-169) — only a non-empty
// list gates it.
func TestEntriesHistoryKeyIsNotOwnershipGated(t *testing.T) {
	t.Parallel()
	m := entriesFixture(false) // someone else's entry
	if !keysFor(m).History.Enabled() {
		t.Error("History binding disabled for a non-owned entry; it must not be ownership-gated")
	}
	m.entriesScreen.entries = nil
	if keysFor(m).History.Enabled() {
		t.Error("History binding enabled with no entries loaded")
	}
}

// TestEntriesConfirmDeleteKeyLabels pins entriesConfirmDelete's label set:
// only y/Y answer yes — "any other key cancels" (entries.go:272) is the
// switch's default clause, which no binding can express (#59 Task 4 step 2).
func TestEntriesConfirmDeleteKeyLabels(t *testing.T) {
	t.Parallel()
	m := entriesFixture(true)
	m.entriesScreen.mode = entriesConfirmDelete
	want := []string{"Y", "ctrl+c", "y"}
	if got := enabledLabels(keysFor(m)); !slices.Equal(got, want) {
		t.Errorf("entries confirm-delete labels = %v, want %v", got, want)
	}
}

// TestEntriesHistoryModeKeyLabels pins the read-only history view's label set.
func TestEntriesHistoryModeKeyLabels(t *testing.T) {
	t.Parallel()
	m := entriesFixture(true)
	m.entriesScreen.mode = entriesHistory
	want := []string{"?", "ctrl+c", "esc"}
	if got := enabledLabels(keysFor(m)); !slices.Equal(got, want) {
		t.Errorf("entries history-mode labels = %v, want %v", got, want)
	}
}

// TestEntriesTagsKeyLabels pins the tag picker's label set (tagNewMode off).
func TestEntriesTagsKeyLabels(t *testing.T) {
	t.Parallel()
	m := entriesFixture(true)
	m.entriesScreen.mode = entriesTags
	want := []string{" ", "?", "ctrl+c", "down", "enter", "esc", "j", "k", "n", "up"}
	if got := enabledLabels(keysFor(m)); !slices.Equal(got, want) {
		t.Errorf("entries tags labels = %v, want %v", got, want)
	}
}

// TestEntriesTagsNewModeKeyLabels pins the "type a new tag name" sub-mode:
// only Enter/Esc are app-level bindings, everything else forwards to the
// textinput (same shape as rates'/setup's free-text steps).
func TestEntriesTagsNewModeKeyLabels(t *testing.T) {
	t.Parallel()
	m := entriesFixture(true)
	m.entriesScreen.mode = entriesTags
	m.entriesScreen.tagNewMode = true
	want := []string{"ctrl+c", "enter", "esc"}
	if got := enabledLabels(keysFor(m)); !slices.Equal(got, want) {
		t.Errorf("entries tags new-mode labels = %v, want %v", got, want)
	}
}

// TestEntriesEditKeyLabelsPerStep pins the edit form's label set at each
// editStep: 0-3 (duration/date/time/note) forward everything but Enter/Esc to
// the textinput; 4 (billable) is the one sub-step with a different set
// (Yes/No instead of Confirm) — mirrors TestSetupKeyLabelsPerStep's shape for
// setup.go's wizard steps.
func TestEntriesEditKeyLabelsPerStep(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		step int
		want []string
	}{
		{"duration", 0, []string{"ctrl+c", "enter", "esc"}},
		{"date", 1, []string{"ctrl+c", "enter", "esc"}},
		{"time", 2, []string{"ctrl+c", "enter", "esc"}},
		{"note", 3, []string{"ctrl+c", "enter", "esc"}},
		{"billable", 4, []string{"?", "N", "Y", "ctrl+c", "enter", "esc", "n", "y"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := entriesFixture(true)
			m.entriesScreen.mode = entriesEdit
			m.entriesScreen.editStep = c.step
			if got := enabledLabels(keysFor(m)); !slices.Equal(got, c.want) {
				t.Errorf("entries edit step %s labels = %v, want %v", c.name, got, c.want)
			}
		})
	}
}
