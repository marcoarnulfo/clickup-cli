package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
)

func labels(as []action) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.label
	}
	return out
}

func hasLabel(as []action, want string) bool {
	for _, a := range as {
		if a.label == want {
			return true
		}
	}
	return false
}

func TestKeyMsgFor(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in     string
		wantOK bool
		want   string
	}{
		{"g", true, "g"},
		{"enter", true, "enter"},
		// PrevMonth and NextMonth are "left"/"h" and "right"/"l", so their
		// first key is not a rune. Without these two the palette would silently
		// drop both month-navigation actions on Home.
		{"left", true, "left"},
		{"right", true, "right"},
		{"tab", false, ""},
		{"shift+tab", false, ""},
		{"up", false, ""},
		{"", false, ""},
	} {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			msg, ok := keyMsgFor(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("keyMsgFor(%q) ok = %v, want %v", tc.in, ok, tc.wantOK)
			}
			// The round trip is the whole point: key.Matches compares
			// msg.String() against the binding's key strings.
			if ok && msg.String() != tc.want {
				t.Errorf("keyMsgFor(%q).String() = %q, want %q", tc.in, msg.String(), tc.want)
			}
		})
	}
}

func TestScreenActionsComeFromTheScreenKeymap(t *testing.T) {
	t.Parallel()
	got := labels(screenActions(newTestModelOnReport()))
	for _, want := range []string{"Grouping", "Export", "Budgets", "Filters", "Entries"} {
		if !hasLabel(screenActions(newTestModelOnReport()), want) {
			t.Errorf("the report screen offers no %q action; got %v", want, got)
		}
	}
	// Cursor mechanics are not commands.
	for _, unwanted := range []string{"Move up", "Move down", "Confirm", "Back", "Help", "Quit"} {
		if hasLabel(screenActions(newTestModelOnReport()), unwanted) {
			t.Errorf("%q leaked into the palette; it is cursor mechanics, not a command", unwanted)
		}
	}
}

// A disabled binding must not appear. This is what makes the palette's gating
// free: the same Enabled() that decides the footer decides the palette.
func TestScreenActionsRespectEnablement(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	m.screen = screenHome
	m.scope = "me"
	if hasLabel(screenActions(m), "Select members") {
		t.Error("the members action appears in the me scope, where its binding is disabled")
	}
	m.scope = "team"
	if !hasLabel(screenActions(m), "Select members") {
		t.Error("the members action is missing in the team scope, where its binding is enabled")
	}
}

// Labels are the footer's own words with the first rune upper-cased, so the two
// surfaces cannot drift. ToUpper on the first BYTE would corrupt any label that
// starts with a multibyte rune.
func TestCapitalizeHandlesMultibyte(t *testing.T) {
	t.Parallel()
	if got := capitalize("èxport"); got != "Èxport" {
		t.Errorf("capitalize(%q) = %q, want %q", "èxport", got, "Èxport")
	}
	if got := capitalize(""); got != "" {
		t.Errorf("capitalize(%q) = %q, want %q", "", got, "")
	}
}

func TestGlobalActionsAreGated(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	m.screen = screenHome
	m.entries = nil
	for _, unwanted := range []string{"Go to report", "Go to budgets", "Go to filters", "Go to entries", "Go to export"} {
		if hasLabel(globalActions(m), unwanted) {
			t.Errorf("%q is offered with no entries loaded; there is no report to show", unwanted)
		}
	}
	m.entries = goldenEntries()
	for _, want := range []string{"Go to report", "Go to budgets", "Go to filters", "Go to entries", "Go to export"} {
		if !hasLabel(globalActions(m), want) {
			t.Errorf("%q is missing although entries are loaded", want)
		}
	}
}

