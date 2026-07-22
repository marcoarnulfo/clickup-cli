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

	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
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
				{"id":"e1","task":{"id":"t1","name":"Task A"},"task_location":{"list_id":"55"},"user":{"id":1,"username":"marco"},"start":"1749110400000","duration":"7200000"},
				{"id":"e2","task":{"id":"t2","name":"Task B"},"task_location":{"list_id":"66"},"user":{"id":1,"username":"marco"},"start":"1749542400000","duration":"3600000"}
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

	for _, want := range []string{
		`"total_hours": 3`,
		`"total_amount": 210`,
		`"currency": "EUR"`,
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
	if !strings.Contains(out, "TOTAL,3,210,EUR") {
		t.Errorf("CSV total row missing; got:\n%s", out)
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
