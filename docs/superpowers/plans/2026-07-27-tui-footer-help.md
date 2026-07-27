# Generated footer and `?` full help (v1.9 tranche B2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace 25 hand-written key-hint lines with one footer generated from `keysFor(m)`, rendered in a single place, and add `?` to expand it into per-screen full help.

**Architecture:** `footer.go` holds a pure `footerView(th, width, showAll, k)` that builds a `bubbles/help` model from the theme on every call. `keys.go` gains the display data it never had: `full` groups for all 12 constructors, key pairs collapsed into one advertised item, and `ForceQuit`/`Help` as declared bindings. `Model.View()` appends the footer once; the 25 inline lines are deleted.

**Tech Stack:** Go 1.26, bubbletea v1.3.10, bubbles v1.0.0 (`bubbles/help` and `bubbles/key` — same module, already direct), lipgloss v1.1.0.

## Global Constraints

- `internal/report` and `internal/duration` stay **pure** — untouched by this plan.
- **No new `go.mod` dependencies.** `bubbles/help` is a subpackage of a module that is already a direct dependency.
- bubbletea value receivers; explicit write-back (`m.sub = x`) before every return.
- Everything in the repo is in **ENGLISH** except `README.it.md` and `CONTRIBUTING.it.md`.
- **Conventional Commits.** **Never** add a `Co-Authored-By` trailer.
- Pre-commit gate, all clean/green, every task: `gofmt -l .`, `go vet ./...`, `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`, `go test ./... -race`.
- **The 24 existing screen goldens in `internal/tui/testdata/` stay byte-identical through Tasks 1-4.** They move exactly once, in Task 5. A golden that moves earlier is a bug — never run `-update` to silence one. `testdata/` holds 26 files: the other two are `palette_dark.golden` and `palette_light.golden` from tranche A, which must not move at any point.
- **The 30 non-hint uses of `th.Help` are not touched.** Only the 25 key-hint lines and the Home timer line's `(c: manage)` suffix are in scope. Empty-state notices, table headers, breadcrumbs, subtotals, progress messages and the update notice all stay exactly as they are.
- `bubbles/key` is imported under its own name `key`; the `tea.KeyMsg` test helper in this package is `keyMsg(s string)`, **not** `key(...)`.
- Two `bubbles/key` gotchas: `SetEnabled` on a `key.Binding` field that was never assigned is a silent no-op (a zero Binding has `keys == nil`), so assign into the struct first and gate second; and `short`/`full` hold **value copies**, so build them after every `SetEnabled`.

---

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `internal/tui/footer.go` (create) | `footerView`, theme→`help.Styles`, `pairHelp`, `anyKeyHelp` | 1 |
| `internal/tui/footer_test.go` (create) | Renderer unit tests, truncation, styles | 1 |
| `internal/tui/keys.go` | `Help` + `ForceQuit` in `keyMap`; `full` for all 12; pairs collapsed in `short`/`full` | 2 |
| per-screen `_test.go` files | Parity tests gain `ctrl+c` and `?` | 2 |
| `internal/tui/footer_golden_test.go` (create) | Two footer goldens per label set, short and full | 3 |
| `internal/tui/app.go` | `helpAll` field, the `?` arm in `Update` | 4 |
| `internal/tui/app.go` | The footer render site; the timer line's `(c: manage)` suffix | 5 |
| 14 view files, `golden_test.go` | The 25 hint lines deleted; the timer fixture updated | 5 |
| `CHANGELOG.md`, `README.md`, `README.it.md` | `?` documented; key tables re-verified | 6 |

---

### Task 1: The footer renderer

A pure function and its tests. Nothing is wired, no `keysFor` constructor changes, no existing golden moves.

**Files:**
- Create: `internal/tui/footer.go`, `internal/tui/footer_test.go`
- Create: `internal/tui/testdata/footer_sample_short.golden`, `internal/tui/testdata/footer_sample_full.golden`

**Interfaces:**
- Consumes: `theme` (`theme.go`), `keyMap` (`keys.go`), `testTheme(dark bool)` (`theme_test.go`), `golden(t, name, got)` (`golden_test.go`).
- Produces:
  - `func footerView(th theme, width int, showAll bool, k keyMap) string`
  - `func pairHelp(a, b key.Binding, keyLabel, desc string) key.Binding`
  - `func anyKeyHelp(desc string) key.Binding`

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/footer_test.go`. Build the `keyMap` by hand — this task must not depend on any `keysFor` constructor, which Task 2 changes.

```go
package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

