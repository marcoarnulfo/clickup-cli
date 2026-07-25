# Keymap registry and navigation stack (v1.9 tranche B1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace 97 string `case` clauses and 66 scattered screen transitions with one declared keymap per screen and one navigation mechanism, so the footer (#69), the command palette (#71) and configurable keybindings (#82) each have a single place to plug into.

**Architecture:** `keys.go` holds a screen-independent defaults table plus a pure `keysFor(m Model) keyMap` that contextually enables bindings and carries its own help ordering. `nav.go` holds a `nav []screen` parent chain on the `Model`, reached only through `goTo`/`replace`/`pop`/`resetTo`, with a truncating push. Each screen's bindings are defined in the same task that migrates its handler. Navigation lands last, so the mechanical migration is provable against unchanged navigation semantics.

**Tech Stack:** Go 1.26, bubbletea v1.3.10, bubbles v1.0.0 (`bubbles/key` — already a direct dependency), lipgloss v1.1.0.

## Global Constraints

- `internal/report` and `internal/duration` stay **pure** — untouched by this plan.
- **No new `go.mod` dependencies.**
- bubbletea value receivers; explicit write-back (`m.sub = x`) before every return.
- Everything in the repo is in **ENGLISH** except `README.it.md` and `CONTRIBUTING.it.md`.
- **Conventional Commits.** **Never** add a `Co-Authored-By` trailer.
- Pre-commit gate, all clean/green, every task: `gofmt -l .`, `go vet ./...`, `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`, `go test ./... -race`.
- **All goldens in `internal/tui/testdata/` stay byte-identical from Task 1 onward.** This tranche changes input and navigation, never layout. A moved golden is a bug — never run `-update` to silence one after Task 1.
- **Goldens are not this tranche's safety net.** They verify rendering, not input; a wrong `key.Matches` makes a key silently mute and no golden notices. The net is the per-screen label-parity tests, the targeted transition tests, and the review rule that every removed `case` label must reappear verbatim in a `WithKeys`.
- **Preserve each handler's step/mode dispatch structure exactly.** Five screens (`log`, `entries`, `rates`, `range`, `setup`) forward unmatched keys into a `textinput`, across nine input contexts. If `key.Matches` is hoisted above the input-forwarding branch, typed characters start firing actions — typing "s" into a note field would stop the timer.
- Demo mode needs no parity work: `demo.go` holds fixtures and commands only, no key handlers and no screen transitions. Its two `case` clauses match space IDs, not keys.

---

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `internal/tui/golden_test.go`, `testdata/*.golden` | Deeper capture: log form, entries edit, confirm-delete, error, loading | 1 |
| `internal/tui/keys.go` (create) | Defaults table, `keyMap` type, `help.KeyMap` implementation, `keysFor` for all 14 screens | 2, 3, 4 |
| `internal/tui/keys_test.go` (create) | Per-screen label-parity tests, enablement⇔guard tests | 2, 3, 4 |
| `internal/tui/app.go` | Global key routing; the quit hack removed | 2 |
| 9 small handlers | `home`, `report`, `filters`, `members`, `export`, `range`, `listbrowser`, `setup`, `budget` — 48 cases | 3 |
| 3 large handlers | `log` 17, `entries` 16, `rates` 16, plus the `tea.Key` sites | 4 |
| `internal/tui/nav.go` (create) | `nav []screen`, the four transition calls, truncating push | 5 |
| All 13 files with transitions | 66 sites routed through the API; both `origin` fields deleted | 5 |
| `internal/tui/report.go`, `keys.go`, `app.go` | Report's new `esc`; three staleness guards | 6 |
| `CHANGELOG.md`, `README.md`, `README.it.md` | Report's new `esc` documented; every key row re-verified | 7 |

---

### Task 1: Deeper goldens (#131)

Captures the screen states later tasks touch, before they are touched. No production file may change.

**Files:**
- Modify: `internal/tui/golden_test.go`
- Create: 5 files in `internal/tui/testdata/`

**Interfaces:**
- Consumes: `golden`, `goldenModel`, `goldenReport`, `goldenEntries`, `goldenFixedTime` (in `golden_test.go`) and `testTheme` (in `theme_test.go`) — all from v1.9 tranche A, same package.
- Produces: goldens named `log_form`, `entries_edit`, `entries_confirm_delete`, `error`, `loading`.

- [ ] **Step 1: Convert `TestGoldenRatesTabs` to subtests**

```go
		t.Run(tc.name, func(t *testing.T) {
			rt := newRates(goldenEntries(), cfg)
			rt.sec = tc.sec
			golden(t, tc.name, rt.view(testTheme(true)))
		})
```

Without this, `golden`'s `t.Fatalf` on a missing file aborts the remaining tabs.

- [ ] **Step 2: Write the new golden tests**

Append to `internal/tui/golden_test.go`. Read `log.go`, `entries.go` and `app.go` first and confirm every field and constant against the source.

**`goldenEntries()` is sorted Start-descending by the browser, so the entry at index 0 is `e2` — UserName "Ada", `UserID: 2`.** The edit and delete keys are ownership-gated by `canEdit`, so a model with `m.userID = 1` cannot open either on the selected row. Move the cursor down first, as below; do not "fix" this by weakening the state you capture.

