package cli

import "testing"

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
