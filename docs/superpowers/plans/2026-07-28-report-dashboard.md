# Report Dashboard Implementation Plan (v1.9, tranche C)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collapse the billing editor's four parallel selection indices into one array (#117), replace the hand-built report with a width-aware `lipgloss/table` (#66), and turn the report into a dashboard with a per-day sparkline and a block-glyph budget gauge (#80).

**Architecture:** Domain stays pure — the per-day series is a new pure function in `internal/report`; everything else is presentation. The table lives in its own file so `report.go` keeps its size, and every choice that `termenv.Ascii` erases (zebra background, over-budget colour, total styling) is tested by asserting on the returned `lipgloss.Style`, never on rendered bytes.

**Tech Stack:** Go 1.26.5, bubbletea v1.3.10, **lipgloss v1.1.0 including its `table` sub-package** (already an indirect part of an existing dependency — `go.mod` must not gain a new module).

**Spec:** `docs/superpowers/specs/2026-07-28-report-dashboard-design.md`

## Global Constraints

- `internal/report` and `internal/duration` stay **pure**: no I/O, no imports of `internal/config` or `internal/clickup`.
- Everything committed to the repo is in **English** — code, identifiers, comments, test messages, UI strings, commit messages. (The spec and this plan are Italian/English docs under `docs/superpowers/`, which is the historical exception.)
- **Conventional Commits.** **Never** add a `Co-Authored-By` trailer to a commit message.
- Before every commit, all five must be clean/green: `gofmt -l .`, `go vet ./...`, `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`, `go test ./... -race`.
- **No new module in `go.mod`.** `github.com/charmbracelet/lipgloss/table` is a sub-package of the existing `lipgloss` requirement. `bubbles/progress` is deliberately NOT adopted (Task 6 explains why).
- **Demo-mode parity:** `CLICKUP_DEMO=1 clup` must show the table, the sparkline and the gauge. No network.
- Never call the real ClickUp API.
- Golden files are regenerated with `go test ./internal/tui -update`, never hand-edited.
- Prefer `any` over `interface{}`; use `slices`/`maps` where they fit.

---

### Task 1: Collapse the billing editor's four selection indices (#117)

**This task is a pure refactor. There is no RED step.** The characterization test in Step 1 must pass *before* you change anything — it pins the property the four indices existed to provide. Do not invent a failing test to satisfy a TDD habit; if the test in Step 1 fails before the refactor, you have written it wrong.

**Files:**
- Modify: `internal/tui/rates.go` (26 occurrences: 4 struct fields, 8 inside `move`, 14 in the Step 6 table)
- Modify: `internal/tui/rates_view.go` (7 occurrences)
- Modify: `internal/tui/listbrowser.go:152` (1 occurrence)
- Test: `internal/tui/rates_test.go` (add one test; change nothing else)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `ratesModel.sel [secCount]int`, replacing the fields `idx`, `memIdx`, `ovIdx`, `ruleIdx`. Later tasks do not touch this screen.

**Background you need:** `ratesModel` is the billing settings editor. It has four sections (`secLists`, `secMembers`, `secOverrides`, `secRules`, with `secCount` as the count), and `rt.sec` holds the active one. Today each section keeps its own selected-row index in its own field. `rt.draft.idx` is a *different* thing — the override-creation wizard's own picker — and **must not be touched**.

- [ ] **Step 1: Write the characterization test**

Add to `internal/tui/rates_test.go`:

```go
// Each section keeps its own selected row: moving in one section must not
// disturb another, and returning to a section must restore where you were.
// This is the property the four parallel indices existed to provide, and the
// one a single array indexed by section has to preserve.
func TestRatesSelectionIsPerSection(t *testing.T) {
	t.Parallel()
	m := Model{screen: screenRates, ratesScreen: ratesModel{
		sec:     secLists,
		rows:    []rateRow{{listID: "l1", name: "Website"}, {listID: "l2", name: "Internal"}},
		members: []memberRow{{id: 1, name: "alice"}, {id: 2, name: "bob"}},
	}}

	// Move to the second list, then switch to Members.
	mm, _ := m.updateRates(keyMsg("down"))
	mm, _ = mm.(Model).updateRates(keyMsg("tab"))
	rt := mm.(Model).ratesScreen
	if rt.sec != secMembers {
		t.Fatalf("tab did not reach the Members section: sec=%v", rt.sec)
	}
	if got := rt.selected(secMembers); got != 0 {
		t.Errorf("Members selection = %d, want 0: a fresh section starts at its first row", got)
	}

	// Move inside Members, then come back to Lists.
	mm, _ = mm.(Model).updateRates(keyMsg("down"))
	mm, _ = mm.(Model).updateRates(keyMsg("shift+tab"))
	rt = mm.(Model).ratesScreen
	if rt.sec != secLists {
		t.Fatalf("shift+tab did not return to the Lists section: sec=%v", rt.sec)
	}
	if got := rt.selected(secLists); got != 1 {
		t.Errorf("Lists selection = %d, want 1: leaving and returning must not reset it", got)
	}
	if got := rt.selected(secMembers); got != 1 {
		t.Errorf("Members selection = %d, want 1: it must survive leaving the section", got)
	}
}
```

This test calls `rt.selected(sec)`, which does not exist yet. Add it to `internal/tui/rates.go` next to `selCount`, reading the *current* fields, so the test compiles and passes before the refactor:

```go
// selected returns the selected row of a section. It exists so tests can name
// a section's selection without depending on how the model stores it.
func (rt ratesModel) selected(sec ratesSection) int {
	switch sec {
	case secMembers:
		return rt.memIdx
	case secOverrides:
		return rt.ovIdx
	case secRules:
		return rt.ruleIdx
	default:
		return rt.idx
	}
}
```

- [ ] **Step 2: Run it — it must PASS**

Run: `go test ./internal/tui -run TestRatesSelectionIsPerSection -v`
Expected: **PASS**. This is a characterization test on unchanged behaviour. A failure here means the test is wrong, not the code.

- [ ] **Step 3: Commit the characterization test**

```bash
git add internal/tui/rates_test.go internal/tui/rates.go
git commit -m "test(tui): pin per-section selection in the billing editor"
```

- [ ] **Step 4: Replace the four fields with one array**

In `internal/tui/rates.go`, in the `ratesModel` struct, delete the fields `idx`, `memIdx`, `ovIdx` and `ruleIdx`, and place this immediately after `sec`:

```go
	// sel is the selected row of each section, indexed by ratesSection. One
	// array instead of four parallel fields means move() needs no switch at
	// all, and a fifth section is a non-event. Its zero value is identical to
	// four zeroed ints, so no golden changes.
	sel [secCount]int
```

Keep the `rows`, `members` and `overrides` slices exactly where they are.

Rewrite `selected` to read the array:

```go
// selected returns the selected row of a section. It exists so tests can name
// a section's selection without depending on how the model stores it.
func (rt ratesModel) selected(sec ratesSection) int { return rt.sel[sec] }
```

- [ ] **Step 5: Rewrite `move`**

Replace the whole of `move` in `internal/tui/rates.go` with:

```go
// move shifts the selection of the active section by delta, clamped.
func (rt ratesModel) move(delta int) ratesModel {
	next := rt.sel[rt.sec] + delta
	if next < 0 || next > rt.selCount()-1 {
		return rt
	}
	rt.sel[rt.sec] = next
	return rt
}
```

**Leave `selCount` exactly as it is.** Its switch is not duplication: the four counts are genuinely different (`len(rt.rows)`, `len(rt.members)`, `len(rt.overrides)+1`, `ruleCount`).

- [ ] **Step 6: Update every remaining read and write**