func TestGlobalActionsSkipTheCurrentScreen(t *testing.T) {
	t.Parallel()
	m := newTestModelOnReport()
	if hasLabel(globalActions(m), "Go to report") {
		t.Error(`"Go to report" is offered while already on the report screen`)
	}
	if !hasLabel(globalActions(m), "Go to rates") {
		t.Error(`"Go to rates" is missing from the report screen`)
	}
}

func TestGlobalMembersActionIsTeamOnly(t *testing.T) {
	t.Parallel()
	m := newTestModelOnReport()
	m.scope = "me"
	if hasLabel(globalActions(m), "Go to members") {
		t.Error(`"Go to members" is offered in the me scope`)
	}
	m.scope = "team"
	if !hasLabel(globalActions(m), "Go to members") {
		t.Error(`"Go to members" is missing in the team scope`)
	}
}

// Screen actions come first: "what can I do here" before "take me elsewhere".
func TestPaletteActionsPutScreenActionsFirst(t *testing.T) {
	t.Parallel()
	got := labels(paletteActions(newTestModelOnReport()))
	firstGlobal := -1
	for i, l := range got {
		if strings.HasPrefix(l, "Go to ") {
			firstGlobal = i
			break
		}
	}
	if firstGlobal <= 0 {
		t.Fatalf("expected screen actions before the first navigation row; got %v", got)
	}
	// Every global row is a "Go to …" except Quit, which is appended last.
	for _, l := range got[firstGlobal:] {
		if !strings.HasPrefix(l, "Go to ") && l != "Quit" {
			t.Errorf("screen action %q appears after the navigation block: %v", l, got)
		}
	}
}

// A screen action must do exactly what pressing its key does. This is the whole
// justification for deriving them instead of writing a second registry.
func TestScreenActionRunMatchesTheKeypress(t *testing.T) {
	t.Parallel()
	m := newTestModelOnReport()
	m.report = goldenReport()

	var run func(Model) (tea.Model, tea.Cmd)
	for _, a := range screenActions(m) {
		if a.label == "Export" {
			run = a.run
		}
	}
	if run == nil {
		t.Fatal(`no "Export" action on the report screen`)
	}
	viaAction, _ := run(m)
	viaKey, _ := m.updateReport(keyMsg("e"))
	if viaAction.(Model).screen != viaKey.(Model).screen {
		t.Errorf("action landed on %v, the keypress on %v",
			viaAction.(Model).screen, viaKey.(Model).screen)
	}
	if viaAction.(Model).screen != screenExport {
		t.Errorf("screen = %v, want screenExport", viaAction.(Model).screen)
	}
}

// Quit is the one global action that is not navigation, so it is appended
// outside the target-skipping loop rather than given a fake target screen.
func TestQuitActionIsAlwaysOffered(t *testing.T) {
	t.Parallel()
	for _, s := range []screen{screenHome, screenReport, screenRates} {
		m := newTestModelOnReport()
		m.screen = s
		if !hasLabel(globalActions(m), "Quit") {
			t.Errorf("Quit is missing on %v", s)
		}
	}
}

func TestGlobalMembersActionUsesTheCallersScreenAsOrigin(t *testing.T) {
	t.Parallel()
	m := newTestModelOnReport()
	m.scope = "team"
	m.screen = screenRates
	m.teamMembers = nil
	m.client = clickup.New("tok")
	m.client.BaseURL = "http://127.0.0.1:1" // refused instantly, no test server needed

	var run func(Model) (tea.Model, tea.Cmd)
	for _, a := range globalActions(m) {
		if a.label == "Go to members" {
			run = a.run
		}
	}
	if run == nil {
		t.Fatal(`no "Go to members" action`)
	}
	_, cmd := run(m)
	if cmd == nil {
		t.Fatal("no command issued with no members cached")
	}
	msg, ok := cmd().(retryableErrMsg)
	if !ok {
		t.Fatalf("command produced %T, want retryableErrMsg", cmd())
	}
	if msg.origin != screenRates {
		t.Errorf("origin = %v, want screenRates", msg.origin)
	}
}