```go
func TestGoldenLogForm(t *testing.T) {
	t.Parallel()
	lg := newLog(goldenEntries(), config.Config{Token: "t", WorkspaceID: "team1"}, screenReport)
	lg.now = goldenFixedTime
	lg = enterForm(lg) // sets step AND initializes the text inputs
	golden(t, "log_form", lg.view(testTheme(true)))
}

func TestGoldenEntriesEdit(t *testing.T) {
	t.Parallel()
	m := goldenModel()
	m.userID = 1
	m.entries = goldenEntries()
	m.screen = screenReport
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // to the own entry
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	mm := next.(Model)
	if mm.entriesScreen.mode != entriesEdit {
		t.Fatalf("did not reach the edit form: mode=%v", mm.entriesScreen.mode)
	}
	golden(t, "entries_edit", mm.View())
}

func TestGoldenEntriesConfirmDelete(t *testing.T) {
	t.Parallel()
	m := goldenModel()
	m.userID = 1
	m.entries = goldenEntries()
	m.screen = screenReport
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	mm := next.(Model)
	if mm.entriesScreen.mode != entriesConfirmDelete {
		t.Fatalf("did not reach confirm-delete: mode=%v", mm.entriesScreen.mode)
	}
	golden(t, "entries_confirm_delete", mm.View())
}

func TestGoldenError(t *testing.T) {
	t.Parallel()
	m := goldenModel()
	m.screen = screenError
	m.err = errors.New("request failed: 500 Internal Server Error")
	golden(t, "error", m.View())
}

func TestGoldenLoading(t *testing.T) {
	t.Parallel()
	m := goldenModel()
	m.screen = screenLoading
	golden(t, "loading", m.View())
}
```

Add `"errors"` to the imports. The two `t.Fatalf` mode assertions are load-bearing: without them a golden that silently captured the browser list would look like a passing test forever.

`enterForm` must be the real helper `log.go` uses to enter the form step — read the file and call whatever initializes the inputs. Setting `lg.step` alone renders a zero-value `textinput`, a state production never shows.

- [ ] **Step 3: Run and watch them fail**

Run: `go test ./internal/tui -run 'TestGoldenLogForm|TestGoldenEntriesEdit|TestGoldenEntriesConfirmDelete|TestGoldenError|TestGoldenLoading' -v`
Expected: all five FAIL with `no such file or directory` — not with a mode assertion, which would mean the key sequence is wrong.

- [ ] **Step 4: Generate and inspect**

```bash
go test ./internal/tui -run 'TestGoldenLogForm|TestGoldenEntriesEdit|TestGoldenEntriesConfirmDelete|TestGoldenError|TestGoldenLoading' -update
```

Read all five. Each must show the state its name claims. `grep -l $'\x1b' testdata/*.golden` must still list only the two `palette_*` files.

- [ ] **Step 5: Verify stability and that nothing else moved**

```bash
go test ./internal/tui -run TestGolden -count=2
git status --short internal/tui/testdata   # only the five new files
```

- [ ] **Step 6: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git status --short   # only golden_test.go and testdata/
git add internal/tui/golden_test.go internal/tui/testdata
git commit -m "test(tui): capture the log form, entries edit and error screens as goldens (#131)"
```

---

### Task 2: The keymap foundation and the global quit binding

Establishes the type, the defaults table and the pattern, on **Home only** plus the two handler-less screens. The other eleven screens arrive in Tasks 3 and 4, each next to its own handler migration.

**Files:**
- Create: `internal/tui/keys.go`, `internal/tui/keys_test.go`
- Modify: `internal/tui/app.go` (global routing only — no handler migrates in this task)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces:
  - `type keyDefaults struct { … key.Binding … }` and `func defaultKeys() keyDefaults`
  - `type keyMap struct { … key.Binding fields …; short []key.Binding; full [][]key.Binding }`
  - `func (k keyMap) ShortHelp() []key.Binding` and `func (k keyMap) FullHelp() [][]key.Binding` — satisfies `bubbles/help`.KeyMap, verified against v1.0.0
  - `func (k keyMap) allBindings() []key.Binding` — every binding field, for the parity tests
  - `func keysFor(m Model) keyMap` — the pure entry point
  - `func enabledLabels(k keyMap) []string` (test helper, in `keys_test.go`)

- [ ] **Step 1: Write the failing parity test for Home**

Create `internal/tui/keys_test.go`:

```go
package tui

import (
	"slices"
	"testing"
)

// enabledLabels is every key label the screen accepts right now, sorted and
// deduplicated — the contract the migration must preserve exactly.
func enabledLabels(k keyMap) []string {
	var out []string
	for _, b := range k.allBindings() {
		if !b.Enabled() {
			continue
		}
		out = append(out, b.Keys()...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func TestHomeKeyLabels(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	m.screen = screenHome
	m.scope = "me"
	want := []string{/* fill from home.go — see below */}
	if got := enabledLabels(keysFor(m)); !slices.Equal(got, want) {
		t.Errorf("home labels (me scope) = %v, want %v", got, want)
	}
}

func TestHomeMembersKeyIsTeamScopeOnly(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	m.screen = screenHome
	m.scope = "me"
	if keysFor(m).Members.Enabled() {
		t.Error("Members binding enabled in me scope")
	}
	m.scope = "team"
	if !keysFor(m).Members.Enabled() {
		t.Error("Members binding disabled in team scope")
	}
}
```

**Derive `want` by reading `home.go`'s `case` clauses.** Every label, verbatim, including the aliases (`case "left", "h"` means both `left` and `h`). Include contextually gated keys only in the state the test sets up.

**Plus the keys handled globally, which appear in no `case` clause of the screen's own file:** `q` (today `app.go:601`, Step 5) and, after Task 4, `ctrl+c`. Both are enabled on Home, so both belong in `want`. A list derived from `home.go` alone will be missing `q` and the first run will fail for the wrong reason.

Getting this list exactly right *is* the task — it is the contract the migration must preserve, and a wrong list either lets a regression through or blocks a correct migration.

- [ ] **Step 2: Run and watch it fail**

Run: `go test ./internal/tui -run TestHomeKey -v`
Expected: build failure — `undefined: keysFor`, `undefined: keyMap`.

- [ ] **Step 3: Write keys.go**

```go
package tui

import "github.com/charmbracelet/bubbles/key"

// keyDefaults is the screen-independent binding table: every key the TUI knows,
// defined exactly once, with its help text. User-configurable keybindings (#82)
// will override entries here and nothing else.
type keyDefaults struct {
	Quit    key.Binding
	Back    key.Binding
	Range   key.Binding
	Members key.Binding
	// … one field per distinct action, added as screens arrive in Tasks 3-4 …
}

func defaultKeys() keyDefaults {
	return keyDefaults{
		Quit:    key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Range:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "range")),
		Members: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "select members")),
	}
}

