package tui

import (
	"reflect"
	"slices"
	"testing"

	"github.com/charmbracelet/bubbles/key"

	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
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
// today (every case label, verbatim) in "me" scope with no running timer,
// plus q — handled globally in app.go, in no case clause of home.go itself.
// Members ("f") and Timer ("c") are excluded here: Members is gated to team
// scope only (see TestHomeMembersKeyIsTeamScopeOnly) and Timer requires a
// running timer (see TestHomeTimerKeyMatchesGuard).
func TestHomeKeyLabels(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	m.screen = screenHome
	m.scope = "me"
	want := []string{"?", "ctrl+c", "ctrl+p", "d", "enter", "h", "l", "left", "n", "q", "right", "t", "w"}
	if got := enabledLabels(keysFor(m)); !slices.Equal(got, want) {
		t.Errorf("home labels (me scope, no running timer) = %v, want %v", got, want)
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

// #59 Task 3 step 4: Timer's inline "if m.runningTimer != nil" guard moved
// into keysFor's enablement, so the two must agree — a binding enabled with
// nothing to manage, or disabled while a timer runs, is a live bug, not a
// display glitch (key.Matches consults Enabled() directly).
func TestHomeTimerKeyMatchesGuard(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	m.screen = screenHome
	if keysFor(m).Timer.Enabled() {
		t.Error("Timer binding enabled with no running timer")
	}
	m.runningTimer = &clickup.RunningTimer{TaskName: "X"}
	if !keysFor(m).Timer.Enabled() {
		t.Error("Timer binding disabled with a running timer")
	}
}

// #59 Task 3 step 4: same as above for PrevMonth/NextMonth's preset gate —
// home.go's old inline guard was "if m.preset != PresetThisMonth ||
// m.periodMode == periodModeWeek { break }"; keysFor must reproduce it
// exactly, in both directions and for both reasons a preset can disable it.
func TestHomeMonthNavKeysMatchGuard(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	m.screen = screenHome
	m.preset = report.PresetThisMonth
	m.periodMode = periodModeMonth
	if !keysFor(m).PrevMonth.Enabled() || !keysFor(m).NextMonth.Enabled() {
		t.Error("month nav should be enabled for this_month preset outside week mode")
	}
	m.periodMode = periodModeWeek
	if keysFor(m).PrevMonth.Enabled() || keysFor(m).NextMonth.Enabled() {
		t.Error("month nav should be disabled in week mode, even with this_month active")
	}
	m.periodMode = periodModeMonth
	m.preset = report.PresetLast7d
	if keysFor(m).PrevMonth.Enabled() || keysFor(m).NextMonth.Enabled() {
		t.Error("month nav should be disabled outside the this_month preset")
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

// ForceQuit is accepted everywhere but advertised only where q cannot quit.
// The two mechanisms are different on purpose: Enabled() is what key.Matches
// consults, short is what the footer renders.
func TestForceQuitAcceptedEverywhereAdvertisedOnSix(t *testing.T) {
	t.Parallel()
	advertised := map[screen]bool{
		screenSetup: true, screenRates: true, screenLog: true,
		screenRange: true, screenListBrowser: true, screenEntries: true,
	}
	for s := range 14 {
		sc := screen(s)
		m := newTestModel()
		m.screen = sc
		k := keysFor(m)
		if !k.ForceQuit.Enabled() {
			t.Errorf("screen %v: ctrl+c not accepted", sc)
		}
		shown := slices.ContainsFunc(k.ShortHelp(), func(b key.Binding) bool {
			return slices.Contains(b.Keys(), "ctrl+c")
		})
		if shown != advertised[sc] {
			t.Errorf("screen %v: ctrl+c advertised = %v, want %v", sc, shown, advertised[sc])
		}
	}
}

// TestAllBindingsCoversEveryField is the structural half of the parity net.
// Every per-screen label test reads its labels through allBindings, so a
// binding field left out of that slice is invisible to all of them at once —
// the key would keep working while nothing pinned it any more. Counting the
// fields by reflection means adding one to keyMap without adding it to
// allBindings fails here instead of quietly weakening the suite.
func TestAllBindingsCoversEveryField(t *testing.T) {
	t.Parallel()
	bindingType := reflect.TypeOf(key.Binding{})
	want := 0
	mapType := reflect.TypeOf(keyMap{})
	for i := range mapType.NumField() {
		if mapType.Field(i).Type == bindingType {
			want++
		}
	}
	if got := len(keyMap{}.allBindings()); got != want {
		t.Errorf("allBindings returns %d bindings, but keyMap has %d key.Binding fields — a field is missing from allBindings", got, want)
	}
}

// The palette must not inherit d.Up/d.Down: those are "up"/"k" and "down"/"j",
// so typing j or k into the query would move the cursor instead of entering a
// letter. paletteKeys uses the arrow-only PaletteUp/PaletteDown defaults.
func TestPaletteKeysAreArrowOnly(t *testing.T) {
	t.Parallel()
	k := paletteKeys(defaultKeys())
	for _, tc := range []struct {
		name    string
		binding key.Binding
	}{
		{"up", k.Up},
		{"down", k.Down},
	} {
		for _, bad := range []string{"j", "k"} {
			if slices.Contains(tc.binding.Keys(), bad) {
				t.Errorf("palette %s binding accepts %q, which the query needs as a character", tc.name, bad)
			}
		}
	}
}

// While the palette is open it owns the keyboard, so q must not quit and ? must
// not toggle help. A zero key.Binding has nil keys, which Enabled() reports as
// false and key.Matches never fires on.
func TestPaletteKeysLeaveQuitAndHelpUnassigned(t *testing.T) {
	t.Parallel()
	k := paletteKeys(defaultKeys())
	if k.Quit.Enabled() {
		t.Error("the palette assigned Quit; q would close the program while typing")
	}
	if k.Help.Enabled() {
		t.Error("the palette assigned Help; ? would toggle full help instead of filtering")
	}
}

// keysFor is what the footer reads, so it must switch to the palette's bindings
// while the overlay is open. screenKeys is what screenActions reads, and it must
// keep answering for the screen underneath — see the spec's section 5.2b.
func TestKeysForFollowsTheOverlayAndScreenKeysDoesNot(t *testing.T) {
	t.Parallel()
	m := newTestModelOnReport()
	if !keysFor(m).GroupBy.Enabled() {
		t.Fatal("the report screen lost its GroupBy binding; the fixture is wrong")
	}
	m.overlay = overlayPalette
	if keysFor(m).GroupBy.Enabled() {
		t.Error("keysFor still returns the report's bindings with the palette open")
	}
	if !screenKeys(m).GroupBy.Enabled() {
		t.Error("screenKeys stopped answering for the screen underneath the overlay")
	}
}

// Every binding the palette offers must be replayable through routeKey, which
// means its first key round-trips through keyMsgFor. Anything else would build
// a KeyMsg whose String() does not match the binding, and the action would
// silently do nothing.
func TestEveryPaletteBindingIsReplayable(t *testing.T) {
	t.Parallel()
	for _, b := range defaultKeys().paletteDefaults() {
		keys := b.Keys()
		if len(keys) == 0 {
			t.Errorf("a palette default has no keys: %+v", b.Help())
			continue
		}
		if _, ok := keyMsgFor(keys[0]); !ok {
			t.Errorf("binding %q (%s) has a first key keyMsgFor cannot rebuild", keys[0], b.Help().Desc)
		}
	}
}