func sampleKeys() keyMap {
	d := defaultKeys()
	k := keyMap{Up: d.Up, Down: d.Down, Confirm: d.Confirm, Back: d.Back, Quit: d.Quit}
	k.short = []key.Binding{pairHelp(k.Up, k.Down, "↑/↓/j/k", "move"), k.Confirm, k.Back, k.Quit}
	k.full = [][]key.Binding{
		{pairHelp(k.Up, k.Down, "↑/↓/j/k", "move"), k.Confirm},
		{k.Back, k.Quit},
	}
	return k
}

func TestFooterShortRendersOneLine(t *testing.T) {
	t.Parallel()
	got := footerView(testTheme(true), 0, false, sampleKeys())
	if strings.Contains(got, "\n") {
		t.Errorf("short footer spans several lines:\n%s", got)
	}
	for _, want := range []string{"↑/↓/j/k", "move", "enter", "esc", "q"} {
		if !strings.Contains(got, want) {
			t.Errorf("short footer missing %q:\n%s", want, got)
		}
	}
}

func TestFooterUsesMiddleDotSeparator(t *testing.T) {
	t.Parallel()
	got := footerView(testTheme(true), 0, false, sampleKeys())
	if !strings.Contains(got, " · ") {
		t.Errorf("footer does not use the house separator:\n%s", got)
	}
	if strings.Contains(got, " • ") {
		t.Errorf("footer still uses the bubbles default separator:\n%s", got)
	}
}

func TestFooterFullRendersColumns(t *testing.T) {
	t.Parallel()
	got := footerView(testTheme(true), 0, true, sampleKeys())
	if !strings.Contains(got, "\n") {
		t.Errorf("full footer is a single line, expected stacked columns:\n%s", got)
	}
}

func TestFooterWidthTruncates(t *testing.T) {
	t.Parallel()
	wide := footerView(testTheme(true), 0, false, sampleKeys())
	narrow := footerView(testTheme(true), 20, false, sampleKeys())
	if narrow == wide {
		t.Errorf("width 20 did not truncate:\n%s", narrow)
	}
	if !strings.Contains(narrow, "…") {
		t.Errorf("truncated footer has no ellipsis:\n%s", narrow)
	}
}

func TestFooterDisabledBindingIsNotShown(t *testing.T) {
	t.Parallel()
	k := sampleKeys()
	k.short[1].SetEnabled(false) // Confirm
	if got := footerView(testTheme(true), 0, false, k); strings.Contains(got, "confirm") {
		t.Errorf("disabled binding still advertised:\n%s", got)
	}
}

func TestPairHelpEnabledWhenEitherIs(t *testing.T) {
	t.Parallel()
	d := defaultKeys()
	a, b := d.Up, d.Down
	a.SetEnabled(false)
	if !pairHelp(a, b, "↑/↓", "move").Enabled() {
		t.Error("pair disabled although one half is enabled")
	}
	b.SetEnabled(false)
	if pairHelp(a, b, "↑/↓", "move").Enabled() {
		t.Error("pair enabled although both halves are disabled")
	}
}

func TestGoldenFooterSample(t *testing.T) {
	t.Parallel()
	golden(t, "footer_sample_short", footerView(testTheme(true), 0, false, sampleKeys()))
	golden(t, "footer_sample_full", footerView(testTheme(true), 0, true, sampleKeys()))
}
```

- [ ] **Step 2: Run and watch them fail**

Run: `go test ./internal/tui -run 'TestFooter|TestPairHelp|TestGoldenFooter' -v`
Expected: build failure — `undefined: footerView`, `undefined: pairHelp`.

- [ ] **Step 3: Write footer.go**

```go
package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
)

// footerView renders one screen's advertised bindings as the bottom help line,
// or as stacked columns when showAll is set.
//
// It is a pure function rather than a sub-model on purpose. help.Model is
// configuration plus two state fields, so rebuilding it per render costs
// nothing, and it cannot fall victim to the one hazard this package keeps
// meeting: a sub-model that must be written back explicitly and silently
// zeroes itself when someone forgets.
//
// help.New() is deliberately NOT used: it derives its styles from lipgloss's
// default renderer, which would bypass the injected-renderer discipline the
// theme exists to enforce.
func footerView(th theme, width int, showAll bool, k keyMap) string {
	h := help.Model{
		Width:   width,
		ShowAll: showAll,
		// The house separator, not bubbles' " • ".
		ShortSeparator: " · ",
		FullSeparator:  "    ",
		Ellipsis:       "…",
		Styles: help.Styles{
			Ellipsis:       th.Help,
			ShortKey:       th.Help.Bold(true),
			ShortDesc:      th.Help,
			ShortSeparator: th.Help,
			FullKey:        th.Help.Bold(true),
			FullDesc:       th.Help,
			FullSeparator:  th.Help,
		},
	}
	return h.View(k)
}

