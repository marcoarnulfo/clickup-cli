# TUI design-system foundations (v1.9 tranche A) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `internal/tui`'s package-level style vars with a theme carried on the root `Model` and passed explicitly to every view, built from a semantic adaptive palette, behind a golden-file regression harness landed first.

**Architecture:** Task 1 adds a golden harness and captures every screen's current output — no production change. Tasks 2 and 3 migrate the 16 files that read styles, in two groups, and must leave every golden byte-identical. Task 4 is the only task that changes what a user sees: the palette's `Light` values diverge from its `Dark` values. Task 5 documents it.

**Tech Stack:** Go 1.26, bubbletea v1.3.10, lipgloss v1.1.0, termenv v0.16.0 (already in the build graph via lipgloss; this work promotes it to a direct dependency). No new modules.

## Global Constraints

- `internal/report` and `internal/duration` stay **pure** — no I/O, no new imports, not touched by this plan.
- **No new `go.mod` dependencies.** `github.com/muesli/termenv` moves from indirect to direct; nothing else changes.
- bubbletea value receivers; explicit write-back (`m.sub = x`) before every return.
- Everything committed to the repo is in **ENGLISH** — code, identifiers, comments, test messages, UI strings, commit messages.
- **Conventional Commits.** **Never** add a `Co-Authored-By` trailer.
- Pre-commit gate, all clean/green, every task: `gofmt -l .`, `go vet ./...`, `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`, `go test ./... -race`. staticcheck runs in CI and broke v1.8's first run — it is not optional locally.
- **A screen golden that changes during Task 2 or Task 3 is a bug, not an update.** Never run `-update` to make a failure go away in those tasks; find what the refactor altered.
- Demo mode (`CLICKUP_DEMO=1`) renders through the same views and must keep working identically.

---

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `internal/tui/golden_test.go` (create) | `TestMain` color-profile pin, the `golden` helper, the `-update` flag, deterministic fixtures (`goldenModel`, `goldenReport`), and every screen golden test | 1 |
| `internal/tui/testdata/*.golden` (create) | Captured output, one file per screen state | 1 |
| `internal/tui/theme.go` (create) | `palette`, `theme`, `newTheme`, `defaultPalette` — the only place a color literal may appear | 2 |
| `internal/tui/theme_test.go` (create) | `testTheme` / `paletteTheme` helpers, palette goldens, `NO_COLOR` regression test | 2, 4 |
| `internal/tui/app.go` (modify) | `theme` field on `Model`, built in `New()`; `View()` passes it to every sub-view | 2 |
| 7 small view files (modify) | `home`, `setup`, `export`, `budget`, `members`, `range`, `listbrowser` | 2 |
| 7 remaining view files (modify) | `report`, `entries`, `log`, `filters`, `rates`, `rates_view`, `history` | 3 |
| `internal/tui/styles.go` (delete) | Holds nothing but the vars `theme.go` replaces | 3 |
| `CHANGELOG.md`, `README.md`, `README.it.md`, `CONTRIBUTING.md` (modify) | User-facing colors + contributor workflow for goldens | 5 |

---

### Task 1: Golden harness and baseline captures

Adds the regression net **before** anything is refactored. No production file is touched in this task — if `git diff --stat` shows a non-test file, the task is wrong.

**Files:**
- Create: `internal/tui/golden_test.go`
- Create: `internal/tui/testdata/` (19 `.golden` files, generated in Step 4: `home`, `home_notices`, `report`, `export`, `budget`, `budget_empty`, `members`, `range`, `filters`, `setup`, `listbrowser`, `history`, `history_empty`, `rates_lists`, `rates_members`, `rates_overrides`, `rates_rules`, `log`, `entries`)
- Modify: `go.mod`, `go.sum` (`termenv` promoted from indirect to direct — see Step 6)

**Interfaces:**
- Consumes: nothing.
- Produces, for later tasks:
  - `golden(t *testing.T, name, got string)` — compares `got` against `testdata/<name>.golden`, rewrites it under `-update`.
  - `goldenModel() Model` — a `Model` with a frozen clock (2026-07-15 10:30 UTC), `loc = time.UTC`, `year = 2026`, `month = time.July`.
  - `goldenReport() report.Report` — a fixed two-bucket EUR report.
  - `TestMain` pins the default renderer to `termenv.Ascii` for the whole package's tests.

- [ ] **Step 1: Write the harness**

Create `internal/tui/golden_test.go`:

```go
package tui

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/*.golden files")

// TestMain pins the default renderer's color profile for the whole package.
// Without it a screen golden captured on a color-capable terminal would carry
// ANSI escapes and fail in CI, where the same code renders plain text.
// CLICKUP_DEMO is cleared because New() switches to fixture data when it is
// set, which would make every golden depend on the developer's shell.
func TestMain(m *testing.M) {
	os.Unsetenv("CLICKUP_DEMO")
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

// golden compares got against testdata/<name>.golden, or rewrites the file
// when the suite runs with -update.
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v\nregenerate with: go test ./internal/tui -update", path, err)
	}
	if got != string(want) {
		t.Errorf("%s does not match the golden file.\n--- want ---\n%s\n--- got ---\n%s", path, want, got)
	}
}

// goldenFixedTime is the instant every golden renders at, so any view showing
// a date, a range label or an elapsed timer stays byte-stable.
var goldenFixedTime = time.Date(2026, time.July, 15, 10, 30, 0, 0, time.UTC)

// goldenModel is a Model with a frozen clock and a fixed configuration.
func goldenModel() Model {
	m := New(config.Config{Token: "t", WorkspaceID: "team1", Currency: "EUR", Rate: 50})
	m.now = func() time.Time { return goldenFixedTime }
	m.loc = time.UTC
	m.year, m.month = 2026, time.July
	return m
}

// goldenReport is a fixed report: one billable list, one non-billable.
func goldenReport() report.Report {
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	return report.Report{
		Start: start, End: start.AddDate(0, 1, 0),
		Scope: "me", GroupBy: report.GroupByList, DefaultCurrency: "EUR",
		Timezone: "UTC", // reportModel.view prints this; unset renders a dangling "tz "
		Buckets: []report.Bucket{
			{Label: "Website", Key: "l1", Hours: 12.5, BillableHours: 12.5, BilledHours: 12.5,
				Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 625}}},
			{Label: "Internal", Key: "l2", Hours: 3, BillableHours: 0, BilledHours: 0,
				Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 0}}},
		},
		CurrencySubtotals: []report.CurrencySubtotal{
			{Currency: "EUR", Hours: 15.5, BillableHours: 12.5, BilledHours: 12.5, Amount: 625},
		},
		TotalHours: 15.5, BillableHours: 12.5, NonBillableHours: 3,
		BilledHours: 12.5, TotalAmount: 625,
	}
}

// goldenEntries is the fixed entry set the browser, filters and rates screens
// render from.
func goldenEntries() []report.TimeEntry {
	start := time.Date(2026, time.July, 14, 9, 0, 0, 0, time.UTC)
	return []report.TimeEntry{
		{ID: "e1", ListID: "l1", ListName: "Website", TaskID: "t1", TaskName: "Landing page",
			Start: start, Duration: 90 * time.Minute, UserID: 1, UserName: "Marco", Billable: true},
		{ID: "e2", ListID: "l2", ListName: "Internal", TaskID: "t2", TaskName: "Standup",
			Start: start.Add(2 * time.Hour), Duration: 30 * time.Minute, UserID: 2, UserName: "Ada", Billable: false},
	}
}
```

Note for the implementer: `report.TimeEntry`'s exact field set is in `internal/report/model.go`. If `UserName` is spelled differently there, use the real name — do not invent fields.

- [ ] **Step 2: Write the screen golden tests**

Append to `internal/tui/golden_test.go`. Every test is `t.Parallel()`-safe: none mutates shared state.

```go
func TestGoldenHome(t *testing.T) {
	t.Parallel()
	m := goldenModel()
	golden(t, "home", m.home.view("July 2026", "me", "", "", ""))
}

func TestGoldenHomeWithNotices(t *testing.T) {
	t.Parallel()
	m := goldenModel()
	m.home.errText = "request failed: 500"
	// The timer line is the string app.go builds for a running timer; keep it
	// verbatim so the golden depicts something the app actually renders.
	golden(t, "home_notices", m.home.view("July 2026", "team", "Members: 2/3", "v1.9.0",
		"⏱  running on Landing page — 00:12:30   (c: manage)"))
}

func TestGoldenReport(t *testing.T) {
	t.Parallel()
	golden(t, "report", newReport(goldenReport(), "").view())
}

func TestGoldenExport(t *testing.T) {
	t.Parallel()
	golden(t, "export", newExport(goldenReport()).view())
}

func TestGoldenBudget(t *testing.T) {
	t.Parallel()
	lines := []report.BudgetLine{
		{ListID: "l1", ListName: "Website", Currency: "EUR",
			Budget: 1000, Billed: 625, Remaining: 375, PercentUsed: 62.5},
	}
	golden(t, "budget", newBudget(lines).view())
}

func TestGoldenBudgetEmpty(t *testing.T) {
	t.Parallel()
	golden(t, "budget_empty", newBudget(nil).view())
}

func TestGoldenMembers(t *testing.T) {
	t.Parallel()
	members := []clickup.Member{{ID: 1, Username: "Marco"}, {ID: 2, Username: "Ada"}}
	golden(t, "members", newMembers(members, map[int]bool{1: true}).view())
}

func TestGoldenRange(t *testing.T) {
	t.Parallel()
	golden(t, "range", newRange(report.PresetThisMonth).view())
}

func TestGoldenFilters(t *testing.T) {
	t.Parallel()
	golden(t, "filters", newFilters(goldenEntries(), nil, nil, nil, nil).view())
}

func TestGoldenSetup(t *testing.T) {
	t.Parallel()
	golden(t, "setup", newSetup().view())
}

func TestGoldenListBrowser(t *testing.T) {
	t.Parallel()
	bs := listBrowserModel{
		origin: screenLog,
		spaces: []clickup.Space{{ID: "s1", Name: "Clients"}, {ID: "s2", Name: "Internal"}},
	}
	golden(t, "listbrowser", bs.view())
}

func TestGoldenHistory(t *testing.T) {
	t.Parallel()
	es := entriesModel{historyChanges: []clickup.HistoryChange{
		{Field: "duration", Before: "1h", After: "1h30m",
			Date: time.Date(2026, time.July, 14, 11, 0, 0, 0, time.UTC), User: "Marco"},
		{Field: "billable", Before: "false", After: "true",
			Date: time.Date(2026, time.July, 14, 11, 5, 0, 0, time.UTC), User: "Marco"},
	}}
	golden(t, "history", entriesHistoryView(es, time.UTC))
}

func TestGoldenHistoryEmpty(t *testing.T) {
	t.Parallel()
	golden(t, "history_empty", entriesHistoryView(entriesModel{}, time.UTC))
}

func TestGoldenRatesTabs(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Token: "t", WorkspaceID: "team1", Currency: "EUR", Rate: 50}
	for _, tc := range []struct {
		name string
		sec  ratesSection
	}{
		{"rates_lists", secLists},
		{"rates_members", secMembers},
		{"rates_overrides", secOverrides},
		{"rates_rules", secRules},
	} {
		rt := newRates(goldenEntries(), cfg)
		rt.sec = tc.sec
		golden(t, tc.name, rt.view())
	}
}

func TestGoldenLog(t *testing.T) {
	t.Parallel()
	lg := newLog(goldenEntries(), config.Config{Token: "t", WorkspaceID: "team1"}, screenReport)
	lg.now = goldenFixedTime
	golden(t, "log", lg.view())
}

// The entries browser has no constructor: it is built inline when 'v' is
// pressed on the report screen, so this drives the real key path.
func TestGoldenEntriesBrowser(t *testing.T) {
	t.Parallel()
	m := goldenModel()
	m.userID = 1
	m.entries = goldenEntries()
	m.screen = screenReport
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	mm := next.(Model)
	if mm.screen != screenEntries {
		t.Fatalf("v did not open the entries browser: screen=%v", mm.screen)
	}
	golden(t, "entries", mm.View())
}
```

