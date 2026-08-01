package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/themes"
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

func TestParseKeyNameAcceptsCanonicalNames(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		type_ tea.KeyType
		alt   bool
		runes string
	}{
		{name: "g", type_: tea.KeyRunes, runes: "g"},
		{name: "界", type_: tea.KeyRunes, runes: "界"},
		{name: "alt+é", type_: tea.KeyRunes, alt: true, runes: "é"},
		{name: " ", type_: tea.KeySpace, runes: " "},
		{name: "alt+ ", type_: tea.KeySpace, alt: true, runes: " "},
		{name: "ctrl+@", type_: tea.KeyCtrlAt},
		{name: "ctrl+a", type_: tea.KeyCtrlA},
		{name: "ctrl+h", type_: tea.KeyCtrlH},
		{name: "tab", type_: tea.KeyTab},
		{name: "ctrl+j", type_: tea.KeyCtrlJ},
		{name: "ctrl+l", type_: tea.KeyCtrlL},
		{name: "enter", type_: tea.KeyEnter},
		{name: "ctrl+n", type_: tea.KeyCtrlN},
		{name: "ctrl+z", type_: tea.KeyCtrlZ},
		{name: "esc", type_: tea.KeyEsc},
		{name: "ctrl+\\", type_: tea.KeyCtrlBackslash},
		{name: "ctrl+]", type_: tea.KeyCtrlCloseBracket},
		{name: "ctrl+^", type_: tea.KeyCtrlCaret},
		{name: "ctrl+_", type_: tea.KeyCtrlUnderscore},
		{name: "backspace", type_: tea.KeyBackspace},
		{name: "alt+ctrl+a", type_: tea.KeyCtrlA, alt: true},
		{name: "alt+enter", type_: tea.KeyEnter, alt: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			msg, err := parseKeyName(tc.name)
			if err != nil {
				t.Fatalf("parseKeyName(%q) = %v", tc.name, err)
			}
			if msg.Type != tc.type_ || msg.Alt != tc.alt || string(msg.Runes) != tc.runes {
				t.Errorf("parseKeyName(%q) = %+v, want type=%v alt=%v runes=%q", tc.name, msg, tc.type_, tc.alt, tc.runes)
			}
			if got := msg.String(); got != tc.name {
				t.Errorf("parseKeyName(%q).String() = %q", tc.name, got)
			}
		})
	}
}

func TestParseKeyNameAcceptsEveryCanonicalSpecialKey(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"up", "down", "right", "left", "shift+tab",
		"home", "end", "pgup", "pgdown", "ctrl+pgup", "ctrl+pgdown",
		"delete", "insert", "ctrl+up", "ctrl+down", "ctrl+right", "ctrl+left",
		"ctrl+home", "ctrl+end", "shift+up", "shift+down", "shift+right", "shift+left",
		"shift+home", "shift+end", "ctrl+shift+up", "ctrl+shift+down",
		"ctrl+shift+right", "ctrl+shift+left", "ctrl+shift+home", "ctrl+shift+end",
		"f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10",
		"f11", "f12", "f13", "f14", "f15", "f16", "f17", "f18", "f19", "f20",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, prefix := range []string{"", "alt+"} {
				full := prefix + name
				msg, err := parseKeyName(full)
				if err != nil {
					t.Errorf("parseKeyName(%q) = %v", full, err)
					continue
				}
				if got := msg.String(); got != full {
					t.Errorf("parseKeyName(%q).String() = %q", full, got)
				}
			}
		})
	}
}

func TestParseKeyNameRejectsAliasesAndUnreachableNames(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		canonical string
	}{
		{name: ""},
		{name: "g g"},
		{name: "gg"},
		{name: "f21"},
		{name: "runes"},
		{name: "alt+"},
		{name: "space", canonical: " "},
		{name: "ctrl+`", canonical: "ctrl+@"},
		{name: "ctrl+i", canonical: "tab"},
		{name: "ctrl+m", canonical: "enter"},
		{name: "ctrl+[", canonical: "esc"},
		{name: "ctrl+?", canonical: "backspace"},
		{name: "alt+ctrl+i", canonical: "alt+tab"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseKeyName(tc.name)
			if err == nil {
				t.Fatalf("parseKeyName(%q) = nil error", tc.name)
			}
			if tc.canonical != "" && !strings.Contains(err.Error(), tc.canonical) {
				t.Errorf("parseKeyName(%q) error %q does not suggest %q", tc.name, err, tc.canonical)
			}
		})
	}
}

func reportModelWithOverrides(t *testing.T, overrides map[string]config.KeySpec) Model {
	t.Helper()
	kt, err := ResolveKeys(overrides)
	if err != nil {
		t.Fatal(err)
	}
	m := New(config.Config{Token: "t", WorkspaceID: "team1"}, themes.Default(), kt)
	m.screen = screenReport
	m.nav = []screen{screenHome}
	m.report = goldenReport()
	return m
}

func TestScreenActionsReplayAControlKeyOverride(t *testing.T) {
	t.Parallel()
	m := reportModelWithOverrides(t, map[string]config.KeySpec{"export": {"ctrl+e"}})

	var run func(Model) (tea.Model, tea.Cmd)
	for _, a := range screenActions(m) {
		if a.label == "Export" {
			run = a.run
		}
	}
	if run == nil {
		t.Fatal("a ctrl+e-only Export binding disappeared from screenActions")
	}
	viaAction, _ := run(m)
	viaKey, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	if got, want := viaAction.(Model).screen, viaKey.(Model).screen; got != want {
		t.Errorf("action landed on %v, configured ctrl+e keypress landed on %v", got, want)
	}
	if viaAction.(Model).screen != screenExport {
		t.Errorf("screen = %v, want screenExport", viaAction.(Model).screen)
	}
}

func TestScreenActionsReplaySpecialKeyOverrides(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		msg  tea.KeyMsg
	}{
		{name: "f1", msg: tea.KeyMsg{Type: tea.KeyF1}},
		{name: "home", msg: tea.KeyMsg{Type: tea.KeyHome}},
		{name: "delete", msg: tea.KeyMsg{Type: tea.KeyDelete}},
		{name: "ctrl+shift+up", msg: tea.KeyMsg{Type: tea.KeyCtrlShiftUp}},
		{name: "alt+f20", msg: tea.KeyMsg{Type: tea.KeyF20, Alt: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := reportModelWithOverrides(t, map[string]config.KeySpec{"export": {tc.name}})
			for _, a := range screenActions(m) {
				if a.label != "Export" {
					continue
				}
				viaAction, _ := a.run(m)
				viaKey, _ := m.Update(tc.msg)
				if got, want := viaAction.(Model).screen, viaKey.(Model).screen; got != want || got != screenExport {
					t.Errorf("action screen = %v, physical %s screen = %v, want %v", got, tc.name, want, screenExport)
				}
				return
			}
			t.Fatal("Export disappeared from screenActions")
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
