package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/export"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
	"github.com/marcoarnulfo/clickup-cli/internal/service"
)

// update regenerates the golden file for TestReportJSONSchemaGolden:
//
//	go test ./internal/cli/ -run TestReportJSONSchemaGolden -update
var update = flag.Bool("update", false, "update golden files")

const goldenPath = "testdata/report_golden.json"

// fakeReportServer serves the endpoints service.LoadEntries touches: the
// workspace's time entries and list-name lookups. Two entries, in two
// different lists, so grouping/rate-per-list produce a non-trivial report.
func fakeReportServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/time_entries"):
			w.Write([]byte(`{"data":[
				{"id":"e1","task":{"id":"t1","name":"Task A"},"task_location":{"list_id":"55"},"user":{"id":1,"username":"marco"},"start":"1749110400000","duration":"7200000","billable":true},
				{"id":"e2","task":{"id":"t2","name":"Task B"},"task_location":{"list_id":"66"},"user":{"id":1,"username":"marco"},"start":"1749542400000","duration":"3600000","billable":true}
			]}`))
		case strings.Contains(r.URL.Path, "/list/55"):
			w.Write([]byte(`{"id":"55","name":"Client A"}`))
		case strings.Contains(r.URL.Path, "/list/66"):
			w.Write([]byte(`{"id":"66","name":"Client B"}`))
		case strings.HasSuffix(r.URL.Path, "/team"):
			w.Write([]byte(`{"teams":[]}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// fakeReportServerMixedBillableTags serves three entries exercising both the
// --billable and --tag filters together: e1 (list 55, billable, tag clientA,
// 2h), e2 (list 66, billable, tag clientB, 1h), e3 (list 55, NON-billable,
// tag clientA, 1h). Every filter test below picks a slice whose expected
// total_hours is unambiguous given these fixed durations.
func fakeReportServerMixedBillableTags(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/time_entries"):
			w.Write([]byte(`{"data":[
				{"id":"e1","task":{"id":"t1","name":"Task A"},"task_location":{"list_id":"55"},"user":{"id":1,"username":"marco"},"start":"1749110400000","duration":"7200000","billable":true,"task_tags":[{"name":"clientA"}]},
				{"id":"e2","task":{"id":"t2","name":"Task B"},"task_location":{"list_id":"66"},"user":{"id":1,"username":"marco"},"start":"1749542400000","duration":"3600000","billable":true,"task_tags":[{"name":"clientB"}]},
				{"id":"e3","task":{"id":"t3","name":"Task C"},"task_location":{"list_id":"55"},"user":{"id":1,"username":"marco"},"start":"1749542400000","duration":"3600000","billable":false,"task_tags":[{"name":"clientA"}]}
			]}`))
		case strings.Contains(r.URL.Path, "/list/55"):
			w.Write([]byte(`{"id":"55","name":"Client A"}`))
		case strings.Contains(r.URL.Path, "/list/66"):
			w.Write([]byte(`{"id":"66","name":"Client B"}`))
		case strings.HasSuffix(r.URL.Path, "/team"):
			w.Write([]byte(`{"teams":[]}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// fixtureConfig is a valid config usable with fakeReportServer's fixtures.
func fixtureConfig() config.Config {
	return config.Config{
		Token:       "tok",
		WorkspaceID: "900",
		Currency:    "EUR",
		Rate:        50,
		Rates:       map[string]float64{"55": 80},
	}
}

// withSeams overrides loadConfig/newClient for the duration of the test,
// restoring the originals on cleanup. newClient points every client at srv.
func withSeams(t *testing.T, cfg config.Config, srv *httptest.Server) {
	t.Helper()
	origLoadConfig, origNewClient := loadConfig, newClient
	t.Cleanup(func() {
		loadConfig = origLoadConfig
		newClient = origNewClient
	})
	loadConfig = func() (config.Config, error) { return cfg, nil }
	newClient = func(token string) *clickup.Client {
		c := clickup.New(token)
		c.BaseURL = srv.URL
		return c
	}
}

// runReportCapture runs the report subcommand with the given args and
// returns whatever it wrote to stdout, plus the RunE error.
func runReportCapture(t *testing.T, args []string) (string, error) {
	t.Helper()
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	cmd := reportCmd()
	cmd.SetArgs(args)
	runErr := cmd.Execute()

	w.Close()
	os.Stdout = origStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String(), runErr
}

// fakeUpdateServer answers like GitHub's releases/latest endpoint with tag,
// and counts how many requests it received — the counter is what lets
// TestReportNoNoticeWhenDisabled prove the server was never even hit.
func fakeUpdateServer(t *testing.T, tag string) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		fmt.Fprintf(w, `{"tag_name":%q}`, tag)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

// withUpdateSeams overrides currentVersion/updateAPIURL/updateCachePath for
// the duration of the test, restoring the originals on cleanup — the same
// pattern withSeams already uses for loadConfig/newClient. Without this
// cleanup, one test would leave its endpoint (and fake current version) live
// for whichever test in the package runs next.
func withUpdateSeams(t *testing.T, current, apiURL, cachePath string) {
	t.Helper()
	origCurrent, origAPIURL, origCachePath := currentVersion, updateAPIURL, updateCachePath
	t.Cleanup(func() {
		currentVersion = origCurrent
		updateAPIURL = origAPIURL
		updateCachePath = origCachePath
	})
	currentVersion = func() string { return current }
	updateAPIURL = apiURL
	updateCachePath = cachePath
}

// runReportCaptureBoth is runReportCapture's sibling for tests that must also
// inspect stderr: the update notice never touches stdout, so proving that
// requires both streams.
func runReportCaptureBoth(t *testing.T, args []string) (stdout, stderr string, err error) {
	t.Helper()
	origStdout, origStderr := os.Stdout, os.Stderr
	outR, outW, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("os.Pipe (stdout): %v", pipeErr)
	}
	errR, errW, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("os.Pipe (stderr): %v", pipeErr)
	}
	os.Stdout = outW
	os.Stderr = errW

	cmd := reportCmd()
	cmd.SetArgs(args)
	runErr := cmd.Execute()

	outW.Close()
	errW.Close()
	os.Stdout = origStdout
	os.Stderr = origStderr

	var outBuf, errBuf bytes.Buffer
	io.Copy(&outBuf, outR)
	io.Copy(&errBuf, errR)
	return outBuf.String(), errBuf.String(), runErr
}