The last test needs `tea "github.com/charmbracelet/bubbletea"` in the import block. `entries.go:133` is where the browser model is built (`entriesModel{entries: sortEntriesByStartDesc(m.visibleEntries())}`) — driving the key press rather than reproducing that literal keeps the golden honest if the construction ever changes. `logModel.now` is a `time.Time`, not a function: `app.go`'s `View()` assigns `m.logScreen.now = m.now()`.

- [ ] **Step 3: Run the tests and watch them fail**

Run: `go test ./internal/tui -run TestGolden -v`

Expected: every `TestGolden*` FAILS with `read testdata/<name>.golden: no such file or directory`. That is the RED state — the harness works, the captures do not exist yet.

- [ ] **Step 4: Generate the goldens**

Run: `go test ./internal/tui -run TestGolden -update`

Then **read every generated file** (`ls testdata/` then open each). Each must contain plain, readable text with no ANSI escape sequences (`grep -l $'\x1b' testdata/*.golden` must print nothing) and no absolute paths, timestamps of "now", or usernames from your machine. A golden containing today's real date means a view is reading an un-frozen clock — fix the fixture, do not accept the file.

- [ ] **Step 5: Verify the goldens now pass, twice**

Run: `go test ./internal/tui -run TestGolden -count=2`
Expected: PASS. `-count=2` proves the output is stable across runs in the same process.

- [ ] **Step 6: Promote termenv to a direct dependency**

`golden_test.go` imports `github.com/muesli/termenv`, which `go.mod` currently
lists as `// indirect`. The build works either way, but the marker is now wrong.

Run: `go mod tidy`
Expected: `go.mod`'s `termenv v0.16.0` line loses its `// indirect` comment and
moves into the first `require` block. This diff is **expected** — do not revert
it. No module version may change; if `go mod tidy` bumps or removes anything
else, stop and report it.

- [ ] **Step 7: Run the full gate**

```bash
gofmt -l .
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go build ./...
go test ./... -race
```
Expected: no output from `gofmt`/`vet`/`staticcheck`, build clean, all tests PASS.

- [ ] **Step 8: Confirm no production code changed**

Run: `git status --short`
Expected: exactly `internal/tui/golden_test.go`, `internal/tui/testdata/*.golden`, `go.mod` and `go.sum`. Any other file listed means production code was touched — revert it.

- [ ] **Step 9: Commit**

```bash
git add internal/tui/golden_test.go internal/tui/testdata go.mod go.sum
git commit -m "test(tui): add golden-file regression harness (#75)"
```

---

### Task 2: The theme, and the eight small files

Introduces `theme.go` and migrates the files that read styles least. The remaining files keep using the package vars in this task — both groups compile side by side, because `styles.go` is not deleted until Task 3.

`history.go` is **not** in this task despite being small: it holds `entriesHistoryView`, which `entries.go` (a Task 3 file) calls, so migrating it here would force edits outside this task's file list. It goes with Task 3.

**Files:**
- Create: `internal/tui/theme.go`, `internal/tui/theme_test.go`
- Modify: `internal/tui/app.go`, `home.go`, `setup.go`, `export.go`, `budget.go`, `members.go`, `range.go`, `listbrowser.go`
- Modify: `internal/tui/golden_test.go`, `budget_test.go`, `home_test.go` (call sites gain the argument)

**Interfaces:**
- Consumes: `golden`, `goldenModel`, `goldenReport`, `goldenEntries` from Task 1.
- Produces:
  - `type palette struct { Primary, Accent, Muted, Danger, Success lipgloss.AdaptiveColor }`
  - `type theme struct { Title, Help, Err, OK, Accent, Box, Header lipgloss.Style }`
  - `func newTheme(r *lipgloss.Renderer, p palette) theme`
  - `func defaultPalette() palette`
  - `Model.theme theme`, set in `New()`
  - `func testTheme(dark bool) theme` (test-only, Ascii profile)
  - Migrated signatures: `homeModel.view(th theme, rangeLabel, scope, membersNote, latestVersion, timerLine string) string`, and `view(th theme) string` for `setupModel`, `exportModel`, `budgetModel`, `membersModel`, `rangeModel`, `listBrowserModel`; `browserRow(th theme, label string, sel bool) string`.

- [ ] **Step 1: Write the failing test**

Create `internal/tui/theme_test.go`:

