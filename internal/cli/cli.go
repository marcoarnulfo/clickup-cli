// Package cli builds the clup cobra command tree. Execute is the single
// entrypoint shared by every cmd/* binary: it never calls os.Exit, so
// callers stay free to run cleanup or print after it returns.
package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/service"
	"github.com/marcoarnulfo/clickup-cli/internal/tui"
	"github.com/spf13/cobra"
)

// programOptions builds the bubbletea options for a config. Extracted from
// runTUI so the mouse decision is reachable from a test: runTUI itself blocks
// on a terminal.
//
// WithMouseCellMotion (DEC mode 1002) and not WithMouseAllMotion (1003): the
// latter reports pointer motion even with no button held, a stream of messages
// this TUI would discard. Neither is "wheel only" — no such DEC mode exists —
// which is why enabling the wheel costs the terminal's native text selection,
// and why config.Mouse exists to turn it back off.
func programOptions(cfg config.Config) []tea.ProgramOption {
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if cfg.MouseEnabled() {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	return opts
}

// rootCmd builds the root command. Unexported so tests can exercise the
// command wiring directly without going through Execute/os.Exit.
func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "clup",
		Version:       service.CurrentVersion(),
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runTUI,
	}
	cmd.AddCommand(reportCmd())
	return cmd
}

// runTUI loads the config and launches the interactive TUI, mirroring the
// previous cmd/clickup main(). Demo mode (CLICKUP_DEMO) is handled inside
// tui.New, so there is nothing demo-specific to do here.
func runTUI(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	p := tea.NewProgram(tui.New(cfg), programOptions(cfg)...)
	_, err = p.Run()
	return err
}

// Execute builds and runs the root command, printing any error once to
// stderr, and returns the process exit code. It never calls os.Exit: callers
// (cmd/*'s main) own that decision.
func Execute() int {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}
