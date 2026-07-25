# Keymap registry and navigation stack (v1.9 tranche B1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace 97 string `case` clauses and 62 scattered screen assignments with one declared keymap per screen and one navigation mechanism, so the footer (#69), the command palette (#71) and configurable keybindings (#82) each have a single place to plug into.

**Architecture:** `keys.go` holds a screen-independent defaults table plus a pure `keysFor(m Model) keyMap` that contextually enables bindings and carries its own help ordering. `nav.go` holds a `nav []screen` parent chain on the `Model`, reached only through `goTo`/`replace`/`pop`/`resetTo`, with a truncating push that makes unbounded growth impossible by construction. Keymap lands before navigation so the mechanical migration is provable against unchanged navigation semantics.

**Tech Stack:** Go 1.26, bubbletea v1.3.10, bubbles v1.0.0 (`bubbles/key` — already a direct dependency), lipgloss v1.1.0.

## Global Constraints

- `internal/report` and `internal/duration` stay **pure** — untouched by this plan.
- **No new `go.mod` dependencies.**
- bubbletea value receivers; explicit write-back (`m.sub = x`) before every return.
- Everything in the repo is in **ENGLISH** except `README.it.md` and `CONTRIBUTING.it.md`.
- **Conventional Commits.** **Never** add a `Co-Authored-By` trailer.
- Pre-commit gate, all clean/green, every task: `gofmt -l .`, `go vet ./...`, `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`, `go test ./... -race`.
- **All goldens in `internal/tui/testdata/` stay byte-identical from Task 1 onward.** This tranche changes input and navigation, never layout. A moved golden is a bug — never run `-update` to silence one after Task 1.
- **Goldens are not this tranche's safety net.** They verify rendering, not input; a wrong `key.Matches` makes a key silently mute and no golden notices. The net is the per-screen label-parity test (Task 2), the targeted transition tests (Task 2), and the review rule that every removed `case` label must reappear verbatim in a `WithKeys`.
- Demo mode needs no parity work: `demo.go` holds fixtures and commands only, no key handlers and no screen transitions.

---

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `internal/tui/golden_test.go`, `testdata/*.golden` (modify/create) | Deeper capture: log form, entries edit form, `screenError`; `t.Run` subtests | 1 |
| `internal/tui/keys.go` (create) | Defaults table, `keyMap` type, `keysFor(m Model) keyMap`, `help.KeyMap` implementation | 2 |
| `internal/tui/keys_test.go` (create) | Label-parity tests per screen, enablement⇔guard tests | 2 |
| `internal/tui/app.go` (modify) | Global key routing; the quit hack removed | 2 |
| 9 small handlers (modify) | `home`, `report`, `filters`, `members`, `export`, `range`, `listbrowser`, `setup`, `budget` — 48 cases | 3 |
| 3 large handlers (modify) | `log` 17, `entries` 16, `rates` 16, plus the 12 `tea.Key…` sites | 4 |
| `internal/tui/nav.go` (create) | `nav []screen`, `goTo`/`replace`/`pop`/`resetTo`, truncating push | 5 |
| All 13 files with transitions (modify) | 62 assignment sites routed through the API; both `origin` fields deleted | 5 |
| `CHANGELOG.md`, `README.md`, `README.it.md` (modify) | Key table corrected; Report's new `esc` announced | 6 |

---

### Task 1: Deeper goldens (#131)

Captures the screen states this tranche's later tasks touch, before they are touched. No production file may change.

**Files:**
- Modify: `internal/tui/golden_test.go`
- Create: 5 new files in `internal/tui/testdata/`

**Interfaces:**
- Consumes: `golden`, `goldenModel`, `goldenReport`, `goldenEntries`, `testTheme` (all in `golden_test.go` from v1.9 tranche A).
- Produces: goldens named `log_form`, `entries_edit`, `entries_confirm_delete`, `error`, `loading`.

- [ ] **Step 1: Convert `TestGoldenRatesTabs` to subtests**

In `internal/tui/golden_test.go`, wrap the loop body:

```go
	for _, tc := range []struct {
		name string
		sec  ratesSection
	}{
		{"rates_lists", secLists},
		{"rates_members", secMembers},
		{"rates_overrides", secOverrides},
		{"rates_rules", secRules},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt := newRates(goldenEntries(), cfg)
			rt.sec = tc.sec
			golden(t, tc.name, rt.view(testTheme(true)))
		})
	}
```

Without this, `golden`'s `t.Fatalf` on a missing file aborts the remaining tabs.

- [ ] **Step 2: Write the new golden tests**

Append to `internal/tui/golden_test.go`. Read `internal/tui/log.go`, `entries.go` and `app.go` first to find the exact field names and step constants each state needs — the constants below are the ones the files use today, but verify each against the source before relying on it.

```go
// The log form is the state a user spends the most time in and the one B2's
// footer will change first.
func TestGoldenLogForm(t *testing.T) {
	t.Parallel()
	lg := newLog(goldenEntries(), config.Config{Token: "t", WorkspaceID: "team1"}, screenReport)
	lg.now = goldenFixedTime
	lg.step = logForm
	lg.taskID = "t1"
	golden(t, "log_form", lg.view(testTheme(true)))
}

func TestGoldenEntriesEdit(t *testing.T) {
	t.Parallel()
	m := goldenModel()
	m.userID = 1
	m.entries = goldenEntries()
	m.screen = screenReport
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	golden(t, "entries_edit", next.(Model).View())
}

func TestGoldenEntriesConfirmDelete(t *testing.T) {
	t.Parallel()
	m := goldenModel()
	m.userID = 1
	m.entries = goldenEntries()
	m.screen = screenReport
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	golden(t, "entries_confirm_delete", next.(Model).View())
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

Add `"errors"` to the imports.

If the entries edit or confirm-delete state cannot be reached by those two key presses (ownership gating requires `m.userID` to match the fixture's `UserID`, which `goldenEntries()`'s first entry sets to 1), read `entries.go`'s handler and drive whatever sequence does reach it. Do not weaken the test to a state that is easier to reach.

- [ ] **Step 3: Run and watch them fail**

Run: `go test ./internal/tui -run 'TestGoldenLogForm|TestGoldenEntriesEdit|TestGoldenEntriesConfirmDelete|TestGoldenError|TestGoldenLoading' -v`
Expected: all five FAIL with `no such file or directory`.

- [ ] **Step 4: Generate and inspect**

```bash
go test ./internal/tui -run 'TestGoldenLogForm|TestGoldenEntriesEdit|TestGoldenEntriesConfirmDelete|TestGoldenError|TestGoldenLoading' -update
```

Read all five files. Each must show the state its name claims — an `entries_edit.golden` that renders the browser list means the key sequence did not reach the edit form, and the test is worthless. Check for machine-dependent content: `grep -l $'\x1b' testdata/*.golden` must still list only the two `palette_*` files.

- [ ] **Step 5: Verify stability and that nothing else moved**

```bash
go test ./internal/tui -run TestGolden -count=2
git status --short internal/tui/testdata
```
Expected: PASS, and `git status` lists only the five new files. If a pre-existing golden moved, something other than this task changed.

- [ ] **Step 6: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git status --short   # must list only golden_test.go and testdata/
git add internal/tui/golden_test.go internal/tui/testdata
git commit -m "test(tui): capture the log form, entries edit and error screens as goldens (#131)"
```

---

### Task 2: The keymap registry and the global keys

**Files:**
- Create: `internal/tui/keys.go`, `internal/tui/keys_test.go`
- Modify: `internal/tui/app.go` (global key routing only — the twelve handlers migrate in Tasks 3 and 4)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces:
  - `type keyMap struct { … key.Binding fields …; short []key.Binding; full [][]key.Binding }`
  - `func (k keyMap) ShortHelp() []key.Binding` / `func (k keyMap) FullHelp() [][]key.Binding` — satisfies `bubbles/help`.KeyMap for B2.
  - `func defaultKeys() keyDefaults` — the screen-independent binding table (#82 will override entries here).
  - `func keysFor(m Model) keyMap` — the pure entry point every consumer uses.

- [ ] **Step 1: Write the failing parity test for one screen**

Create `internal/tui/keys_test.go`. Start with Home alone; the remaining screens are added in Step 5 once the shape is proven.

```go
package tui

import (
	"slices"
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

// enabledLabels is every key label the screen currently accepts, sorted and
// deduplicated — the contract the migration must preserve exactly.
func enabledLabels(k keyMap) []string {
	var out []string
	for _, b := range allBindings(k) {
		if !b.Enabled() {
			continue
		}
		out = append(out, b.Keys()...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// The label set Home accepts must match, exactly, the labels its handler
// accepted before the migration. This is what catches a transcription error —
// the actual failure mode of a mechanical migration.
func TestHomeKeyLabels(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	m.screen = screenHome
	m.scope = "me"
	want := []string{"d", "enter", "left", "n", "q", "right", "t", "w", "◂", "▸"}
	if got := enabledLabels(keysFor(m)); !slices.Equal(got, want) {
		t.Errorf("home labels (me scope) = %v, want %v", got, want)
	}
}

// f is contextual: team scope only.
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

The `want` list above is a **starting hypothesis**: derive the real one by reading `home.go`'s `case` clauses before running the test, and correct it. Getting this list right *is* the task — it is the contract the migration must preserve. Note `◂`/`▸` are the actual runes `home.go` matches; keep them.

You will need `allBindings(k keyMap) []key.Binding` — add it in `keys.go` as a method or helper that returns every binding field, so the parity test does not have to know the struct's field list.

- [ ] **Step 2: Run and watch it fail**

Run: `go test ./internal/tui -run TestHomeKey -v`
Expected: build failure — `undefined: keysFor`, `undefined: keyMap`.

- [ ] **Step 3: Write keys.go**

Create `internal/tui/keys.go`. The structure below is the contract; fill the remaining screens' constructors in Step 5.

```go
package tui

import "github.com/charmbracelet/bubbles/key"

// keyDefaults is the screen-independent binding table. Every binding the TUI
// knows is defined exactly once here, with its help text. User-configurable
// keybindings (#82) will override entries in this table and nothing else.
type keyDefaults struct {
	Quit  key.Binding
	Back  key.Binding
	Range key.Binding
	// … one field per distinct action …
}

func defaultKeys() keyDefaults {
	return keyDefaults{
		Quit:  key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		Back:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Range: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "range")),
		// …
	}
}

// keyMap is one screen's active bindings, plus the order the footer (#69) and
// the ? overlay will render them in. A zero Binding is disabled and matches
// nothing — that is how contextual keys are switched off.
type keyMap struct {
	Quit, Back, Range /* … */ key.Binding

	short []key.Binding   // footer order for this screen
	full  [][]key.Binding // grouped, for the ? overlay
}