```go
package tui

import (
	"io"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// testTheme builds a theme on its own renderer, so tests never touch the
// global default renderer and stay parallel-safe.
func testTheme(dark bool) theme {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.Ascii)
	r.SetHasDarkBackground(dark)
	return newTheme(r, defaultPalette())
}

// The default palette must keep today's colors on a dark background, so the
// refactor is a no-op for the terminals most users are on.
func TestDefaultPaletteKeepsCurrentDarkColors(t *testing.T) {
	t.Parallel()
	p := defaultPalette()
	for _, tc := range []struct {
		name string
		got  string
		want string
	}{
		{"Primary", p.Primary.Dark, "205"},
		{"Accent", p.Accent.Dark, "205"},
		{"Muted", p.Muted.Dark, "240"},
		{"Danger", p.Danger.Dark, "196"},
		{"Success", p.Success.Dark, "42"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s dark = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

// Task 2 ships Light == Dark on purpose: the adaptive values land in Task 4,
// so this task cannot change a single rendered byte. Task 4 deletes this test.
func TestDefaultPaletteIsNotYetAdaptive(t *testing.T) {
	t.Parallel()
	p := defaultPalette()
	for _, tc := range []struct {
		name string
		c    lipgloss.AdaptiveColor
	}{
		{"Primary", p.Primary}, {"Accent", p.Accent}, {"Muted", p.Muted},
		{"Danger", p.Danger}, {"Success", p.Success},
	} {
		if tc.c.Light != tc.c.Dark {
			t.Errorf("%s: Light %q != Dark %q; the adaptive split belongs to Task 4",
				tc.name, tc.c.Light, tc.c.Dark)
		}
	}
}

// A theme built on a private renderer must not disturb the package default.
func TestThemeRendererIsIsolated(t *testing.T) {
	t.Parallel()
	before := lipgloss.ColorProfile()
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.TrueColor)
	_ = newTheme(r, defaultPalette())
	if after := lipgloss.ColorProfile(); after != before {
		t.Errorf("building a theme changed the default color profile: %v -> %v", before, after)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/tui -run 'TestDefaultPalette|TestThemeRenderer' -v`
Expected: FAIL to build — `undefined: theme`, `undefined: newTheme`, `undefined: defaultPalette`.

- [ ] **Step 3: Write theme.go**

Create `internal/tui/theme.go`:

```go
package tui

import "github.com/charmbracelet/lipgloss"

// palette holds the semantic colors the whole TUI is built from. It is the
// only place a color literal belongs: styles derive from these, and a future
// user-supplied theme (#82) will override these five values, not the styles.
type palette struct {
	Primary lipgloss.AdaptiveColor // titles and headings
	Accent  lipgloss.AdaptiveColor // selection, highlighted values
	Muted   lipgloss.AdaptiveColor // help lines and secondary text
	Danger  lipgloss.AdaptiveColor // errors
	Success lipgloss.AdaptiveColor // confirmations
}

// theme is the styled surface the views render through. It travels as an
// explicit argument rather than living in package state, so no view can render
// with a half-built theme and tests can hold two themes at once.
//
// The palette is deliberately not kept as a field: nothing reads it in this
// tranche, and a write-only field is dead weight. It comes back when a
// user-supplied theme (#82) needs to be inspected.
type theme struct {
	Title  lipgloss.Style
	Help   lipgloss.Style
	Err    lipgloss.Style
	OK     lipgloss.Style
	Accent lipgloss.Style
	Box    lipgloss.Style
	Header lipgloss.Style // bold, uncolored: the report's column header row
}

// newTheme builds the styles for a palette on a specific renderer. Production
// passes lipgloss.DefaultRenderer(); tests pass a renderer with a pinned color
// profile and background so output is deterministic.
func newTheme(r *lipgloss.Renderer, p palette) theme {
	// Resolve the terminal background now, while we still own the terminal.
	// lipgloss otherwise resolves AdaptiveColor lazily, at the first Render,
	// by querying the terminal over OSC-11 — by then bubbletea's input reader
	// is competing for the reply, termenv times out and falls back to "dark",
	// and a light terminal never gets the light palette.
	_ = r.HasDarkBackground()

	return theme{
		Title:  r.NewStyle().Bold(true).Foreground(p.Primary).MarginBottom(1),
		Help:   r.NewStyle().Foreground(p.Muted),
		Err:    r.NewStyle().Foreground(p.Danger).Bold(true),
		OK:     r.NewStyle().Foreground(p.Success),
		Accent: r.NewStyle().Foreground(p.Accent),
		Box:    r.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1),
		Header: r.NewStyle().Bold(true),
	}
}

// defaultPalette is the built-in palette. Light and Dark are deliberately
// identical here: this keeps the theme refactor byte-for-byte invisible. The
// adaptive Light values land with #63's second half.
func defaultPalette() palette {
	dup := func(c string) lipgloss.AdaptiveColor {
		return lipgloss.AdaptiveColor{Light: c, Dark: c}
	}
	return palette{
		Primary: dup("205"), // magenta, ClickUp-ish
		Accent:  dup("205"),
		Muted:   dup("240"),
		Danger:  dup("196"),
		Success: dup("42"),
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/tui -run 'TestDefaultPalette|TestThemeRenderer' -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Put the theme on the Model**

In `internal/tui/app.go`, add to the `Model` struct, right after the `err error` field:

```go
	// theme carries every style the views render through (#54). It is passed
	// explicitly to each view rather than read from package state, so a view
	// can never render with an unset theme.
	theme theme
```

In `New(cfg config.Config) Model`, add `theme` to the struct literal:

```go
	m := Model{
		cfg:    cfg,
		demo:   demo,
		scope:  "me",
		preset: report.PresetThisMonth,
		client: clickup.New(cfg.Token),
		now:    time.Now,
		theme:  newTheme(lipgloss.DefaultRenderer(), defaultPalette()),
	}
