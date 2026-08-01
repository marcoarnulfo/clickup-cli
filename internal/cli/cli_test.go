package cli

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

// An unresolvable theme must stop the launch with an error the user can act on,
// not start a TUI with the wrong colors. The check is on resolveTheme, which is
// the production code path runTUI takes: runTUI itself blocks on a terminal and
// cannot be called from a test, the same reason programOptions was extracted.
func TestResolveThemeRejectsAnUnknownName(t *testing.T) {
	t.Parallel()
	_, err := resolveTheme(config.Config{Theme: "nope"})
	if err == nil {
		t.Fatal("resolveTheme of an unknown theme = nil error, want one")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q does not name the theme the user asked for", err)
	}
	// The prefix is what tells the user which part of their config is at fault
	// once Execute prints it as "error: …".
	if !strings.HasPrefix(err.Error(), "theme:") {
		t.Errorf("error %q is not prefixed with the config section it comes from", err)
	}
}

func TestResolveThemeAcceptsABuiltin(t *testing.T) {
	t.Parallel()
	if _, err := resolveTheme(config.Config{Theme: "dracula"}); err != nil {
		t.Errorf("resolveTheme(dracula) = %v, want nil", err)
	}
	if _, err := resolveTheme(config.Config{}); err != nil {
		t.Errorf("resolveTheme with no theme set = %v, want nil (the default)", err)
	}
}

func TestResolveKeysRejectsAnUnknownBinding(t *testing.T) {
	t.Parallel()
	_, err := resolveKeys(config.Config{Keys: map[string]config.KeySpec{"nope": {"x"}}})
	if err == nil {
		t.Fatal("resolveKeys of an unknown binding = nil error, want one")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q does not name the binding the user asked for", err)
	}
	if !strings.HasPrefix(err.Error(), "keys:") {
		t.Errorf("error %q is not prefixed with the config section it comes from", err)
	}
}

func TestBuildModelRoutesWithTheResolvedKeyTable(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		Token:       "t",
		WorkspaceID: "team1",
		Keys:        map[string]config.KeySpec{"log_hours": {"L"}},
	}

	m, err := buildModel(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")})
	if view := got.View(); !strings.Contains(view, "Choose the mode:") {
		t.Errorf("configured L did not open Log hours:\n%s", view)
	}

	m, err = buildModel(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if view := got.View(); strings.Contains(view, "Choose the mode:") {
		t.Errorf("default n still opened Log hours after the override:\n%s", view)
	}
}