func (k keyMap) ShortHelp() []key.Binding  { return k.short }
func (k keyMap) FullHelp() [][]key.Binding { return k.full }

// keysFor returns the bindings the given Model state accepts right now.
// It is pure: no state is cached, so there is no second copy of the truth to
// keep in sync. Contextual gating lives here AND in the handlers' guards —
// once a binding is disabled, key.Matches fails and the guard becomes
// unreachable, so enablement is load-bearing for behavior, not just display.
func keysFor(m Model) keyMap {
	d := defaultKeys()
	switch m.screen {
	case screenHome:
		return homeKeys(m, d)
	// … one case per screen …
	}
	return keyMap{}
}

func homeKeys(m Model, d keyDefaults) keyMap {
	k := keyMap{Quit: d.Quit, Range: d.Range /* … */}
	k.Members.SetEnabled(m.scope == "team")
	k.short = []key.Binding{k.Range, k.Scope, k.Report, k.Log, k.Quit}
	k.full = [][]key.Binding{{k.Range, k.Week, k.PrevMonth, k.NextMonth}, {k.Scope, k.Members}, {k.Report, k.Log, k.Timer}, {k.Quit}}
	return k
}
```

Derive each screen's help text from the inline help strings that screen's `view` already renders — the wording exists, reuse it verbatim.

- [ ] **Step 4: Run the Home tests to green**

Run: `go test ./internal/tui -run TestHomeKey -v`
Expected: PASS. If the label list differs from your hypothesis, the *test* is what to correct — it must state what `home.go` actually accepts today.

- [ ] **Step 5: Fill in the remaining eleven screens and their parity tests**

One constructor and one parity test per screen: `report`, `export`, `rates`, `log`, `members`, `range`, `filters`, `listbrowser`, `budget`, `entries`, `setup`.

For each, read that file's `case` clauses and its `msg.Type == tea.Key…` comparisons, and enumerate every label it accepts. Three rules while enumerating:

- **`"space"` is a dead label.** bubbletea maps the space rune to `KeySpace`, whose `String()` is `" "`, so the `"space"` arm in `filters.go:133`, `members.go:54` and `entries.go:234` never fires. Declare `key.WithKeys(" ")` with `key.WithHelp("space", …)` and leave the dead label out — record it in your report so the reviewer knows it was dropped deliberately.
- **Screens with modes or steps accept different labels per mode.** `keysFor` must take that into account (`entriesModel.mode`, `logModel.step`, `ratesModel.editing`, `rangeModel` editing, `setupModel` step). A parity test per mode, not just per screen.
- **Input modes disable action bindings.** In any step that forwards unmatched keys to a `textinput`, only the navigation/confirm bindings stay enabled.

- [ ] **Step 6: Write the enablement⇔guard test**

Contextual gating now lives in two places. Assert they agree:

```go
// Once keysFor disables a binding, key.Matches fails and the handler's guard
// is unreachable — so the two must never disagree about the same condition.
func TestEntriesEditKeyMatchesOwnershipGuard(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name string; userID int; want bool }{
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
				t.Errorf("canEdit = %v, want %v — guard and keymap disagree", got, tc.want)
			}
		})
	}
}
```

Do the same for Home's team-scope `f`.

- [ ] **Step 7: Replace the global quit hack**

`app.go:601` currently reads:

```go
	if msg.String() == "q" && m.screen != screenSetup && m.screen != screenRates && m.screen != screenRange && m.screen != screenListBrowser && m.screen != screenLog && m.screen != screenEntries {
```

Replace it with a check against the declared binding:

```go
	if key.Matches(msg, keysFor(m).Quit) {
```

**The exclusion set must be reproduced exactly**, not re-derived from a rule. `screenListBrowser` is excluded despite having no text input, and `entries` is excluded even in list mode; "screens with text inputs" is *not* the rule. `q` also currently quits from `screenError` before its any-key handler runs — preserve that. Write a test asserting `Quit.Enabled()` per screen against the current six exclusions, so a later change to the set is a deliberate act.

This is also where B2's `?` and B3's `ctrl+p` will hook in — the pre-route global check. Leave the structure ready for that without adding either binding now.

- [ ] **Step 8: Verify no handler changed yet**

Run: `git diff --stat`
Expected: only `keys.go`, `keys_test.go` and `app.go` appear. The twelve handlers still use their string cases — they migrate in Tasks 3 and 4.

- [ ] **Step 9: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git status --short internal/tui/testdata   # must be empty
git add internal/tui/keys.go internal/tui/keys_test.go internal/tui/app.go
git commit -m "feat(tui): add the keymap registry and declare the quit binding (#59)"
```

---

### Task 3: Handler group A — nine screens, 48 cases

**Files:**
- Modify: `internal/tui/home.go` (9), `report.go` (9), `filters.go` (8), `members.go` (6), `export.go` (4), `range.go` (4), `listbrowser.go` (4), `setup.go` (3), `budget.go` (1)
- Modify: the corresponding `_test.go` files only where a test asserts on a key that changes representation

**Interfaces:**
- Consumes: `keysFor(m Model) keyMap` and the per-screen constructors from Task 2.
- Produces: no new exported surface. Every `case "x":` in these nine files becomes an `if key.Matches(msg, k.X)` arm.

- [ ] **Step 1: Migrate one screen and establish the shape**

Start with `budget.go` (one case). Pattern:

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

`switch { case key.Matches(…) }` — a tagless switch — keeps the arms visually parallel to the string switch it replaces and reads better than a chain of ifs.

- [ ] **Step 2: Verify budget still works**

Run: `go test ./internal/tui -run 'TestBudget' -v`
Expected: PASS, unchanged.

- [ ] **Step 3: Migrate the remaining eight screens, one at a time**

For each screen, in this order — `export`, `members`, `range`, `listbrowser`, `setup`, `filters`, `home`, `report` — do all of: read the handler, substitute its cases, run that screen's tests, and only then move to the next. Do not batch them; a mistake found four screens later is four screens of diff to bisect.

**Preserve each handler's structure exactly.** Screens with steps or modes (`range` editing, `setup`'s three steps) dispatch on the step *first* and only then on the key. Keep that nesting. If `key.Matches` is hoisted above the step dispatch, typed characters start triggering actions.

**Two patterns stay as `default` clauses and must not become bindings:** `screenError`'s "press any key" (`app.go:855`) — not in this task's files, but do not touch it — and any "any other key cancels" arm you encounter.

- [ ] **Step 4: Verify label parity**

Run: `go test ./internal/tui -run 'Key' -v`
Expected: every parity test from Task 2 still PASSes. These tests encode what each screen accepted *before* this task; if one fails, the migration dropped or altered a label.

- [ ] **Step 5: Add the missing targeted transition tests**

These action keys have no test that would fail if they went mute. Add one transition test each, asserting the resulting `screen`:

- `report.go`: `m` → Home, `s` → Home, `r` → Loading, `e` → Export
- `range.go`: `esc` (list mode and editing mode) → Home
- `export.go`: `esc` → Report
- `budget.go`: `esc` → Report and `b` → Report

Example:

```go
func TestReportMReturnsHome(t *testing.T) {
	m := newTestModelOnReport()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	if got := next.(Model).screen; got != screenHome {
		t.Errorf("m from report → %v, want screenHome", got)
	}
}
```

- [ ] **Step 6: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git status --short internal/tui/testdata   # must be empty
git add internal/tui
git commit -m "refactor(tui): route the nine smaller screens through the keymap (#59)"
```

---

### Task 4: Handler group B — log, entries, rates, and the tea.Key sites

The three densest handlers, and the twelve `msg.Type == tea.Key…` comparisons that #82 cannot remap otherwise.

**Files:**
- Modify: `internal/tui/log.go` (17 cases, 5 `tea.Key` sites), `entries.go` (16 cases, 2), `rates.go` (16 cases, 1)
- Modify: `internal/tui/app.go` (1 `tea.Key` site), `setup.go` (3 `tea.Key` sites)

**Interfaces:**
- Consumes: `keysFor` and the per-screen/per-mode constructors from Task 2.
- Produces: no new exported surface.

- [ ] **Step 1: Migrate `rates.go`**

16 cases plus one `tea.Key` site. `rates` has an `editing` mode that forwards to a `textinput` (`rates.go:459-484`): the key dispatch must stay *inside* the existing `if rt.editing { … }` branch structure. Run `go test ./internal/tui -run TestRates -v` (28 cases) before moving on.

- [ ] **Step 2: Migrate `entries.go`**

16 cases plus 2 `tea.Key` sites, across five modes (`entriesList`, `entriesConfirmDelete`, `entriesEdit`, `entriesHistory`, `entriesTags`). The mode dispatch stays outermost.

`entriesConfirmDelete`'s "any other key cancels" (`entries.go:272`) stays a `default` clause — it is the absence of a match, which no binding can express.

The `" "`/`"space"` toggle at `entries.go:234`: declare `WithKeys(" ")`, drop the dead `"space"` label.

Run `go test ./internal/tui -run 'TestEntries|TestV|TestTag|TestHistory' -v` before moving on.

- [ ] **Step 3: Migrate `log.go`**

17 cases plus 5 `tea.Key` sites, across the log flow's steps. Two specifics:

- **Delete `log.go:314-316` rather than migrating it.** That `case "esc"` inside `logTimerRunning` is unreachable: the guard at `log.go:275` catches `esc` for every non-input step first. Migrating it mechanically preserves a latent wrong-destination bug (it returns to Report even when the flow's origin is Home). Remove it and leave a one-line comment saying the outer guard handles `esc`.
- The ID-input and form steps forward to a `textinput`. Their key dispatch stays inside those steps' branches.

Run `go test ./internal/tui -run 'TestLog|TestTimer' -v` before moving on.

- [ ] **Step 4: Migrate the remaining `tea.Key` sites**

`app.go` (1) and `setup.go` (3). `setup.go`'s three sites are inside its three wizard steps; the step dispatch stays outermost.

- [ ] **Step 5: Verify no string key comparison survives**

```bash
grep -rnE 'case "[a-z]+"|msg\.String\(\) ==|msg\.Type == tea\.Key' internal/tui/*.go | grep -v _test | grep -v demo.go
```
Expected: no output. `demo.go`'s two `case` clauses match space IDs, not keys, and are excluded deliberately.

- [ ] **Step 6: Verify label parity and full suite**

```bash
go test ./internal/tui -run 'Key' -v      # every parity test still passes
go test ./... -race
git status --short internal/tui/testdata  # must be empty
```

- [ ] **Step 7: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui
git commit -m "refactor(tui): route log, entries and rates through the keymap (#59)"
```

---

### Task 5: The navigation stack

The one semantic change in this tranche. Everything before it was mechanical.

**Files:**
- Create: `internal/tui/nav.go`, `internal/tui/nav_test.go`
- Modify: all 13 files holding a `.screen = screen…` assignment
- Modify: `internal/tui/log.go` and `listbrowser.go` — delete both `origin` fields

**Interfaces:**
- Consumes: everything from Tasks 2–4.
- Produces:
  - `Model.nav []screen` — the **parent chain only**; the current screen is `m.screen` and is not the top of `nav`. Empty `nav` means "nowhere to go back to".
  - `func (m Model) goTo(s screen) Model` — forward navigation. **Truncating:** if `s` is already in `nav`, truncate above it instead of appending, so `nav` never holds a duplicate and cannot grow without bound.
  - `func (m Model) replace(s screen) Model` — same logical step, different screen. `nav` unchanged.
  - `func (m Model) pop() Model` — return to the parent. No-op on an empty `nav`.
  - `func (m Model) resetTo(s screen) Model` — clear `nav` and go to `s`.

- [ ] **Step 1: Write the failing invariant tests**

Create `internal/tui/nav_test.go`:

```go
package tui

import "testing"

func TestNavGoToPushesParent(t *testing.T) {
	t.Parallel()
	m := newTestModel().resetTo(screenHome)
	m = m.goTo(screenReport).goTo(screenEntries)
	if m.screen != screenEntries {
		t.Fatalf("screen = %v, want screenEntries", m.screen)
	}
	if len(m.nav) != 2 || m.nav[0] != screenHome || m.nav[1] != screenReport {
		t.Errorf("nav = %v, want [home report]", m.nav)
	}
}

func TestNavPopReturnsToParent(t *testing.T) {
	t.Parallel()
	m := newTestModel().resetTo(screenHome).goTo(screenReport).goTo(screenEntries)
	m = m.pop()
	if m.screen != screenReport {
		t.Errorf("after pop screen = %v, want screenReport", m.screen)
	}
	m = m.pop()
	if m.screen != screenHome || len(m.nav) != 0 {
		t.Errorf("after second pop screen = %v nav = %v, want home and empty", m.screen, m.nav)
	}
}

func TestNavPopOnEmptyIsNoOp(t *testing.T) {
	t.Parallel()
	m := newTestModel().resetTo(screenHome).pop()
	if m.screen != screenHome {
		t.Errorf("pop on empty nav moved to %v", m.screen)
	}
}

// The invariant that makes unbounded growth impossible: this is the cycle that
// a "clear on Home" rule does not catch, because it never visits Home.
// Report → n → Log → save → r → reload → the entriesMsg handler lands on
// Report again.
func TestNavGoToTruncatesInsteadOfDuplicating(t *testing.T) {
	t.Parallel()
	m := newTestModel().resetTo(screenHome).goTo(screenReport).goTo(screenLog)
	for range 5 {
		m = m.goTo(screenReport) // the reload cycle
	}
	if m.screen != screenReport {
		t.Fatalf("screen = %v, want screenReport", m.screen)
	}
	if len(m.nav) != 1 || m.nav[0] != screenHome {
		t.Errorf("nav = %v, want [home] — the stack grew or kept a duplicate", m.nav)
	}
}

func TestNavReplaceLeavesParentChain(t *testing.T) {
	t.Parallel()
	m := newTestModel().resetTo(screenHome).goTo(screenReport).replace(screenLoading)
	if m.screen != screenLoading {
		t.Fatalf("screen = %v, want screenLoading", m.screen)
	}
	if len(m.nav) != 1 || m.nav[0] != screenHome {
		t.Errorf("nav = %v, want [home] — replace must not push", m.nav)
	}
	if m = m.pop(); m.screen != screenHome {
		t.Errorf("pop after replace → %v, want screenHome", m.screen)
	}
}
```

- [ ] **Step 2: Run and watch them fail**

Run: `go test ./internal/tui -run TestNav -v`
Expected: build failure — `undefined: goTo`, `replace`, `pop`, `resetTo`, and `m.nav`.

- [ ] **Step 3: Write nav.go**

```go
package tui

import "slices"

// goTo navigates forward to s, remembering the current screen as its parent.
//
// The push truncates: if s is already in the parent chain, everything above it
// is dropped rather than appended to. Without this, any cycle that does not
// pass through a stack-clearing screen grows nav without bound — for example
// Report → n → Log → save → r → reload, whose entriesMsg handler lands back on
// Report. With it, nav can never hold a duplicate and its depth is bounded by
// the number of screens.
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
// what Home needs — no special case.
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

`slices.Clone` on push matters: `Model` is a value receiver, so two Models can otherwise share a backing array and one's append can overwrite the other's entry.

- [ ] **Step 4: Run the invariant tests to green**

Run: `go test ./internal/tui -run TestNav -v`
Expected: PASS (5 tests).

- [ ] **Step 5: Route all 62 transition sites**

Every `.screen = screen…` assignment becomes one of the four calls. **This table is the classification — do not re-derive it.** Line numbers are from the pre-migration tree; match on the surrounding context, not the number.

| Site | Context | Call |
|---|---|---|
| `app.go:180`, `:183` | `New()` initial screen | `resetTo` |
| `app.go:301`, `:317`, `:616`, `:632` | error paths → `screenError` | `replace` |
| `app.go:523` | `openListBrowser` | `goTo` |
| `app.go:613`, `:623` | 401 → setup wizard relaunch | `resetTo` |
| `app.go:630` | `retryableErrMsg` origin Home | `resetTo(screenHome)` |
| `app.go:641` | `logErrMsg` → stay on Log | `replace` |
| `app.go:667` | `entriesMsg` → Report | `replace` |
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
| `home.go:81`, `:85` | `f` → Members | `goTo` |
| `home.go:92` | `enter` → Loading | `replace` |
| `listbrowser.go:151`, `:157` | list chosen → Rates / Log | `pop` |
| `log.go:312`, `:339`, `:371`, `:462`, `:478` | → Loading | `replace` |
| `log.go:315` | unreachable `esc` | **deleted in Task 4** |
| `members.go:67` | confirm | `pop` |
| `members.go:70` | `esc` (discards `mm`) | `pop` |
| `range.go:75`, `:136` | pick / confirm | `pop` |
| `range.go:139` | `esc` | `pop` |
| `rates.go:451` | `esc` | `pop` |
| `rates.go:870` | save | `pop` |
| `report.go:72` | `m` → Home | `pop` |
| `report.go:74` | `r` → Loading | `replace` |
| `report.go:78`, `:81`, `:84`, `:137` | `e` / `p` / `n` / `b` | `goTo` |
| `report.go:90`, `:94` | `f` → Filters | `goTo` |
| `setup.go:126` | wizard done → Home | `resetTo` |

`report.go`'s `s` case (scope toggle, also assigning Home) is a `pop` for the same reason as `m`.

- [ ] **Step 6: Delete both origin fields**

`logModel.origin` (`log.go:52`, read at `:276`, `:354`, `:483`) and `listBrowserModel.origin` (`listbrowser.go`, set at `app.go:522`) are now redundant: `pop()` returns to whichever screen pushed. Delete both fields, their constructor parameters (`newLog`'s third argument) and every read. Update the call sites and the tests that pass an origin.

Keeping either one would reproduce the two-mechanism ambiguity this task exists to remove.

- [ ] **Step 7: Add the missing staleness guards**

Three async handlers force a screen with no staleness check, unlike `tagsMsg` (`app.go:710`) and `spacesMsg` (`app.go:794`) which have one. With a navigation stack, a late reply that yanks the user back is worse than before. Add the same guard shape those two use to:

- `membersMsg` (`app.go:768`)
- `statusesMsg` (`app.go:780`)
- `historyMsg` (`app.go:701`)

Each should ignore the message when the user has navigated away from the screen that requested it.

- [ ] **Step 8: Verify no naked assignment survives**

```bash
grep -rnE '\.screen = ' internal/tui/*.go | grep -v _test | grep -v nav.go
```
Expected: no output. Every transition goes through the API; a surviving assignment silently desyncs `nav`, and a value-receiver Model makes that invisible at runtime.

- [ ] **Step 9: Verify behavior is unchanged**

```bash
go test ./... -race
git status --short internal/tui/testdata   # must be empty
```
The existing transition tests encode today's navigation. They must pass **unchanged** — the stack mirrors the tree, so `esc` still lands exactly where it did. The two exceptions to expect, both intended:

- Report now accepts `esc` (→ Home), which it did not before.
- Tests that constructed `newLog(entries, cfg, screenReport)` lose their third argument.

- [ ] **Step 10: Full gate and commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui
git commit -m "refactor(tui): replace ad-hoc screen assignments with a navigation stack (#59)"
```

---

### Task 6: Documentation

**Files:**
- Modify: `CHANGELOG.md`, `README.md`, `README.it.md`

**Interfaces:** consumes everything above; produces no code.

- [ ] **Step 1: Correct the README key tables**

Both READMEs carry a key table. Read the current one, then verify **every row** against `keys.go` — that file is now the single source of truth for what each screen accepts. Known wrong today: `q — "Everywhere except setup"` (it is also inert on rates, range, list browser, log and entries — a checkbox in #28).

Add Report's new `esc`.

- [ ] **Step 2: CHANGELOG**

Add to `### Changed` under `## [Unreleased]` (create the section if the release commit has since removed it):

```markdown
- The Report screen now accepts `esc` to go back, like every other screen.
  Navigation is handled by a single back-stack, so `esc` always returns to
  wherever you came from — including when the same screen is reachable from
  more than one place.
```

- [ ] **Step 3: README.it.md**

The same row and entry in Italian, in the matching table. Fluent native Italian with correct accents (à, è, é, ì, ò, ù) — never ASCII substitutes.

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

- `grep -rnE 'case "[a-z]+"|msg\.String\(\) ==|msg\.Type == tea\.Key' internal/tui/*.go` (excluding tests and `demo.go`) returns nothing.
- `grep -rnE '\.screen = ' internal/tui/*.go` (excluding tests and `nav.go`) returns nothing.
- Neither `logModel` nor `listBrowserModel` has an `origin` field.
- Every golden in `internal/tui/testdata/` is byte-identical to Task 1's state.
- `go test ./... -race` passes.
- #131 closes; #59 keeps its overlay-compositor item for B2.