```

Add `"github.com/charmbracelet/lipgloss"` to `app.go`'s imports if it is not already there.

- [ ] **Step 6: Migrate the seven small view files**

Mechanical substitution inside these files only: `styleTitle`→`th.Title`, `styleHelp`→`th.Help`, `styleErr`→`th.Err`, `styleOK`→`th.OK`, `styleAccent`→`th.Accent`, `styleBox`→`th.Box`.

Signatures — `th theme` goes **first**, before existing parameters:

| File | Before | After |
|---|---|---|
| `home.go` | `func (m homeModel) view(rangeLabel, scope, membersNote, latestVersion, timerLine string) string` | `func (m homeModel) view(th theme, rangeLabel, scope, membersNote, latestVersion, timerLine string) string` |
| `setup.go` | `func (s setupModel) view() string` | `func (s setupModel) view(th theme) string` |
| `export.go` | `func (e exportModel) view() string` | `func (e exportModel) view(th theme) string` |
| `budget.go` | `func (bm budgetModel) view() string` | `func (bm budgetModel) view(th theme) string` |
| `members.go` | `func (mm membersModel) view() string` | `func (mm membersModel) view(th theme) string` |
| `range.go` | `func (rs rangeModel) view() string` | `func (rs rangeModel) view(th theme) string` |
| `listbrowser.go` | `func (bs listBrowserModel) view() string` | `func (bs listBrowserModel) view(th theme) string` |
| `listbrowser.go` | `func browserRow(label string, sel bool) string` | `func browserRow(th theme, label string, sel bool) string` |

**The substitution is 1:1 and must stay 1:1.** Screen goldens render under the
Ascii profile, where termenv strips color *and* bold — so rendering with
`th.Err` where `th.OK` belongs produces a byte-identical golden and passes every
check in this task. Nothing but care catches a swapped role here. Do the
substitution mechanically, one symbol at a time, and never "improve" a mapping.

In `app.go`'s `View()`, pass `m.theme` to the migrated cases and to the two inline uses:

```go
	case screenSetup:
		return m.setup.view(m.theme)
	...
		return m.home.view(m.theme, m.rangeLabel(), m.scope, m.homeMembersNote(), m.latestVersion, timerLine)
	case screenLoading:
		return m.theme.Title.Render("Loading hours…")
	case screenExport:
		return m.export.view(m.theme)
	case screenMembers:
		return m.membersScreen.view(m.theme)
	case screenRange:
		return m.rangeScreen.view(m.theme)
	case screenListBrowser:
		return m.browserScreen.view(m.theme)
	case screenBudget:
		return m.budgetScreen.view(m.theme)
	case screenError:
		return m.theme.Err.Render("Error: ") + m.err.Error() + "\n\n" +
			m.theme.Help.Render("press a key to return home")
```

The `screenReport`, `screenRates`, `screenLog`, `screenFilters` and `screenEntries` cases are **left untouched** — those views migrate in Task 3.

Find every remaining caller of the one changed helper with:

```bash
grep -rn 'browserRow(' internal/tui/
```

and pass `th` (inside a view) or the test's theme (in a test).

- [ ] **Step 7: Update the affected test call sites**

In `golden_test.go`, the migrated goldens now build a theme:

```go
func TestGoldenHome(t *testing.T) {
	t.Parallel()
	m := goldenModel()
	golden(t, "home", m.home.view(testTheme(true), "July 2026", "me", "", "", ""))
}
```

Apply the same change to `TestGoldenHomeWithNotices`, `TestGoldenExport`, `TestGoldenBudget`, `TestGoldenBudgetEmpty`, `TestGoldenMembers`, `TestGoldenRange`, `TestGoldenSetup`, `TestGoldenListBrowser`. Leave `TestGoldenReport`, `TestGoldenFilters`, `TestGoldenRatesTabs`, `TestGoldenLog`, `TestGoldenHistory`, `TestGoldenHistoryEmpty` and `TestGoldenEntriesBrowser` alone — they migrate in Task 3.

Then fix the remaining compile errors in `budget_test.go` (2 call sites) and `home_test.go` (1 call site) by passing `testTheme(true)`. Compile errors are the checklist here:

```bash
go build ./... && go vet ./...
```

- [ ] **Step 8: Verify the goldens did not move**

Run: `go test ./internal/tui -run TestGolden`
Expected: PASS, with **no** `.golden` file modified. Confirm with:

```bash
git status --short internal/tui/testdata
```
Expected: empty output. If a golden changed, the refactor altered rendering — find the difference and fix the code. **Do not run `-update`.**

- [ ] **Step 9: Run the full gate**

```bash
gofmt -l .
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go build ./...
go test ./... -race
```
Expected: all clean/green.

- [ ] **Step 10: Commit**

```bash
git add internal/tui
git commit -m "refactor(tui): add theme registry and migrate the small views (#54, #63)"
```

---

### Task 3: The remaining files, and the end of package styles

Completes the migration and deletes `styles.go`. After this task, `grep -rn 'styleTitle\|styleHelp\|styleErr\|styleOK\|styleAccent\|styleBox\|colAccent\|colDim\|colErr\|colOK' internal/` returns nothing.

**Files:**
- Modify: `internal/tui/report.go`, `entries.go`, `log.go`, `filters.go`, `rates.go`, `rates_view.go`, `history.go`, `app.go`
- Delete: `internal/tui/styles.go`
- Modify: `internal/tui/golden_test.go`, `report_test.go`, `rates_test.go`, `history_test.go` (call sites gain the argument)

**Interfaces:**
- Consumes: `theme`, `testTheme`, `Model.theme` from Task 2.
- Produces: `view(th theme) string` for `reportModel`, `logModel`, `filtersModel`, `ratesModel`; `Model.entriesView(th theme) string`; `func (rt ratesModel) tabs(th theme) string`; `func billingRow(th theme, sel bool, line string) string`; `func historyLine(th theme, c clickup.HistoryChange, loc *time.Location) string`; `func entriesHistoryView(th theme, es entriesModel, loc *time.Location) string`. The same first-parameter change applies to `entriesEditView` and to `ratesModel`'s five section renderers (`listsView`, `membersView`, `overridesView`, `rulesView`, `draftView`) — the compiler enumerates them.

- [ ] **Step 1: Migrate the seven files**

Same mechanical substitution as Task 2 (`styleX` → `th.X`), and the same warning applies: under the Ascii profile the goldens cannot tell `th.Err` from `th.OK`, so keep the mapping strictly 1:1. Signatures:

| File | Before | After |
|---|---|---|
| `report.go` | `func (rm reportModel) view() string` | `func (rm reportModel) view(th theme) string` |
| `log.go` | `func (lg logModel) view() string` | `func (lg logModel) view(th theme) string` |
| `filters.go` | `func (fs filtersModel) view() string` | `func (fs filtersModel) view(th theme) string` |
| `rates.go` | `func (rt ratesModel) view() string` | `func (rt ratesModel) view(th theme) string` |
| `rates_view.go` | `func (rt ratesModel) tabs() string` | `func (rt ratesModel) tabs(th theme) string` |
| `rates_view.go` | `func billingRow(sel bool, line string) string` | `func billingRow(th theme, sel bool, line string) string` |
| `rates_view.go` | `listsView`, `membersView`, `overridesView`, `rulesView`, `draftView` | each takes `th theme` as its first parameter |
| `history.go` | `func entriesHistoryView(es entriesModel, loc *time.Location) string` | `func entriesHistoryView(th theme, es entriesModel, loc *time.Location) string` |
| `history.go` | `func historyLine(c clickup.HistoryChange, loc *time.Location) string` | `func historyLine(th theme, c clickup.HistoryChange, loc *time.Location) string` |
| `entries.go` | `entriesView` and its render helpers | each takes `th theme` as its first parameter |

`entries.go` is the one file to read before editing rather than substituting blindly: `entriesView` fans out to per-mode renderers (list, confirm-delete, edit, history, tags) and to helpers such as `tagBadges`. It also calls `entriesHistoryView` (at `entries.go:587`), which changes signature in this same task. Thread `th` through every one of them; the compiler finds them all.

Also fold in the one inline style that no grep finds — `report.go:225` builds `lipgloss.NewStyle().Bold(true)` directly on the global default renderer, inside a view:

```go
	header := th.Header.Render(...)   // was: lipgloss.NewStyle().Bold(true).Render(...)