// pairHelp returns a display-only binding that advertises two related bindings
// as a single footer item, so "↑/↓/j/k move" replaces "↑/k move up · ↓/j move
// down".
//
// It is never passed to key.Matches: matching only ever reads keyMap's own
// fields, while short and full are the advertisement list. That separation is
// the same one ForceQuit uses — enablement says what is accepted, short says
// what is shown.
func pairHelp(a, b key.Binding, keyLabel, desc string) key.Binding {
	p := key.NewBinding(
		key.WithKeys(append(append([]string{}, a.Keys()...), b.Keys()...)...),
		key.WithHelp(keyLabel, desc),
	)
	p.SetEnabled(a.Enabled() || b.Enabled())
	return p
}

// anyKeyHelp returns a display-only binding for a handler whose behaviour is
// triggered by the ABSENCE of a match — the delete confirmation's "any other
// key cancels". No real binding can express that, and dropping it from the
// footer would lose information the hand-written line carried.
func anyKeyHelp(desc string) key.Binding {
	return key.NewBinding(key.WithKeys("any"), key.WithHelp("any key", desc))
}
```

`th.Help.Bold(true)` returns a copy; the theme is not mutated.

- [ ] **Step 4: Generate the two goldens and read them**

```bash
go test ./internal/tui -run TestGoldenFooterSample -update
cat internal/tui/testdata/footer_sample_short.golden
cat internal/tui/testdata/footer_sample_full.golden
```

The short one must be a single line reading roughly `↑/↓/j/k move · enter confirm · esc back · q quit`. The full one must be two columns side by side. **If either looks wrong, the renderer is wrong — do not accept the file.**

- [ ] **Step 5: Confirm nothing else moved**

```bash
git status --short internal/tui/testdata   # only the two new footer_sample_* files
git diff --stat                            # only footer.go, footer_test.go
```

- [ ] **Step 6: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui/footer.go internal/tui/footer_test.go internal/tui/testdata
git commit -m "feat(tui): render a screen's bindings as a help footer (#69)"
```

---

### Task 2: The display data

`keys.go` gains what it needs to be rendered. Still nothing renders it, so the 21 existing goldens must not move.

**Files:**
- Modify: `internal/tui/keys.go`
- Modify: `internal/tui/keys_test.go` and the per-screen `_test.go` files holding parity tests (`home_test.go` or `keys_test.go`, `report_test.go`, `export_test.go`, `members_test.go`, `filters_test.go`, `budget_test.go`, `range_test.go`, `listbrowser_test.go`, `setup_test.go`, `log_test.go`, `entries_test.go`, `rates_test.go` — find them with `grep -rn enabledLabels internal/tui/*_test.go`)
- Create: `internal/tui/testdata/footer_<screen>[_<mode>].golden`, one per distinct label set

**Interfaces:**
- Consumes: `footerView`, `pairHelp`, `anyKeyHelp` from Task 1.
- Produces: `keyMap.Help`, `keyMap.ForceQuit`, and a populated `full` on every constructor.

- [ ] **Step 1: Add the two bindings to `keyMap`**

`ForceQuit` already exists in `keyDefaults` (`keys.go`, bound to `ctrl+c`). Add a `Help` default:

```go
	Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
```

Add `Help` and `ForceQuit` as `keyMap` fields, and list both in `allBindings()`. `TestAllBindingsCoversEveryField` counts `key.Binding` fields by reflection and will fail until you do.

Replace `keyDefaults.ForceQuit`'s doc comment, which currently states it lives *only* in `keyDefaults` and never in `keyMap`. That was true before this task and is now wrong.

- [ ] **Step 2: Set `ForceQuit` and `Help` in every constructor**

The rule, applied to all 12 constructors plus the inline Loading/Error case:

- **`ForceQuit` is assigned in every constructor** — `ctrl+c` is accepted on all 14 screens (`app.go` checks it before routing). It goes into `short`/`full` **only where `Quit` is not assigned**: `setup`, `rates`, `log`, `range`, `listbrowser`, `entries`. On the other eight, `Quit` is the advertised one.
- **`Help` is assigned everywhere except these 12 contexts**, where a `?` keypress already means something else:

| Screen | Context | Why |
|---|---|---|
| Setup | `stepToken` | textinput |
| Setup | `stepRate` | textinput |
| Setup | `stepCurrency` | textinput |
| Log | `logIDInput` | textinput |
| Log | `logForm`, `formField != 3` | textinput |
| Range | `editing` (both fields) | textinput |
| Rates | `editing` | textinput |
| Entries | `entriesTags` + `tagNewMode` | textinput |
| Entries | `entriesEdit`, `editStep != 4` | textinput |
| Entries | `entriesConfirmDelete` | any other key cancels |
| Error | the whole screen | any key returns Home |
| Loading | — | **enabled**: routes no keys, so `?` is free |

Range's editing mode covers two textinputs (`fromInput`, `toInput`) but is one `keysFor` branch, so it is one entry here — that is the 10 input contexts collapsed into 9 rows plus the two any-key screens.

**Loading and Error share one branch today and must stop sharing it.** `keys.go` has

```go
	case screenLoading, screenError:
		return keyMap{Quit: d.Quit, short: []key.Binding{d.Quit}}
```

but this task needs Loading to enable `Help` and Error not to. Split it into two cases:

```go
	case screenLoading:
		k := keyMap{Quit: d.Quit, ForceQuit: d.ForceQuit, Help: d.Help}
		k.short = []key.Binding{k.Help, k.Quit}
		k.full = [][]key.Binding{{k.Help, k.Quit}}
		return k
	case screenError:
		// Every key returns Home here, so ? must not claim one. ForceQuit is
		// assigned because ctrl+c is accepted — it is simply not advertised.
		k := keyMap{Quit: d.Quit, ForceQuit: d.ForceQuit}
		k.short = []key.Binding{k.Quit}
		return k
```

Leaving them merged fails either Step 5's ForceQuit test or Task 4's `TestQuestionMarkDoesNotHijackAnyKeyScreens` — and Task 4 may not edit `keys.go`.

- [ ] **Step 3: Collapse the four key pairs in `short` and `full`**

Wherever both halves are advertised, replace them with one `pairHelp` item:

| Pair | Label | Desc |
|---|---|---|
| `Up` / `Down` | `↑/↓/j/k` | `move` |
| `NextField` / `PrevField` | `tab/⇧tab` | `next/prev field` |
| `NextSection` / `PrevSection` | `tab/⇧tab` | `section` |
| `PrevMonth` / `NextMonth` | `◂/▸/h/l` | `change month` |

Use `◂` and `▸` for the month pair, not `←`/`→`: those are the glyphs `keyDefaults` and both READMEs already use.

The month pair is advertised in exactly one place — `homeKeys.full`'s first column, `{PrevMonth, NextMonth, ToggleWeek}`. Collapsing it there leaves that column with two items. Step 4 says not to *reshape* `homeKeys`/`reportKeys`: that means keep the columns and their order, not the item count.

Both halves stay assigned to their `keyMap` fields and stay enabled — only the advertisement collapses. The parity tests read `allBindings()`, not `short`, so their expected label sets do **not** change because of this step.

- [ ] **Step 4: Give every constructor a `full`**

`homeKeys` and `reportKeys` already have one, correctly grouped. **Do not reshape them** — only add `Help` to their last column. For the other ten, follow the same convention: one column per coherent group, movement first where the screen has it, **globals always last**.

