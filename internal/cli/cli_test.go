package cli

import (
	"testing"

	"github.com/marcoarnulfo/clickup-cli/internal/config"
)

// TestRootCmdRejectsUnknownArgs verifies the pinned cobra.NoArgs wiring: an
// unexpected positional argument must be rejected before anything (like the
// TUI) gets a chance to launch.
func TestRootCmdRejectsUnknownArgs(t *testing.T) {
	cmd := rootCmd()
	cmd.SetArgs([]string{"bogus"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() with an unexpected arg = nil error, want non-nil")
	}
}

// TestRootCmdSettings pins the root command's cobra configuration, since
// Task 11 depends on this exact contract.
func TestRootCmdSettings(t *testing.T) {
	cmd := rootCmd()

	if got, want := cmd.Use, "clup"; got != want {
		t.Errorf("Use = %q, want %q", got, want)
	}
	if cmd.Version == "" {
		t.Error("Version is empty, want a non-empty build version string")
	}
	if !cmd.SilenceUsage {
		t.Error("SilenceUsage = false, want true")
	}
	if !cmd.SilenceErrors {
		t.Error("SilenceErrors = false, want true")
	}
	if cmd.RunE == nil {
		t.Error("RunE is nil, want the TUI-launching function")
	}
}

// tea.ProgramOption is an opaque function: this test can prove that an extra
// option is appended when the mouse is on, not WHICH option it is. That is the
// honest limit of what is checkable here — the real coverage of the decision
// lives on config.MouseEnabled, which is pure. This test exists to catch the
// forgotten append, and nothing more.
func TestProgramOptionsAddsOneWhenMouseIsEnabled(t *testing.T) {
	t.Parallel()
	no := false
	off := len(programOptions(config.Config{Mouse: &no}))

	// Pins tea.WithAltScreen() being present even with the mouse off. Without
	// this, dropping WithAltScreen from programOptions entirely still passes
	// the difference assertion below (0 vs 1 is still a difference of one),
	// so the suite would stay green while the TUI silently stopped using the
	// alt screen.
	if off != 1 {
		t.Errorf("programOptions with the mouse off = %d options, want exactly 1 (tea.WithAltScreen())", off)
	}

	on := len(programOptions(config.Config{}))
	if on != off+1 {
		t.Errorf("programOptions: %d options with the mouse on, %d with it off; want exactly one more", on, off)
	}
}