```

Drop `report.go`'s `lipgloss` import if that was its only use.

In `app.go`'s `View()`, pass `m.theme` to the five remaining cases:

```go
	case screenReport:
		return m.rep.view(m.theme)
	case screenRates:
		return m.ratesScreen.view(m.theme)
	case screenLog:
		m.logScreen.now = m.now()
		return m.logScreen.view(m.theme)
	case screenFilters:
		return m.filtersScreen.view(m.theme)
	case screenEntries:
		return m.entriesView(m.theme)
```

- [ ] **Step 2: Delete styles.go**

```bash
git rm internal/tui/styles.go
```

- [ ] **Step 3: Verify no style var survives**

```bash
grep -rn 'styleTitle\|styleHelp\|styleErr\|styleOK\|styleAccent\|styleBox\|colAccent\|colDim\|colErr\|colOK' internal/ cmd/
```
Expected: no output. (One legitimate hit is allowed: `entries.go`'s `msgErr bool` comment, which names `styleErr`/`styleOK` in prose. Reword it to "render msg with th.Err; false → th.OK" so the grep stays clean.)

Then confirm no inline style survives either:

```bash
grep -rn 'lipgloss.NewStyle()' internal/tui/ | grep -v theme.go
```
Expected: no output — `theme.go` is the only file allowed to build styles.

- [ ] **Step 4: Build and fix the test call sites**

Run: `go build ./... && go vet ./...`
Expected: the remaining errors are all test call sites. Fix them by passing `testTheme(true)`:
- `golden_test.go`: `TestGoldenReport`, `TestGoldenFilters`, `TestGoldenRatesTabs`, `TestGoldenLog`, `TestGoldenHistory`, `TestGoldenHistoryEmpty` (`TestGoldenEntriesBrowser` calls `m.View()`, which needs no change).
- `report_test.go` (3 sites), `rates_test.go` (8 sites), `history_test.go` (2 sites, both calling `entriesHistoryView`), `entries_test.go` (uses `mm.View()` — no change).

- [ ] **Step 5: Verify the goldens still did not move**

Run: `go test ./internal/tui -run TestGolden` then `git status --short internal/tui/testdata`
Expected: PASS and empty output. A changed golden here is a bug — **do not run `-update`**.

- [ ] **Step 6: Run the full gate**

```bash
gofmt -l .
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go build ./...
go test ./... -race
```
Expected: all clean/green, with no exceptions. staticcheck being clean is a hard gate here, not a judgement call.

- [ ] **Step 7: Commit**

```bash
git add -A internal/tui
git commit -m "refactor(tui): migrate remaining views to the theme and drop styles.go (#54)"
```

---

### Task 4: The adaptive palette, and the color-environment contract

The only task that changes what a user sees. `Dark` values stay exactly as they are today, so users on dark terminals — the majority — notice nothing; `Light` values gain enough contrast to be legible on a white background, which today they are not.

**Files:**
- Modify: `internal/tui/theme.go` (`defaultPalette`)
- Modify: `internal/tui/theme_test.go` (drop the not-yet-adaptive test, add the adaptive + palette-golden + `NO_COLOR` tests)
- Create: `internal/tui/testdata/palette_light.golden`, `internal/tui/testdata/palette_dark.golden`

**Interfaces:**
- Consumes: `newTheme`, `defaultPalette`, `testTheme`, `golden`.
- Produces: `func paletteTheme(dark bool) theme` (test-only, TrueColor profile), `func paletteSample(th theme) string` (test-only).

- [ ] **Step 1: Write the failing tests**

In `internal/tui/theme_test.go`, **delete** `TestDefaultPaletteIsNotYetAdaptive` (Task 2 shipped it precisely so it would be removed here) and add:

```go
// paletteTheme renders with real colors, so the palette goldens capture the
// exact escape sequences each token produces.
func paletteTheme(dark bool) theme {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.TrueColor)
	r.SetHasDarkBackground(dark)
	return newTheme(r, defaultPalette())
}

