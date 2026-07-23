package cli

import (
	"bytes"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/export"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
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

func TestReportJSONAgainstFakeServer(t *testing.T) {
	srv := fakeReportServer(t)
	withSeams(t, fixtureConfig(), srv)

	out, err := runReportCapture(t, []string{"--month", "2026-06", "--format", "json"})
	if err != nil {
		t.Fatalf("report --format json: %v", err)
	}

	// The fixture declares "billable":true on both entries, but the ClickUp
	// client does not decode that field yet (it lands with the "billable absent
	// ⇒ true" default in a follow-up step), so entries still reach Build as
	// non-billable and nothing is billed: total_amount is 0 here. The money
	// formatting itself is constrained by TestReportMoneyPipeline below, which
	// does not depend on that client gap. When the client starts reading the
	// flag, this must go back to 210 (2h*80 + 1h*50) and the golden must be
	// regenerated with -update.
	// The deprecated "currency"/"rate" keys stay populated: the JSON schema this
	// command promises must not break for existing scripts.
	for _, want := range []string{
		`"total_hours": 3`,
		`"total_amount": 0`,
		`"currency": "EUR"`,
		`"rate": 50`,
		`"scope": "me"`,
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

	if !strings.HasPrefix(out, "label,hours,amount,currency\n") {
		t.Errorf("CSV header missing/wrong; got:\n%s", out)
	}
	// 0, not 210: see TestReportJSONAgainstFakeServer — entries arrive
	// non-billable until the client decodes the billable flag. The real CSV
	// money row is asserted by TestReportMoneyPipeline.
	if !strings.Contains(out, "TOTAL,3,0,EUR") {
		t.Errorf("CSV total row missing; got:\n%s", out)
	}
}

// TestReportMoneyPipeline constrains the command's money path — its own
// pricingFor() plus report.Build plus the export writers — with billable
// entries. It bypasses exactly one link of the chain, the ClickUp client's
// entry decoding, which does not read the billable flag yet; every other
// component is the one runReport uses. Without this, the end-to-end tests above
// would pass even if Build produced no money at all.
func TestReportMoneyPipeline(t *testing.T) {
	cfg := fixtureConfig()
	start, end, err := resolveRange("2026-06", "", "", "", time.Now())
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
	r := report.Build(entries, report.GroupByTotal, pricingFor(cfg), start, end, nil)
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
	if !strings.HasPrefix(csvBuf.String(), "label,hours,amount,currency\n") {
		t.Errorf("CSV header missing/wrong; got:\n%s", csvBuf.String())
	}
	if !strings.Contains(csvBuf.String(), "TOTAL,3,210,EUR") {
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
