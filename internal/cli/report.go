package cli

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/export"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
	"github.com/marcoarnulfo/clickup-cli/internal/service"
	"github.com/spf13/cobra"
)

// loadConfig, newClient, and currentVersion are package-level seams so tests
// can swap in a fixture config, a client pointed at an httptest server, and a
// fake release version, without ever touching the real config file, the real
// ClickUp API, or requiring a `go install`-built release binary (in a `go
// test` binary service.CurrentVersion reports "(devel)", which is not a
// release version and would make CheckForUpdate a permanent no-op in tests).
var loadConfig = config.Load
var newClient = func(token string) *clickup.Client { return clickup.New(token) }
var currentVersion = service.CurrentVersion

// rangePrecedence documents (and is quoted by) every range flag's help text,
// so the priority order is discoverable from --help alone rather than only
// from source or docs.
const rangePrecedence = "range priority: --month > --week > --from/--to > --preset > current month (default)"

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
	cmd.Flags().String("month", "", "report month, YYYY-MM; "+rangePrecedence)
	cmd.Flags().String("week", "", "report ISO-8601 week, YYYY-Www (e.g. 2026-W30); "+rangePrecedence)
	cmd.Flags().String("from", "", "range start date, YYYY-MM-DD (use with --to); "+rangePrecedence)
	cmd.Flags().String("to", "", "range end date, YYYY-MM-DD, inclusive (use with --from); "+rangePrecedence)
	cmd.Flags().String("preset", "", "range preset: this_month|last_month|last_7d|last_30d|this_week; "+rangePrecedence)
	cmd.Flags().String("scope", "me", "report scope: me|team")
	cmd.Flags().String("group", report.GroupByTotal, "group by: task|list|day|member|tag|total")
	cmd.Flags().String("format", "json", "output format: json|csv|md|html|csv-invoice")
	cmd.Flags().Bool("billable", false, "filter to billable entries only (pass --billable=false to keep non-billable entries only); default: no filter")
	cmd.Flags().StringArray("tag", nil, "filter to entries carrying this tag (repeatable; matches any of the given tags)")
	cmd.Flags().String("tz", "", "IANA timezone for range boundaries and the report's timezone field (default: config's timezone, else UTC)")
	return cmd
}