// paletteSample renders one labelled line per style, so a single golden pins
// every token's color.
func paletteSample(th theme) string {
	var b strings.Builder
	for _, row := range []struct {
		name string
		st   lipgloss.Style
	}{
		{"Title", th.Title}, {"Help", th.Help}, {"Err", th.Err},
		{"OK", th.OK}, {"Accent", th.Accent}, {"Box", th.Box},
	} {
		fmt.Fprintf(&b, "%-8s%s\n", row.name, row.st.Render(row.name))
	}
	return b.String()
}

func TestGoldenPaletteDark(t *testing.T) {
	t.Parallel()
	golden(t, "palette_dark", paletteSample(paletteTheme(true)))
}

func TestGoldenPaletteLight(t *testing.T) {
	t.Parallel()
	golden(t, "palette_light", paletteSample(paletteTheme(false)))
}

// The four tokens that are unreadable on white must differ between
// backgrounds. Muted must NOT: 240 already clears 7:1 on white, so changing it
// would be churn dressed up as accessibility.
func TestPaletteIsAdaptive(t *testing.T) {
	t.Parallel()
	p := defaultPalette()
	for _, tc := range []struct {
		name string
		c    lipgloss.AdaptiveColor
	}{
		{"Primary", p.Primary}, {"Accent", p.Accent},
		{"Danger", p.Danger}, {"Success", p.Success},
	} {
		if tc.c.Light == tc.c.Dark {
			t.Errorf("%s is not adaptive: Light == Dark == %q", tc.name, tc.c.Dark)
		}
	}
	if p.Muted.Light != p.Muted.Dark {
		t.Errorf("Muted was adapted (%q/%q) but reads fine on both backgrounds",
			p.Muted.Light, p.Muted.Dark)
	}
}

// NO_COLOR is honored by termenv inside lipgloss's renderer; this pins that
// contract so a future lipgloss bump cannot silently break it.
func TestNoColorProducesNoEscapes(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(r.Output().EnvColorProfile())
	th := newTheme(r, defaultPalette())
	out := paletteSample(th)
	if strings.Contains(out, "\x1b") {
		t.Errorf("NO_COLOR=1 still produced escape sequences:\n%q", out)
	}
}

// CLICOLOR_FORCE is the force-color variable termenv implements (FORCE_COLOR,
// the npm convention, is deliberately not supported).
func TestCliColorForceKeepsColor(t *testing.T) {
	t.Setenv("CLICOLOR_FORCE", "1")
	r := lipgloss.NewRenderer(io.Discard)
	if got := r.Output().EnvColorProfile(); got == termenv.Ascii {
		t.Error("CLICOLOR_FORCE=1 resolved to the Ascii profile, expected color")
	}
}
```

`TestNoColorProducesNoEscapes` and `TestCliColorForceKeepsColor` use `t.Setenv`, which forbids `t.Parallel()` — that is why they omit it. Add `"fmt"` and `"strings"` to the file's imports.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/tui -run 'TestGoldenPalette|TestPaletteIsAdaptive|TestNoColor|TestCliColorForce' -v`
Expected: `TestPaletteIsAdaptive` FAILS with four "is not adaptive" errors (Primary, Accent, Danger, Success — its Muted assertion already passes); the two palette goldens FAIL with "no such file or directory". `TestNoColorProducesNoEscapes` and `TestCliColorForceKeepsColor` should already PASS — they pin behavior lipgloss already has.

- [ ] **Step 3: Make the palette adaptive**

Replace `defaultPalette` in `internal/tui/theme.go`:

```go
// defaultPalette is the built-in palette. Dark keeps the xterm indices the TUI
// has always shipped, so a dark terminal renders exactly as before and a user's
// customized 256-color palette is still honored (a hex triple would override
// it). Light overrides only the tokens that are illegible on white, measured as
// WCAG contrast against #FFFFFF:
//
//	205 -> 127   205 (#FF5FAF) is ~1.9:1 on white; 127 (#AF00AF) is ~7.5:1
//	196 -> 124   196 (#FF0000) is ~4:1, under the 4.5:1 floor; 124 (#AF0000) ~8:1
//	 42 ->  28    42 (#00D787) is ~1.8:1 on white;  28 (#008700) is ~6.5:1
//
// Muted (240, #585858) is left alone: it already clears 7:1 on white. Adaptive
// means legible on both backgrounds, not different on both.
func defaultPalette() palette {
	return palette{
		Primary: lipgloss.AdaptiveColor{Light: "127", Dark: "205"},
		Accent:  lipgloss.AdaptiveColor{Light: "127", Dark: "205"},
		Muted:   lipgloss.AdaptiveColor{Light: "240", Dark: "240"},
		Danger:  lipgloss.AdaptiveColor{Light: "124", Dark: "196"},
		Success: lipgloss.AdaptiveColor{Light: "28", Dark: "42"},
	}
}
```

