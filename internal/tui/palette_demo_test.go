package tui

import (
	"testing"

	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/themes"
)

// Demo mode must offer the same palette, and open it without any I/O: the demo
// exists so the TUI can be tried without an account, and a palette that reached
// the network there would break that promise.
//
// t.Setenv makes this test non-parallel by construction, which is what keeps it
// from racing TestMain's os.Unsetenv of the same variable.
func TestPaletteWorksInDemoMode(t *testing.T) {
	t.Setenv("CLICKUP_DEMO", "1")
	m := New(demoConfig(), themes.Default())
	m.entries = goldenEntries()
	// A non-empty Status is load-bearing for the "Go to filters" assertion
	// below: demoStatusEnrichCmd echoes each entry's own Status, and
	// goldenEntries() sets none, so without this the demo branch would also
	// report an all-empty byTask and the assertion could not tell it apart
	// from the real branch failing to reach every task.
	for i := range m.entries {
		m.entries[i].Status = "open"
	}
	m.screen = screenReport
	m.nav = []screen{screenHome}

	// A client pointed at a refused local port rather than the real API: if
	// the demo guard in openFilters/openMembers were ever bypassed, the real
	// branch fails in milliseconds instead of hanging for its 120s/30s
	// timeout — a fast red rather than a slow one.
	m.client = clickup.New("DEMO")
	m.client.BaseURL = "http://127.0.0.1:1"

	m = m.openPalette()
	if len(m.palette.items) == 0 {
		t.Fatal("the palette is empty in demo mode")
	}
	if !hasLabel(paletteActions(m), "Go to rates") {
		t.Error("demo mode lost the navigation rows")
	}

	// The two navigation rows that issue commands must take their demo
	// branch. A bare "cmd() != nil" cannot discriminate that: both the demo
	// commands and the real ones (statusEnrichCmd, loadMembersCmd) always
	// return a non-nil message, including on failure (statusesMsg with every
	// status cached as "", or errMsg/retryableErrMsg). So each command's
	// result is asserted on its concrete type and, for "Go to filters", its
	// content — that is what actually tells the two branches apart.
	for _, label := range []string{"Go to filters", "Go to members"} {
		m.scope = "team"
		m.teamMembers = nil
		for _, a := range paletteActions(m) {
			if a.label != label {
				continue
			}
			_, cmd := a.run(m)
			if cmd == nil {
				continue
			}
			msg := cmd()
			switch label {
			case "Go to filters":
				// The real branch, unable to reach any task, still yields a
				// statusesMsg — just one where every cached status is "".
				sm, ok := msg.(statusesMsg)
				if !ok {
					t.Errorf("%s produced %T, want statusesMsg — the real branch ran instead of the demo one", label, msg)
					continue
				}
				nonEmpty := false
				for _, st := range sm.byTask {
					if st != "" {
						nonEmpty = true
						break
					}
				}
				if !nonEmpty {
					t.Errorf("%s: every cached status was empty — the real branch ran instead of the demo one", label)
				}
			case "Go to members":
				// The real branch, unable to reach the API, yields
				// retryableErrMsg, not membersMsg — the type alone tells the
				// two branches apart here.
				if _, ok := msg.(membersMsg); !ok {
					t.Errorf("%s produced %T, want membersMsg — the real branch ran instead of the demo one", label, msg)
				}
			}
		}
	}
}
