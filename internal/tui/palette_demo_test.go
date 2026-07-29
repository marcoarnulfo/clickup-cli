package tui

import (
	"testing"
)

// Demo mode must offer the same palette, and open it without any I/O: the demo
// exists so the TUI can be tried without an account, and a palette that reached
// the network there would break that promise.
//
// t.Setenv makes this test non-parallel by construction, which is what keeps it
// from racing TestMain's os.Unsetenv of the same variable.
func TestPaletteWorksInDemoMode(t *testing.T) {
	t.Setenv("CLICKUP_DEMO", "1")
	m := New(demoConfig())
	m.entries = goldenEntries()
	m.screen = screenReport
	m.nav = []screen{screenHome}

	m = m.openPalette()
	if len(m.palette.items) == 0 {
		t.Fatal("the palette is empty in demo mode")
	}
	if !hasLabel(paletteActions(m), "Go to rates") {
		t.Error("demo mode lost the navigation rows")
	}

	// The two navigation rows that issue commands must take their demo branch.
	for _, label := range []string{"Go to filters", "Go to members"} {
		m.scope = "team"
		m.teamMembers = nil
		for _, a := range paletteActions(m) {
			if a.label != label {
				continue
			}
			if _, cmd := a.run(m); cmd != nil {
				// Running the command is safe here precisely because the demo
				// branch never dials: if this hangs or errors, the branch was
				// missed.
				if msg := cmd(); msg == nil {
					t.Errorf("%s produced a nil message in demo mode", label)
				}
			}
		}
	}
}