// TestReportNoticeGoesToStderrAndStdoutStaysJSON is the load-bearing test for
// this task: `clup report --json` feeds scripts, so a notice line on stdout
// would break every jq pipeline downstream.
func TestReportNoticeGoesToStderrAndStdoutStaysJSON(t *testing.T) {
	t.Setenv("CLUP_NO_UPDATE_CHECK", "")
	reportSrv := fakeReportServer(t)
	withSeams(t, fixtureConfig(), reportSrv)
	updateSrv, _ := fakeUpdateServer(t, "v99.0.0")
	withUpdateSeams(t, "v1.0.0", updateSrv.URL, filepath.Join(t.TempDir(), "update.json"))

	stdout, stderr, err := runReportCaptureBoth(t, []string{"--month", "2026-06", "--format", "json"})
	if err != nil {
		t.Fatalf("report --format json: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout)
	}
	if strings.Contains(stdout, "is available") {
		t.Error("the update notice leaked into stdout")
	}
	if !strings.Contains(stderr, "is available") {
		t.Errorf("no notice on stderr:\n%s", stderr)
	}
	if !strings.Contains(stderr, "CLUP_NO_UPDATE_CHECK") {
		t.Error("the notice must tell the user how to turn it off")
	}
}

// TestReportNoNoticeWhenDisabled confirms CLUP_NO_UPDATE_CHECK=1 not only
// suppresses the notice but skips the network call entirely.
func TestReportNoNoticeWhenDisabled(t *testing.T) {
	t.Setenv("CLUP_NO_UPDATE_CHECK", "1")
	reportSrv := fakeReportServer(t)
	withSeams(t, fixtureConfig(), reportSrv)
	updateSrv, calls := fakeUpdateServer(t, "v99.0.0")
	withUpdateSeams(t, "v1.0.0", updateSrv.URL, filepath.Join(t.TempDir(), "update.json"))

	_, stderr, err := runReportCaptureBoth(t, []string{"--month", "2026-06", "--format", "json"})
	if err != nil {
		t.Fatalf("report --format json: %v", err)
	}
	if strings.Contains(stderr, "is available") {
		t.Errorf("notice printed despite CLUP_NO_UPDATE_CHECK=1:\n%s", stderr)
	}
	if got := atomic.LoadInt32(calls); got != 0 {
		t.Errorf("update server was called %d times despite CLUP_NO_UPDATE_CHECK=1", got)
	}
}

func TestReportJSONAgainstFakeServer(t *testing.T) {
	srv := fakeReportServer(t)
	withSeams(t, fixtureConfig(), srv)

	out, err := runReportCapture(t, []string{"--month", "2026-06", "--format", "json"})
	if err != nil {
		t.Fatalf("report --format json: %v", err)
	}

	// The fixture declares "billable":true on both entries, and the ClickUp
	// client now decodes that field, so both entries reach Build as billable:
	// total_amount is 210 (2h*80 + 1h*50). The deprecated "currency"/"rate"
	// keys stay populated: the JSON schema this command promises must not
	// break for existing scripts.
	for _, want := range []string{
		`"total_hours": 3`,
		`"total_amount": 210`,
		`"currency": "EUR"`,
		`"rate": 50`,
		`"scope": "me"`,
		`"schema_version": 1`,
		`"timezone": "UTC"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q; got:\n%s", want, out)
		}
	}
}

// TestReportJSONSchemaGolden pins the frozen scripting schema (snake_case
// keys, RFC3339 times) that the headless report command promises. Regenerate
// with: go test ./internal/cli/ -run TestReportJSONSchemaGolden -update
func TestReportJSONSchemaGolden(t *testing.T) {
	srv := fakeReportServer(t)
	withSeams(t, fixtureConfig(), srv)

	out, err := runReportCapture(t, []string{"--month", "2026-06", "--group", "list", "--format", "json"})
	if err != nil {
		t.Fatalf("report: %v", err)
	}

	if *update {
		if err := os.WriteFile(goldenPath, []byte(out), 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		t.Logf("golden file updated: %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden file (run with -update to create it): %v", err)
	}
	if out != string(want) {
		t.Errorf("stdout does not match golden file %s\n--- got ---\n%s\n--- want ---\n%s", goldenPath, out, string(want))
	}
}

func TestReportCSVFormat(t *testing.T) {
	srv := fakeReportServer(t)
	withSeams(t, fixtureConfig(), srv)

	out, err := runReportCapture(t, []string{"--month", "2026-06", "--format", "csv"})
	if err != nil {
		t.Fatalf("report --format csv: %v", err)
	}

	if !strings.HasPrefix(out, "label,hours,billable_hours,billed_hours,amount,currency\n") {
		t.Errorf("CSV header missing/wrong; got:\n%s", out)
	}
	// 210 (2h*80 + 1h*50): the fixture entries are billable and the client
	// now decodes that flag, so the real CSV money row lands here too.
	if !strings.Contains(out, "TOTAL,3,3,3,210,EUR") {
		t.Errorf("CSV total row missing; got:\n%s", out)
	}
}

// TestReportMoneyPipeline constrains the command's money path — service's
// PricingFromConfig plus report.Build plus the export writers — with billable
// entries. It bypasses exactly one link of the chain, the ClickUp client's
// entry decoding, which does not read the billable flag yet; every other
// component is the one runReport uses. Without this, the end-to-end tests above
// would pass even if Build produced no money at all.
func TestReportMoneyPipeline(t *testing.T) {
	cfg := fixtureConfig()
	start, end, err := resolveRange("2026-06", "", "", "", "", time.Now(), time.UTC)
	if err != nil {
		t.Fatalf("resolveRange: %v", err)
	}
	entries := []report.TimeEntry{
		{ID: "e1", TaskID: "t1", TaskName: "Task A", ListID: "55", ListName: "Client A",
			UserID: 1, UserName: "marco", Start: time.Date(2026, 6, 5, 8, 0, 0, 0, time.UTC),
			Duration: 2 * time.Hour, Billable: true},
		{ID: "e2", TaskID: "t2", TaskName: "Task B", ListID: "66", ListName: "Client B",
			UserID: 1, UserName: "marco", Start: time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC),
			Duration: time.Hour, Billable: true},
	}
	p, err := service.PricingFromConfig(cfg)
	if err != nil {
		t.Fatalf("PricingFromConfig: %v", err)
	}
	r := report.Build(entries, report.GroupByTotal, p, start, end, nil)
	r.Scope = "me"

	var jsonBuf bytes.Buffer
	if err := export.JSON(&jsonBuf, r); err != nil {
		t.Fatalf("export.JSON: %v", err)
	}
	for _, want := range []string{
		`"total_hours": 3`,
		`"total_amount": 210`, // 2h * 80 (list 55 override) + 1h * 50 (default)
		`"currency": "EUR"`,
		`"rate": 50`,
		`"scope": "me"`,
	} {
		if !strings.Contains(jsonBuf.String(), want) {
			t.Errorf("JSON missing %q; got:\n%s", want, jsonBuf.String())
		}
	}

	var csvBuf bytes.Buffer
	if err := export.CSV(&csvBuf, r); err != nil {
		t.Fatalf("export.CSV: %v", err)
	}
	if !strings.HasPrefix(csvBuf.String(), "label,hours,billable_hours,billed_hours,amount,currency\n") {
		t.Errorf("CSV header missing/wrong; got:\n%s", csvBuf.String())
	}
	if !strings.Contains(csvBuf.String(), "TOTAL,3,3,3,210,EUR") {
		t.Errorf("CSV total row missing; got:\n%s", csvBuf.String())
	}
}

func TestReportInvalidConfigErrors(t *testing.T) {
	srv := fakeReportServer(t) // should never be hit
	withSeams(t, config.Config{}, srv)

	out, err := runReportCapture(t, []string{"--month", "2026-06"})
	if err == nil {
		t.Fatal("report with invalid config: err = nil, want non-nil")
	}
	const want = "run 'clup' to configure, or set CLICKUP_TOKEN"
	if err.Error() != want {
		t.Errorf("err = %q, want %q", err.Error(), want)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty (invalid config must produce no output)", out)
	}
}

func TestReportRejectsUnknownGroupAndPreset(t *testing.T) {
	srv := fakeReportServer(t) // should never be hit
	withSeams(t, fixtureConfig(), srv)

	t.Run("bogus group", func(t *testing.T) {
		out, err := runReportCapture(t, []string{"--month", "2026-06", "--group", "bogus"})
		if err == nil {
			t.Fatal("report --group bogus: err = nil, want non-nil")
		}
		if out != "" {
			t.Errorf("stdout = %q, want empty (invalid --group must produce no output)", out)
		}
	})

	t.Run("bogus preset", func(t *testing.T) {
		out, err := runReportCapture(t, []string{"--preset", "bogus"})
		if err == nil {
			t.Fatal("report --preset bogus: err = nil, want non-nil")
		}
		if out != "" {
			t.Errorf("stdout = %q, want empty (invalid --preset must produce no output)", out)
		}
	})

	t.Run("custom preset rejected", func(t *testing.T) {
		out, err := runReportCapture(t, []string{"--preset", "custom"})
		if err == nil {
			t.Fatal("report --preset custom: err = nil, want non-nil (custom ranges come from --from/--to)")
		}
		if out != "" {
			t.Errorf("stdout = %q, want empty (invalid --preset must produce no output)", out)
		}
	})
}

func TestReportIgnoresDemoEnv(t *testing.T) {
	t.Setenv("CLICKUP_DEMO", "1")
	srv := fakeReportServer(t)
	withSeams(t, fixtureConfig(), srv)

	out, err := runReportCapture(t, []string{"--month", "2026-06", "--format", "json"})
	if err != nil {
		t.Fatalf("report --format json: %v", err)
	}

	// The demo fixtures (internal/tui/demo.go) use different totals/currency;
	// asserting our real fixture's totals proves CLICKUP_DEMO was ignored.
	if !strings.Contains(out, `"total_hours": 3`) || !strings.Contains(out, `"currency": "EUR"`) {
		t.Errorf("report used demo data instead of the real config path; got:\n%s", out)
	}
}

// TestResolveRangeWeekFlag exercises --week's parsing and bounds checking
// directly against resolveRange. report.WeekRange itself does no bounds
// checking (it silently extrapolates for week 0 or 54), so this flag layer
// is the only guard; every malformed/out-of-range case must error, naming
// the offending value.
func TestResolveRangeWeekFlag(t *testing.T) {
	t.Run("valid week resolves via WeekRange", func(t *testing.T) {
		start, end, err := resolveRange("", "2026-W30", "", "", "", time.Now(), time.UTC)
		if err != nil {
			t.Fatalf("resolveRange: %v", err)
		}
		wantStart, wantEnd := report.WeekRange(2026, 30, time.UTC)
		if !start.Equal(wantStart) || !end.Equal(wantEnd) {
			t.Errorf("got [%s, %s), want [%s, %s)", start, end, wantStart, wantEnd)
		}
	})

	for _, tc := range []struct {
		name string
		week string
	}{
		{"week 00 is out of range", "2026-W00"},
		{"week 54 is out of range", "2026-W54"},
		{"missing W is malformed", "2026-30"},
		{"single-digit week is malformed", "2026-W3"},
		{"garbage is malformed", "bogus"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := resolveRange("", tc.week, "", "", "", time.Now(), time.UTC)
			if err == nil {
				t.Fatalf("resolveRange with --week %q: err = nil, want non-nil", tc.week)
			}
			if !strings.Contains(err.Error(), tc.week) {
				t.Errorf("error %q does not name the offending value %q", err.Error(), tc.week)
			}
		})
	}
}

// TestResolveRangeMonthWinsOverWeek documents and pins the precedence order:
// --month sits above --week.
func TestResolveRangeMonthWinsOverWeek(t *testing.T) {
	start, end, err := resolveRange("2026-06", "2026-W30", "", "", "", time.Now(), time.UTC)
	if err != nil {
		t.Fatalf("resolveRange: %v", err)
	}
	wantStart, wantEnd := report.MonthRange(2026, time.June, time.UTC)
	if !start.Equal(wantStart) || !end.Equal(wantEnd) {
		t.Errorf("--month did not win over --week: got [%s, %s), want [%s, %s)", start, end, wantStart, wantEnd)
	}
}

// TestResolveRangeDefaultMonthFollowsLoc pins the fix for the "current month
// follows the machine's local clock, not loc" finding: the preset/default
// branches must derive year/month from now.In(loc), not from now's own
// location. now is built in a fixed zone 13 hours ahead of UTC, at 00:30 on
// the 1st of the month — local calendar says August, but in UTC (loc) it is
// still 11:30 on July 31. Both the default (no flags) and the explicit
// this_month preset must resolve to July in UTC, matching the headless
// path's "defaults to UTC and never changes silently" constraint.
func TestResolveRangeDefaultMonthFollowsLoc(t *testing.T) {
	farAhead := time.FixedZone("UTC+13", 13*60*60)
	now := time.Date(2026, time.August, 1, 0, 30, 0, 0, farAhead)
	wantStart, wantEnd := report.MonthRange(2026, time.July, time.UTC)

	t.Run("default (no flags)", func(t *testing.T) {
		start, end, err := resolveRange("", "", "", "", "", now, time.UTC)
		if err != nil {
			t.Fatalf("resolveRange: %v", err)
		}
		if !start.Equal(wantStart) || !end.Equal(wantEnd) {
			t.Errorf("got [%s, %s), want [%s, %s) (July in UTC, not August-local)", start, end, wantStart, wantEnd)
		}
	})

	t.Run("--preset this_month", func(t *testing.T) {
		start, end, err := resolveRange("", "", "", "", report.PresetThisMonth, now, time.UTC)
		if err != nil {
			t.Fatalf("resolveRange: %v", err)
		}
		if !start.Equal(wantStart) || !end.Equal(wantEnd) {
			t.Errorf("got [%s, %s), want [%s, %s) (July in UTC, not August-local)", start, end, wantStart, wantEnd)
		}
	})
}

// TestReportWeekFlag confirms --week reaches the command end-to-end: the
// emitted start/end match report.WeekRange for the requested ISO week.
func TestReportWeekFlag(t *testing.T) {
	srv := fakeReportServer(t)
	withSeams(t, fixtureConfig(), srv)

	out, err := runReportCapture(t, []string{"--week", "2026-W30", "--format", "json"})
	if err != nil {
		t.Fatalf("report --week 2026-W30: %v", err)
	}

	wantStart, wantEnd := report.WeekRange(2026, 30, time.UTC)
	for _, want := range []string{
		`"start": "` + wantStart.Format(time.RFC3339) + `"`,
		`"end": "` + wantEnd.Format(time.RFC3339) + `"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q; got:\n%s", want, out)
		}
	}
}