```
setupKeys      stepWorkspace: {pair(Up,Down), Confirm} | {Help, ForceQuit}
               other steps:   {Confirm}                | {ForceQuit}
logKeys        modeSelect:    {PickGuided, PickByID, PickTimer} | {Help, Back, ForceQuit}
               timerPick:     {PickGuided, PickByID}            | {Help, Back, ForceQuit}
               list/taskPick: {pair(Up,Down), Confirm}          | {Help, Back, ForceQuit}
               idInput:       {Confirm}                         | {Back, ForceQuit}
               form ff!=3:    {Confirm}                         | {Back, ForceQuit}
               form ff==3:    {Yes, No}                         | {Help, Back, ForceQuit}
               timerRunning:  {StopTimer}                       | {Help, Back, ForceQuit}
               done:          {Reload, Confirm}                 | {Help, Back, ForceQuit}
entriesKeys    confirmDelete: {ConfirmDelete, anyKeyHelp("cancel")} | {ForceQuit}
               edit step!=4:  {Confirm}                         | {Back, ForceQuit}
               edit step==4:  {Yes, No}                         | {Help, Back, ForceQuit}
               history:       {}                                | {Help, Back, ForceQuit}
               tags+new:      {Confirm}                         | {Back, ForceQuit}
               tags:          {pair(Up,Down), ToggleItem} | {NewTag, Confirm} | {Help, Back, ForceQuit}
               list:          {pair(Up,Down)} | {Edit, Delete, Tags, History} | {Help, Back, ForceQuit}
ratesKeys      editing:       {Confirm}                         | {Back, ForceQuit}
               draft:         {pair(Up,Down), Confirm}          | {Help, Back, ForceQuit}
               normal:        {pair(NextSection,PrevSection), pair(Up,Down), Confirm}
                              | {ListCurrency, ListBudget, NewOverride, ClearValue, BrowseList}
                              | {Save, Help, Back, ForceQuit}
rangeKeys      editing:       {pair(NextField,PrevField), Confirm} | {Back, ForceQuit}
               list:          {pair(Up,Down), Confirm}          | {Help, Back, ForceQuit}
listBrowserKeys               {pair(Up,Down), Confirm}          | {Help, Back, ForceQuit}
exportKeys                    {pair(Up,Down), Confirm}          | {Help, Back, Quit}
membersKeys                   {pair(Up,Down), ToggleItem, SelectAll} | {Confirm} | {Help, Back, Quit}
filtersKeys                   {pair(NextField,PrevField), pair(Up,Down), ToggleItem, SelectAll}
                              | {Confirm} | {Help, Back, Quit}
budgetKeys                    {Budget}                          | {Help, Back, Quit}
loading        (split case)   {}                                | {Help, Quit}
error          (split case)   ForceQuit assigned, nothing advertised beyond today's q;
                              no full — the screen keeps its inline sentence
```

An empty first column is fine: `help.FullHelpView` skips groups whose bindings are all disabled or absent.

`short` keeps its existing curation on every screen; add `Help` to it wherever `Help` is assigned, immediately before the quit item.

- [ ] **Step 5: Update the parity tests**

