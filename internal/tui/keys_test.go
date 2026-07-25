package tui

import (
	"slices"
	"testing"
)

// enabledLabels is every key label the screen accepts right now, sorted and
// deduplicated — the contract the migration must preserve exactly.
func enabledLabels(k keyMap) []string {
	var out []string
	for _, b := range k.allBindings() {
		if !b.Enabled() {
			continue
		}
		out = append(out, b.Keys()...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// TestHomeKeyLabels pins the exact label set home.go's updateHome accepts
// today (every case label, verbatim) in "me" scope, plus q — handled
// globally in app.go, in no case clause of home.go itself. Members ("f") is
// excluded here: it is gated to team scope only (see
// TestHomeMembersKeyIsTeamScopeOnly).
func TestHomeKeyLabels(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	m.screen = screenHome
	m.scope = "me"
	want := []string{"c", "d", "enter", "h", "l", "left", "n", "q", "right", "t", "w"}
	if got := enabledLabels(keysFor(m)); !slices.Equal(got, want) {
		t.Errorf("home labels (me scope) = %v, want %v", got, want)
	}
}

func TestHomeMembersKeyIsTeamScopeOnly(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	m.screen = screenHome
	m.scope = "me"
	if keysFor(m).Members.Enabled() {
		t.Error("Members binding enabled in me scope")
	}
	m.scope = "team"
	if !keysFor(m).Members.Enabled() {
		t.Error("Members binding disabled in team scope")
	}
}

// The quit exclusion set is behavior, not policy: reproduce it exactly, so a
// later change to it is a deliberate act with a failing test to update.
func TestQuitBindingPerScreen(t *testing.T) {
	t.Parallel()
	quits := map[screen]bool{
		screenSetup: false, screenHome: true, screenLoading: true,
		screenReport: true, screenExport: true, screenRates: false,
		screenLog: false, screenError: true, screenMembers: true,
		screenRange: false, screenFilters: true, screenListBrowser: false,
		screenBudget: true, screenEntries: false,
	}
	for s, want := range quits {
		m := newTestModel()
		m.screen = s
		if got := keysFor(m).Quit.Enabled(); got != want {
			t.Errorf("screen %v: Quit enabled = %v, want %v", s, got, want)
		}
	}
}
