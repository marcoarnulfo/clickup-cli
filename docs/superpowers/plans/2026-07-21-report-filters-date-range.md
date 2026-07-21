# Report Filters + Custom Date Range Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the report cover an arbitrary date range (presets + custom) and be filtered by list, tag, and status.

**Architecture:** Two phases. Phase 1 replaces the month-only period with a range (`Report.Start/End`, a Home range picker, pure preset helpers). Phase 2 adds a client-side `report.Filter` over the loaded entries, a sectioned Filters screen (Lists/Tags/Statuses), tags fetched with the entries (`include_task_tags`), and statuses fetched lazily on first opening Filters. Filters compose with the #3 member selection.

**Tech Stack:** Go 1.26, bubbletea/lipgloss (Charm), `net/http`+`httptest` for the client, standard `testing` (table-driven).

## Global Constraints

- Go **1.26**; no new third-party dependencies.
- Everything in the repo is written in **English**: identifiers, comments, test messages, UI strings, commit messages. (Design docs under `docs/superpowers/` stay in Italian.)
- `internal/report` and `internal/duration` stay **pure**: no I/O, no `time.Now()` inside them, no imports of `config`/`clickup`. Preset/range helpers take an explicit `now time.Time`.
- **Conventional Commits**. **NEVER** add a `Co-Authored-By` trailer to commit messages.
- Before each commit: `gofmt -l .` (empty), `go vet ./...`, `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`, `go test ./... -race` — all clean/green. staticcheck is a hard gate: a symbol must have a real reference at the commit that introduces it (a written-but-unread struct field is fine; a lone const in a used iota group is fine; an unused type/func is NOT).
- TUI convention: `Model` is a value receiver; write sub-models back explicitly (`m.sub = x`) before `return`. Screens are `updateX(msg)`/`view()`. Async work is a `tea.Cmd` returning a typed msg handled in `Update`.
- Filter semantics: within a dimension = OR; across dimensions = AND; an empty dimension = no constraint. `FilterCriteria.Empty()` ⇒ the report is identical to today's.
- `report.Build` recomputes buckets/amounts/totals over whatever entry slice it is given; per-list rates keep applying. Filtering happens by feeding `Build` the output of `report.Filter`.

---

## File Structure

- `internal/report/period.go` (new) — preset consts + `RangeForPreset`, `PeriodLabel`, `PeriodFileSlug` (pure).
- `internal/report/filter.go` (new) — `FilterCriteria`, `Filter` (pure).
- `internal/report/model.go` — `Report.Start/End` (replaces `Year/Month`); `TimeEntry.Tags/Status`.
- `internal/report/aggregate.go` — `Build` signature `(…, start, end time.Time)`.
- `internal/clickup/timeentries.go` — `include_task_tags` + `Tags` parse.
- `internal/clickup/task.go` (new) — `TaskStatus(ctx, taskID)`.
- `internal/export/export.go` — JSON `start/end`, Markdown header via `PeriodLabel`.
- `internal/tui/range.go` (new) — `rangeModel` + range picker screen.
- `internal/tui/filters.go` (new) — `filtersModel` + Filters screen.
- `internal/tui/app.go`/`home.go`/`report.go`/`export.go`/`rates.go`/`demo.go` — root state, wiring, `currentRange`, `filterCriteria`, enrichment.
- Tests alongside each file; `README.md`/`README.it.md` in the last task of each phase.

---

# Phase 1 — Custom date range

### Task 1: report — period helpers (pure)

**Files:**
- Create: `internal/report/period.go`
- Test: `internal/report/period_test.go`

**Interfaces:**
- Consumes: existing `MonthRange(year, month) (start, end time.Time)`.
- Produces: preset consts `PresetThisMonth/LastMonth/Last7d/Last30d/ThisWeek/Custom`; `RangeForPreset(preset string, year int, month time.Month, now time.Time) (start, end time.Time)`; `PeriodLabel(start, end time.Time) string`; `PeriodFileSlug(start, end time.Time) string`.

- [ ] **Step 1: Write the failing tests**

Create `internal/report/period_test.go`:

```go
package report

import (
	"testing"
	"time"
)

func TestRangeForPreset(t *testing.T) {
	now := time.Date(2026, time.July, 15, 13, 0, 0, 0, time.UTC) // a Wednesday
	cases := []struct {
		preset     string
		wantStart  string
		wantEnd    string
	}{
		{PresetThisMonth, "2026-07-01", "2026-08-01"},
		{PresetLastMonth, "2026-06-01", "2026-07-01"},
		{PresetLast7d, "2026-07-09", "2026-07-16"},
		{PresetLast30d, "2026-06-16", "2026-07-16"},
		{PresetThisWeek, "2026-07-13", "2026-07-20"}, // Monday..next Monday
		{"unknown", "2026-07-01", "2026-08-01"},      // falls back to this_month
	}
	for _, c := range cases {
		start, end := RangeForPreset(c.preset, 2026, time.July, now)
		if start.Format("2006-01-02") != c.wantStart || end.Format("2006-01-02") != c.wantEnd {
			t.Errorf("%s: got [%s,%s), want [%s,%s)", c.preset,
				start.Format("2006-01-02"), end.Format("2006-01-02"), c.wantStart, c.wantEnd)
		}
	}
}

func TestPeriodLabelAndSlug(t *testing.T) {
	jul1 := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	aug1 := jul1.AddDate(0, 1, 0)
	if got := PeriodLabel(jul1, aug1); got != "July 2026" {
		t.Errorf("month label = %q", got)
	}
	if got := PeriodFileSlug(jul1, aug1); got != "2026-07" {
		t.Errorf("month slug = %q", got)
	}
	// custom [Jul 1, Jul 16) -> inclusive "to" is Jul 15
	mid := time.Date(2026, time.July, 16, 0, 0, 0, 0, time.UTC)
	if got := PeriodLabel(jul1, mid); got != "2026-07-01 → 2026-07-15" {
		t.Errorf("custom label = %q", got)
	}
	if got := PeriodFileSlug(jul1, mid); got != "2026-07-01_2026-07-15" {
		t.Errorf("custom slug = %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/report/ -run 'TestRangeForPreset|TestPeriodLabelAndSlug' -v`
Expected: FAIL — `undefined: PresetThisMonth` / `RangeForPreset`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/report/period.go`:

```go
package report

import "time"

// Range preset identifiers.
const (
	PresetThisMonth = "this_month"
	PresetLastMonth = "last_month"
	PresetLast7d    = "last_7d"
	PresetLast30d   = "last_30d"
	PresetThisWeek  = "this_week"
	PresetCustom    = "custom"
)