`TestDefaultPaletteKeepsCurrentDarkColors` from Task 2 asserts these exact `Dark` values (`"205"`, `"205"`, `"240"`, `"196"`, `"42"`) and must keep passing untouched — it is what proves dark terminals are unaffected. **Do not convert any index to hex**: the conversions are error-prone (205 is `#FF5FAF`, one digit from 206's `#FF5FD7`) and a hex triple stops honoring a user's customized terminal palette.

- [ ] **Step 4: Generate the palette goldens and verify**

```bash
go test ./internal/tui -run TestGoldenPalette -update
go test ./internal/tui -run 'TestGoldenPalette|TestPaletteIsAdaptive' -count=2
```
Expected: PASS. Then confirm the two files genuinely differ and genuinely carry color:

```bash
diff testdata/palette_light.golden testdata/palette_dark.golden   # must report differences
grep -c $'\x1b' testdata/palette_light.golden                     # must be > 0
```

- [ ] **Step 5: Verify the screen goldens still did not move**

Run: `go test ./internal/tui -run TestGolden` then `git status --short internal/tui/testdata`
Expected: PASS, and the only files listed are the two new `palette_*.golden`. The screen goldens render under the Ascii profile, so a palette change cannot touch them — if one moved, something else did.

- [ ] **Step 6: Check it by eye**

```bash
CLICKUP_DEMO=1 go run ./cmd/clup
```
Walk Home → Report (`Enter`) → Rates (`p`) → Entries (`v`) on a dark terminal background, then on a light one. Dark must look exactly as before; light must be legible. Quit with `q`. Also run `NO_COLOR=1 CLICKUP_DEMO=1 go run ./cmd/clup` and confirm the UI renders with no color at all.

- [ ] **Step 7: Run the full gate**

```bash
gofmt -l .
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go build ./...
go test ./... -race
```
Expected: all clean/green.

- [ ] **Step 8: Commit**

```bash
git add internal/tui
git commit -m "feat(tui): adapt the palette to light and dark terminals (#63, #74)"
```

---

### Task 5: Documentation

**Files:**
- Modify: `CHANGELOG.md`, `README.md`, `README.it.md`, `CONTRIBUTING.md`

**Interfaces:**
- Consumes: everything above. Produces no code.

- [ ] **Step 1: CHANGELOG**

Add a new `### Changed` section under `## [Unreleased]`, after the existing `### Added` section:

```markdown
### Changed
- Colors now adapt to the terminal background: on a light background the
  palette switches to darker, legible variants. Dark terminals render exactly
  as before, down to the same 256-color indices. `NO_COLOR=1` disables color
  entirely and `CLICOLOR_FORCE=1` keeps it when output is piped.
```

- [ ] **Step 2: README.md**

The README documents environment variables in prose, not in a table — `CLICKUP_TOKEN` is covered around line 430 in the configuration section. Add a new `### Colors` subsection immediately after that `CLICKUP_TOKEN` paragraph:

```markdown
### Colors

Colors adapt automatically to a light or dark terminal background. Two standard
environment variables override the behavior:

- `NO_COLOR` (any non-empty value) disables color output entirely
  ([no-color.org](https://no-color.org/)).
- `CLICOLOR_FORCE=1` keeps colors even when the output is piped to a file or
  another command.
```

- [ ] **Step 3: README.it.md**

The same subsection in Italian, in the matching position (the `CLICKUP_TOKEN` paragraph is around line 460):

```markdown
### Colori

I colori si adattano automaticamente a uno sfondo del terminale chiaro o scuro.
Due variabili d'ambiente standard ne cambiano il comportamento:

- `NO_COLOR` (qualsiasi valore non vuoto) disattiva completamente i colori
  ([no-color.org](https://no-color.org/)).
- `CLICOLOR_FORCE=1` mantiene i colori anche quando l'output è rediretto su file
  o su un altro comando.
```

- [ ] **Step 4: CONTRIBUTING.md**

Add a subsection at the end of `## Before opening a PR` (the section around line 39 that lists `go test ./... -race`):

```markdown
### Golden files

The TUI's rendered output is locked by golden files in
`internal/tui/testdata/`. When a change alters a screen on purpose, regenerate
them and **read the diff before committing** — that diff is the review of your
UI change:

    go test ./internal/tui -update
    git diff internal/tui/testdata

Screen goldens render with colors disabled, so they stay readable in a diff.
`palette_light.golden` and `palette_dark.golden` capture the color of every
theme token and are the only goldens containing escape sequences.
```

- [ ] **Step 5: Verify the docs are accurate**

Run: `go test ./internal/tui -update && git status --short internal/tui/testdata`
Expected: empty output — regenerating changes nothing, which proves the documented command is safe and the goldens are current.

- [ ] **Step 6: Run the full gate**

```bash
gofmt -l .
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go build ./...
go test ./... -race
```
Expected: all clean/green.

- [ ] **Step 7: Commit**

```bash
git add CHANGELOG.md README.md README.it.md CONTRIBUTING.md
git commit -m "docs: document adaptive colors, color env vars and golden files"
```

---

## Definition of done

- `grep -rn 'styleTitle\|styleHelp\|styleErr\|styleOK\|styleAccent\|styleBox\|colAccent\|colDim\|colErr\|colOK' internal/ cmd/` returns nothing.
- `grep -rn 'lipgloss.NewStyle()' internal/tui/ | grep -v theme.go` returns nothing.
- `internal/tui/styles.go` no longer exists; `internal/tui/theme.go` is the only file holding color literals or building styles.
- `go.mod` lists `github.com/muesli/termenv` as a direct dependency.
- `go test ./... -race` passes; `go test ./internal/tui -update` leaves `testdata/` unchanged.
- The screen goldens are byte-identical to the ones generated in Task 1.
- Issues #75, #54, #63 close; #74 keeps its mouse-support and downconvert items for tranche D.
