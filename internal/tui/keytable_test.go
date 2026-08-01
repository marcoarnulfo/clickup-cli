package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/marcoarnulfo/clickup-cli/internal/config"
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