// midnightUTC returns the UTC midnight of t's date.
func midnightUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// RangeForPreset returns the half-open interval [start, end) for a non-custom
// preset. this_month uses the given year/month; the relative presets use now.
// An unknown preset falls back to this_month.
func RangeForPreset(preset string, year int, month time.Month, now time.Time) (start, end time.Time) {
	switch preset {
	case PresetLastMonth:
		m := time.Date(now.UTC().Year(), now.UTC().Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
		return m, m.AddDate(0, 1, 0)
	case PresetLast7d:
		e := midnightUTC(now).AddDate(0, 0, 1)
		return e.AddDate(0, 0, -7), e
	case PresetLast30d:
		e := midnightUTC(now).AddDate(0, 0, 1)
		return e.AddDate(0, 0, -30), e
	case PresetThisWeek:
		d := midnightUTC(now)
		wd := int(d.Weekday()) // Sunday=0..Saturday=6
		if wd == 0 {
			wd = 7 // treat Sunday as day 7 (ISO week ends Sunday)
		}
		mon := d.AddDate(0, 0, -(wd - 1))
		return mon, mon.AddDate(0, 0, 7)
	default: // PresetThisMonth and unknown
		return MonthRange(year, month)
	}
}

// isCalendarMonth reports whether [start, end) is exactly one calendar month.
func isCalendarMonth(start, end time.Time) bool {
	start = start.UTC()
	if start.Day() != 1 || start.Hour() != 0 || start.Minute() != 0 ||
		start.Second() != 0 || start.Nanosecond() != 0 {
		return false
	}
	return end.Equal(start.AddDate(0, 1, 0))
}

// PeriodLabel renders [start, end) as a calendar month ("January 2006") when it
// is exactly one, else "2006-01-02 → 2006-01-02" with the second date inclusive.
func PeriodLabel(start, end time.Time) string {
	if isCalendarMonth(start, end) {
		return start.UTC().Format("January 2006")
	}
	return start.UTC().Format("2006-01-02") + " → " + end.AddDate(0, 0, -1).UTC().Format("2006-01-02")
}

// PeriodFileSlug renders the period for an export filename: "2006-01" for a
// calendar month, else "2006-01-02_2006-01-02" (second date inclusive).
func PeriodFileSlug(start, end time.Time) string {
	if isCalendarMonth(start, end) {
		return start.UTC().Format("2006-01")
	}
	return start.UTC().Format("2006-01-02") + "_" + end.AddDate(0, 0, -1).UTC().Format("2006-01-02")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/report/ -race` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/report/period.go internal/report/period_test.go
git commit -m "feat(report): add date-range preset and period helpers"
```

---

### Task 2: report/export — Build over a period (Start/End), cross-cutting

**Files:**
- Modify: `internal/report/model.go`, `internal/report/aggregate.go`, `internal/report/aggregate_test.go`
- Modify: `internal/export/export.go`, `internal/export/export_test.go`
- Modify: `internal/tui/report.go`, `internal/tui/export.go`, `internal/tui/app.go`, `internal/tui/rates.go`
- Modify: `internal/tui/demo_test.go`, `internal/tui/log_test.go`, `internal/tui/app_test.go`

**Interfaces:**
- Consumes: `report.PeriodLabel`/`PeriodFileSlug` (Task 1), `report.MonthRange`.
- Produces: `Report.Start, End time.Time` (replaces `Year int`/`Month time.Month`); `Build(entries []TimeEntry, groupBy string, rates Rates, currency string, start, end time.Time) Report`.

This is a mechanical, wide change — keep it behavior-preserving (month period only, no UI change yet). Every `Build` call at this commit passes `report.MonthRange(m.year, m.month)` (TUI) or explicit month bounds (tests).

- [ ] **Step 1: Update the domain and its tests (RED via build break)**

In `internal/report/model.go`, replace the two `Report` period fields:

```go
// Report is the aggregated result ready for presentation/export.
type Report struct {
	Start       time.Time // period [Start, End)
	End         time.Time
	Scope       string // "me" | "team"
	GroupBy     string
	Currency    string
	Rate        float64
	Buckets     []Bucket
	TotalHours  float64
	TotalAmount float64
}
```

In `internal/report/aggregate.go`, change `Build`:

```go
func Build(entries []TimeEntry, groupBy string, rates Rates, currency string, start, end time.Time) Report {
	r := Report{
		Start:    start,
		End:      end,
		GroupBy:  groupBy,
		Currency: currency,
		Rate:     rates.Default,
	}
	// ... rest unchanged ...
```

In `internal/report/aggregate_test.go`, add package-level period vars and replace every `2026, time.July` Build argument pair with `julStart, julEnd`:

```go
var (
	julStart = time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	julEnd   = julStart.AddDate(0, 1, 0)
)
```

(e.g. `Build(sampleEntries(), GroupByTotal, Rates{Default: 50}, "EUR", julStart, julEnd)`.) No test asserts `Year`/`Month`, so only the call args change.

- [ ] **Step 2: Update export for the period**

In `internal/export/export.go`: add `"time"` to imports; change `jsonReport` and the Markdown header:

```go
type jsonReport struct {
	Start       string          `json:"start"`
	End         string          `json:"end"`
	Scope       string          `json:"scope"`
	GroupBy     string          `json:"group_by"`
	Currency    string          `json:"currency"`
	Rate        float64         `json:"rate"`
	Buckets     []report.Bucket `json:"buckets"`
	TotalHours  float64         `json:"total_hours"`
	TotalAmount float64         `json:"total_amount"`
}

func JSON(w io.Writer, r report.Report) error {
	jr := jsonReport{
		Start: r.Start.Format(time.RFC3339), End: r.End.Format(time.RFC3339),
		Scope: r.Scope, GroupBy: r.GroupBy,
		Currency: r.Currency, Rate: r.Rate, Buckets: r.Buckets,
		TotalHours: r.TotalHours, TotalAmount: r.TotalAmount,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jr)
}

func Markdown(w io.Writer, r report.Report) error {
	fmt.Fprintf(w, "# Hours report %s\n\n", report.PeriodLabel(r.Start, r.End))
	// ... rest unchanged ...
```

In `internal/export/export_test.go`, fix `sample()` and add a header assertion:

```go
func sample() report.Report {
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	return report.Report{
		Start: start, End: start.AddDate(0, 1, 0),
		Scope: "me", GroupBy: "list", Currency: "EUR", Rate: 50,
		Buckets: []report.Bucket{
			{Label: "Client A", Hours: 3, Amount: 150},
			{Label: "Client B", Hours: 3, Amount: 150},
		},
		TotalHours: 6, TotalAmount: 300,
	}
}
```

Add to `TestMarkdownTable`: assert the header renders the month label:

```go
	if !strings.Contains(out, "# Hours report July 2026") {
		t.Fatalf("missing md period header: %q", out)
	}
```

And in `TestJSONRoundTrips`, replace any year/month assertion is unnecessary (none exists); optionally assert `"start"` appears:

```go
	if !strings.Contains(b.String(), `"start":`) {
		t.Fatalf("json missing start: %s", b.String())
	}
```

- [ ] **Step 3: Update the TUI call sites (behavior-preserving: month period)**

In `internal/tui/report.go` `view`, render the period label:

```go
	title := styleTitle.Render(fmt.Sprintf("Report %s — scope %s%s — grouped by %s",
		report.PeriodLabel(r.Start, r.End), r.Scope, rm.note, r.GroupBy))
```

In `internal/tui/export.go`, the filename uses the slug:

```go
	path := fmt.Sprintf("clickup-report-%s.%s", report.PeriodFileSlug(e.r.Start, e.r.End), f.ext)
```

Add `"github.com/marcoarnulfo/clickup-cli/internal/report"` to `export.go` imports (it already imports it — confirm).

In the three production `Build` call sites, pass the month period explicitly for now:
- `internal/tui/app.go` (entriesMsg handler):
```go
		start, end := report.MonthRange(m.year, m.month)
		m.report = report.Build(msg.entries, groupBy, ratesFromConfig(m.cfg), m.cfg.Currency, start, end)
```
- `internal/tui/report.go` (`g` case) and `internal/tui/rates.go` (`s` case): same two-line pattern (`start, end := report.MonthRange(m.year, m.month)` then `Build(..., start, end)`).

Also fix `internal/tui/app_test.go` `TestExportWritesFile` (~line 334), which constructs a `report.Report` literal with the removed `Year`/`Month` fields — switch it to July `Start`/`End` bounds (so `PeriodFileSlug` still yields `2026-07` and the existing `clickup-report-2026-07.csv` assertion holds):

```go
	jStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	m.report = report.Report{Start: jStart, End: jStart.AddDate(0, 1, 0), Currency: "EUR",
		Buckets: []report.Bucket{{Label: "A", Hours: 1, Amount: 0}}, TotalHours: 1}
```

(`app_test.go` already imports `time` and `report`.)

In tests: `internal/tui/demo_test.go:98` and `internal/tui/log_test.go:30` — replace the `2026, time.July` / `m.year, m.month` trailing args with a period, e.g.:
```go
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	r := report.Build(entries, report.GroupByList, rates, "EUR", start, start.AddDate(0, 1, 0))
```
and for `log_test.go`:
```go
	start, end := report.MonthRange(m.year, m.month)
	m.report = report.Build(m.entries, report.GroupByTotal, ratesFromConfig(cfg), "EUR", start, end)
```

- [ ] **Step 4: Run the full suite**

Run: `go build ./...` → clean. `go test ./... -race` → PASS. `gofmt -l .` empty; `go vet ./...`, `staticcheck ./...` clean.

- [ ] **Step 5: Commit**

```bash
git add internal/report/model.go internal/report/aggregate.go internal/report/aggregate_test.go internal/export/ internal/tui/report.go internal/tui/export.go internal/tui/app.go internal/tui/rates.go internal/tui/demo_test.go internal/tui/log_test.go internal/tui/app_test.go
git commit -m "refactor(report): build over a [start,end) period instead of year/month"
```

---

### Task 3: root range state + range-aware loading (no UI yet)

**Files:**
- Modify: `internal/tui/app.go`
- Test: `internal/tui/app_test.go`

**Interfaces:**
- Consumes: `report.RangeForPreset`/`PresetThisMonth`/`PresetCustom` (Task 1), `report.MonthRange`.
- Produces: `Model.preset string`, `Model.customStart, customEnd time.Time`; `func (m Model) currentRange() (start, end time.Time)`; `loadEntriesCmd(c, teamID string, start, end time.Time, scope string, assignees []int) tea.Cmd` (period instead of year/month).

- [ ] **Step 1: Write the failing tests**

Add to `internal/tui/app_test.go`:

```go
func TestCurrentRangeDefaultsToMonth(t *testing.T) {
	m := Model{year: 2026, month: time.July, preset: report.PresetThisMonth}
	start, end := m.currentRange()
	ws, we := report.MonthRange(2026, time.July)
	if !start.Equal(ws) || !end.Equal(we) {
		t.Errorf("currentRange = [%s,%s), want month", start, end)
	}
}

func TestCurrentRangeCustomIsInclusive(t *testing.T) {
	from := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	m := Model{preset: report.PresetCustom, customStart: from, customEnd: to}
	start, end := m.currentRange()
	if !start.Equal(from) || !end.Equal(to.AddDate(0, 0, 1)) {
		t.Errorf("custom range = [%s,%s), want [%s, %s+1d)", start, end, from, to)
	}
}

func TestLoadEntriesUsesGivenRange(t *testing.T) {
	var gotStart, gotEnd string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/time_entries") {
			gotStart = r.URL.Query().Get("start_date")
			gotEnd = r.URL.Query().Get("end_date")
			w.Write([]byte(`{"data":[]}`))
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := clickup.New("tok")
	c.BaseURL = srv.URL
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 10)
	if _, ok := loadEntriesCmd(c, "900", start, end, "me", nil)().(entriesMsg); !ok {
		t.Fatal("expected entriesMsg")
	}
	if gotStart != strconv.FormatInt(start.UnixMilli(), 10) || gotEnd != strconv.FormatInt(end.UnixMilli(), 10) {
		t.Errorf("range not forwarded: start=%s end=%s", gotStart, gotEnd)
	}
}
```

(Add `"strconv"` to the test imports if missing.)

- [ ] **Step 2: Run to verify it fails**

Run: `go build ./... 2>&1 | head` — Expected: `undefined: currentRange` / signature mismatch.

- [ ] **Step 3: Write minimal implementation**

In `internal/tui/app.go`:

(a) Add root fields (near `year`/`month`):

```go
	preset      string    // report.Preset* ; default report.PresetThisMonth
	customStart time.Time // used when preset == report.PresetCustom
	customEnd   time.Time
```

(b) Initialize `preset` in `New`:

```go
	m := Model{
		cfg:    cfg,
		demo:   demo,
		year:   now.Year(),
		month:  now.Month(),
		scope:  "me",
		preset: report.PresetThisMonth,
		client: clickup.New(cfg.Token),
	}
```

(c) Add `currentRange`:

```go
// currentRange returns the [start, end) period the report should cover, from the
// active preset (custom uses the inclusive customStart..customEnd).
func (m Model) currentRange() (start, end time.Time) {
	if m.preset == report.PresetCustom {
		return m.customStart, m.customEnd.AddDate(0, 0, 1)
	}
	return report.RangeForPreset(m.preset, m.year, m.month, time.Now())
}
```

(d) Change `loadEntriesCmd` to take `start, end time.Time` instead of `year, month`, and drop the internal `MonthRange` call:

```go
func loadEntriesCmd(c *clickup.Client, teamID string, start, end time.Time, scope string, assignees []int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if scope == "team" && len(assignees) == 0 {
			members, err := c.TeamMembers(ctx, teamID)
			if err != nil {
				return errMsg{err: err}
			}
			for _, mem := range members {
				assignees = append(assignees, mem.ID)
			}
		}

		entries, err := c.TimeEntries(ctx, teamID, start, end, assignees)
		if err != nil {
			return errMsg{err: err}
		}
		// ... list-name resolution unchanged ...
		return entriesMsg{entries: entries}
	}
}
```

(e) `reloadEntriesCmd` passes the current range:

```go
func (m Model) reloadEntriesCmd() tea.Cmd {
	var assignees []int
	if m.scope == "team" {
		assignees = m.selectedAssignees()
	}
	start, end := m.currentRange()
	if m.demo {
		if m.scope != "team" {
			assignees = []int{demoSelfID}
		}
		return demoEntriesCmd(start, end, assignees)
	}
	return loadEntriesCmd(m.client, m.cfg.WorkspaceID, start, end, m.scope, assignees)
}
```

(f) In the `entriesMsg` handler, build over `m.currentRange()` (replaces ONLY the `MonthRange`/`Build` lines added in Task 2 — keep the existing `groupBy` computation above them intact, including the `GroupByMember → GroupByTotal` guard when `scope != "team"`):

```go
		start, end := m.currentRange()
		m.report = report.Build(msg.entries, groupBy, ratesFromConfig(m.cfg), m.cfg.Currency, start, end)
```

Do the same in `report.go` (`g`) and `rates.go` (`s`): `start, end := m.currentRange()`.

Note: `demoEntriesCmd` now receives `start, end` (see Task 3a below) — its signature changes from `(year, month, assignees)` to `(start, end, assignees)`. Update `demo.go` accordingly.

- [ ] **Step 3a: Update demo to a range**

In `internal/tui/demo.go`, change `demoEntriesCmd` to filter by the range on `Start` (demo entries carry `Start` in the current month; a range outside it yields none — acceptable for demo):

```go
// demoEntriesCmd delivers the fake entries as entriesMsg, filtered by the
// selected member ids and clipped to [start, end).
func demoEntriesCmd(start, end time.Time, assignees []int) tea.Cmd {
	return func() tea.Msg {
		entries := filterByUsers(demoEntries(start.Year(), start.Month()), assignees)
		out := entries[:0]
		for _, e := range entries {
			if !e.Start.Before(start) && e.Start.Before(end) {
				out = append(out, e)
			}
		}
		return entriesMsg{entries: out}
	}
}
```

Update the demo test `TestReloadDemoFiltersMembers`/`TestReloadEntriesCmdUsesDemo`/`TestReloadDemoMeScopeIsSingleSelfUser` if they call `demoEntriesCmd` directly — they call `reloadEntriesCmd`, so they keep working (default preset this_month covers the demo month).

- [ ] **Step 4: Update existing loadEntriesCmd call sites in tests**

`internal/tui/app_test.go` existing tests call `loadEntriesCmd(c, "900", 2026, time.July, "team"|"me", …)`. Replace the `2026, time.July` pair with a July period, e.g.:
```go
	jStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	jEnd := jStart.AddDate(0, 1, 0)
	msg := loadEntriesCmd(c, "900", jStart, jEnd, "team", nil)()
```
Apply to all existing `loadEntriesCmd(...)` calls in `app_test.go`.

- [ ] **Step 5: Run the full suite**

Run: `go build ./...`, `go test ./... -race`, `gofmt -l .`, `go vet ./...`, `staticcheck ./...` — all clean/green.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go internal/tui/report.go internal/tui/rates.go internal/tui/demo.go
git commit -m "feat(tui): load and build the report over the active date range"
```

---

### Task 4: range picker screen + Home entry + range docs

**Files:**
- Create: `internal/tui/range.go`
- Modify: `internal/tui/app.go` (screen const, routeKey, View), `internal/tui/home.go` (`d` key, view label)
- Test: `internal/tui/range_test.go` (new)
- Modify: `README.md`, `README.it.md`

**Interfaces:**
- Consumes: `report.Preset*`, `Model.preset/customStart/customEnd`, `newNumberInput`/`newTextInput` from `setup.go` (text input helper).
- Produces: `screenRange` (screen const), `rangeModel`, `newRange(current string) rangeModel`, `func (m Model) updateRange(msg tea.KeyMsg) (tea.Model, tea.Cmd)`, `Model.rangeScreen rangeModel`.

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/range_test.go`:

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func TestRangeSelectPreset(t *testing.T) {
	m := Model{screen: screenRange, preset: report.PresetThisMonth, rangeScreen: newRange(report.PresetThisMonth)}
	// move to "last_7d" and confirm (order: this_month, last_month, last_7d, ...)
	m.rangeScreen.idx = 2
	u, _ := m.updateRange(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if m.preset != report.PresetLast7d {
		t.Errorf("preset = %q, want last_7d", m.preset)
	}
	if m.screen != screenHome {
		t.Errorf("screen = %v, want home", m.screen)
	}
}

func TestRangeCustomValidDates(t *testing.T) {
	m := Model{screen: screenRange, rangeScreen: newRange(report.PresetThisMonth)}
	m.rangeScreen.idx = 5 // "custom"
	u, _ := m.updateRange(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	rs := m.rangeScreen
	rs.editing = true
	rs.fromInput.SetValue("2026-07-01")
	rs.toInput.SetValue("2026-07-15")
	rs.field = 1 // on the "to" field
	m.rangeScreen = rs
	u, _ = m.updateRange(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if m.preset != report.PresetCustom {
		t.Fatalf("preset = %q, want custom", m.preset)
	}
	if m.customStart.Format("2006-01-02") != "2026-07-01" || m.customEnd.Format("2006-01-02") != "2026-07-15" {
		t.Errorf("custom = %s..%s", m.customStart.Format("2006-01-02"), m.customEnd.Format("2006-01-02"))
	}
	if m.screen != screenHome {
		t.Errorf("screen = %v, want home after valid custom", m.screen)
	}
}

func TestRangeCustomInvalidStays(t *testing.T) {
	m := Model{screen: screenRange, rangeScreen: newRange(report.PresetThisMonth)}
	rs := m.rangeScreen
	rs.idx = 5
	rs.editing = true
	rs.fromInput.SetValue("nope")
	rs.toInput.SetValue("2026-07-15")
	rs.field = 1
	m.rangeScreen = rs
	u, _ := m.updateRange(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if m.screen != screenRange {
		t.Errorf("invalid custom should stay on range screen, got %v", m.screen)
	}
	if m.rangeScreen.msg == "" {
		t.Error("expected a validation message")
	}
}

func TestHomeDOpensRange(t *testing.T) {
	m := Model{screen: screenHome, preset: report.PresetThisMonth}
	u, _ := m.updateHome(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = u.(Model)
	if m.screen != screenRange {
		t.Errorf("d should open range screen, got %v", m.screen)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go build ./... 2>&1 | head` — Expected: `undefined: newRange` / `screenRange` / `rangeScreen`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/tui/range.go`:

```go
package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// rangePreset pairs a preset id with its menu label.
type rangePreset struct {
	id    string
	label string
}

var rangePresets = []rangePreset{
	{report.PresetThisMonth, "This month"},
	{report.PresetLastMonth, "Last month"},
	{report.PresetLast7d, "Last 7 days"},
	{report.PresetLast30d, "Last 30 days"},
	{report.PresetThisWeek, "This week"},
	{report.PresetCustom, "Custom…"},
}

type rangeModel struct {
	idx       int  // selected preset row
	editing   bool // custom from/to inputs shown
	field     int  // 0 = from, 1 = to
	fromInput textinput.Model
	toInput   textinput.Model
	msg       string // validation error
}

// newRange builds the picker with the current preset preselected.
func newRange(current string) rangeModel {
	rm := rangeModel{}
	for i, p := range rangePresets {
		if p.id == current {
			rm.idx = i
		}
	}
	return rm
}

func (m Model) updateRange(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rs := m.rangeScreen

	if rs.editing {
		switch msg.Type {
		case tea.KeyEnter:
			if rs.field == 0 {
				rs.field = 1
				rs.fromInput.Blur()
				rs.toInput.Focus()
				m.rangeScreen = rs
				return m, nil
			}
			from, errF := time.Parse("2006-01-02", rs.fromInput.Value())
			to, errT := time.Parse("2006-01-02", rs.toInput.Value())
			if errF != nil || errT != nil {
				rs.msg = "Invalid date: use YYYY-MM-DD"
				m.rangeScreen = rs
				return m, nil
			}
			if to.Before(from) {
				rs.msg = "'To' must not be before 'From'"
				m.rangeScreen = rs
				return m, nil
			}
			m.preset = report.PresetCustom
			m.customStart = from
			m.customEnd = to
			m.screen = screenHome
			return m, nil
		case tea.KeyEsc:
			rs.editing = false
			rs.msg = ""
			m.rangeScreen = rs
			return m, nil
		}
		var cmd tea.Cmd
		if rs.field == 0 {
			rs.fromInput, cmd = rs.fromInput.Update(msg)
		} else {
			rs.toInput, cmd = rs.toInput.Update(msg)
		}
		m.rangeScreen = rs
		return m, cmd
	}

	switch msg.String() {
	case "up", "k":
		if rs.idx > 0 {
			rs.idx--
		}
	case "down", "j":
		if rs.idx < len(rangePresets)-1 {
			rs.idx++
		}
	case "enter":
		p := rangePresets[rs.idx]
		if p.id == report.PresetCustom {
			rs.editing = true
			rs.field = 0
			rs.msg = ""
			rs.fromInput = newTextInput("From (YYYY-MM-DD)")
			rs.toInput = newTextInput("To (YYYY-MM-DD)")
			rs.fromInput.Focus()
			m.rangeScreen = rs
			return m, nil
		}
		m.preset = p.id
		m.screen = screenHome
		return m, nil
	case "esc":
		m.screen = screenHome
		return m, nil
	}
	m.rangeScreen = rs
	return m, nil
}

func (rs rangeModel) view() string {
	b := styleTitle.Render("Report range") + "\n\n"
	for i, p := range rangePresets {
		cursor := "  "
		line := p.label
		if i == rs.idx {
			cursor = "▸ "
			line = styleAccent.Render(line)
		}
		b += cursor + line + "\n"
	}
	if rs.editing {
		b += "\n" + rs.fromInput.View() + "\n" + rs.toInput.View() + "\n"
	}
	if rs.msg != "" {
		b += "\n" + styleErr.Render(rs.msg)
	}
	b += "\n" + styleHelp.Render("↑/↓ select · Enter: choose/next · Esc: back")
	return b
}
```

In `internal/tui/app.go`: append `screenRange` to the `screen` iota; add the field `rangeScreen rangeModel`; add `case screenRange: return m.updateRange(msg)` to `routeKey`; add `case screenRange: return m.rangeScreen.view()` to `View`.

In `internal/tui/home.go` `updateHome`, add:

```go
	case "d":
		m.rangeScreen = newRange(m.preset)
		m.screen = screenRange
		return m, nil
```

And show the range in the Home view — replace the `sel` line and help. Change the `view` signature to also receive a range label; simplest: pass `m.rangeLabel()` as an extra parameter. Add to `home.go`:

```go
// rangeLabel returns a short label for the active range shown on Home.
func (m Model) rangeLabel() string {
	start, end := m.currentRange()
	return report.PeriodLabel(start, end)
}
```

Change `homeModel.view` to `view(rangeLabel, scope, membersNote string)` and render `Range: <rangeLabel>` in place of the month line; keep `◂/▸` working only for the month preset (guard the `left`/`right` cases in `updateHome` with `if m.preset == report.PresetThisMonth`). Update the root `View` call: `m.home.view(m.rangeLabel(), m.scope, m.homeMembersNote())`. Add the `report` import to `home.go`. Update the help line to include `d: range` and keep `◂/▸ change month` only meaningful in the month preset.

> NOTE for the implementer: `newTextInput` is defined in `setup.go` — reuse it (do not redefine). Confirm its signature there before use.

- [ ] **Step 4: Run the full suite**

Run: `go build ./...`, `go test ./... -race`, `gofmt -l .`, `go vet ./...`, `staticcheck ./...` — all clean/green.

- [ ] **Step 5: Update the READMEs (range)**

In `README.md` and `README.it.md`: add a `d` key row to the TUI commands table (Home → open the report range picker: presets + custom from/to); note that the report is no longer month-only. Keep both languages in sync.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/range.go internal/tui/range_test.go internal/tui/app.go internal/tui/home.go README.md README.it.md
git commit -m "feat(tui): report range picker (presets + custom dates)"
```

---

# Phase 2 — Filters (list / tag / status)

### Task 5: report — Tags/Status fields + Filter (pure)

**Files:**
- Modify: `internal/report/model.go`
- Create: `internal/report/filter.go`
- Test: `internal/report/filter_test.go`

**Interfaces:**
- Produces: `TimeEntry.Tags []string`, `TimeEntry.Status string`; `FilterCriteria{ Lists, Tags, Statuses map[string]bool }` with `Empty() bool`; `Filter(entries []TimeEntry, c FilterCriteria) []TimeEntry`.

- [ ] **Step 1: Write the failing tests**

Create `internal/report/filter_test.go`:

```go
package report

import (
	"testing"
	"time"
)

func fe(list, status string, tags ...string) TimeEntry {
	return TimeEntry{ListName: list, Status: status, Tags: tags, Duration: time.Hour}
}

func names(entries []TimeEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.ListName + "/" + e.Status
	}
	return out
}

func TestFilterEmptyReturnsAll(t *testing.T) {
	in := []TimeEntry{fe("A", "open"), fe("B", "done")}
	if got := Filter(in, FilterCriteria{}); len(got) != 2 {
		t.Fatalf("empty criteria should return all, got %d", len(got))
	}
}

func TestFilterListOR(t *testing.T) {
	in := []TimeEntry{fe("A", "open"), fe("B", "open"), fe("C", "open")}
	c := FilterCriteria{Lists: map[string]bool{"A": true, "C": true}}
	if got := Filter(in, c); len(got) != 2 {
		t.Fatalf("list OR should keep A and C, got %v", names(got))
	}
}

func TestFilterAcrossDimensionsAND(t *testing.T) {
	in := []TimeEntry{
		fe("A", "open", "urgent"),
		fe("A", "done", "urgent"),
		fe("B", "open", "urgent"),
	}
	c := FilterCriteria{Lists: map[string]bool{"A": true}, Statuses: map[string]bool{"open": true}}
	got := Filter(in, c)
	if len(got) != 1 || got[0].ListName != "A" || got[0].Status != "open" {
		t.Fatalf("list AND status should keep 1, got %v", names(got))
	}
}

func TestFilterTagAnyMatch(t *testing.T) {
	in := []TimeEntry{
		fe("A", "open", "frontend", "urgent"),
		fe("A", "open", "backend"),
	}
	c := FilterCriteria{Tags: map[string]bool{"urgent": true}}
	if got := Filter(in, c); len(got) != 1 {
		t.Fatalf("tag any-match should keep 1, got %v", names(got))
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/report/ -run TestFilter -v` → FAIL (`undefined: FilterCriteria` / `Filter` / `TimeEntry.Status`).

- [ ] **Step 3: Write minimal implementation**

In `internal/report/model.go`, add to `TimeEntry`:

```go
	Tags     []string
	Status   string
```

Create `internal/report/filter.go`:

```go
package report

// FilterCriteria selects entries by list, tag, and status. Within a dimension the
// match is OR (any selected value); across dimensions it is AND. An empty
// dimension imposes no constraint.
type FilterCriteria struct {
	Lists    map[string]bool
	Tags     map[string]bool
	Statuses map[string]bool
}

// Empty reports whether no dimension constrains anything.
func (c FilterCriteria) Empty() bool {
	return countTrue(c.Lists) == 0 && countTrue(c.Tags) == 0 && countTrue(c.Statuses) == 0
}

func countTrue(m map[string]bool) int {
	n := 0
	for _, v := range m {
		if v {
			n++
		}
	}
	return n
}

// Filter returns the entries matching the criteria. An entry matches when, for
// every constrained dimension, it satisfies that dimension: its list is selected,
// at least one of its tags is selected, and its status is selected.
func Filter(entries []TimeEntry, c FilterCriteria) []TimeEntry {
	if c.Empty() {
		return entries
	}
	nL, nT, nS := countTrue(c.Lists), countTrue(c.Tags), countTrue(c.Statuses)
	out := make([]TimeEntry, 0, len(entries))
	for _, e := range entries {
		if nL > 0 && !c.Lists[e.ListName] {
			continue
		}
		if nT > 0 && !anyTagSelected(e.Tags, c.Tags) {
			continue
		}
		if nS > 0 && !c.Statuses[e.Status] {
			continue
		}
		out = append(out, e)
	}
	return out
}

func anyTagSelected(tags []string, sel map[string]bool) bool {
	for _, t := range tags {
		if sel[t] {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests** → `go test ./internal/report/ -race` PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/report/model.go internal/report/filter.go internal/report/filter_test.go
git commit -m "feat(report): entry tags/status and a pure report filter"
```

---

### Task 6: clickup — task tags on load + TaskStatus

**Files:**
- Modify: `internal/clickup/timeentries.go`, `internal/clickup/clickup_test.go`
- Create: `internal/clickup/task.go`, `internal/clickup/task_test.go`

**Interfaces:**
- Produces: `TimeEntry.Tags` populated from `task_tags`; `TaskStatus(ctx, taskID string) (string, error)`.

- [ ] **Step 1: Write the failing tests**

In `internal/clickup/clickup_test.go`, extend the time-entries fixture to include `task_tags` and assert `Tags`. Add (or adapt an existing `TimeEntries` test) a case whose entry JSON has:
`"task_tags":[{"name":"urgent"},{"name":"frontend"}]` and assert the returned entry's `Tags == []string{"urgent","frontend"}` and that the request carried `include_task_tags=true`.

```go
func TestTimeEntriesParsesTaskTags(t *testing.T) {
	var gotInclude string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotInclude = r.URL.Query().Get("include_task_tags")
		w.Write([]byte(`{"data":[{"id":"e1","task":{"id":"t","name":"T"},"task_tags":[{"name":"urgent"},{"name":"frontend"}],"task_location":{"list_id":"5"},"user":{"id":1,"username":"x"},"start":"1751360400000","duration":"3600000"}]}`))
	}))
	defer srv.Close()
	c := New("tok")
	c.BaseURL = srv.URL
	start := time.UnixMilli(1751360400000).UTC()
	entries, err := c.TimeEntries(context.Background(), "900", start, start.Add(24*time.Hour), nil)
	if err != nil {
		t.Fatal(err)
	}
	if gotInclude != "true" {
		t.Errorf("include_task_tags = %q, want true", gotInclude)
	}
	if len(entries) != 1 || len(entries[0].Tags) != 2 || entries[0].Tags[0] != "urgent" {
		t.Errorf("tags = %+v", entries[0].Tags)
	}
}
```

Create `internal/clickup/task_test.go`:

```go
package clickup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTaskStatus(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"id":"t1","status":{"status":"in progress","type":"custom"}}`))
	}))
	defer srv.Close()
	c := New("tok")
	c.BaseURL = srv.URL
	st, err := c.TaskStatus(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/task/t1" {
		t.Errorf("path = %q", gotPath)
	}
	if st != "in progress" {
		t.Errorf("status = %q", st)
	}
}
```

- [ ] **Step 2: Run to verify it fails** → FAIL (`undefined: (*Client).TaskStatus`; tags field/param missing).

- [ ] **Step 3: Write minimal implementation**

> VERIFY FIRST (implementer): confirm the ClickUp response shape for tags when `include_task_tags=true`. This plan assumes an entry-level `task_tags: [{name}]` array. If ClickUp instead nests tags under `task.tags`, adjust the `rawEntry` field + JSON tag accordingly (and the test fixture) — the rest of the task is unchanged. Use context7 / the ClickUp API docs to check.

In `internal/clickup/timeentries.go`: add `include_task_tags` to the query map in `TimeEntries`:
```go
	q := map[string]string{
		"start_date":        strconv.FormatInt(start.UnixMilli(), 10),
		"end_date":          strconv.FormatInt(end.UnixMilli(), 10),
		"include_task_tags": "true",
	}
```
Add `TaskTags` to `rawEntry` and map it in `toTimeEntry`:
```go
	TaskTags []struct {
		Name string `json:"name"`
	} `json:"task_tags"`
```
```go
	tags := make([]string, 0, len(r.TaskTags))
	for _, t := range r.TaskTags {
		tags = append(tags, t.Name)
	}
	return report.TimeEntry{
		// ... existing fields ...
		Tags: tags,
	}, nil
```

Create `internal/clickup/task.go`:

```go
package clickup

import "context"

// TaskStatus returns the current status name of a task (GET /task/{id}).
func (c *Client) TaskStatus(ctx context.Context, taskID string) (string, error) {
	var resp struct {
		Status struct {
			Status string `json:"status"`
		} `json:"status"`
	}
	if err := c.get(ctx, "/task/"+taskID, nil, &resp); err != nil {
		return "", err
	}
	return resp.Status.Status, nil
}
```

- [ ] **Step 4: Run tests** → `go test ./internal/clickup/ -race` PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/clickup/timeentries.go internal/clickup/clickup_test.go internal/clickup/task.go internal/clickup/task_test.go
git commit -m "feat(clickup): task tags on time entries and TaskStatus lookup"
```

---

### Task 7: root filter state + apply filter to every build (no screen yet)

**Files:**
- Modify: `internal/tui/app.go`, `internal/tui/report.go`, `internal/tui/rates.go`, `internal/tui/demo.go`
- Test: `internal/tui/app_test.go`

**Interfaces:**
- Consumes: `report.Filter`/`FilterCriteria` (Task 5).
- Produces: `Model.filterLists, filterTags, filterStatuses map[string]bool`; `filterCriteria() report.FilterCriteria`; `visibleEntries() []report.TimeEntry`; `filteredNote() string`. Demo entries gain tags/status.

- [ ] **Step 1: Write the failing tests**

Add to `internal/tui/app_test.go`:

```go
func TestVisibleEntriesAppliesFilter(t *testing.T) {
	m := Model{
		entries: []report.TimeEntry{
			{ListName: "A", Duration: time.Hour},
			{ListName: "B", Duration: time.Hour},
		},
		filterLists: map[string]bool{"A": true},
	}
	got := m.visibleEntries()
	if len(got) != 1 || got[0].ListName != "A" {
		t.Fatalf("visibleEntries = %+v", got)
	}
	if m.filteredNote() != " · filtered" {
		t.Errorf("filteredNote = %q", m.filteredNote())
	}
	m.filterLists = nil
	if m.filteredNote() != "" {
		t.Errorf("no filter -> empty note, got %q", m.filteredNote())
	}
}

func TestEntriesMsgBuildsFilteredReport(t *testing.T) {
	m := Model{year: 2026, month: time.July, preset: report.PresetThisMonth, filterLists: map[string]bool{"A": true}}
	u, _ := m.Update(entriesMsg{entries: []report.TimeEntry{
		{ListName: "A", Duration: time.Hour, Start: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)},
		{ListName: "B", Duration: 2 * time.Hour, Start: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)},
	}})
	m = u.(Model)
	if m.report.TotalHours != 1 {
		t.Errorf("filtered report total = %v, want 1", m.report.TotalHours)
	}
}
```

- [ ] **Step 2: Run to verify it fails** → build error (`undefined: visibleEntries`).

- [ ] **Step 3: Write minimal implementation**

In `internal/tui/app.go` add root fields and helpers:

```go
	filterLists    map[string]bool
	filterTags     map[string]bool
	filterStatuses map[string]bool
```

```go
// filterCriteria assembles the active client-side filter from session state.
func (m Model) filterCriteria() report.FilterCriteria {
	return report.FilterCriteria{Lists: m.filterLists, Tags: m.filterTags, Statuses: m.filterStatuses}
}

// visibleEntries applies the active filter to the loaded entries.
func (m Model) visibleEntries() []report.TimeEntry {
	return report.Filter(m.entries, m.filterCriteria())
}

// filteredNote returns " · filtered" when any client-side filter is active.
func (m Model) filteredNote() string {
	if m.filterCriteria().Empty() {
		return ""
	}
	return " · filtered"
}
```

Route every report build through `visibleEntries()` and combine the notes. Change ONLY the `Build` entries argument (`msg.entries`/`m.entries` → `m.visibleEntries()`) and the `newReport` note argument — leave each site's `groupBy` computation untouched (the `entriesMsg` handler keeps its default-total + `GroupByMember→GroupByTotal` guard; the `g` case keeps its `nextGroupBy(...)`). In the `entriesMsg` handler, `report.go` (`g`) and `rates.go` (`s`):
```go
		start, end := m.currentRange()
		m.report = report.Build(m.visibleEntries(), groupBy, ratesFromConfig(m.cfg), m.cfg.Currency, start, end)
		m.report.Scope = m.scope
		m.rep = newReport(m.report, m.memberFilterNote()+m.filteredNote())
```
(In the `entriesMsg` handler the local is `groupBy`; in `g`/`s` it is `g`. Keep `newRates(m.entries, m.cfg)` on the FULL entries — you configure rates for all lists, not the filtered subset.)

In `internal/tui/demo.go`, give demo entries tags and a status so tag/status filtering is meaningful. Change `mk` and the rows to include `tags []string, status string`, e.g.:
```go
	mk := func(id, taskID, task, listID, list string, uid int, user string, tags []string, status string, start time.Time, dur time.Duration) report.TimeEntry {
		return report.TimeEntry{
			ID: id, TaskID: taskID, TaskName: task,
			ListID: listID, ListName: list,
			UserID: uid, UserName: user,
			Tags: tags, Status: status,
			Start: start, Duration: dur,
		}
	}
```
and distribute plausible tags (`"frontend"`, `"backend"`, `"qa"`) and statuses (`"in progress"`, `"done"`) across the 6 rows. (`TestDemoEntriesBuildReport`/`MultipleUsers` still pass — lists/users unchanged.)

- [ ] **Step 4: Run the full suite** → all clean/green.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go internal/tui/report.go internal/tui/rates.go internal/tui/demo.go
git commit -m "feat(tui): apply a client-side report filter over loaded entries"
```

---

### Task 8: Filters screen (Lists / Tags / Statuses)

**Files:**
- Create: `internal/tui/filters.go`
- Modify: `internal/tui/app.go` (screen const, field, routeKey, View), `internal/tui/report.go` (`f` key + help)
- Test: `internal/tui/filters_test.go` (new)

**Interfaces:**
- Consumes: `report.TimeEntry` fields, `Model.filterLists/Tags/Statuses`.
- Produces: `screenFilters`, `filtersModel`, `newFilters(entries []report.TimeEntry, lists, tags, statuses map[string]bool) filtersModel`, `func (m Model) updateFilters(msg tea.KeyMsg) (tea.Model, tea.Cmd)`, `Model.filtersScreen filtersModel`.

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/filters_test.go`:

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func filtersFixture() Model {
	entries := []report.TimeEntry{
		{ListName: "Website", Tags: []string{"frontend"}, Status: "in progress"},
		{ListName: "Mobile", Tags: []string{"backend"}, Status: "done"},
	}
	m := Model{screen: screenFilters, entries: entries}
	m.filtersScreen = newFilters(entries, nil, nil, nil)
	return m
}

func TestFiltersToggleAndApply(t *testing.T) {
	m := filtersFixture()
	// section 0 = Lists; toggle first option (row 0)
	u, _ := m.updateFilters(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = u.(Model)
	u, _ = m.updateFilters(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if m.screen != screenReport {
		t.Fatalf("apply should go to report, got %v", m.screen)
	}
	if len(m.filterLists) == 0 {
		t.Fatal("expected a list filter written to root")
	}
}

func TestFiltersTabChangesSection(t *testing.T) {
	m := filtersFixture()
	u, _ := m.updateFilters(tea.KeyMsg{Type: tea.KeyTab})
	m = u.(Model)
	if m.filtersScreen.sec != 1 {
		t.Errorf("tab should move to section 1, got %d", m.filtersScreen.sec)
	}
}

func TestFiltersEscDiscards(t *testing.T) {
	m := filtersFixture()
	u, _ := m.updateFilters(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = u.(Model)
	u, _ = m.updateFilters(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(Model)
	if m.screen != screenReport {
		t.Errorf("esc should return to report, got %v", m.screen)
	}
	if len(m.filterLists) != 0 {
		t.Error("esc must not write filters to root")
	}
}

func TestReportFOpensFilters(t *testing.T) {
	m := Model{screen: screenReport, entries: []report.TimeEntry{{ListName: "A"}}}
	u, _ := m.updateReport(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	m = u.(Model)
	if m.screen != screenFilters {
		t.Errorf("f should open filters, got %v", m.screen)
	}
}
```

- [ ] **Step 2: Run to verify it fails** → build error (`undefined: newFilters` / `screenFilters`).

- [ ] **Step 3: Write minimal implementation**

Create `internal/tui/filters.go`:

```go
package tui

import (
	"slices"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// filterSection is one dimension of the Filters screen.
type filterSection struct {
	title    string
	options  []string
	selected map[string]bool
}

type filtersModel struct {
	sections        []filterSection // [Lists, Tags, Statuses]
	sec             int             // active section index
	row             int             // active row within the section
	loadingStatuses bool
}

// newFilters builds the screen from the entries' lists/tags/statuses, preselecting
// from the current criteria (copied defensively so Esc can discard).
func newFilters(entries []report.TimeEntry, lists, tags, statuses map[string]bool) filtersModel {
	listOpts := map[string]bool{}
	tagOpts := map[string]bool{}
	statusOpts := map[string]bool{}
	for _, e := range entries {
		if e.ListName != "" {
			listOpts[e.ListName] = true
		}
		for _, t := range e.Tags {
			tagOpts[t] = true
		}
		if e.Status != "" {
			statusOpts[e.Status] = true
		}
	}
	return filtersModel{
		sections: []filterSection{
			{title: "Lists", options: sortedKeys(listOpts), selected: copyBool(lists)},
			{title: "Tags", options: sortedKeys(tagOpts), selected: copyBool(tags)},
			{title: "Statuses", options: sortedKeys(statusOpts), selected: copyBool(statuses)},
		},
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func copyBool(m map[string]bool) map[string]bool {
	out := make(map[string]bool, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (m Model) updateFilters(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	fs := m.filtersScreen
	if fs.loadingStatuses {
		return m, nil
	}
	cur := &fs.sections[fs.sec]
	switch msg.String() {
	case "tab":
		fs.sec = (fs.sec + 1) % len(fs.sections)
		fs.row = 0
	case "shift+tab":
		fs.sec = (fs.sec - 1 + len(fs.sections)) % len(fs.sections)
		fs.row = 0
	case "up", "k":
		if fs.row > 0 {
			fs.row--
		}
	case "down", "j":
		if fs.row < len(cur.options)-1 {
			fs.row++
		}
	case " ", "space":
		if len(cur.options) > 0 {
			opt := cur.options[fs.row]
			cur.selected[opt] = !cur.selected[opt]
		}
	case "a":
		all := allChosen(*cur)
		for _, o := range cur.options {
			cur.selected[o] = !all
		}
	case "enter":
		m.filterLists = fs.sections[0].selected
		m.filterTags = fs.sections[1].selected
		m.filterStatuses = fs.sections[2].selected
		m.filtersScreen = fs
		m.applyReport()
		m.screen = screenReport
		return m, nil
	case "esc":
		m.screen = screenReport
		return m, nil
	}
	m.filtersScreen = fs
	return m, nil
}

// allChosen reports whether every option in a section is selected.
func allChosen(s filterSection) bool {
	if len(s.options) == 0 {
		return false
	}
	for _, o := range s.options {
		if !s.selected[o] {
			return false
		}
	}
	return true
}

func (fs filtersModel) view() string {
	if fs.loadingStatuses {
		return styleTitle.Render("Loading statuses…")
	}
	b := styleTitle.Render("Filters") + "\n\n"
	for si, sec := range fs.sections {
		head := sec.title
		if si == fs.sec {
			head = styleAccent.Render("▸ " + sec.title)
		} else {
			head = "  " + head
		}
		b += head + "\n"
		if len(sec.options) == 0 {
			b += "    " + styleHelp.Render("(none)") + "\n"
		}
		for ri, o := range sec.options {
			box := "[ ]"
			if sec.selected[o] {
				box = "[x]"
			}
			line := "    " + box + " " + o
			if si == fs.sec && ri == fs.row {
				line = "    " + box + " " + styleAccent.Render(o)
			}
			b += line + "\n"
		}
	}
	b += "\n" + styleHelp.Render("Tab/⇧Tab section · ↑/↓ move · Space toggle · a: all/none · Enter: apply · Esc: cancel")
	return b
}
```

Add `applyReport` to `internal/tui/report.go` (rebuilds the report from the current filter/range — reused by the Filters apply):

```go
// applyReport rebuilds m.report from the visible entries over the current range,
// keeping the active grouping.
func (m *Model) applyReport() {
	g := m.report.GroupBy
	if g == "" {
		g = report.GroupByTotal
	}
	start, end := m.currentRange()
	m.report = report.Build(m.visibleEntries(), g, ratesFromConfig(m.cfg), m.cfg.Currency, start, end)
	m.report.Scope = m.scope
	m.rep = newReport(m.report, m.memberFilterNote()+m.filteredNote())
}
```

> NOTE for the implementer: `applyReport` has a POINTER receiver (it mutates m). In `updateFilters` (value receiver) call it as `m.applyReport()` on the local copy — Go auto-takes the address of the addressable local `m`. This is the one pointer-receiver helper; keep the rest value-receiver.

In `internal/tui/app.go`: append `screenFilters` to the iota; add `filtersScreen filtersModel` field; add `case screenFilters: return m.updateFilters(msg)` to `routeKey`; add `case screenFilters: return m.filtersScreen.view()` to `View`.

In `internal/tui/report.go` `updateReport`, add the `f` key and mention it in help:

```go
	case "f":
		m.filtersScreen = newFilters(m.entries, m.filterLists, m.filterTags, m.filterStatuses)
		m.screen = screenFilters
		return m, nil
```
Help line: add `f: filters` to the report help string.

- [ ] **Step 4: Run the full suite** → all clean/green.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/filters.go internal/tui/filters_test.go internal/tui/app.go internal/tui/report.go
git commit -m "feat(tui): sectioned filters screen (lists/tags/statuses)"
```

---

### Task 9: lazy status enrichment + open-Filters flow + docs

**Files:**
- Modify: `internal/tui/app.go` (statusesMsg + handler + enrich cmd + cache), `internal/tui/report.go` (`f` opens with enrichment), `internal/tui/demo.go` (`demoStatusEnrichCmd`)
- Test: `internal/tui/app_test.go`, `internal/tui/filters_test.go`
- Modify: `README.md`, `README.it.md`

**Interfaces:**
- Consumes: `clickup.TaskStatus` (Task 6), `newFilters` (Task 8).
- Produces: `statusesMsg struct{ byTask map[string]string }`; `Model.taskStatus map[string]string`; `statusEnrichCmd(c *clickup.Client, taskIDs []string) tea.Cmd`; `demoStatusEnrichCmd(entries []report.TimeEntry) tea.Cmd`; enrichment-aware `f` open.

- [ ] **Step 1: Write the failing tests**

Add to `internal/tui/app_test.go`:

```go
func TestStatusesMsgAssignsAndOpens(t *testing.T) {
	m := Model{
		screen:  screenFilters,
		entries: []report.TimeEntry{{TaskID: "t1", ListName: "A"}, {TaskID: "t2", ListName: "A"}},
	}
	u, _ := m.Update(statusesMsg{byTask: map[string]string{"t1": "open", "t2": "done"}})
	m = u.(Model)
	if m.screen != screenFilters {
		t.Fatalf("screen = %v, want filters", m.screen)
	}
	if m.entries[0].Status != "open" || m.entries[1].Status != "done" {
		t.Errorf("statuses not assigned: %+v", m.entries)
	}
	// the Statuses section now has both options
	if len(m.filtersScreen.sections[2].options) != 2 {
		t.Errorf("status options = %v", m.filtersScreen.sections[2].options)
	}
}

func TestReportFEnrichesWhenStatusMissing(t *testing.T) {
	m := Model{screen: screenReport, demo: true, entries: []report.TimeEntry{{TaskID: "t1", ListName: "A"}}}
	u, cmd := m.updateReport(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	m = u.(Model)
	if m.screen != screenFilters || !m.filtersScreen.loadingStatuses {
		t.Fatalf("expected loading-statuses filters screen")
	}
	if cmd == nil {
		t.Fatal("expected a status enrichment command")
	}
	if _, ok := cmd().(statusesMsg); !ok {
		t.Fatal("expected statusesMsg from the enrich command")
	}
}
```

- [ ] **Step 2: Run to verify it fails** → build error (`undefined: statusesMsg` / `demoStatusEnrichCmd`).

- [ ] **Step 3: Write minimal implementation**

In `internal/tui/app.go`:

(a) Add the message type and root cache:
```go
	statusesMsg struct{ byTask map[string]string }
```
```go
	taskStatus map[string]string // task id -> current status (session cache)
```

(b) Enrichment command (per unique task id; cache-miss list passed in):
```go
// statusEnrichCmd fetches the current status of each task id and returns them
// as a statusesMsg (or errMsg on the first failure).
func statusEnrichCmd(c *clickup.Client, taskIDs []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		byTask := make(map[string]string, len(taskIDs))
		for _, id := range taskIDs {
			st, err := c.TaskStatus(ctx, id)
			if err != nil {
				return errMsg{err: err}
			}
			byTask[id] = st
		}
		return statusesMsg{byTask: byTask}
	}
}
```

(c) Helpers on Model:
```go
// tasksMissingStatus returns the distinct task ids of loaded entries whose status
// is not yet cached.
func (m Model) tasksMissingStatus() []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range m.entries {
		if e.TaskID == "" || seen[e.TaskID] {
			continue
		}
		seen[e.TaskID] = true
		if _, ok := m.taskStatus[e.TaskID]; !ok {
			out = append(out, e.TaskID)
		}
	}
	return out
}

// assignStatuses copies cached statuses onto the loaded entries.
func (m *Model) assignStatuses() {
	for i := range m.entries {
		if st, ok := m.taskStatus[m.entries[i].TaskID]; ok {
			m.entries[i].Status = st
		}
	}
}
```

(d) `statusesMsg` handler in `Update`:
```go
	case statusesMsg:
		if m.taskStatus == nil {
			m.taskStatus = map[string]string{}
		}
		for id, st := range msg.byTask {
			m.taskStatus[id] = st
		}
		m.assignStatuses()
		m.filtersScreen = newFilters(m.entries, m.filterLists, m.filterTags, m.filterStatuses)
		m.screen = screenFilters
		return m, nil
```

In `internal/tui/report.go`, replace the Task-8 `f` case with the enrichment-aware version:
```go
	case "f":
		missing := m.tasksMissingStatus()
		if len(missing) == 0 {
			m.assignStatuses()
			m.filtersScreen = newFilters(m.entries, m.filterLists, m.filterTags, m.filterStatuses)
			m.screen = screenFilters
			return m, nil
		}
		m.filtersScreen = filtersModel{loadingStatuses: true}
		m.screen = screenFilters
		if m.demo {
			return m, demoStatusEnrichCmd(m.entries)
		}
		return m, statusEnrichCmd(m.client, missing)
```

In `internal/tui/demo.go`, add:
```go
// demoStatusEnrichCmd returns the demo entries' statuses as a statusesMsg (no I/O).
func demoStatusEnrichCmd(entries []report.TimeEntry) tea.Cmd {
	return func() tea.Msg {
		byTask := make(map[string]string, len(entries))
		for _, e := range entries {
			byTask[e.TaskID] = e.Status
		}
		return statusesMsg{byTask: byTask}
	}
}
```

- [ ] **Step 4: Run the full suite** → all clean/green.

- [ ] **Step 5: Update the READMEs (filters)**

In `README.md` and `README.it.md`: add an `f` key row on the Report screen (open the Filters screen: lists/tags/statuses); document that statuses are fetched on first open; note filters compose with member selection and the date range; add a short "Filters" subsection. Drop "report filters" from the pending Roadmap highlights. Keep both languages in sync.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go internal/tui/report.go internal/tui/demo.go internal/tui/app_test.go internal/tui/filters_test.go README.md README.it.md
git commit -m "feat(tui): lazy status enrichment for the filters screen and docs"
```

---

## Self-Review notes (author)

- **Spec coverage:** date range presets+custom (T1/T3/T4) ✓; period label/export (T1/T2/T4) ✓; list+tag+status filters (T5/T6/T7/T8) ✓; tags eager / status lazy (T6/T9) ✓; filter semantics OR-intra/AND-inter/empty=all (T5) ✓; compose with member selection & range (T7 visibleEntries + currentRange) ✓; purity of report (T1/T5) ✓; demo parity (T3a/T7/T9) ✓.
- **Build-green / staticcheck ordering:** each new screen const (`screenRange` T4, `screenFilters` T8) lands with its routeKey/View wiring in the same task; `statusesMsg`/`taskStatus` land in T9 with their producer/handler; the three report-build call sites are re-pointed task-by-task (month → currentRange → visibleEntries) and stay compilable each step. `newFilters` is used the moment it is introduced (T8 report `f`).
- **Type consistency:** `Build(…, start, end time.Time)`, `loadEntriesCmd(…, start, end, scope, assignees)`, `demoEntriesCmd(start, end, assignees)`, `newFilters(entries, lists, tags, statuses)`, `FilterCriteria{Lists,Tags,Statuses}` used identically across producers/consumers.
- **Watch (flag for reviewer):** `applyReport` is the only pointer-receiver method — confirm it does not fight the value-receiver convention (it is called on an addressable local `m`); demo `me`-scope self-filter + range clipping interact in `demoEntriesCmd` (T3a) — confirm the demo month still yields entries under the default preset.