// runReport is the report subcommand's RunE. It deliberately ignores
// CLICKUP_DEMO: the headless path always goes through the real config/API
// seams (loadConfig/newClient), never tui's demo data.
func runReport(cmd *cobra.Command, args []string) error {
	month, _ := cmd.Flags().GetString("month")
	week, _ := cmd.Flags().GetString("week")
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")
	preset, _ := cmd.Flags().GetString("preset")
	scope, _ := cmd.Flags().GetString("scope")
	group, _ := cmd.Flags().GetString("group")
	format, _ := cmd.Flags().GetString("format")
	tags, _ := cmd.Flags().GetStringArray("tag")
	billable, _ := cmd.Flags().GetBool("billable")
	tzFlag, _ := cmd.Flags().GetString("tz")

	if scope != "me" && scope != "team" {
		return fmt.Errorf("unsupported --scope %q, want \"me\" or \"team\"", scope)
	}
	switch group {
	case report.GroupByTotal, report.GroupByTask, report.GroupByList, report.GroupByDay, report.GroupByMember, report.GroupByTag:
	default:
		return fmt.Errorf("unsupported --group %q, want one of %q, %q, %q, %q, %q, %q",
			group, report.GroupByTotal, report.GroupByTask, report.GroupByList, report.GroupByDay, report.GroupByMember, report.GroupByTag)
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

	// Start the update check alongside the report fetch. Running it serially
	// would add up to two seconds to a `clup report` once a day.
	updates := startUpdateCheck(cmd.Context(), cfg)

	// Timezone two-track (global constraint): headless defaults to UTC and
	// never changes silently. Resolution order: --tz flag, then config
	// timezone, then UTC.
	tzName := tzFlag
	if tzName == "" {
		tzName = cfg.Timezone
	}
	loc, err := service.LoadLocation(tzName, time.UTC)
	if err != nil {
		return err
	}

	start, end, err := resolveRange(month, week, from, to, preset, time.Now(), loc)
	if err != nil {
		return err
	}

	c := newClient(cfg.Token)
	entries, err := service.LoadEntries(cmd.Context(), c, cfg.WorkspaceID, start, end, scope, nil)
	if err != nil {
		return err
	}

	// A3: the billable filter already exists on report.FilterCriteria — use
	// it rather than inventing a private pre-filter. Billable is only set
	// (non-nil) when --billable was actually passed on the command line, so
	// omitting the flag imposes no constraint (both billable and
	// non-billable entries are kept).
	criteria := report.FilterCriteria{Tags: tagSet(tags)}
	if cmd.Flags().Changed("billable") {
		b := billable
		criteria.Billable = &b
	}
	if !criteria.Empty() {
		entries = report.Filter(entries, criteria)
	}

	p, err := service.PricingFromConfig(cfg)
	if err != nil {
		return err
	}

	r := report.Build(entries, group, p, start, end, loc)
	r.Scope = scope

	var writeErr error
	switch format {
	case "json":
		writeErr = export.JSON(os.Stdout, r)
	case "csv":
		writeErr = export.CSV(os.Stdout, r)
	case "md":
		writeErr = export.Markdown(os.Stdout, r)
	case "html":
		writeErr = export.HTML(os.Stdout, r)
	case "csv-invoice":
		writeErr = export.InvoiceCSV(os.Stdout, r)
	default:
		return fmt.Errorf("unsupported --format %q, want \"json\", \"csv\", \"md\", \"html\" or \"csv-invoice\"", format)
	}

	// Collect the update check started above. The channel is always closed,
	// so this receive never blocks the command from exiting even if the
	// check is disabled, failed, or found nothing newer.
	if latest, ok := <-updates; ok {
		fmt.Fprintf(os.Stderr, "\nclup %s is available (you have %s)\n"+
			"  go install github.com/marcoarnulfo/clickup-cli/cmd/clup@latest\n"+
			"  disable: CLUP_NO_UPDATE_CHECK=1\n", latest, currentVersion())
	}

	return writeErr
}

// updateAPIURL and updateCachePath are empty in production; tests set them to
// point the check at a local server and a temporary cache.
var (
	updateAPIURL    string
	updateCachePath string
)

// startUpdateCheck runs the update check in the background and returns a
// channel yielding the newer version, if any. The channel is always closed, so
// a receive never blocks the command from finishing.
//
// It passes demo=false to service.UpdateCheckEnabled deliberately: the
// headless path ignores CLICKUP_DEMO (see runReport's doc comment and
// TestReportIgnoresDemoEnv). Demo mode only applies to the TUI.
func startUpdateCheck(ctx context.Context, cfg config.Config) <-chan string {
	out := make(chan string, 1)
	if !service.UpdateCheckEnabled(cfg, false) {
		close(out)
		return out
	}
	go func() {
		defer close(out)
		latest, newer := service.CheckForUpdate(ctx, service.UpdateOptions{
			Current:   currentVersion(),
			APIURL:    updateAPIURL,
			CachePath: updateCachePath,
		})
		if newer {
			out <- latest
		}
	}()
	return out
}

// tagSet converts a repeatable --tag flag's values into the map
// report.FilterCriteria.Tags expects. A nil/empty input returns a nil map,
// which is fine: FilterCriteria.Empty() treats it as no constraint.
func tagSet(tags []string) map[string]bool {
	if len(tags) == 0 {
		return nil
	}
	m := make(map[string]bool, len(tags))
	for _, t := range tags {
		m[t] = true
	}
	return m
}

// weekFlagPattern matches the --week flag's required YYYY-Www shape (a
// 4-digit ISO year, literal "W", 2-digit week number). Bounds on the week
// number itself are checked separately by parseWeekFlag, so the error
// message can distinguish "malformed" from "out of range".
var weekFlagPattern = regexp.MustCompile(`^(\d{4})-W(\d{2})$`)

// parseWeekFlag parses --week's YYYY-Www value into an ISO year/week pair,
// rejecting malformed strings and week numbers outside 1-53 (amendment M4).
// This is the only bounds guard: report.WeekRange itself does no validation
// and would silently extrapolate past the real weeks of a year (e.g. week 0
// or 54).
func parseWeekFlag(week string) (isoYear, isoWeek int, err error) {
	m := weekFlagPattern.FindStringSubmatch(week)
	if m == nil {
		return 0, 0, fmt.Errorf("invalid --week %q: want format YYYY-Www (e.g. 2026-W30)", week)
	}
	isoYear, _ = strconv.Atoi(m[1])
	isoWeek, _ = strconv.Atoi(m[2])
	if isoWeek < 1 || isoWeek > 53 {
		return 0, 0, fmt.Errorf("invalid --week %q: week %d out of range, want 1-53", week, isoWeek)
	}
	return isoYear, isoWeek, nil
}

// resolveRange picks exactly one range source, in priority order: --month,
// --week, --from/--to, --preset, else this month. Boundaries are computed in
// loc (the caller resolves loc via service.LoadLocation before calling this,
// defaulting to UTC).
func resolveRange(month, week, from, to, preset string, now time.Time, loc *time.Location) (start, end time.Time, err error) {
	// n is now resolved into loc before reading its Year()/Month(): the
	// preset/default branches below pick a calendar month, and that pick
	// must follow the range's own timezone (UTC by default, never the
	// machine's local clock) rather than now's original location. Without
	// this, a run near a month boundary could report a different month in
	// UTC than the pre-v1.7 CLI did (see the headless "never changes
	// silently" constraint).
	n := now.In(loc)
	switch {
	case month != "":
		t, err := time.Parse("2006-01", month)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --month %q: %w", month, err)
		}
		start, end := report.RangeForPreset(report.PresetThisMonth, t.Year(), t.Month(), now, loc)
		return start, end, nil
	case week != "":
		isoYear, isoWeek, err := parseWeekFlag(week)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		start, end := report.WeekRange(isoYear, isoWeek, loc)
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
		start, end := report.CustomRange(f, tt, loc)
		return start, end, nil
	case preset != "":
		start, end := report.RangeForPreset(preset, n.Year(), n.Month(), now, loc)
		return start, end, nil
	default:
		start, end := report.RangeForPreset(report.PresetThisMonth, n.Year(), n.Month(), now, loc)
		return start, end, nil
	}
}
