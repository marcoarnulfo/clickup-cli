package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/export"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
	"github.com/marcoarnulfo/clickup-cli/internal/service"
	"github.com/spf13/cobra"
)

// loadConfig and newClient are package-level seams so tests can swap in a
// fixture config and a client pointed at an httptest server without ever
// touching the real config file or the real ClickUp API.
var loadConfig = config.Load
var newClient = func(token string) *clickup.Client { return clickup.New(token) }

// reportCmd builds the headless `report` subcommand: it loads the config,
// builds a report for a range/scope, and writes it to stdout. It never
// starts bubbletea.
func reportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "report",
		Short:         "Print an hours report to stdout (headless, no TUI)",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runReport,
	}
	cmd.Flags().String("month", "", "report month, YYYY-MM (default: current month)")
	cmd.Flags().String("from", "", "range start date, YYYY-MM-DD (use with --to)")
	cmd.Flags().String("to", "", "range end date, YYYY-MM-DD, inclusive (use with --from)")
	cmd.Flags().String("preset", "", "range preset: this_month|last_month|last_7d|last_30d|this_week")
	cmd.Flags().String("scope", "me", "report scope: me|team")
	cmd.Flags().String("group", report.GroupByTotal, "group by: task|list|day|member|total")
	cmd.Flags().String("format", "json", "output format: json|csv|md")
	return cmd
}

// runReport is the report subcommand's RunE. It deliberately ignores
// CLICKUP_DEMO: the headless path always goes through the real config/API
// seams (loadConfig/newClient), never tui's demo data.
func runReport(cmd *cobra.Command, args []string) error {
	month, _ := cmd.Flags().GetString("month")
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")
	preset, _ := cmd.Flags().GetString("preset")
	scope, _ := cmd.Flags().GetString("scope")
	group, _ := cmd.Flags().GetString("group")
	format, _ := cmd.Flags().GetString("format")

	if scope != "me" && scope != "team" {
		return fmt.Errorf("unsupported --scope %q, want \"me\" or \"team\"", scope)
	}
	switch group {
	case report.GroupByTotal, report.GroupByTask, report.GroupByList, report.GroupByDay, report.GroupByMember:
	default:
		return fmt.Errorf("unsupported --group %q, want one of %q, %q, %q, %q, %q",
			group, report.GroupByTotal, report.GroupByTask, report.GroupByList, report.GroupByDay, report.GroupByMember)
	}
	if preset != "" {
		switch preset {
		case report.PresetThisMonth, report.PresetLastMonth, report.PresetLast7d, report.PresetLast30d, report.PresetThisWeek:
		default:
			return fmt.Errorf("unsupported --preset %q, want one of %q, %q, %q, %q, %q",
				preset, report.PresetThisMonth, report.PresetLastMonth, report.PresetLast7d, report.PresetLast30d, report.PresetThisWeek)
		}
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if !cfg.Valid() {
		return fmt.Errorf("run 'clup' to configure, or set CLICKUP_TOKEN")
	}

	start, end, err := resolveRange(month, from, to, preset, time.Now())
	if err != nil {
		return err
	}

	c := newClient(cfg.Token)
	entries, err := service.LoadEntries(cmd.Context(), c, cfg.WorkspaceID, start, end, scope, nil)
	if err != nil {
		return err
	}

	// Rates construction mirrors internal/tui's ratesFromConfig exactly; this
	// is intentionally inlined rather than importing tui, to avoid coupling a
	// headless command to the TUI package for a two-line struct literal.
	rates := report.Rates{Default: cfg.Rate, ByList: cfg.Rates}

	r := report.Build(entries, group, rates, cfg.Currency, start, end)
	r.Scope = scope

	switch format {
	case "json":
		return export.JSON(os.Stdout, r)
	case "csv":
		return export.CSV(os.Stdout, r)
	case "md":
		return export.Markdown(os.Stdout, r)
	default:
		return fmt.Errorf("unsupported --format %q, want \"json\", \"csv\" or \"md\"", format)
	}
}

// resolveRange picks exactly one range source, in priority order: --month,
// then --from/--to, then --preset, else this month. All boundaries are UTC.
func resolveRange(month, from, to, preset string, now time.Time) (start, end time.Time, err error) {
	now = now.UTC()
	switch {
	case month != "":
		t, err := time.Parse("2006-01", month)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --month %q: %w", month, err)
		}
		start, end := report.RangeForPreset(report.PresetThisMonth, t.Year(), t.Month(), now)
		return start, end, nil
	case from != "" || to != "":
		if from == "" || to == "" {
			return time.Time{}, time.Time{}, fmt.Errorf("--from and --to must be given together")
		}
		f, err := time.Parse("2006-01-02", from)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --from %q: %w", from, err)
		}
		tt, err := time.Parse("2006-01-02", to)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --to %q: %w", to, err)
		}
		start, end := report.CustomRange(f, tt)
		return start, end, nil
	case preset != "":
		start, end := report.RangeForPreset(preset, now.Year(), now.Month(), now)
		return start, end, nil
	default:
		start, end := report.RangeForPreset(report.PresetThisMonth, now.Year(), now.Month(), now)
		return start, end, nil
	}
}