Each of these keeps its surrounding `switch` — the *behaviour* differs per section, only the index lookup changes. (`commit`'s switch is on `rt.edit`, not on the section; it only reads the indices.) Inside a `case secLists:` write `rt.sel[secLists]`, **not** `rt.sel[rt.sec]`: the constant names the invariant, the variable hides it.

⚠️ **Match by the "Was" text, not by the line number.** The numbers below are measured on the file as it stands *before* Step 1. Step 1 inserts the `selected` method, Step 4 rewrites the struct and Step 5 shrinks `move`, so by the time you reach this step every rates.go line number has shifted by several lines. Two entries look ambiguous — `switch rt.ruleIdx {` appears twice and `id := rt.rows[rt.idx].listID` appears twice — but each pair takes an identical replacement, so matching on text is safe.

In `internal/tui/rates.go`:

| Line (pre-Step-1) | Was | Becomes |
|---|---|---|
| 588 | `if rt.ovIdx >= len(rt.overrides) {` | `if rt.sel[secOverrides] >= len(rt.overrides) {` |
| 594 | `switch rt.ruleIdx {` | `switch rt.sel[secRules] {` |
| 644 | `rt.rates[rt.rows[rt.idx].listID] = f` | `rt.rates[rt.rows[rt.sel[secLists]].listID] = f` |
| 648 | `id := rt.rows[rt.idx].listID` | `id := rt.rows[rt.sel[secLists]].listID` |
| 662 | `id := rt.rows[rt.idx].listID` | `id := rt.rows[rt.sel[secLists]].listID` |
| 684 | `rt.memberRates[rt.members[rt.memIdx].id] = f` | `rt.memberRates[rt.members[rt.sel[secMembers]].id] = f` |
| 695 | `rt.ovIdx = indexOfOverride(...)` | `rt.sel[secOverrides] = indexOfOverride(...)` |
| 698 | `rt.overrides[rt.ovIdx].rate = f` | `rt.overrides[rt.sel[secOverrides]].rate = f` |
| 768 | `delete(rt.rates, rt.rows[rt.idx].listID)` | `delete(rt.rates, rt.rows[rt.sel[secLists]].listID)` |
| 772 | `delete(rt.memberRates, rt.members[rt.memIdx].id)` | `delete(rt.memberRates, rt.members[rt.sel[secMembers]].id)` |
| 775 | `if rt.ovIdx < len(rt.overrides) {` | `if rt.sel[secOverrides] < len(rt.overrides) {` |
| 778 | `slices.Delete(rt.overrides, rt.ovIdx, rt.ovIdx+1)` | `slices.Delete(rt.overrides, rt.sel[secOverrides], rt.sel[secOverrides]+1)` |
| 781 | `switch rt.ruleIdx {` | `switch rt.sel[secRules] {` |

In `internal/tui/rates_view.go`:

| Line | Was | Becomes |
|---|---|---|
| 54 | `billingRow(th, i == rt.idx, line)` | `billingRow(th, i == rt.sel[secLists], line)` |
| 56 | `sel := rt.rows[rt.idx]` | `sel := rt.rows[rt.sel[secLists]]` |
| 77 | `billingRow(th, i == rt.memIdx, line)` | `billingRow(th, i == rt.sel[secMembers], line)` |
| 79 | `sel := rt.members[rt.memIdx]` | `sel := rt.members[rt.sel[secMembers]]` |
| 95 | `billingRow(th, i == rt.ovIdx, line)` | `billingRow(th, i == rt.sel[secOverrides], line)` |
| 97 | `billingRow(th, rt.ovIdx >= len(rt.overrides), ...)` | `billingRow(th, rt.sel[secOverrides] >= len(rt.overrides), ...)` |
| 134 | `billingRow(th, i == rt.ruleIdx, ...)` | `billingRow(th, i == rt.sel[secRules], ...)` |

In `internal/tui/listbrowser.go:152`: `rt.idx = found` becomes `rt.sel[secLists] = found`.

**Do not touch `rt.draft.idx`** anywhere — it is the override wizard's own picker, not a section selection.

- [ ] **Step 7: Verify nothing is left behind**

Run: `grep -rn 'rt\.idx\|rt\.memIdx\|rt\.ovIdx\|rt\.ruleIdx' internal/`
Expected: **no output.**

- [ ] **Step 8: Run the full suite**

Run: `go build ./... && go test ./internal/tui -race`
Expected: PASS, including all 31 `TestRates*` tests **unchanged** and every golden **byte-identical** (the array's zero value equals four zeroed ints). If a golden changed, you altered behaviour — find out what before continuing.

- [ ] **Step 9: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui/rates.go internal/tui/rates_view.go internal/tui/listbrowser.go
git commit -m "refactor(tui): collapse the billing editor's four selection indices into one array (#117)"
```

---

### Task 2: Theme tokens for the grid

**Files:**
- Modify: `internal/tui/theme.go`
- Test: `internal/tui/theme_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `palette.Subtle lipgloss.AdaptiveColor`; `theme.Cell`, `theme.Border`, `theme.Zebra` (all `lipgloss.Style`). Task 3 uses all four.

- [ ] **Step 1: Write the failing tests**

In `internal/tui/theme_test.go`, add `Subtle` to the adaptive list inside `TestPaletteIsAdaptive`, so its slice literal reads:

```go
		{"Primary", p.Primary}, {"Accent", p.Accent},
		{"Danger", p.Danger}, {"Success", p.Success},
		{"Subtle", p.Subtle},
```

Add `Border` and `Zebra` to `paletteSample`'s slice literal, so it reads:

```go
		{"Title", th.Title}, {"Help", th.Help}, {"Err", th.Err},
		{"OK", th.OK}, {"Accent", th.Accent}, {"Box", th.Box},
		{"Border", th.Border}, {"Zebra", th.Zebra},
```

And add this new test:

```go
// The zebra style must actually carry a background: it is the only thing that
// distinguishes an odd row, and the package goldens run under termenv.Ascii,
// which strips backgrounds — so nothing else in the suite would notice it
// going missing.
func TestZebraCarriesBackground(t *testing.T) {
	t.Parallel()
	th := paletteTheme(true)
	if th.Zebra.GetBackground() == th.Cell.GetBackground() {
		t.Error("Zebra has the same background as a plain cell, so odd rows are indistinguishable")
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/tui -run 'TestPaletteIsAdaptive|TestZebraCarriesBackground|TestGoldenPalette' -v`
Expected: FAIL — `p.Subtle`, `th.Border`, `th.Zebra` and `th.Cell` are undefined (compile error).

- [ ] **Step 3: Add the palette token**

In `internal/tui/theme.go`, add to the `palette` struct:

```go
	Subtle  lipgloss.AdaptiveColor // zebra row background
```

and to `defaultPalette`:

```go
		Subtle:  lipgloss.AdaptiveColor{Light: "254", Dark: "236"},
```

Extend the doc comment above `defaultPalette` with this paragraph:

```go
// Subtle is a background, so it is judged by a different rule than the five
// foregrounds above: 236 (#303030) on dark and 254 (#E4E4E4) on light are
// chosen so the DEFAULT FOREGROUND still clears contrast when painted on top
// of them, not so they contrast with the terminal's own background. A zebra
// stripe that swallows its own text is worse than no stripe at all.
```

- [ ] **Step 4: Add the three styles**

In the `theme` struct:

```go
	Cell   lipgloss.Style // a plain report-table cell: no colour, just the renderer
	Border lipgloss.Style // the report table's frame
	Zebra  lipgloss.Style // alternate report-table row
```

In `newTheme`'s returned literal:

```go
		Cell:   r.NewStyle(),
		Border: r.NewStyle().Foreground(p.Muted),
		Zebra:  r.NewStyle().Background(p.Subtle),
```

`Cell` exists so the table's style function never has to call `lipgloss.NewStyle()`, which would silently render base rows through the *default* renderer while zebra rows go through the injected one. It is the same discipline that kept `help.New()` out of the footer.

Finally, `theme.go`'s file-level comment above `palette` says a future user-supplied theme "will override these five values". There are now six. Update the sentence.

- [ ] **Step 5: Run the tests, then regenerate the palette goldens**

Run: `go test ./internal/tui -run 'TestPaletteIsAdaptive|TestZebraCarriesBackground' -v`
Expected: PASS.

Then: `go test ./internal/tui -run TestGoldenPalette -update && go test ./internal/tui -run TestGoldenPalette -v`
Expected: PASS. Inspect `internal/tui/testdata/palette_dark.golden` and `palette_light.golden` — each must have gained exactly two lines, `Border` and `Zebra`.

- [ ] **Step 6: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui/theme.go internal/tui/theme_test.go internal/tui/testdata/palette_dark.golden internal/tui/testdata/palette_light.golden
git commit -m "feat(tui): add Subtle palette token and Cell/Border/Zebra styles"
```

---

### Task 3: The report grid (#66)

**Files:**
- Create: `internal/tui/report_table.go`
- Create: `internal/tui/report_table_test.go`
- Modify: `internal/tui/report.go` (`reportModel.view`)
- Modify: `internal/tui/app.go` (`screenBody`, the `screenReport` case)
- Modify: `internal/tui/report_test.go`, `internal/tui/golden_test.go` (call sites of `view`)

**Interfaces:**
- Consumes: `theme.Cell`, `theme.Border`, `theme.Zebra`, `theme.Header`, `theme.OK` (Task 2).
- Produces:
  - `reportTable(th theme, r report.Report, width int) string`
  - `reportRows(r report.Report) (rows [][]string, firstTotal int)`
  - `reportItemWidth(rows [][]string, headers []string, width int) int`
  - `reportStyleFunc(th theme, firstTotal int) table.StyleFunc`
  - `reportModel.view(th theme, width int) string` — **signature change**; Task 5 changes it no further.

**Background:** `report.Report` carries `Buckets []Bucket` (each with `Label`, `Hours`, `BilledHours`, `Amounts`), `CurrencySubtotals`, `TotalHours`, `BilledHours`, `TotalAmount`, `DefaultCurrency`. The existing helpers `formatAmounts(amounts, fallback)`, `hoursOf(h)` and `truncate(s, n)` are in `report.go` and must be reused, not reimplemented.

`lipgloss/table`'s defaults are hostile here and must all be overridden: `borderColumn: true` (draws separators between columns), `wrap: true` (wraps a long cell onto a second line instead of letting us truncate), and `borderStyle: lipgloss.NewStyle()` (the *default* renderer, bypassing the injected one).

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/report_table_test.go`:

```go
package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// tableFixture is a two-bucket, single-currency report.
func tableFixture() report.Report {
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	return report.Report{
		Start: start, End: start.AddDate(0, 1, 0),
		Scope: "me", GroupBy: report.GroupByList, DefaultCurrency: "EUR", Timezone: "UTC",
		Buckets: []report.Bucket{
			{Label: "Website", Key: "l1", Hours: 12.5, BillableHours: 12.5, BilledHours: 12.5,
				Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 625}}},
			{Label: "Internal", Key: "l2", Hours: 3, BillableHours: 0, BilledHours: 0,
				Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 0}}},
		},
		CurrencySubtotals: []report.CurrencySubtotal{
			{Currency: "EUR", Hours: 15.5, BillableHours: 12.5, BilledHours: 12.5, Amount: 625},
		},
		TotalHours: 15.5, BillableHours: 12.5, NonBillableHours: 3, BilledHours: 12.5, TotalAmount: 625,
	}
}

// The table must never be wider than the terminal. help's arithmetic taught us
// that "approximately fits" is not a property — this asserts the real thing.
//
// The widths start at 60 because below roughly 48 the Item column hits its
// 12-column floor and the table deliberately overflows instead of shrinking
// into illegibility; TestReportTableFloorsRatherThanVanishing covers that end.
func TestReportTableNeverExceedsWidth(t *testing.T) {
	t.Parallel()
	r := tableFixture()
	r.Buckets[0].Label = "A very long list name that will not fit anywhere near a narrow terminal"
	for _, width := range []int{60, 80, 100, 120} {
		out := reportTable(testTheme(true), r, width)
		for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Errorf("width %d: line is %d columns:\n%s", width, w, line)
			}
		}
	}
}

// Under about 48 columns there is not enough room for a 12-column label plus
// the numeric columns, and the table overflows rather than cutting labels down
// to nothing. Pinning it makes the trade-off deliberate instead of accidental.
func TestReportTableFloorsRatherThanVanishing(t *testing.T) {
	t.Parallel()
	out := reportTable(testTheme(true), tableFixture(), 30)
	if !strings.Contains(out, "Website") {
		t.Errorf("the label column collapsed past usefulness at 30 columns:\n%s", out)
	}
}

// The Item column is the only one allowed to lose: truncating an amount would
// hide money, and the numeric columns are the point of having a grid.
func TestReportItemWidthGivesSlackToTheLabel(t *testing.T) {
	t.Parallel()
	headers := []string{"Item", "Hours", "Billed", "Amount"}
	rows, _ := reportRows(tableFixture())
	rows[0][0] = strings.Repeat("x", 200) // a label longer than any terminal

	narrow := reportItemWidth(rows, headers, 60)
	wide := reportItemWidth(rows, headers, 120)
	if narrow >= wide {
		t.Errorf("item width did not grow with the terminal: 60 -> %d, 120 -> %d", narrow, wide)
	}
	if narrow < 12 {
		t.Errorf("item width %d fell below the 12-column floor", narrow)
	}
}

// width <= 0 is the first render, before the terminal has sent WindowSizeMsg.
// It must reproduce the fixed 32-column layout the report had before #66.
func TestReportItemWidthZeroIsNaturalWidth(t *testing.T) {
	t.Parallel()
	headers := []string{"Item", "Hours", "Billed", "Amount"}
	rows, _ := reportRows(tableFixture())
	rows[0][0] = strings.Repeat("x", 200)
	if got := reportItemWidth(rows, headers, 0); got != 32 {
		t.Errorf("natural item width = %d, want 32", got)
	}
}

// A long label must be cut short, never wrapped onto a second line: one bucket
// is one row. lipgloss/table wraps by default, so this pins Wrap(false).
func TestReportTableTruncatesRatherThanWraps(t *testing.T) {
	t.Parallel()
	r := tableFixture()
	r.Buckets[0].Label = strings.Repeat("long ", 40)
	out := reportTable(testTheme(true), r, 60)
	// top border, header, header separator, 2 buckets, TOTAL, bottom border.
	if n := len(strings.Split(strings.TrimRight(out, "\n"), "\n")); n != 7 {
		t.Errorf("table has %d lines, want 7 — a cell wrapped instead of truncating:\n%s", n, out)
	}
}

// The multi-currency total block: a TOTAL row with an empty Amount cell,
// followed by one subtotal row per currency.
func TestReportRowsMultiCurrencyTotals(t *testing.T) {
	t.Parallel()
	r := tableFixture()
	r.CurrencySubtotals = []report.CurrencySubtotal{
		{Currency: "EUR", Hours: 12.5, BilledHours: 12.5, Amount: 625},
		{Currency: "USD", Hours: 3, BilledHours: 3, Amount: 150},
	}
	rows, firstTotal := reportRows(r)
	if firstTotal != 2 {
		t.Fatalf("firstTotal = %d, want 2 (the two buckets come first)", firstTotal)
	}
	if len(rows) != 5 {
		t.Fatalf("got %d rows, want 5 (2 buckets + TOTAL + 2 subtotals)", len(rows))
	}
	if rows[2][0] != "TOTAL" || rows[2][3] != "" {
		t.Errorf("TOTAL row = %q, want an empty Amount cell when the report is multi-currency", rows[2])
	}
	if !strings.Contains(rows[3][0], "subtotal EUR") || !strings.Contains(rows[4][0], "subtotal USD") {
		t.Errorf("subtotal rows = %q, %q", rows[3][0], rows[4][0])
	}
}

// TestReportStyleFunc is the ONLY check on the grid's colours. The package
// goldens run under termenv.Ascii, which strips backgrounds and bold, so a
// broken zebra or an uncoloured TOTAL would leave every golden byte-identical.
// It therefore asserts on the Style the function returns, not on rendered text.
func TestReportStyleFunc(t *testing.T) {
	t.Parallel()
	th := paletteTheme(true) // real colours, so backgrounds are comparable
	const firstTotal = 2
	f := reportStyleFunc(th, firstTotal)

	if got := f(table.HeaderRow, 0); !got.GetBold() {
		t.Error("header row is not bold")
	}
	if f(0, 0).GetBackground() == th.Zebra.GetBackground() {
		t.Error("row 0 is striped; zebra must start on the second row")
	}
	if f(1, 0).GetBackground() != th.Zebra.GetBackground() {
		t.Error("row 1 is not striped")
	}
	if f(firstTotal, 0).GetBackground() == th.Zebra.GetBackground() {
		t.Error("the TOTAL row is striped; totals are not part of the zebra sequence")
	}
	if f(firstTotal, 0).GetForeground() != th.OK.GetForeground() {
		t.Error("the TOTAL row does not carry the OK colour")
	}
	if !f(firstTotal, 0).GetBold() {
		t.Error("the TOTAL row is not bold")
	}
	if f(0, 0).GetAlign() != lipgloss.Left {
		t.Error("the Item column is not left-aligned")
	}
	for col := 1; col <= 3; col++ {
		if f(0, col).GetAlign() != lipgloss.Right {
			t.Errorf("column %d is not right-aligned", col)
		}
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/tui -run 'TestReport(Table|ItemWidth|Rows|StyleFunc)' -v`
Expected: FAIL — compile error, none of the four functions exist.

- [ ] **Step 3: Write the table**

Create `internal/tui/report_table.go`:

```go
package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// The report grid's fixed dimensions. Slack is taken from and given to the
// Item column alone: the numeric columns are the reason to have a grid, and an
// amount must never be truncated because a truncated amount hides money.
const (
	reportNumWidth     = 8  // reserved for each of Hours and Billed
	reportMinItemWidth = 12 // below this the labels stop being labels
	reportNatItemWidth = 32 // the fixed width the report had before #66
	// reportTableChrome is the border and padding the table spends on itself:
	// two outer borders (column separators are off) plus two padding columns
	// on each of the four columns.
	reportTableChrome = 2 + 4*2
)

var reportHeaders = []string{"Item", "Hours", "Billed", "Amount"}

// reportRows renders the report as table cells and reports the index at which
// the total rows begin. Bucket rows come first, then either a single TOTAL row
// (one currency) or a TOTAL row with an empty Amount followed by one subtotal
// row per currency.
//
// Per-bucket Amounts are indicative: PerDay rounding can drift a few cents
// from the subtotals at a fine grouping. CurrencySubtotals is authoritative
// and is never re-derived from the bucket rows.
func reportRows(r report.Report) ([][]string, int) {
	rows := make([][]string, 0, len(r.Buckets)+len(r.CurrencySubtotals)+1)
	for _, b := range r.Buckets {
		rows = append(rows, []string{
			b.Label,
			fmt.Sprintf("%.2f", b.Hours),
			fmt.Sprintf("%.2f", b.BilledHours),
			formatAmounts(b.Amounts, r.DefaultCurrency),
		})
	}
	firstTotal := len(rows)

	if len(r.CurrencySubtotals) <= 1 {
		rows = append(rows, []string{
			"TOTAL",
			fmt.Sprintf("%.2f", r.TotalHours),
			fmt.Sprintf("%.2f", r.BilledHours),
			fmt.Sprintf("%.2f %s", r.TotalAmount, r.DefaultCurrency),
		})
		return rows, firstTotal
	}

	rows = append(rows, []string{
		"TOTAL",
		fmt.Sprintf("%.2f", r.TotalHours),
		fmt.Sprintf("%.2f", r.BilledHours),
		"",
	})
	for _, s := range r.CurrencySubtotals {
		rows = append(rows, []string{
			"  subtotal " + s.Currency,
			fmt.Sprintf("%.2f", s.Hours),
			fmt.Sprintf("%.2f", s.BilledHours),
			fmt.Sprintf("%.2f %s", s.Amount, s.Currency),
		})
	}
	return rows, firstTotal
}

// reportItemWidth splits width between the fixed numeric columns and the Item
// column. It never stretches Item past the longest label (empty space is not a
// feature) and never shrinks it below reportMinItemWidth — unless the labels
// themselves are shorter than that floor.
//
// Reserving reportNumWidth for Hours and Billed is a worst case: real values
// are shorter, so the table often renders narrower than the terminal. Narrower
// is fine; wider is the bug this arithmetic exists to prevent.
func reportItemWidth(rows [][]string, headers []string, width int) int {
	maxLabel := lipgloss.Width(headers[0])
	amount := lipgloss.Width(headers[3])
	for _, row := range rows {
		maxLabel = max(maxLabel, lipgloss.Width(row[0]))
		amount = max(amount, lipgloss.Width(row[3]))
	}
	if width <= 0 {
		return min(maxLabel, reportNatItemWidth)
	}
	floor := min(reportMinItemWidth, maxLabel)
	item := width - reportTableChrome - 2*reportNumWidth - amount
	return max(floor, min(maxLabel, item))
}

// reportStyleFunc decides every per-cell style: alignment, the header, the
// zebra stripe and the totals.
//
// It is a separate function because a golden cannot see what it does. TestMain
// pins termenv.Ascii, which strips backgrounds and bold, so a broken stripe or
// an uncoloured TOTAL leaves every golden byte-identical. The test asserts on
// the Style this returns.
//
// It never calls lipgloss.NewStyle(): that would build on the DEFAULT
// renderer, so plain rows and themed rows would render through different
// renderers. th.Cell is the plain style on the right one.
func reportStyleFunc(th theme, firstTotal int) table.StyleFunc {
	return func(row, col int) lipgloss.Style {
		st := th.Cell
		switch {
		case row == table.HeaderRow:
			st = th.Header
		case row >= firstTotal:
			st = th.OK
			if row == firstTotal {
				st = st.Bold(true) // the TOTAL line, not the per-currency subtotals
			}
		case row%2 == 1:
			st = th.Zebra
		}
		st = st.Padding(0, 1)
		if col > 0 {
			st = st.Align(lipgloss.Right)
		}
		return st
	}
}

// reportTable renders the buckets and the totals as one table sized to width.
// width <= 0 means natural width: the first render, before the terminal has
// sent its WindowSizeMsg.
//
// Three lipgloss/table defaults are wrong for this grid and are all turned
// off: BorderColumn draws separators between columns, Wrap sends a long label
// onto a second line instead of letting truncate cut it, and the zero
// BorderStyle renders the frame through the default renderer.
//
// Sizing is implicit and Table.Width is deliberately NOT called. With
// BorderColumn off, the library's resizer counts only the separators between
// columns, so it believes the frame costs nothing, while String clips the
// result with MaxWidth — the table would render two columns too wide and then
// have its right border sliced off. Pre-formatted numeric cells, labels
// truncated to itemW and Padding(0, 1) from the style function give the exact
// widths this arithmetic assumes.
func reportTable(th theme, r report.Report, width int) string {
	rows, firstTotal := reportRows(r)
	itemW := reportItemWidth(rows, reportHeaders, width)
	for i := range rows {
		rows[i][0] = truncate(rows[i][0], itemW)
	}
	return table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(th.Border).
		BorderColumn(false).
		BorderHeader(true).
		Wrap(false).
		Headers(reportHeaders...).
		Rows(rows...).
		StyleFunc(reportStyleFunc(th, firstTotal)).
		String()
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/tui -run 'TestReport(Table|ItemWidth|Rows|StyleFunc)' -v`
Expected: PASS.

- [ ] **Step 5: Wire the table into the report view**

In `internal/tui/report.go`, replace the body of `reportModel.view` from the `header :=` line down to (and including) the `return` with:

```go
	body := reportTable(th, r, width)
	if len(r.Buckets) == 0 {
		body = th.Box.Render("No hours to show.")
	}
	// The billable split is a note, not a row of data: inside the table it
	// would take the zebra stripe and be split across columns.
	split := th.Help.Render(fmt.Sprintf("  billable %s · non-billable %s",
		hoursOf(r.BillableHours), hoursOf(r.NonBillableHours)))
	return title + "\n\n" + summary + "\n\n" + body + "\n" + split
```

Change the signature to `func (rm reportModel) view(th theme, width int) string`. Delete the now-unused local variables (`header`, `rows`, `total`) — `staticcheck` will name any you miss. Keep `title` and `summary` exactly as they are.

- [ ] **Step 6: Update the call sites**

In `internal/tui/app.go`, the `screenReport` case of `screenBody` becomes:

```go
	case screenReport:
		return m.rep.view(m.theme, m.width)
```

In the tests, add a width argument to every `view(testTheme(true))` call on a `reportModel`. There are **four**:
- `internal/tui/golden_test.go:120` → `newReport(goldenReport(), "").view(testTheme(true), 80)`
- `internal/tui/report_test.go:104` and `:122` → `.view(testTheme(true), 80)`
- `internal/tui/report_test.go:157` → `mm.rep.view(testTheme(true), 80)` (inside `TestReportViewShowsSummaryAndBillableSplit`)

Run `go vet ./...` — **not** `go build ./...`, which does not compile `_test.go` files and would report a clean tree while the tests are broken — and fix any call site it names.

- [ ] **Step 7: Regenerate the report golden and add two width goldens**

In `internal/tui/golden_test.go`, replace `TestGoldenReport` with:

```go
func TestGoldenReport(t *testing.T) {
	t.Parallel()
	// One bucket gets a label long enough that the three widths actually
	// diverge. goldenReport's own labels are 8 columns, so without this all
	// three goldens would be byte-identical and would pin nothing about width.
	// At 60 the label is cut to 24 columns, at 80 to 44, and at 120 it fits
	// whole — which is also the case that proves the column does not stretch
	// past its content.
	r := goldenReport()
	r.Buckets[0].Label = "Website — landing page redesign and checkout hardening"

	for _, tc := range []struct {
		name  string
		width int
	}{{"report_narrow", 60}, {"report", 80}, {"report_wide", 120}} {
		golden(t, tc.name, newReport(r, "").view(testTheme(true), tc.width))
	}
}

// A multi-currency report renders a TOTAL row with an empty Amount cell plus
// one subtotal row per currency — the widest shape the total block takes.
func TestGoldenReportMultiCurrency(t *testing.T) {
	t.Parallel()
	r := goldenReport()
	r.Buckets = append(r.Buckets, report.Bucket{
		Label: "Mobile", Key: "l3", Hours: 4, BillableHours: 4, BilledHours: 4,
		Amounts: []report.CurrencyAmount{{Currency: "USD", Amount: 200}},
	})
	r.CurrencySubtotals = []report.CurrencySubtotal{
		{Currency: "EUR", Hours: 15.5, BillableHours: 12.5, BilledHours: 12.5, Amount: 625},
		{Currency: "USD", Hours: 4, BillableHours: 4, BilledHours: 4, Amount: 200},
	}
	r.TotalHours, r.BilledHours = 19.5, 16.5
	golden(t, "report_multicurrency", newReport(r, "").view(testTheme(true), 80))
}
```

Run: `go test ./internal/tui -run 'TestGoldenReport' -update && go test ./internal/tui -run 'TestGoldenReport' -v`
Expected: PASS.

**Then read all four golden files.** Check by eye: the header sits above a separator line, `Hours`/`Billed`/`Amount` are right-aligned and incolumnated (the old layout did not align `Amount` at all), `TOTAL` is present, the billable split sits *below* the border, and `report_narrow` is not wider than 60 columns. **The three width goldens must differ from each other** — the long label is cut at 60 and 80 and whole at 120. If they came out identical, `reportItemWidth` is not being consulted.

- [ ] **Step 8: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui/report_table.go internal/tui/report_table_test.go internal/tui/report.go internal/tui/app.go internal/tui/golden_test.go internal/tui/report_test.go internal/tui/testdata/
git commit -m "feat(tui): render the report as a width-aware lipgloss table (#66)"
```

---

### Task 4: `report.DailyHours` (#80, domain half)

**Files:**
- Create: `internal/report/daily.go`
- Create: `internal/report/daily_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `report.DailyHours(entries []TimeEntry, start, end time.Time, loc *time.Location) []float64` — one element per day in `[start, end)`. Task 5 consumes it.

**Background:** `internal/report` is **pure** — no I/O, no imports outside the standard library and this module's pure packages. `TimeEntry` has `Start time.Time` and `Duration time.Duration`. `aggregate.go` already defines `dayFormat`, and `period.go` defines `midnightIn(t, loc)` and `normLoc(loc)` — reuse them; do not write a second day-truncation.

A day must be advanced with `AddDate(0, 0, 1)` and identified by its formatted key, **never** by adding 24 hours: across a daylight-saving transition a day is 23 or 25 hours long, and second-arithmetic silently drops or duplicates one.

- [ ] **Step 1: Write the failing tests**

Create `internal/report/daily_test.go`:

```go
package report

import (
	"testing"
	"time"
)

func TestDailyHours(t *testing.T) {
	t.Parallel()
	utc := time.UTC
	day := func(y int, m time.Month, d int) time.Time {
		return time.Date(y, m, d, 0, 0, 0, 0, utc)
	}
	entry := func(start time.Time, h float64) TimeEntry {
		return TimeEntry{Start: start, Duration: time.Duration(h * float64(time.Hour))}
	}

	for _, tc := range []struct {
		name        string
		entries     []TimeEntry
		start, end  time.Time
		want        []float64
	}{
		{
			name:  "empty range yields nil",
			start: day(2026, time.July, 5), end: day(2026, time.July, 5),
			want: nil,
		},
		{
			name:  "end before start yields nil",
			start: day(2026, time.July, 5), end: day(2026, time.July, 1),
			want: nil,
		},
		{
			name:  "no entries still yields one zero per day",
			start: day(2026, time.July, 1), end: day(2026, time.July, 4),
			want: []float64{0, 0, 0},
		},
		{
			name: "idle days stay zero rather than disappearing",
			entries: []TimeEntry{
				entry(day(2026, time.July, 1).Add(9*time.Hour), 2),
				entry(day(2026, time.July, 3).Add(9*time.Hour), 1.5),
			},
			start: day(2026, time.July, 1), end: day(2026, time.July, 4),
			want: []float64{2, 0, 1.5},
		},
		{
			name: "several entries on one day are summed",
			entries: []TimeEntry{
				entry(day(2026, time.July, 2).Add(9*time.Hour), 2),
				entry(day(2026, time.July, 2).Add(14*time.Hour), 3),
			},
			start: day(2026, time.July, 1), end: day(2026, time.July, 3),
			want: []float64{0, 5},
		},
		{
			name: "entries outside the range are ignored",
			entries: []TimeEntry{
				entry(day(2026, time.June, 30).Add(9*time.Hour), 8),
				entry(day(2026, time.July, 2).Add(9*time.Hour), 1),
				entry(day(2026, time.July, 9).Add(9*time.Hour), 8),
			},
			start: day(2026, time.July, 1), end: day(2026, time.July, 3),
			want: []float64{0, 1},
		},
		{
			name: "an entry is credited to the day it starts on",
			entries: []TimeEntry{
				// Starts at 23:00 on the 1st and runs three hours into the 2nd.
				entry(day(2026, time.July, 1).Add(23*time.Hour), 3),
			},
			start: day(2026, time.July, 1), end: day(2026, time.July, 3),
			want: []float64{3, 0},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := DailyHours(tc.entries, tc.start, tc.end, utc)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d days %v, want %d days %v", len(got), got, len(tc.want), tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("day %d = %v, want %v (full series %v)", i, got[i], tc.want[i], got)
				}
			}
		})
	}
}

// A day is not 24 hours across a DST transition. Rome springs forward on
// 2026-03-29, making that day 23 hours long: a series built by adding 24h
// would drift into the next day and mis-credit every entry after it.
func TestDailyHoursSurvivesDST(t *testing.T) {
	t.Parallel()
	rome, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	start := time.Date(2026, time.March, 28, 0, 0, 0, 0, rome)
	end := time.Date(2026, time.March, 31, 0, 0, 0, 0, rome)
	entries := []TimeEntry{
		{Start: time.Date(2026, time.March, 28, 10, 0, 0, 0, rome), Duration: time.Hour},
		{Start: time.Date(2026, time.March, 29, 10, 0, 0, 0, rome), Duration: 2 * time.Hour},
		{Start: time.Date(2026, time.March, 30, 10, 0, 0, 0, rome), Duration: 3 * time.Hour},
	}
	got := DailyHours(entries, start, end, rome)
	want := []float64{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("got %d days %v, want 3", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("day %d = %v, want %v (full series %v)", i, got[i], want[i], got)
		}
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/report -run TestDailyHours -v`
Expected: FAIL — `undefined: DailyHours`.

- [ ] **Step 3: Implement it**

Create `internal/report/daily.go`:

```go
package report

import "time"

// DailyHours returns one element per day in [start, end): the total hours of
// the entries that STARTED that day, read in loc. It returns nil when the
// range is empty or inverted.
//
// It counts ALL hours, billable and not — the same total Bucket.Hours carries.
// The series answers "when did I work", not "when did I bill"; the billable
// share has its own line under the report table.
//
// The hours of an entry land entirely on its start day, which is the same rule
// groupKeys uses for GroupByDay — the two views cannot disagree about which
// day an overnight entry belongs to.
//
// This exists instead of reusing GroupByDay buckets because Build creates no
// bucket for a day with no entries. A series built from buckets would close
// the gaps, drawing a full month of work for three days of it.
//
// Days are advanced with AddDate and identified by their formatted key, never
// by adding 24 hours: across a daylight-saving transition a day is 23 or 25
// hours long, and second-arithmetic would silently drop or duplicate one.
func DailyHours(entries []TimeEntry, start, end time.Time, loc *time.Location) []float64 {
	loc = normLoc(loc)
	day := midnightIn(start, loc)
	last := midnightIn(end, loc)
	if !day.Before(last) {
		return nil
	}

	index := map[string]int{}
	var out []float64
	for d := day; d.Before(last); d = d.AddDate(0, 0, 1) {
		index[d.Format(dayFormat)] = len(out)
		out = append(out, 0)
	}

	for _, e := range entries {
		if i, ok := index[e.Start.In(loc).Format(dayFormat)]; ok {
			out[i] += e.Duration.Hours()
		}
	}
	return out
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/report -run TestDailyHours -v`
Expected: PASS, both tests and all seven sub-cases.

- [ ] **Step 5: Confirm the package is still pure**

Run: `go list -deps ./internal/report | grep 'clickup-cli'`
Expected: exactly two lines — `.../internal/duration` (imported by `model.go` for `duration.RoundMode`) and `.../internal/report` itself, since `-deps` includes the root package. **Neither `internal/config` nor `internal/clickup` may appear**; that is the whole assertion. Anything else in the list is a purity break.

- [ ] **Step 6: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/report/daily.go internal/report/daily_test.go
git commit -m "feat(report): add DailyHours, a zero-filled per-day series (#80)"
```

---

### Task 5: The sparkline (#80, presentation half)

**Files:**
- Create: `internal/tui/sparkline.go`
- Create: `internal/tui/sparkline_test.go`
- Modify: `internal/tui/report.go` (`reportModel`, `newReport`, `view`, and the new `Model.dailySeries` helper — it goes here, next to `memberFilterNote`, not in app.go)
- Modify: `internal/tui/app.go` (the `newReport` call at line ~689)
- Modify: `internal/tui/rates.go:869`, `internal/tui/report.go:73` and `:195` (the other `newReport` call sites)
- Modify: `internal/tui/golden_test.go`, `internal/tui/report_test.go`, `internal/tui/log_test.go` (test call sites)

**Interfaces:**
- Consumes: `report.DailyHours` (Task 4); `reportModel.view(th, width)` (Task 3).
- Produces:
  - `sparkline(values []float64, maxCells int) string`
  - `newReport(r report.Report, note string, daily []float64) reportModel` — **signature change**
  - `(m Model) dailySeries() []float64`

**Background:** `reportModel` holds only a `report.Report` and a note; it has no entries, which is why the series is passed in rather than computed inside. The `Model` has `visibleEntries()`, `currentRange()` and `loc`. Do **not** add a `Daily` field to `report.Report`: that would change a domain type and the CLI's JSON output for a rendering need.

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/sparkline_test.go`:

```go
package tui

import (
	"strings"
	"testing"
)

func TestSparkline(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		values   []float64
		maxCells int
		want     string
	}{
		{"empty", nil, 10, ""},
		{"all zero renders as gaps", []float64{0, 0, 0}, 10, "   "},
		{"a single value is full height", []float64{5}, 10, "█"},
		{"equal values are all full height", []float64{4, 4, 4}, 10, "███"},
		{"an idle day is a gap, not a low bar", []float64{8, 0, 8}, 10, "█ █"},
		{"the maximum is full and the minimum non-zero is lowest", []float64{1, 8}, 10, "▁█"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := sparkline(tc.values, tc.maxCells); got != tc.want {
				t.Errorf("sparkline(%v, %d) = %q, want %q", tc.values, tc.maxCells, got, tc.want)
			}
		})
	}
}

// A custom range of a year is 365 days and cannot be drawn one cell per day.
// Resampling keeps the line inside the terminal.
func TestSparklineResamples(t *testing.T) {
	t.Parallel()
	values := make([]float64, 365)
	for i := range values {
		values[i] = float64(i)
	}
	got := sparkline(values, 30)
	if n := len([]rune(got)); n != 30 {
		t.Errorf("got %d cells, want 30", n)
	}
	// The series rises monotonically, so the resampled line must too.
	runes := []rune(got)
	for i := 1; i < len(runes); i++ {
		if strings.IndexRune(sparkLevels, runes[i]) < strings.IndexRune(sparkLevels, runes[i-1]) {
			t.Errorf("cell %d (%q) is lower than cell %d (%q) in a rising series: %q",
				i, runes[i], i-1, runes[i-1], got)
		}
	}
}

// Fewer values than cells must not be stretched: 5 days is 5 cells.
func TestSparklineDoesNotStretch(t *testing.T) {
	t.Parallel()
	if got := sparkline([]float64{1, 2, 3, 4, 5}, 40); len([]rune(got)) != 5 {
		t.Errorf("sparkline of 5 values = %q (%d cells), want 5 cells", got, len([]rune(got)))
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/tui -run TestSparkline -v`
Expected: FAIL — `undefined: sparkline`, `undefined: sparkLevels`.

- [ ] **Step 3: Implement it**

Create `internal/tui/sparkline.go`:

```go
package tui

import "math"

// sparkLevels are the eight bar heights, lowest first. Index 0 is the lowest
// NON-ZERO level: an exact zero renders as a space instead, because a day with
// no work must read as a gap and ▁ would read as a little work.
const sparkLevels = "▁▂▃▄▅▆▇█"

// sparkline renders values as block glyphs, one cell per value, resampled to
// at most maxCells when there are more values than cells.
//
// Heights are relative to the largest value in the series, so the shape shows
// the period's rhythm rather than an absolute scale.
func sparkline(values []float64, maxCells int) string {
	if len(values) == 0 {
		return ""
	}
	if maxCells > 0 && len(values) > maxCells {
		values = resample(values, maxCells)
	}

	peak := 0.0
	for _, v := range values {
		peak = max(peak, v)
	}

	levels := []rune(sparkLevels)
	out := make([]rune, 0, len(values))
	for _, v := range values {
		if v <= 0 || peak == 0 {
			out = append(out, ' ')
			continue
		}
		lvl := int(math.Ceil(v / peak * float64(len(levels))))
		out = append(out, levels[min(max(lvl, 1), len(levels))-1])
	}
	return string(out)
}

// resample averages values into k contiguous buckets. Bucket i covers
// values[i*n/k : (i+1)*n/k], which distributes the remainder deterministically
// and leaves no bucket empty as long as k <= n.
func resample(values []float64, k int) []float64 {
	n := len(values)
	out := make([]float64, k)
	for i := range out {
		lo, hi := i*n/k, (i+1)*n/k
		if hi <= lo {
			hi = lo + 1
		}
		sum := 0.0
		for _, v := range values[lo:hi] {
			sum += v
		}
		out[i] = sum / float64(hi-lo)
	}
	return out
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/tui -run TestSparkline -v`
Expected: PASS.

- [ ] **Step 5: Carry the series into the report model**

In `internal/tui/report.go`, extend the model and its constructor:

```go
type reportModel struct {
	r     report.Report
	note  string
	daily []float64 // one entry per day of the range; see report.DailyHours
}

func newReport(r report.Report, note string, daily []float64) reportModel {
	return reportModel{r: r, note: note, daily: daily}
}
```

Add the `Model` helper, next to `memberFilterNote`:

```go
// dailySeries is the per-day hours of the visible entries over the current
// range. It lives on the Model because reportModel has no entries — the report
// is a rendering of an already-aggregated value.
func (m Model) dailySeries() []float64 {
	start, end := m.currentRange()
	return report.DailyHours(m.visibleEntries(), start, end, m.loc)
}
```

- [ ] **Step 6: Render it**

In `reportModel.view`, insert the sparkline between `summary` and `body`. Replace the `return` line with:

```go
	out := title + "\n\n" + summary + "\n\n"
	if line := rm.sparkView(th, width); line != "" {
		out += line + "\n\n"
	}
	return out + body + "\n" + split
```

and add:

```go
// sparkLabel is appended to the sparkline. Its width plus a margin is what
// sparkView reserves, and it guarantees the line never ends in the space a
// zero-hours day renders as.
const sparkLabel = " hours/day"

// sparkView renders the per-day sparkline, or "" when there is nothing worth
// drawing: a range of one day is a single cell, and a report with no buckets
// already says so in its body.
func (rm reportModel) sparkView(th theme, width int) string {
	if len(rm.daily) < 2 || len(rm.r.Buckets) == 0 {
		return ""
	}
	cells := 31 // the longest month, used until the terminal reports its width
	if width > 0 {
		cells = max(1, width-len(sparkLabel)-2)
	}
	return th.Accent.Render(sparkline(rm.daily, cells)) + th.Help.Render(sparkLabel)
}
```

- [ ] **Step 7: Update the four production call sites**

Each already has the range and the entries in scope, so all four become the same shape:

- `internal/tui/app.go:689` → `m.rep = newReport(m.report, m.memberFilterNote()+m.filteredNote(), m.dailySeries())`
- `internal/tui/rates.go:869` → same
- `internal/tui/report.go:73` → same
- `internal/tui/report.go:195` (`applyReport`) → same

Then add `nil` as the third argument at these five test call sites — `log_test.go:51`, `report_test.go:57`, `:73`, `:104`, `:122` — and run `go vet ./...` to catch any that moved. Use `go vet`, **not** `go build ./...`: `go build` does not compile `_test.go` files, so it would report a clean tree while every test call site is still broken. The golden tests are the exception: Step 8 gives them a real series instead of `nil`.

- [ ] **Step 8: Give the goldens a real series and regenerate**

In `internal/tui/golden_test.go`, add this fixture next to `goldenReport`:

```go
// goldenDaily is a fixed per-day series with two idle days, so the golden
// pins both a bar and a gap.
func goldenDaily() []float64 {
	return []float64{3.5, 0, 1.25, 4, 0, 2, 6}
}
```

and pass `goldenDaily()` instead of `nil` in `TestGoldenReport` and `TestGoldenReportMultiCurrency`.

Run: `go test ./internal/tui -run TestGoldenReport -update && go test ./internal/tui -race`
Expected: PASS.

**Read `internal/tui/testdata/report.golden`.** The sparkline must sit between the summary and the table, show gaps where the series is zero, and end with `hours/day`.

- [ ] **Step 9: Check demo mode by hand**

Run: `go run ./cmd/clup` with `CLICKUP_DEMO=1`, press Enter to generate the report.
Expected: a sparkline with visible gaps, a bordered table with aligned numbers, and a footer. The fixture entries fall on days **2, 3, 5, 6, 7, 9 and 10** of the range, so days 1, 4 and 8 are gaps. Day 2 is the non-billable sprint-planning session and **must** appear as a bar: the series counts all hours, not only billable ones.

- [ ] **Step 10: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui/sparkline.go internal/tui/sparkline_test.go internal/tui/report.go internal/tui/app.go internal/tui/rates.go internal/tui/golden_test.go internal/tui/report_test.go internal/tui/log_test.go internal/tui/testdata/
git commit -m "feat(tui): draw a per-day sparkline on the report (#80)"
```

---

### Task 6: The budget gauge (#80, gauge half)

**Files:**
- Modify: `internal/tui/budget.go`
- Modify: `internal/tui/budget_test.go`

**Interfaces:**
- Consumes: `theme.OK`, `theme.Err`, `theme.Help` (all pre-existing).
- Produces: `renderBudgetBar(th theme, percentUsed float64) string` — **signature change**.

**Background — why not `bubbles/progress`:** #80's checkbox names it, and it is the wrong tool here. `progress.percentageView` runs `math.Max(0, math.Min(1, percent))` *before* formatting, so a budget burned to 130% renders as `100%` — it hides the single fact this screen exists to show. (`progress.New()` also reads `termenv.ColorProfile()`, the real terminal rather than the injected renderer, which would force the colour profile onto the `theme` for one caller.) The existing pure function already gets this right and only needs better glyphs and colour.

- [ ] **Step 1: Write the failing tests**

In `internal/tui/budget_test.go`, replace `TestRenderBudgetBarClampsFillNotPercent` with:

```go
// A list over its budget must still be visible in the number even though the
// bar itself caps its fill at 100%. This is exactly what bubbles/progress
// cannot do — it clamps the percentage to 100 before formatting it — and it is
// why this screen keeps its own bar.
func TestRenderBudgetBarClampsFillNotPercent(t *testing.T) {
	t.Parallel()
	out := renderBudgetBar(testTheme(true), 150)
	if !strings.Contains(out, "150%") {
		t.Errorf("renderBudgetBar(150) = %q, want the unclamped 150%% in the label", out)
	}
	if full := strings.Repeat(string(gaugeFull), budgetBarWidth); !strings.Contains(out, full) {
		t.Errorf("renderBudgetBar(150) = %q, want a fully filled bar", out)
	}
}

// Over budget is the state the view exists to surface, so it must not look the
// same as a healthy one. The goldens run under termenv.Ascii, which strips the
// colour entirely, so this asserts on the style rather than the output.
func TestBudgetBarColoursOverBudget(t *testing.T) {
	t.Parallel()
	th := paletteTheme(true) // real colours, so foregrounds are comparable
	under := budgetFillStyle(th, 60)
	over := budgetFillStyle(th, 130)
	if under.GetForeground() != th.OK.GetForeground() {
		t.Error("a bar under budget is not drawn in the OK colour")
	}
	if over.GetForeground() != th.Err.GetForeground() {
		t.Error("a bar over budget is not drawn in the Err colour")
	}
}
```

And in `TestBudgetViewRendersProgressBar`, replace the `#`/`-` assertion with:

```go
	if !strings.ContainsRune(out, gaugeFull) || !strings.ContainsRune(out, gaugeEmpty) {
		t.Errorf("expected a bar built from %q and %q; got:\n%s", gaugeFull, gaugeEmpty, out)
	}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/tui -run 'TestRenderBudgetBar|TestBudgetBarColours|TestBudgetViewRendersProgressBar' -v`
Expected: FAIL — `gaugeFull`, `gaugeEmpty` and `budgetFillStyle` are undefined, and `renderBudgetBar` takes one argument.

- [ ] **Step 3: Implement**

In `internal/tui/budget.go`, replace `renderBudgetBar` and add the two helpers:

```go
// The gauge's glyphs. Blocks rather than '#' and '-': at any font size they
// read as a filled bar instead of as text.
const (
	gaugeFull  = '█'
	gaugeEmpty = '░'
)

// budgetFillStyle is the colour of the filled part of a gauge. Over budget is
// the state this screen exists to surface, so it must not look like a healthy
// one. It is a named function because the package goldens run under
// termenv.Ascii, which strips the colour: asserting on the style is the only
// way to test the choice.
func budgetFillStyle(th theme, percentUsed float64) lipgloss.Style {
	if percentUsed > 100 {
		return th.Err
	}
	return th.OK
}

// renderBudgetBar renders a percent-used value as a fixed-width gauge, e.g.
// "████████████░░░░░░░░ 60%". The fill is clamped to [0, 100] — a list can run
// over budget, but the bar cannot render past full — while the percentage in
// the label is shown UNCLAMPED, so an over-100% burn stays visible in the
// number. That asymmetry is the whole point: bubbles/progress clamps the
// number too, which is why this screen does not use it.
func renderBudgetBar(th theme, percentUsed float64) string {
	fillPct := min(max(percentUsed, 0), 100)
	filled := int(fillPct / 100 * budgetBarWidth)
	bar := budgetFillStyle(th, percentUsed).Render(strings.Repeat(string(gaugeFull), filled)) +
		th.Help.Render(strings.Repeat(string(gaugeEmpty), budgetBarWidth-filled))
	return fmt.Sprintf("%s %.0f%%", bar, percentUsed)
}
```

Add `"github.com/charmbracelet/lipgloss"` to the imports. The clamp uses the
built-in `min`/`max`, so no `math` import is needed.

In `budgetModel.view`, the call becomes `renderBudgetBar(th, l.PercentUsed)`, and the format string loses the brackets that used to wrap the bar:

```go
		rows.WriteString(fmt.Sprintf("%-24s %s  %.2f / %.2f %s (remaining %.2f)\n",
			truncate(l.ListName, 24), renderBudgetBar(th, l.PercentUsed), l.Billed, l.Budget, l.Currency, l.Remaining))
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/tui -run 'TestBudget|TestRenderBudgetBar' -v`
Expected: PASS.

- [ ] **Step 5: Regenerate the budget golden**

Run: `go test ./internal/tui -run TestGoldenBudget -update && go test ./internal/tui -race`
Expected: PASS.

**Read `internal/tui/testdata/budget.golden`** — the bar must be built from `█` and `░`, and the percentage must still be there.

- [ ] **Step 6: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui/budget.go internal/tui/budget_test.go internal/tui/testdata/budget.golden
git commit -m "feat(tui): draw the budget burn-down as a block gauge (#80)"
```

---

### Task 7: Documentation and closing

**Files:**
- Modify: `CHANGELOG.md`
- Modify: `docs/demo.gif` (regenerated, not hand-edited)

**Interfaces:**
- Consumes: everything from Tasks 1-6.
- Produces: nothing code depends on.

- [ ] **Step 1: Add the CHANGELOG entries**

Under `## [Unreleased]` in `CHANGELOG.md`, in the existing `### Added` / `### Changed` subsections (create them if this is the first entry since the last release):

```markdown
### Added

- The report screen now draws a per-day sparkline of the selected range, with
  idle days rendered as gaps rather than as low bars (#80).

### Changed

- The report is rendered as a real table that sizes itself to the terminal:
  the numeric columns are right-aligned and the label column takes the slack,
  instead of a fixed 32-column layout that wrapped on a narrow terminal (#66).
- The budget burn-down bars are drawn with block glyphs and coloured — green
  under budget, red over it — while still showing the true, unclamped
  percentage (#80).
```

- [ ] **Step 2: Regenerate the demo GIF**

The report's appearance changed, so `docs/demo.gif` — embedded in both READMEs — is stale.

Run: `vhs docs/demo.tape`
Expected: `docs/demo.gif` is rewritten. If `vhs` is not installed (`brew install vhs`), stop and report this step as blocked rather than committing a stale GIF.

**Then look at the result.** Extract a frame from the report section and check the table renders with its border and aligned columns, and that the sparkline is visible.

- [ ] **Step 3: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add CHANGELOG.md docs/demo.gif
git commit -m "docs: record the report dashboard and refresh the demo GIF"
```

- [ ] **Step 4: Report what still needs a human**

These are for the controller to do after the branch merges — they are **not** commits:

1. **Open an issue** for the budget screen's width: its rendered line is 94 columns and wraps on an 80-column terminal. This is pre-existing (it predates this tranche) and out of scope here, but this tranche touched that exact line's glyphs and colours, so it must not be left unrecorded. Bilingual body, `area:tui` + `area:billing`.
2. **Close #117** with a note that the four indices are now `sel [secCount]int` and that `selCount`'s switch was deliberately kept, since its four counts genuinely differ.
3. **Close #66**, noting that column separators and row separators are off by design and that `TOTAL` is distinguished by weight and colour because `lipgloss/table`'s `BorderRow` is all-or-nothing.
4. **Close #80**, noting that `bubbles/progress` was evaluated and rejected: it clamps the percentage to 100 before formatting, which would hide exactly the over-budget state the view exists to show.

---

## Self-Review

**Spec coverage:**

| Spec section | Task |
|---|---|
| §6.1 `sel [secCount]int` | 1 |
| §6.2 table, column arithmetic, borders, `Wrap(false)` | 3 |
| §6.2 `Subtle`/`Border`/`Zebra` (+ `Cell`) | 2 |
| §6.3 `DailyHours` | 4 |
| §6.3 `sparkline`, `dailySeries`, `newReport` signature | 5 |
| §6.4 gauge | 6 |
| §7.1 characterization test | 1, Step 1 |
| §7.2 pure tests | 4 Step 1, 5 Step 1 |
| §7.3 width test | 3 Step 1 |
| §7.4 style-function tests | 3 Step 1, 6 Step 1 |
| §7.5 goldens | 3 Step 7, 5 Step 8, 6 Step 5 |
| §8 execution order | Tasks 1-7 in order |
| §5 issue for the budget width | 7 Step 4 |

**Deviation from the spec, recorded deliberately:** §6.2 lists two new theme
styles (`Border`, `Zebra`); this plan adds a third, `Cell`. Without it
`reportStyleFunc` would have to call `lipgloss.NewStyle()` for unstyled rows,
building them on the *default* renderer while themed rows use the injected one
— the discipline tranche B2 established when it refused `help.New()`. The
spec's §6.2 arithmetic and §7 test list are unaffected.

**Type consistency:** `reportModel.view(th theme, width int)` is introduced in
Task 3 and unchanged by Task 5. `newReport` gains its third parameter in
Task 5 only. `renderBudgetBar` gains `th` in Task 6 only. `sparkLevels`,
`gaugeFull` and `gaugeEmpty` are referenced by tests in the same task that
defines them.