// keyMap is one screen's active bindings, plus the order the footer (#69) and
// the ? overlay (#69) will render them in. A zero Binding is disabled and
// matches nothing — that is how contextual keys are switched off.
type keyMap struct {
	Quit, Back, Range, Members key.Binding

	short []key.Binding
	full  [][]key.Binding
}

func (k keyMap) ShortHelp() []key.Binding  { return k.short }
func (k keyMap) FullHelp() [][]key.Binding { return k.full }

// allBindings returns every binding field, so the parity tests do not have to
// know the struct's shape. Keep it in sync when a field is added.
func (k keyMap) allBindings() []key.Binding {
	return []key.Binding{k.Quit, k.Back, k.Range, k.Members}
}

// keysFor returns the bindings the given Model state accepts right now. It is
// pure: nothing is cached, so there is no second copy of the truth to keep in
// sync. Contextual gating lives here AND in the handlers' guards — once a
// binding is disabled key.Matches fails and the guard is unreachable, so
// enablement is load-bearing for behavior, not just for display.
func keysFor(m Model) keyMap {
	d := defaultKeys()
	switch m.screen {
	case screenHome:
		return homeKeys(m, d)
	case screenLoading, screenError:
		// No key handler, but both accept q today; a zero keyMap would
		// silently disable it and leave Loading with no exit at all.
		return keyMap{Quit: d.Quit, short: []key.Binding{d.Quit}}
	}
	return keyMap{}
}