Every per-screen label test gains `"ctrl+c"`, and gains `"?"` in every **state** where `Help` is assigned — log, entries, rates, range and setup have per-state parity tests, and `Help` is assigned in some of their states and not others (Step 2's table). Sort order is plain string sort: `"?"` sorts before every letter, `"ctrl+c"` sorts after `"c"` and before `"d"`.

`TestQuitBindingPerScreen` is unaffected — `Quit`'s enablement does not change.

Add one test pinning the advertisement/acceptance split:

```go
// ForceQuit is accepted everywhere but advertised only where q cannot quit.
// The two mechanisms are different on purpose: Enabled() is what key.Matches
// consults, short is what the footer renders.
func TestForceQuitAcceptedEverywhereAdvertisedOnSix(t *testing.T) {
	t.Parallel()
	advertised := map[screen]bool{
		screenSetup: true, screenRates: true, screenLog: true,
		screenRange: true, screenListBrowser: true, screenEntries: true,
	}
	for s := range 14 {
		sc := screen(s)
		m := newTestModel()
		m.screen = sc
		k := keysFor(m)
		if !k.ForceQuit.Enabled() {
			t.Errorf("screen %v: ctrl+c not accepted", sc)
		}
		shown := slices.ContainsFunc(k.ShortHelp(), func(b key.Binding) bool {
			return slices.Contains(b.Keys(), "ctrl+c")
		})
		if shown != advertised[sc] {
			t.Errorf("screen %v: ctrl+c advertised = %v, want %v", sc, shown, advertised[sc])
		}
	}
}
```

Confirm `screen(13)` is the last constant before trusting the `range 14`; if the iota has a different length, use the same explicit map style as `TestQuitBindingPerScreen`.

- [ ] **Step 6: Confirm no golden moved at all**

```bash
git status --short internal/tui/testdata
```
Expected: **no output whatsoever** — this task adds no golden and must change none. Task 3 adds the footer goldens; if one of the 24 screen goldens changed here, something in this task rendered, which it must not.

- [ ] **Step 7: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui
git commit -m "feat(tui): declare full help, ctrl+c and ? for every screen (#69)"
```

---

### Task 3: Footer goldens, short and full

Two goldens per label set. **This is the only check on Task 2's `full` columns**: a binding named in a `full` group but never assigned in that branch is a zero `Binding`, which `help.FullHelpView` silently drops — no compile error, no failing parity test, nothing.

**Files:**
- Create: `internal/tui/footer_golden_test.go`
- Create: `internal/tui/testdata/footer_<state>_short.golden` and `footer_<state>_full.golden`, two per label set

**Interfaces:**
- Consumes: `footerView` (Task 1), `keysFor` with its populated `short`/`full` (Task 2), `golden`, `testTheme`, `newTestModel`, `newTestModelOnReport`.
- Produces: nothing later tasks import.

- [ ] **Step 1: Enumerate the label sets**

The set of states is exactly the set the parity tests already cover — same granularity, different assertion. Find them:

```bash
grep -rn 'enabledLabels' internal/tui/*_test.go
```

Every case in every parity test (including each row of the table-driven per-step tests for `log`, `entries`, `rates`, `range`, `setup`) is one entry here, plus `screenLoading` and `screenError`.

- [ ] **Step 2: Write the table-driven test**

```go
func TestGoldenFooters(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		model func() Model
	}{
		{"home", func() Model { m := newTestModel(); m.screen = screenHome; return m }},
		{"report", newTestModelOnReport},
		// … one entry per label set, mirroring the parity tests' cases …
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := keysFor(tc.model())
			golden(t, "footer_"+tc.name+"_short", footerView(testTheme(true), 0, false, k))
			golden(t, "footer_"+tc.name+"_full", footerView(testTheme(true), 0, true, k))
		})
	}
}
```

The `t.Run` per case is not decoration: `golden` calls `t.Fatalf` when a file is missing, so without subtests the first missing golden aborts every remaining state.

Build each state the way its parity test builds it — same fields, same sub-model setup. Do not invent a shortcut; a state built differently from the one the parity test asserts on gives two nets that do not cover the same ground.

- [ ] **Step 3: Generate and read every file**

```bash
go test ./internal/tui -run TestGoldenFooters -update
git status --short internal/tui/testdata
```

Then **read all of them**. What to look for, state by state:

- the `_short` file is one line, and names the same actions the screen's old hand-written hint line named;
- the `_full` file has the columns Task 2 assigned, with the globals last;
- **no column is missing and no column is empty.** A `full` group whose bindings were never assigned renders as nothing — the column simply is not there. That is the defect this task exists to catch, and it is only visible by counting columns against Task 2's table.

Report any state whose full help came out with fewer columns than Task 2 specifies; that is a Task 2 bug to fix here, not a golden to accept.

- [ ] **Step 4: Confirm the 24 screen goldens did not move**

```bash
git status --short internal/tui/testdata | grep -v '^?? '
```
Expected: no output. Only new files.

- [ ] **Step 5: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui
git commit -m "test(tui): pin every screen's short and full footer (#69)"
```

---

### Task 4: The `?` toggle

Small and self-contained. Nothing renders `helpAll` yet, so no golden moves.

**Files:**
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/app_test.go`

**Interfaces:**
- Consumes: `keysFor(m).Help` from Task 2, and its per-state gating.
- Produces: `Model.helpAll bool`.

- [ ] **Step 1: Write the failing tests**

```go
func TestQuestionMarkTogglesFullHelp(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	m.screen = screenHome
	next, _ := m.Update(keyMsg("?"))
	if !next.(Model).helpAll {
		t.Fatal("? did not turn full help on")
	}
	again, _ := next.(Model).Update(keyMsg("?"))
	if again.(Model).helpAll {
		t.Error("? did not turn full help back off")
	}
}

// On screenError every key returns Home; on the delete confirmation every key
// but y cancels. A ? binding that fired there would take away the only exit.
func TestQuestionMarkDoesNotHijackAnyKeyScreens(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	m.screen = screenError
	m.err = errors.New("boom")
	next, _ := m.Update(keyMsg("?"))
	if got := next.(Model).screen; got != screenHome {
		t.Errorf("? on the error screen → %v, want screenHome", got)
	}
	if next.(Model).helpAll {
		t.Error("? toggled help on a screen where every key means 'go home'")
	}
}

func TestQuestionMarkTypesIntoInputs(t *testing.T) {
	t.Parallel()
	m := newTestModelOnReport()
	next, _ := m.Update(keyMsg("n")) // → log flow
	next, _ = next.(Model).Update(keyMsg("2")) // → ID input
	next, _ = next.(Model).Update(keyMsg("?"))
	mm := next.(Model)
	if mm.helpAll {
		t.Error("? toggled help while an input was focused")
	}
	if got := mm.logScreen.input.Value(); got != "?" {
		t.Errorf("input value = %q, want %q", got, "?")
	}
}
```

Add a case to the delete-confirmation tests asserting `?` still cancels.

- [ ] **Step 2: Run and watch them fail**

Run: `go test ./internal/tui -run TestQuestionMark -v`
Expected: build failure — `m.helpAll` undefined.

- [ ] **Step 3: Add the field and the arm**

Add `helpAll bool` next to `width, height int` on `Model`.

In `Update`'s `tea.KeyMsg` arm, immediately after the `ForceQuit` check and **before** `return m.routeKey(msg)`:

```go
		if key.Matches(msg, keysFor(m).Help) {
			m.helpAll = !m.helpAll
			return m, nil
		}
```

Placing it here — beside `Quit` and `ForceQuit` rather than in `routeKey` — is what makes `?` work identically on every screen that enables it, including `screenLoading`, which `routeKey` has no case for.

- [ ] **Step 4: Run the tests to green**

Run: `go test ./internal/tui -run 'TestQuestionMark|TestEntriesConfirm' -v` → PASS.

- [ ] **Step 5: Confirm no golden moved and gate**

```bash
git status --short internal/tui/testdata   # must be empty
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui
git commit -m "feat(tui): toggle full help with ? (#69)"
```

---

### Task 5: Wiring, and the 25 lines deleted

The one task where goldens move. They move **once**.

**Files:**
- Modify: `internal/tui/app.go`
- Modify: `home.go`, `report.go`, `export.go`, `budget.go`, `members.go`, `filters.go`, `range.go`, `listbrowser.go`, `setup.go`, `history.go`, `entries.go`, `log.go`, `rates.go`, `rates_view.go`
- Modify: `internal/tui/golden_test.go` — one fixture string (Step 3)
- Update: the 24 existing screen goldens

- [ ] **Step 1: Append the footer in `Model.View()`**

Rename the existing method body to produce a `body string`, then:

```go
func (m Model) View() string {
	body := m.screenBody()
	if m.screen == screenError {
		// Every key returns Home here, which is not a binding — the screen
		// says so in its own sentence instead.
		return body
	}
	return body + "\n\n" + footerView(m.theme, m.width, m.helpAll, keysFor(m))
}
```

- [ ] **Step 2: Delete the 25 hint lines**

Find them, do not trust these line numbers — Tasks 1-4 shifted them:

```bash
grep -rn 'Help.Render' internal/tui --include='*.go' | grep -v _test
```

The 25 to delete, by file and by what they say:

| File | The line |
|---|---|
| `home.go` | the `help` string built with `+=` above `out := title + …` — delete the variable and the `th.Help.Render(help)` term |
| `report.go` | `g: grouping · e: export · …` |
| `export.go` | `↑/↓ select · Enter: export · Esc: back to report` |
| `budget.go` | `Esc/b: back to report · q: quit` (one variable, two return paths) |
| `members.go` | `↑/↓ move · Space toggle · a: all/none · Enter: confirm · Esc: cancel` |
| `filters.go` | `Tab/⇧Tab section · ↑/↓ move · …` |
| `range.go` | the `help` variable with its `if rs.editing` branch |
| `listbrowser.go` | `↑/↓ move · Enter: open/select · Esc: up / back` |
| `setup.go` | `Enter: confirm · Ctrl+C: quit` |
| `history.go` | `Esc: back to entries` |
| `entries.go` | seven lines: confirm-delete, tags-loading, tags-new, tags, empty list, list, and the edit view's `Esc: cancel` |
| `log.go` | six lines: no-timer, timer running, list pick, task pick, done, and the unconditional `Esc: cancel · Ctrl+C: quit` |
| `rates.go` | the `rt.help()` call site |
| `rates_view.go` | the `help()` method itself |

**Leave every other `th.Help` use alone.** The empty-state notices, table headers, breadcrumbs, subtotals, progress messages and the update notice are not hints and are out of scope. In particular `rates_view.go`'s `No lists in the current report — press 'b' to browse the workspace.` stays: it is an instruction inside a sentence, not a legend.

- [ ] **Step 3: Strip `(c: manage)` from the timer line**

In `app.go`'s `screenHome` case, the timer line is built as
`"⏱  running on " + name + " — " + label + "   (c: manage)"`. Drop the suffix.
The `Timer` binding is gated on `m.runningTimer != nil`, so the footer shows
`c manage timer` exactly when that line exists — the information survives, in
one place instead of two.

**`golden_test.go` carries the same suffix as a literal.** `TestGoldenHomeWithNotices` passes the timer line in by hand, with a comment promising it is "the string app.go builds… verbatim". Strip `"   (c: manage)"` there too, or the fixture depicts something the app no longer renders and nothing fails to say so.

- [ ] **Step 4: Regenerate the goldens, once**

```bash
go test ./internal/tui -update
git diff internal/tui/testdata
```

**Read every diff.** `git status --short internal/tui/testdata` should list **24 modified files and nothing else**. The expectation, stated so a surprise is visible:

- **19 body goldens** (sub-model views) each lose **exactly one** hint line. Log and Setup are no exception: Log renders two hint lines only in its timer/list/task/done steps, and the goldens capture `logModeSelect` and `logForm`; Setup never renders two at all.
- `home_notices` additionally loses the `(c: manage)` suffix, from the Step 3 fixture edit.
- **5 composed goldens** (`entries`, `entries_edit`, `entries_confirm_delete`, `error`, `loading`) lose their hint line and gain a footer — except `error`, which gains nothing (it keeps its sentence and gets no footer), and `loading`, which gains a footer where it had none.
- **`palette_dark.golden` and `palette_light.golden` must not move.** They are tranche A's color goldens and have nothing to do with this work.
- **The footer goldens from Task 3 must not move.** If one does, the wiring changed the data, which it must not.

Anything else is a bug in this task.

- [ ] **Step 5: Verify no hint line survives**

```bash
grep -rnE 'Help\.Render\("[^"]*(Esc|Enter|↑/↓|Tab|any other key)' internal/tui --include='*.go' | grep -v _test
grep -rnE 'Help\.Render\((help|rt\.help)' internal/tui --include='*.go' | grep -v _test
grep -rn 'g: grouping' internal/tui --include='*.go' | grep -v _test
```
Expected: no output from any of the three.

**The grep is a backstop, not the check.** Five of the 25 render a variable or a method result (`home.go` and `range.go` build a `help` string; `rates.go` calls `rt.help()`), and two more are literals that happen to contain none of the words above — which is why the second and third patterns exist. The real check is reading the golden diff in Step 4: a hint line that survived is a line the golden still shows.

- [ ] **Step 6: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui
git commit -m "refactor(tui): render every screen's key hints from the keymap (#69)"
```

---

### Task 6: Documentation

**Files:** `CHANGELOG.md`, `README.md`, `README.it.md`

- [ ] **Step 1: CHANGELOG**

Under `## [Unreleased]`, extending the existing `### Added` / `### Changed` sections rather than adding duplicates:

```markdown
### Added

- `?` opens a full list of the keys the current screen accepts.

### Changed

- Every screen now shows the same generated key footer, built from the same
  keymap that dispatches the keys — so it can no longer drift from what the
  keys actually do. Four screens that never showed how to quit now do.
```

- [ ] **Step 2: Both READMEs**

Add `?` to the key tables. Re-verify every other row against `keys.go`, which is the single source of truth. `README.it.md` in fluent native Italian with correct diacritics (à, è, é, ì, ò, ù) — never ASCII substitutes.

- [ ] **Step 3: Gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add CHANGELOG.md README.md README.it.md
git commit -m "docs: document the generated key footer and ? full help"
```

---

## Definition of done

- All three greps in Task 5 Step 5 return nothing.
- Every `keysFor` branch populates `full` (except `screenError`, which renders no footer), and `ForceQuit` is assigned in all of them.
- Every `full` column named in Task 2 Step 4 actually appears in the corresponding `footer_<state>_full.golden`. A missing column means a binding was never assigned.
- `?` toggles full help on every screen except the 12 contexts listed in Task 2 Step 2, and types a literal `?` in the 10 input ones.
- The 24 pre-existing screen goldens moved exactly once, in Task 5, and each diff was read. `palette_dark.golden` and `palette_light.golden` never moved.
- `go test ./... -race` passes.
- #69 closes. **#59 stays open** on its overlay-compositor checkbox, which ships with the command palette in B3.
