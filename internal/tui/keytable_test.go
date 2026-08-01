package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/themes"
)

// The zero KeyTable is what a Model built by hand in a test carries, and 108
// such Models exist. It must behave as the built-in defaults.
func TestZeroKeyTableIsTheDefaults(t *testing.T) {
	t.Parallel()
	var zero KeyTable
	if got, want := zero.bindings().Quit.Keys(), defaultKeys().Quit.Keys(); len(got) != len(want) || got[0] != want[0] {
		t.Errorf("zero table Quit = %v, want the default %v", got, want)
	}
}

func TestResolveKeysOverridesAndKeepsTheRest(t *testing.T) {
	t.Parallel()
	kt, err := ResolveKeys(map[string]config.KeySpec{"log_hours": {"L"}})
	if err != nil {
		t.Fatal(err)
	}
	d := kt.bindings()
	if got := d.LogHours.Keys(); len(got) != 1 || got[0] != "L" {
		t.Errorf("LogHours = %v, want [L]", got)
	}
	if got, want := d.Quit.Keys(), defaultKeys().Quit.Keys(); got[0] != want[0] {
		t.Errorf("Quit = %v, want the untouched default %v", got, want)
	}
}

// The help string carries the key inside it, so an overridden binding that kept
// its old help would lie in the footer and in the ? overlay.
// The keys are chosen so the override stays collision-free once Task 4's rule
// lands: "k" is already Up's own, and "ctrl+u" is claimed by nothing. An
// earlier draft used "w", which toggle_week claims — Task 4 would then have
// rejected it and this test could never have passed.
func TestResolveKeysRegeneratesTheHelp(t *testing.T) {
	t.Parallel()
	kt, err := ResolveKeys(map[string]config.KeySpec{"up": {"k", "ctrl+u"}})
	if err != nil {
		t.Fatal(err)
	}
	h := kt.bindings().Up.Help()
	if h.Key != "k/ctrl+u" {
		t.Errorf("help key = %q, want %q", h.Key, "k/ctrl+u")
	}
	if want := defaultKeys().Up.Help().Desc; h.Desc != want {
		t.Errorf("help desc = %q, want the original %q", h.Desc, want)
	}
}

// The footer is where a remapped binding would lie: the per-screen builders
// write the key label as a literal, so regenerating it inside ResolveKeys is
// not enough on its own.
func TestFooterShowsRemappedKeys(t *testing.T) {
	kt, err := ResolveKeys(map[string]config.KeySpec{"up": {"ctrl+u"}})
	if err != nil {
		t.Fatal(err)
	}
	m := Model{screen: screenExport, keys: kt, width: 120}

	foot := footerView(testTheme(true), m.width, false, keysFor(m))
	if !strings.Contains(foot, "ctrl+u") {
		t.Errorf("footer does not advertise the remapped key:\n%s", foot)
	}
	if strings.Contains(foot, "↑/↓/j/k") {
		t.Errorf("footer still shows the default label for a remapped binding:\n%s", foot)
	}
}

// A zero KeyTable means defaults, including the typography in literal labels.
func TestFooterKeepsItsLabelsWhenNothingIsRemapped(t *testing.T) {
	m := Model{screen: screenExport, width: 120}
	if foot := footerView(testTheme(true), m.width, false, keysFor(m)); !strings.Contains(foot, "↑/↓/j/k") {
		t.Errorf("footer lost its default label:\n%s", foot)
	}
}

func TestResolveKeysDoesNotAliasConfigInput(t *testing.T) {
	t.Parallel()
	keys := config.KeySpec{"k", "ctrl+u"}
	kt, err := ResolveKeys(map[string]config.KeySpec{"up": keys})
	if err != nil {
		t.Fatal(err)
	}

	keys[0] = "x"
	keys[1] = "ctrl+x"
	b := kt.bindings().Up
	if got := b.Keys(); len(got) != 2 || got[0] != "k" || got[1] != "ctrl+u" {
		t.Errorf("keys after mutating config input = %v, want [k ctrl+u]", got)
	}
	if got := b.Help().Key; got != "k/ctrl+u" {
		t.Errorf("help after mutating config input = %q, want %q", got, "k/ctrl+u")
	}
}

func TestResolveKeysErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   map[string]config.KeySpec
		says []string
	}{
		{
			name: "unknown binding lists the valid names",
			in:   map[string]config.KeySpec{"log_hourz": {"L"}},
			says: []string{"log_hourz", "log_hours"},
		},
		{
			name: "force_quit is not remappable",
			in:   map[string]config.KeySpec{"force_quit": {"ctrl+x"}},
			says: []string{"force_quit", "ctrl+c"},
		},
		{
			name: "an empty list is rejected",
			in:   map[string]config.KeySpec{"quit": {}},
			says: []string{"quit", "at least one key"},
		},
		{
			name: "an empty key inside the list is rejected",
			in:   map[string]config.KeySpec{"quit": {"Q", ""}},
			says: []string{"quit", "empty"},
		},
		{
			// Without its own check this reaches the collision rule, which
			// reports "already claimed by " with an empty list — the only
			// other claimant being the binding itself.
			name: "the same key twice in one list is rejected",
			in:   map[string]config.KeySpec{"quit": {"Q", "Q"}},
			says: []string{"quit", "\"Q\"", "twice"},
		},
	} {
		_, err := ResolveKeys(tc.in)
		if err == nil {
			t.Errorf("%s: ResolveKeys = nil error, want one", tc.name)
			continue
		}
		for _, s := range tc.says {
			if !strings.Contains(err.Error(), s) {
				t.Errorf("%s: error %q does not mention %q", tc.name, err, s)
			}
		}
	}
}

func TestResolveKeysRejectsNoncanonicalControlAliases(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		alias, canonical, colliding string
	}{
		{alias: "ctrl+i", canonical: "tab", colliding: "n"},
		{alias: "ctrl+m", canonical: "enter", colliding: "g"},
	} {
		t.Run(tc.alias, func(t *testing.T) {
			t.Parallel()
			// The second key deliberately collides. Alias validation must win
			// before collision analysis and before the binding help is built.
			_, err := ResolveKeys(map[string]config.KeySpec{
				"export": {tc.alias, tc.colliding},
			})
			if err == nil {
				t.Fatal("ResolveKeys = nil error, want one")
			}
			for _, want := range []string{`"export"`, fmt.Sprintf("%q", tc.alias), fmt.Sprintf("%q", tc.canonical)} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %s", err, want)
				}
			}
		})
	}
}

// ctrl+c must survive a config that remaps everything else: it is the only way
// out of a TUI whose Quit the user has moved somewhere they cannot reach.
//
// Each binding gets a distinct fN key so the sweep stays collision-free once
// Task 4's rule lands. Nothing checks that a key string is one a terminal can
// actually produce — the table is just strings — so f1..f51 serve here purely
// as names nothing else claims (f13 goes unused: force_quit is skipped). A sweep
// built from, say, "ctrl+"+name[:1] would collide (back, budget and browse_list
// would all want ctrl+b) and this test would quietly turn into one that skips
// forever.
func TestForceQuitSurvivesEveryOverride(t *testing.T) {
	t.Parallel()
	over := map[string]config.KeySpec{}
	for i, n := range BindingNames() {
		if n == forceQuitName {
			continue
		}
		over[n] = config.KeySpec{fmt.Sprintf("f%d", i+1)}
	}
	kt, err := ResolveKeys(over)
	if err != nil {
		t.Fatalf("a collision-free sweep was rejected: %v", err)
	}
	if got, want := kt.bindings().ForceQuit.Keys(), defaultKeys().ForceQuit.Keys(); got[0] != want[0] {
		t.Errorf("ForceQuit = %v, want %v", got, want)
	}
	// And the sweep really did land, so the assertion above is not vacuous.
	if got := kt.bindings().Quit.Keys(); len(got) != 1 || got[0] == "q" {
		t.Errorf("Quit = %v, want the swept key — the sweep did not take effect", got)
	}
}