// TestReportBillableFilter confirms --billable narrows entries via
// report.FilterCriteria.Billable, not a private pre-filter: true keeps only
// billable entries (e1+e2 = 3h), --billable=false keeps only the
// non-billable one (e3 = 1h), and omitting the flag keeps everything (4h).
func TestReportBillableFilter(t *testing.T) {
	srv := fakeReportServerMixedBillableTags(t)
	withSeams(t, fixtureConfig(), srv)

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"no filter keeps all entries", []string{"--format", "json"}, `"total_hours": 4`},
		{"--billable keeps billable only", []string{"--billable", "--format", "json"}, `"total_hours": 3`},
		{"--billable=false keeps non-billable only", []string{"--billable=false", "--format", "json"}, `"total_hours": 1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runReportCapture(t, tc.args)
			if err != nil {
				t.Fatalf("report %v: %v", tc.args, err)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("stdout missing %q; got:\n%s", tc.want, out)
			}
		})
	}
}

// TestReportTagFilterAndGroup confirms --tag (repeatable, OR within the
// dimension) narrows entries via FilterCriteria.Tags, and that --group tag
// is now reachable from the CLI whitelist.
func TestReportTagFilterAndGroup(t *testing.T) {
	srv := fakeReportServerMixedBillableTags(t)
	withSeams(t, fixtureConfig(), srv)

	t.Run("--tag narrows to matching entries", func(t *testing.T) {
		// Only e2 carries clientB: 1h, billable, list 66 (default rate 50).
		out, err := runReportCapture(t, []string{"--tag", "clientB", "--format", "json"})
		if err != nil {
			t.Fatalf("report --tag clientB: %v", err)
		}
		if !strings.Contains(out, `"total_hours": 1`) {
			t.Errorf("stdout missing total_hours 1; got:\n%s", out)
		}
	})

	t.Run("--tag plus --group tag buckets by tag", func(t *testing.T) {
		// e1 (2h) + e3 (1h) both carry clientA: filtered total is 3h, all in
		// one "clientA" bucket.
		out, err := runReportCapture(t, []string{"--tag", "clientA", "--group", "tag", "--format", "json"})
		if err != nil {
			t.Fatalf("report --tag clientA --group tag: %v", err)
		}
		for _, want := range []string{
			`"group_by": "tag"`,
			`"label": "clientA"`,
			`"total_hours": 3`,
		} {
			if !strings.Contains(out, want) {
				t.Errorf("stdout missing %q; got:\n%s", want, out)
			}
		}
	})
}

// TestReportFormatHTMLAndCSVInvoice confirms --format now dispatches to
// export.HTML and export.InvoiceCSV.
func TestReportFormatHTMLAndCSVInvoice(t *testing.T) {
	srv := fakeReportServer(t)
	withSeams(t, fixtureConfig(), srv)

	t.Run("html", func(t *testing.T) {
		out, err := runReportCapture(t, []string{"--month", "2026-06", "--format", "html"})
		if err != nil {
			t.Fatalf("report --format html: %v", err)
		}
		if !strings.Contains(out, "<html") || !strings.Contains(out, "</html>") {
			t.Errorf("stdout is not an HTML document; got:\n%s", out)
		}
	})

	t.Run("csv-invoice", func(t *testing.T) {
		out, err := runReportCapture(t, []string{"--month", "2026-06", "--format", "csv-invoice"})
		if err != nil {
			t.Fatalf("report --format csv-invoice: %v", err)
		}
		if !strings.HasPrefix(out, "date,list_id,client,user,description,qty_hours,rate,amount,currency,billable\n") {
			t.Errorf("csv-invoice header missing/wrong; got:\n%s", out)
		}
	})
}

// TestReportTzFlag confirms the timezone two-track: default is UTC (never
// changes silently), --tz overrides it, and --tz takes precedence over the
// config's timezone.
func TestReportTzFlag(t *testing.T) {
	srv := fakeReportServer(t)

	t.Run("default stays UTC with no --tz and no config timezone", func(t *testing.T) {
		withSeams(t, fixtureConfig(), srv)
		out, err := runReportCapture(t, []string{"--month", "2026-06", "--format", "json"})
		if err != nil {
			t.Fatalf("report: %v", err)
		}
		for _, want := range []string{`"timezone": "UTC"`, `"start": "2026-06-01T00:00:00Z"`} {
			if !strings.Contains(out, want) {
				t.Errorf("stdout missing %q; got:\n%s", want, out)
			}
		}
	})

	t.Run("--tz changes boundaries and the timezone field", func(t *testing.T) {
		withSeams(t, fixtureConfig(), srv)
		out, err := runReportCapture(t, []string{"--month", "2026-06", "--tz", "Europe/Rome", "--format", "json"})
		if err != nil {
			t.Fatalf("report --tz Europe/Rome: %v", err)
		}
		// June is CEST (+02:00): the month boundary is midnight in Rome, not UTC.
		for _, want := range []string{`"timezone": "Europe/Rome"`, `"start": "2026-06-01T00:00:00+02:00"`} {
			if !strings.Contains(out, want) {
				t.Errorf("stdout missing %q; got:\n%s", want, out)
			}
		}
	})

	t.Run("--tz overrides the config timezone", func(t *testing.T) {
		cfg := fixtureConfig()
		cfg.Timezone = "Europe/Rome"
		withSeams(t, cfg, srv)
		out, err := runReportCapture(t, []string{"--month", "2026-06", "--tz", "America/New_York", "--format", "json"})
		if err != nil {
			t.Fatalf("report --tz America/New_York: %v", err)
		}
		if !strings.Contains(out, `"timezone": "America/New_York"`) {
			t.Errorf("stdout missing overridden timezone; got:\n%s", out)
		}
	})

	t.Run("config timezone applies when --tz is absent", func(t *testing.T) {
		cfg := fixtureConfig()
		cfg.Timezone = "Europe/Rome"
		withSeams(t, cfg, srv)
		out, err := runReportCapture(t, []string{"--month", "2026-06", "--format", "json"})
		if err != nil {
			t.Fatalf("report: %v", err)
		}
		if !strings.Contains(out, `"timezone": "Europe/Rome"`) {
			t.Errorf("stdout missing config timezone; got:\n%s", out)
		}
	})

	t.Run("invalid --tz errors with no output", func(t *testing.T) {
		withSeams(t, fixtureConfig(), srv)
		out, err := runReportCapture(t, []string{"--month", "2026-06", "--tz", "Not/AZone"})
		if err == nil {
			t.Fatal("report --tz Not/AZone: err = nil, want non-nil")
		}
		if out != "" {
			t.Errorf("stdout = %q, want empty (invalid --tz must produce no output)", out)
		}
	})
}