func homeKeys(m Model, d keyDefaults) keyMap {
	k := keyMap{Quit: d.Quit, Range: d.Range, Members: d.Members /* … */}
	// SetEnabled mutates the copy in k, so it must run BEFORE short/full are
	// built from those fields — the slices hold value copies, not references.
	k.Members.SetEnabled(m.scope == "team")
	k.short = []key.Binding{k.Range, k.Members, k.Quit}
	k.full = [][]key.Binding{{k.Range}, {k.Members}, {k.Quit}}
	return k
}
```

Two hazards the sketch is written to avoid, and which a copy-paste will otherwise reproduce:

- A binding must be **assigned into `k` before** `SetEnabled` is called on it. `SetEnabled` on a never-assigned field is a no-op: a zero `Binding` has `keys == nil` and stays disabled regardless.
- `short`/`full` hold **value copies**. Build them after every `SetEnabled`, or the footer will show stale enablement.

Take each binding's help text from the inline help string that screen's `view` already renders — the wording exists; reuse it verbatim.

- [ ] **Step 4: Run the Home tests to green**

Run: `go test ./internal/tui -run TestHomeKey -v`
Expected: PASS. If the labels differ from your list, correct the *test* — it must state what `home.go` accepts today, not what you would prefer.

- [ ] **Step 5: Replace the global quit hack**

`app.go:601` currently reads:

```go
	if msg.String() == "q" && m.screen != screenSetup && m.screen != screenRates && m.screen != screenRange && m.screen != screenListBrowser && m.screen != screenLog && m.screen != screenEntries {
```

Replace with:

```go
	if key.Matches(msg, keysFor(m).Quit) {
```

**Reproduce the exclusion set exactly; do not re-derive it from a rule.** `screenListBrowser` is excluded despite having no text input, and `entries` is excluded even in list mode — "screens with text inputs" is *not* the rule. `q` also quits from `screenError` today, before its any-key handler runs.

Until Tasks 3 and 4 add the other screens' constructors, `keysFor` returns a zero `keyMap` for them — which disables `Quit` on exactly the screens that should have it. So this step must also add a **minimal** constructor for each not-yet-migrated screen returning just the correct `Quit` state. Tasks 3 and 4 then fill each one out.

- [ ] **Step 6: Write the quit-parity test over all 14 screens**

```go
// The quit exclusion set is behavior, not policy: reproduce it exactly, so a
// later change to it is a deliberate act with a failing test to update.
func TestQuitBindingPerScreen(t *testing.T) {
	t.Parallel()
	quits := map[screen]bool{
		screenSetup: false, screenHome: true, screenLoading: true,
		screenReport: true, screenExport: true, screenRates: false,
		screenLog: false, screenError: true, screenMembers: true,
		screenRange: false, screenFilters: true, screenListBrowser: false,
		screenBudget: true, screenEntries: false,
	}
	for s, want := range quits {
		m := newTestModel()
		m.screen = s
		if got := keysFor(m).Quit.Enabled(); got != want {
			t.Errorf("screen %v: Quit enabled = %v, want %v", s, got, want)
		}
	}
}
```

Verify each entry against `app.go:601`'s exclusion list before trusting the map above.

- [ ] **Step 7: Verify no handler changed**

Run: `git diff --stat`
Expected: only `keys.go`, `keys_test.go` and `app.go`.

- [ ] **Step 8: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git status --short internal/tui/testdata   # must be empty
git add internal/tui/keys.go internal/tui/keys_test.go internal/tui/app.go
git commit -m "feat(tui): add the keymap registry and declare the quit binding (#59)"
```

---

### Task 3: Group A — nine screens, bindings and handlers together

Each screen gets its `keysFor` constructor, its parity test, and its handler migration **in the same pass**, so a binding definition is never separated from the code it governs.

**Files:**
- Modify: `internal/tui/keys.go`, `keys_test.go`
- Modify: `home.go` (9 cases), `report.go` (9), `filters.go` (8), `members.go` (6), `export.go` (4), `range.go` (4 cases + 3 `case tea.Key…` arms), `listbrowser.go` (4), `setup.go` (3 cases + 3 `msg.Type == tea.Key` sites), `budget.go` (1)
- Modify: the corresponding `_test.go` files only where a test asserts on a key whose representation changes

**Interfaces:**
- Consumes: `keyDefaults`, `keyMap`, `keysFor`, `enabledLabels` from Task 2.
- Produces: a constructor per screen (`reportKeys`, `filtersKeys`, …) wired into `keysFor`'s switch; new `keyDefaults` fields for every action these screens introduce.

- [ ] **Step 1: Migrate `budget.go` first and establish the shape**

One case. Pattern:

```go
func (m Model) updateBudget(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := keysFor(m)
	switch {
	case key.Matches(msg, k.Back), key.Matches(msg, k.Budget):
		m.screen = screenReport
		return m, nil
	}
	return m, nil
}
```

A tagless `switch { case key.Matches(…) }` keeps the arms parallel to the string switch it replaces. Add `budgetKeys`, its parity test, then run `go test ./internal/tui -run TestBudget -v`.

- [ ] **Step 2: Migrate the remaining eight, one at a time**

Order: `export`, `members`, `range`, `listbrowser`, `setup`, `filters`, `home`, `report`. For each: read the handler, write its constructor and parity test, substitute its cases, run that screen's tests. **Do not batch** — a mistake found four screens later is four screens of diff to bisect.

Specifics that are not mechanical:

- **`range.go` and `setup.go` have `tea.Key` dispatch too.** `range.go:51,77,82` are `case tea.KeyEnter:`-style arms inside the editing mode; `setup.go` has three `msg.Type == tea.Key…` comparisons across its wizard steps. These migrate as well — #82 cannot remap `enter`/`esc` inside a form otherwise. The step/mode dispatch stays outermost in both.
- **`range` editing and `setup`'s steps accept a different label set per step.** Each needs its own parity test, not one per screen.
- **`filters.go:133` and `members.go:54` have a dead `"space"` label.** bubbletea maps the space rune to `KeySpace`, whose `String()` is `" "`, so the `"space"` arm never fires. Declare `key.WithKeys(" ")` with `key.WithHelp("space", …)` and drop the dead label — note it in your report so the reviewer knows it was deliberate.
- **`home.go`'s month keys are `case "left", "h"` and `case "right", "l"`.** The `◂`/`▸` characters appear only in the rendered help text, never as key labels.

- [ ] **Step 3: Add the missing targeted transition tests**

These action keys have no test that would fail if they went mute:

- `report.go`: `m` → Home, `s` → Home, `r` → Loading, `e` → Export
- `range.go`: `esc` in **list** mode → Home (`range.go:139`). `esc` in
  **editing** mode does *not* navigate — it closes the custom-date editor and
  stays on `screenRange` with `editing == false` (`range.go:77-81`). Assert
  that, not a screen change. Writing the test the other way round and then
  making it pass would delete the two-step "back to the preset list" behavior:
  an unannounced regression no golden covers.
- `export.go`: `esc` → Report
- `budget.go`: `esc` → Report, and `b` → Report

```go
func TestReportMReturnsHome(t *testing.T) {
	m := newTestModelOnReport()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	if got := next.(Model).screen; got != screenHome {
		t.Errorf("m from report → %v, want screenHome", got)
	}
}
```

**Reach the screen under test by driving keys wherever you can**, as above, rather than hand-setting `m.screen`. Task 5 turns these `screen =` assignments into stack operations, and a test that navigated its own way there keeps passing untouched; a test that hand-seeded a screen will need a `nav` seed added in Task 5. Task 5 Step 7 fixes `newTestModelOnReport` centrally, so building on that helper is safe either way.

- [ ] **Step 4: Add the enablement⇔guard tests for this group**

Contextual gating now lives in two places; assert they agree. For this group: Home's `f` (team scope), Home's `c` (only with a running timer), Home's `left`/`right` (preset gate).

```go
func TestHomeTimerKeyMatchesGuard(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	m.screen = screenHome
	if keysFor(m).Timer.Enabled() {
		t.Error("Timer binding enabled with no running timer")
	}
	m.runningTimer = &clickup.RunningTimer{TaskName: "X"}
	if !keysFor(m).Timer.Enabled() {
		t.Error("Timer binding disabled with a running timer")
	}
}
```

- [ ] **Step 5: Verify parity and gate**

```bash
go test ./internal/tui -run 'Key' -v      # every parity test passes
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git status --short internal/tui/testdata  # must be empty
git add internal/tui
git commit -m "refactor(tui): route the nine smaller screens through the keymap (#59)"
```

---

### Task 4: Group B — log, entries, rates

The three densest handlers, each with several modes.

**Files:**
- Modify: `internal/tui/keys.go`, `keys_test.go`
- Modify: `log.go` (17 cases, 5 `tea.Key` sites), `entries.go` (16 cases, 2 `tea.Key` sites, 2 `case tea.Key…` arms), `rates.go` (16 cases, 1 `tea.Key` site, 2 `case tea.Key…` arms)
- Modify: `app.go` (1 `tea.Key` site)

**Interfaces:**
- Consumes: everything from Tasks 2 and 3.
- Produces: `logKeys`, `entriesKeys`, `ratesKeys` and their per-mode variants, wired into `keysFor`.

- [ ] **Step 1: Migrate `rates.go`**

16 cases, one `msg.Type ==` site, two `case tea.Key…` arms (`rates.go:461,465`). Three specifics:

- The `editing` mode forwards to a `textinput` (`rates.go:459-484`). Key dispatch stays **inside** the existing `if rt.editing { … }` branch.
- **`rates.go:472` is a `KeyRunes` class filter** for numeric fields — it tests the *kind* of key, not which key. It cannot become a binding. **Leave it exactly as it is** and note it in your report as the one documented exception to Step 5's grep. Do *not* "clean it up" into a `len(msg.Runes) > 0` check: bubbletea delivers space as `Key{Type: KeySpace, Runes: []rune{' '}}`, so a runes-presence test would start intercepting space, which the type test lets through to the input today. This tranche changes no behavior here.
- **`ratesModel.updateDraft` (`rates.go:487`) has a `ratesModel` receiver** and cannot call `keysFor(m)`. Pass the keyMap in as a parameter. The draft flow has three steps with different label sets — one parity test per step.

Run `go test ./internal/tui -run TestRates -v` (33 functions — 24 in `rates_test.go`, 9 in `app_test.go`) before moving on.

- [ ] **Step 2: Migrate `entries.go`**

16 cases across five modes (`entriesList`, `entriesConfirmDelete`, `entriesEdit`, `entriesHistory`, `entriesTags`), plus `tagNewMode` and the `editStep` sub-steps, each with its own label set and its own parity test. The mode dispatch stays outermost.

- `entriesConfirmDelete`'s "any other key cancels" (`entries.go:272`) stays a `default` clause — it is the *absence* of a match, which no binding can express.
- The `" "`/`"space"` toggle at `entries.go:234`: declare `WithKeys(" ")`, drop the dead label.

Run `go test ./internal/tui -run 'TestEntries|TestV|TestTag|TestHistory' -v`.

- [ ] **Step 3: Migrate `log.go`**

17 cases plus 5 `tea.Key` sites across the flow's steps, including the `formField` sub-steps.

**Delete `log.go:314-316` rather than migrating it.** That `case "esc"` inside `logTimerRunning` is unreachable: the guard at `log.go:275` catches `esc` for every non-input step first. Migrating it mechanically preserves a latent wrong-destination bug — it returns to Report even when the flow's origin is Home. Remove it with a one-line comment saying the outer guard handles `esc`.

The ID-input and form steps forward to a `textinput`; their dispatch stays inside those branches.

Run `go test ./internal/tui -run 'TestLog|TestTimer' -v`.

- [ ] **Step 4: Migrate `app.go`'s remaining `tea.Key` site and add the missing tests**

Transition tests still missing after Task 3: entries-list `esc` → Report, and logDone `r` → Loading.

Enablement⇔guard tests for this group: entries `e`/`x`/`t` (ownership via `canEdit`), entries `h` (non-empty list, *not* ownership-gated), rates `c`/`g` (Lists section, non-empty), rates `n` (Overrides section).

```go
func TestEntriesEditKeyMatchesOwnershipGuard(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		userID int
		want   bool
	}{
		{"own entry", 1, true},
		{"someone else's entry", 99, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel()
			m.screen = screenEntries
			m.userID = tc.userID
			m.entries = []report.TimeEntry{{ID: "e1", UserID: 1, TaskName: "X"}}
			m.entriesScreen = entriesModel{entries: m.entries}
			if got := keysFor(m).Edit.Enabled(); got != tc.want {
				t.Errorf("Edit binding enabled = %v, want %v", got, tc.want)
			}
			if got := canEdit(m.entries[0], m.userID); got != tc.want {
				t.Errorf("canEdit = %v, want %v — keymap and guard disagree", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 5: Verify no string key dispatch survives**

```bash
grep -rnE 'case "[a-z0-9]+"|msg\.String\(\) ==|msg\.Type == tea\.Key[A-Z]|case tea\.Key[A-Z]' internal/tui/*.go | grep -v _test | grep -v demo.go
```
**Expected: exactly one line** — `rates.go`'s numeric class filter
(`msg.Type == tea.KeyRunes`, Step 1). It is a key-*class* test, not a key
comparison, and it stays. Any other line is a missed migration.

Two details of the pattern are deliberate:

- The `[a-z0-9]` class matters — `log.go` has `case "1"`, `"2"`, `"3"`.
- The trailing `[A-Z]` on the `tea.Key` alternatives excludes `app.go:600`'s
  `case tea.KeyMsg:`. That is `Update`'s message-type arm, not a key
  comparison; it stays forever, and a pattern that matches it can never
  return nothing.

- [ ] **Step 6: Verify parity, gate, commit**

```bash
go test ./internal/tui -run 'Key' -v
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git status --short internal/tui/testdata   # must be empty
git add internal/tui
git commit -m "refactor(tui): route log, entries and rates through the keymap (#59)"
```

---

### Task 5: The navigation stack

The mechanism, and the routing of every existing transition through it. **No user-visible behavior changes here** — every flow must end on exactly the screen it ends on today. The one intended behavior change (Report's `esc`) is Task 6.

**Files:**
- Create: `internal/tui/nav.go`, `internal/tui/nav_test.go`
- Modify: all 13 files holding a transition; `log.go` and `listbrowser.go` also lose their `origin` fields
- Modify tests: `range_test.go`, `members_test.go`, `filters_test.go`, `listbrowser_test.go`, `rates_test.go`, `log_test.go`, `timer_ui_test.go`, `app_test.go`, `golden_test.go`

**Interfaces:**
- Consumes: everything from Tasks 2–4.
- Produces:
  - `Model.nav []screen` — the **parent chain only**. The current screen is `m.screen` and is *not* the top of `nav`. Empty `nav` means "nowhere to go back to".
  - `func (m Model) goTo(s screen) Model` — forward navigation, **truncating**: if `s` is already in `nav`, truncate above it rather than appending.
  - `func (m Model) replace(s screen) Model` — same logical step, different screen. `nav` unchanged.
  - `func (m Model) pop() Model` — return to the parent; no-op on an empty chain.
  - `func (m Model) resetTo(s screen) Model` — clear the chain and go to `s`.

- [ ] **Step 1: Write the failing invariant tests**

Create `internal/tui/nav_test.go` with: `goTo` pushes the parent; `pop` returns to it and empties correctly; `pop` on an empty chain is a no-op; `replace` leaves the chain untouched and a later `pop` still reaches the parent; and the truncation invariant:

```go
// Truncation is what bounds the stack structurally, rather than relying on
// anyone remembering to clear it at the right moment.
func TestNavGoToTruncatesInsteadOfDuplicating(t *testing.T) {
	t.Parallel()
	m := newTestModel().resetTo(screenHome).goTo(screenReport).goTo(screenLog)
	for range 5 {
		m = m.goTo(screenReport)
	}
	if m.screen != screenReport {
		t.Fatalf("screen = %v, want screenReport", m.screen)
	}
	if len(m.nav) != 1 || m.nav[0] != screenHome {
		t.Errorf("nav = %v, want [home] — the chain grew or kept a duplicate", m.nav)
	}
}
```

- [ ] **Step 2: Run and watch them fail**

Run: `go test ./internal/tui -run TestNav -v`
Expected: build failure — `undefined: goTo`, `replace`, `pop`, `resetTo`, `m.nav`.

- [ ] **Step 3: Write nav.go**

```go
package tui

import "slices"

// goTo navigates forward to s, remembering the current screen as its parent.
//
// The push truncates: if s is already in the chain, everything above it is
// dropped rather than appended to, so nav can never hold a duplicate and its
// depth is bounded by the number of screens. This is a structural invariant,
// not a rule anyone has to remember to apply.
func (m Model) goTo(s screen) Model {
	if i := slices.Index(m.nav, s); i >= 0 {
		m.nav = m.nav[:i]
		m.screen = s
		return m
	}
	if m.screen == s {
		return m
	}
	m.nav = append(slices.Clone(m.nav), m.screen)
	m.screen = s
	return m
}

// replace swaps the current screen without touching the parent chain: the
// loading and error screens, and the async handlers that land on a result
// screen, are the same logical step as whatever launched them.
func (m Model) replace(s screen) Model {
	m.screen = s
	return m
}

// pop returns to the parent screen. On an empty chain it is a no-op, which is
// exactly what Home needs — no special case.
func (m Model) pop() Model {
	if len(m.nav) == 0 {
		return m
	}
	m.screen = m.nav[len(m.nav)-1]
	m.nav = m.nav[:len(m.nav)-1]
	return m
}

// resetTo clears the parent chain: Home, and the setup wizard relaunched by a
// 401.
func (m Model) resetTo(s screen) Model {
	m.nav = nil
	m.screen = s
	return m
}
```

`slices.Clone` on push is load-bearing: `Model` is a value receiver, so two Models can otherwise share a backing array and one's append overwrites the other's entry.

- [ ] **Step 4: Run the invariant tests to green**

Run: `go test ./internal/tui -run TestNav -v` → PASS.

- [ ] **Step 5: Route all 66 transition sites**

**This table is the classification — do not re-derive it.** Line numbers are pre-migration; match on surrounding context, not on the number.

| Site | Context | Call |
|---|---|---|
| `app.go:180`, `:183` | `New()` initial screen | `resetTo` |
| `app.go:301`, `:317`, `:616`, `:632` | error paths → `screenError` | `replace` |
| `app.go:523` | `openListBrowser` | `goTo` |
| `app.go:613`, `:623` | 401 → setup wizard relaunch | `resetTo` |
| `app.go:630` | `retryableErrMsg`, origin Home | `resetTo(screenHome)` |
| `app.go:641` | `logErrMsg` → stay on Log | `replace` |
| **`app.go:667`** | **`entriesMsg` → Report** | **`resetTo(screenHome).goTo(screenReport)`** — see below |
| `app.go:689`, `:698`, `:706` | `entriesReloadedMsg` / `entriesErrMsg` / `historyMsg` → Entries | `replace` |
| `app.go:728`, `:737`, `:765` | `logDoneMsg` / `timerStoppedMsg` / `timerMsg` → Log | `replace` |
| `app.go:777` | `membersMsg` → Members | `replace` |
| `app.go:789` | `statusesMsg` → Filters | `replace` |
| `app.go:857`, `:860` | `screenError` any-key exit | `resetTo` |
| `budget.go:22` | `esc` / `b` | `pop` |
| `entries.go:134` | `openEntries` | `goTo` |
| `entries.go:144` | `esc` | `pop` |
| `entries.go:167`, `:255`, `:270`, `:399` | → Loading | `replace` |
| `export.go:52` | `esc` | `pop` |
| `filters.go:161` | apply | `pop` |
| `filters.go:165` | `esc` | `pop` |
| `home.go:57` | `d` → Range | `goTo` |
| `home.go:67`, `:73` | `n` / `c` → Log | `goTo` |
| `home.go:81`, `:85` | `f` → Members (two exclusive paths in one case) | `goTo` |
| `home.go:92` | `enter` → Loading | `replace` |
| `listbrowser.go:78` | `= bs.origin` (esc at top level) | `pop` |
| `listbrowser.go:151`, `:157` | list chosen → Rates / Log | `pop` |
| `log.go:276`, `:354`, `:483` | `= lg.origin` | `pop` |
| `log.go:312`, `:339`, `:371`, `:462`, `:478` | → Loading | `replace` |
| `log.go:315` | unreachable `esc` | **deleted in Task 4** |
| `members.go:67` | confirm | `pop` |
| `members.go:70` | `esc` — the early `return` that discards `mm` stays | `pop` |
| `range.go:75`, `:136` | pick / confirm | `pop` |
| `range.go:139` | `esc` | `pop` |
| `rates.go:451` | `esc` | `pop` |
| `rates.go:870` | save | `pop` |
| `report.go:72` | `m` / `s` → Home (one case clause, both keys) | `pop` |
| `report.go:74` | `r` → Loading | `replace` |
| `report.go:78`, `:81`, `:84`, `:137` | `e` / `p` / `n` / `b` | `goTo` |
| `report.go:90`, `:94` | `f` → Filters | `goTo` |
| `setup.go:126` | wizard done → Home | `resetTo` |

**Why `app.go:667` re-roots instead of replacing.** Report is reached three ways — Home `enter`, Report `r`, and the logDone `r` reload — and all three arrive through `entriesMsg`. A plain `replace` would leave the chain empty on the Home path, making Report's own `m`/`s`/`esc` no-ops on the app's most common flow; and on the logDone path it would leave `[Home, Report]` with `screen == Report`, so the first `esc` returns Report to itself — a visible dead key. Re-rooting makes every arrival converge on `nav == [Home]`, which is exactly today's semantics: Report's back target is unconditionally Home.

- [ ] **Step 6: Replace both `origin` fields**

`logModel.origin` (`log.go:52`) and `listBrowserModel.origin` are now redundant for navigation: `pop()` returns to whichever screen pushed. Delete both fields, `newLog`'s third parameter, `openListBrowser`'s `origin` parameter, and every read.

**But `listBrowserModel.origin` is not only navigation state.** `selectBrowsedList` (`listbrowser.go:135`) branches on `origin == screenRates` to decide **which sub-model receives the chosen list** — a rates row, or the log task-pick flow. Deleting the field without a replacement breaks that routing. Use the parent at the top of the chain as the discriminator, read *before* popping:

```go
	if len(m.nav) > 0 && m.nav[len(m.nav)-1] == screenRates {
```

- [ ] **Step 7: Fix the test fixtures that hand-seed a screen**

Many tests construct a Model with a screen set directly and an empty `nav`, so a `pop()` is a no-op and their assertions fail. **This is expected and is the fixtures' problem, not the code's.**

**Start with the one-line fix that covers the most call sites.** `newTestModelOnReport()` (`log_test.go:35`) hand-sets `m.screen = screenReport` on a fresh `New(cfg)`, leaving `nav` empty. It has ~18 call sites in `log_test.go` alone, and every transition test Tasks 3 and 4 added is built on it. Report is only ever reached from Home, so add to the helper:

```go
	m.nav = []screen{screenHome}
```

That single line fixes the whole Report-rooted family at once, including `TestReportMReturnsHome`, the entries-list `esc` test, and the budget/export/`logDone` tests.

Then seed the remaining hand-seeded fixtures with the parent chain each one implies:

| File | What to seed | Chain |
|---|---|---|
| `range_test.go` | every `Model{screen: screenRange, …}` literal (there are 8; the ones that assert `screen == screenHome` after a commit are `TestRangeSelectPreset` and `TestRangeCustomValidDates`, but seed them all — it costs nothing and stops the next test from tripping) | `nav: []screen{screenHome}` |
| `members_test.go` | the members fixture used by the toggle/confirm/esc tests | `nav: []screen{screenHome}` |
| `filters_test.go` | the `Model{screen: screenFilters, …}` fixtures (the apply and esc paths both `pop`) | `nav: []screen{screenHome, screenReport}` |
| `listbrowser_test.go` | every `Model{screen: screenListBrowser}` literal | `nav: []screen{screenHome, screenReport, screenLog}`, or `…, screenRates` where the test exercises the rates path — Step 7's discriminator reads the top of this chain, so the wrong one silently routes the picked list to the wrong sub-model |

**Line numbers are deliberately absent from that table: they have already drifted once.** Find the fixtures with

```bash
grep -rn 'Model{screen:\|m.screen = screen' internal/tui/*_test.go
```

and work through the list. A fixture whose assertions never depend on a `pop()` needs no seed, but seeding it is harmless.

Tests that reach their screen by driving keys — `TestLogEscReturnsToReport`, `TestRatesScreenEscDiscardsAndReturns` and the like — need no change: they build the chain by navigating. **This is the pattern to prefer when writing new tests.**

Separately, these call sites lose the `origin` argument or assertion and must be updated to compile: `rates_test.go:19,40`, `log_test.go:402`, `listbrowser_test.go:95,117,134`, `timer_ui_test.go:34`, `app_test.go:1206`, `golden_test.go:165,207`.

- [ ] **Step 8: Verify no naked assignment survives**

```bash
grep -rnE '\.screen = ' internal/tui/*.go | grep -v _test | grep -v nav.go
```
Expected: no output. A surviving assignment silently desyncs `nav`, and a value-receiver Model makes that invisible at runtime.

- [ ] **Step 9: Trace the four flows by hand**

Before the gate, walk these against the code and confirm `nav` at each step:

1. Home → `enter` → Loading → `entriesMsg` → Report → `v` → Entries → `e` → edit → save → Loading → `entriesReloadedMsg` → Entries → `esc`. Expect Report, with `nav == [Home]`.
2. Home → `n` → Log → start timer → stop → `esc`. Expect Home.
3. Any screen → 401 → setup wizard → done. Expect Home with `nav` empty.
4. Report → `p` → Rates → `b` → ListBrowser → pick a list → Rates → `esc`. Expect Report, and the picked list must land in the **rates** row, not the log flow.

Record the observed chains in your report.

- [ ] **Step 10: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git status --short internal/tui/testdata   # must be empty
git add internal/tui
git commit -m "refactor(tui): replace ad-hoc screen assignments with a navigation stack (#59)"
```

---

### Task 6: Report's `esc`, and three staleness guards

Task 5 was routing only — no behavior moved. This task is the tranche's **one intended behavior change** plus the three defensive guards the stack makes worth having. Both are small and additive, and they are split out from Task 5 so they arrive as a diff a reviewer can hold in their head rather than as two paragraphs buried in a 13-file refactor.

**Files:**
- Modify: `internal/tui/keys.go`, `report.go`, `app.go`
- Modify tests: `internal/tui/keys_test.go`, `report_test.go` (or wherever Report's parity test landed in Task 3)

**Interfaces:**
- Consumes: `keysFor`, `pop` from Tasks 2–5.
- Produces: nothing new for later tasks; Task 7 documents the `esc`.

- [ ] **Step 1: Give Report its `esc`**

Nothing in Task 5 produces this — Report has no `esc` site to classify, so the classification table could not create one. Add all four pieces:

1. `Back` to `reportKeys` in `keys.go`
2. a `case key.Matches(msg, k.Back):` arm in `updateReport` calling `pop()`
3. `esc` added to Report's parity test's expected label set
4. a transition test:

```go
func TestReportEscReturnsHome(t *testing.T) {
	m := newTestModelOnReport() // Task 5 seeded its nav with [screenHome]
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if got := next.(Model).screen; got != screenHome {
		t.Errorf("esc from report → %v, want screenHome", got)
	}
}
```

Report's rendered help line must **not** change: goldens stay byte-identical. `esc` becomes discoverable with B2's generated footer.

- [ ] **Step 2: Add the staleness guards**

`membersMsg` (`app.go:768`) and `statusesMsg` (`app.go:780`) force a screen with no staleness check, unlike `tagsMsg` (`app.go:710`) and `spacesMsg` (`app.go:794`). Add the same guard shape those two use.

**`historyMsg` (`app.go:701`) needs a different guard.** `tagsMsg`'s works because its fetch is dispatched while staying on `screenEntries`; `h` instead dispatches from `screenLoading` (`entries.go:167`). A `if m.screen != screenEntries` guard there would drop **every** history reply and strand the user on Loading, which swallows all keys but quit. The history guard must accept `screenLoading`.

Other async handlers force a screen unguarded as well (`logErrMsg`, `logDoneMsg`, `timerStoppedMsg`, `entriesReloadedMsg`, `entriesErrMsg`). Leave them: each can only arrive from a Loading state that swallows keys, so no competing navigation can have happened in between. **Do not widen the scope**, and do not read the three above as an exhaustive audit.

- [ ] **Step 3: Prove the history guard does not break `h`**

Run: `go test ./internal/tui -run 'TestHistory|TestEntries' -v`
Expected: PASS. If a history test now hangs on Loading, the guard rejected `screenLoading` — that is exactly the failure this step exists to catch.

- [ ] **Step 4: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git status --short internal/tui/testdata   # must be empty
git add internal/tui
git commit -m "feat(tui): accept esc on the report screen and guard three stale async replies (#59)"
```

---

### Task 7: Documentation

**Files:** `CHANGELOG.md`, `README.md`, `README.it.md`

- [ ] **Step 1: Correct the README key tables**

Both READMEs carry a key table. Verify **every row** against `keys.go`, which is now the single source of truth.

**The `q` row is already correct** — `README.md:142` and `README.it.md:145` both list the full exclusion set (setup / rates / range / list browser / log hours / time entries), with an explanatory paragraph below. Do not "fix" it. The one row missing is Report's new `esc`.

- [ ] **Step 2: CHANGELOG**

Add under `## [Unreleased]`, in a `### Changed` section:

```markdown
- The Report screen now accepts `esc` to go back, like every other screen.
  Navigation is handled by a single back-stack, so `esc` always returns to
  wherever you came from — including when the same screen is reachable from
  more than one place.
```

- [ ] **Step 3: README.it.md**

The same row and entry, in fluent native Italian with correct accents (à, è, é, ì, ò, ù) — never ASCII substitutes.

- [ ] **Step 4: Verify the tables against the code**

For each screen in the table, confirm the listed keys match that screen's constructor in `keys.go`. A key table that drifts from the keymap is worse than none — it is the document a user trusts.

- [ ] **Step 5: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add CHANGELOG.md README.md README.it.md
git commit -m "docs: document the back-stack navigation and correct the key tables"
```

---

## Definition of done

- `grep -rnE 'case "[a-z0-9]+"|msg\.String\(\) ==|msg\.Type == tea\.Key[A-Z]|case tea\.Key[A-Z]' internal/tui/*.go` (excluding tests and `demo.go`) returns **exactly one line**: `rates.go`'s numeric class filter. The `[A-Z]` suffix is what keeps `app.go`'s `case tea.KeyMsg:` — `Update`'s message-type arm, which must survive — out of the pattern.
- `grep -rnE '\.screen = ' internal/tui/*.go` (excluding tests and `nav.go`) returns nothing.
- Neither `logModel` nor `listBrowserModel` has an `origin` field, and `selectBrowsedList` routes by the parent chain.
- Report accepts `esc`, with a transition test proving it.
- Every golden in `internal/tui/testdata/` is byte-identical to Task 1's state.
- `go test ./... -race` passes.
- #131 closes; #59 keeps its overlay-compositor item for B2.