// The defaults are heavily overloaded on purpose — measured, 20 physical keys
// are claimed by more than one binding, because screenKeys enables only a
// subset per screen. A rule that forbade any sharing would reject what we
// ship, so the rule is narrower: when a post-override key has multiple
// claimants, they must all have claimed it before.
func TestCollisionRule(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   map[string]config.KeySpec
		ok   bool
		says []string
	}{
		{
			// n is shared by log_hours, new_override, new_tag and no. Moving
			// log_hours away leaves a subset behind, which is fine.
			name: "moving a binding off a shared key is allowed",
			in:   map[string]config.KeySpec{"log_hours": {"L"}},
			ok:   true,
		},
		{
			name: "taking a free key is allowed",
			in:   map[string]config.KeySpec{"export": {"ctrl+e"}},
			ok:   true,
		},
		{
			name: "adding a claimant to a contested key is rejected",
			in:   map[string]config.KeySpec{"export": {"n"}},
			says: []string{"\"n\"", "export"},
		},
		{
			name: "adding a claimant to a singly claimed key is rejected",
			in:   map[string]config.KeySpec{"export": {"q"}},
			says: []string{"\"q\"", "export", "quit"},
		},
		{
			// A clean swap IS allowed, and this case exists to keep anyone
			// from "fixing" the rule into rejecting it. quit owns "q" and
			// reload owns "r" outright, so trading them leaves one claimant
			// on each key. Measured against the real defaults.
			name: "a clean swap between two bindings is allowed",
			in:   map[string]config.KeySpec{"quit": {"r"}, "reload": {"q"}},
			ok:   true,
		},
		{
			// This is the declared cost from the design doc's section 2.2, and
			// it is NOT "swaps are rejected": it is that taking a key someone
			// else still claims is rejected even when the two never share a
			// screen. export lives on the report, list_budget on the rates
			// screen, and the rule cannot know that. The user picks another
			// key; the alternative is a table of every screen state, kept in
			// sync forever.
			name: "taking a key another binding still claims is rejected — the declared cost",
			in:   map[string]config.KeySpec{"export": {"g"}},
			says: []string{"export", "\"g\"", "group_by", "list_budget"},
		},
	} {
		_, err := ResolveKeys(tc.in)
		if tc.ok {
			if err != nil {
				t.Errorf("%s: ResolveKeys = %v, want nil", tc.name, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: ResolveKeys = nil error, want one", tc.name)
			continue
		}
		for _, s := range tc.says {
			if !strings.Contains(err.Error(), s) {
				t.Errorf("%s: error %q does not mention %s", tc.name, err, s)
			}
		}
	}
}

// The rule must never reject the table we ship.
func TestDefaultsPassTheCollisionRule(t *testing.T) {
	t.Parallel()
	if _, err := ResolveKeys(nil); err != nil {
		t.Fatalf("the built-in defaults do not satisfy the collision rule: %v", err)
	}
}

// The end-to-end guard for the zero-means-defaults decision: an override has to
// survive all the way into Update's routing. If cli ever stopped passing the
// table, the zero value would quietly fall back to the defaults and only this
// test would notice.
func TestAnOverrideReachesUpdate(t *testing.T) {
	kt, err := ResolveKeys(map[string]config.KeySpec{"log_hours": {"L"}})
	if err != nil {
		t.Fatal(err)
	}
	m := New(config.Config{Token: "t", WorkspaceID: "team1"}, themes.Default(), kt)
	m.screen = screenReport
	m.nav = []screen{screenHome}

	if got, _ := m.Update(keyMsg("L")); got.(Model).screen != screenLog {
		t.Errorf("L did not open the log screen; screen = %v", got.(Model).screen)
	}
	if got, _ := m.Update(keyMsg("n")); got.(Model).screen == screenLog {
		t.Error("n still opens the log screen, so the override did not take effect")
	}
}
