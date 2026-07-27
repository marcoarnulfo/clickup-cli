# Generated footer and `?` full help (v1.9, tranche B2) — Design

> Spec for the third of four tranches of the **v1.9 TUI design system** milestone.
> Covers #69 in full. Branch: `feat/tui-footer-help`.

## 1. Goal

Replace the 25 hand-written key-hint lines with **one footer generated from
`keysFor(m)`**, rendered in a single place, and add `?` to expand it into full
per-screen help.

The user-visible outcome: every screen shows the same footer in the same
position; four screens gain a quit hint they never had; the key list can no
longer drift from what the keys actually do, because there is only one of it.

## 2. Why this is tranche B2, and why it does not build an overlay compositor

The v1.9 plan assigned the **overlay compositor** to this tranche alongside
#69. It is deliberately deferred to B3.

`bubbles/help` already toggles between short and full help through its own
`ShowAll` field. If `?` is satisfied by that — and it is — then a compositor
built here would have no real consumer: its first genuine one is the command
palette (#71), which must float over a screen while the user types into it.
Shipping a compositor whose only client renders a box that could equally be a
taller footer repeats the mistake tranche A avoided when it kept `Warning`,
`Border` and the table styles out of the theme until code needed them.

**#59 therefore stays open on its third checkbox.** #69 closes, with a note
recording that `?` shipped as an expanded footer rather than a floating panel.
Promoting it later is a change to one function.

## 3. Current state (measured, not assumed)

- **56 uses of `th.Help`** across 15 files, and they divide exactly:
  - **24 render a key-hint line.** These are what the footer replaces.
  - **1** is the error screen's `press a key to return home`, which reads like a
    hint but describes the absence of a binding and is **kept** (§4.2).
  - **30** are empty-state notices, table headers, breadcrumbs, subtotals,
    progress messages and the update notice. **None of them is touched.**
  - **1** is the Home timer status line, whose embedded `(c: manage)` suffix is
    the only non-hint string this tranche edits.

  The implementation deletes **25 things**: those 24 render sites plus
  `rates_view.go`'s `help()` method, whose body holds no `th.Help` of its own —
  its render site (`rates.go`) is one of the 24.
- **Log renders two hint lines at once** in four of its eight steps: the
  per-step line for `logTimerRunning`, `logListPick`, `logTaskPick` and
  `logDone`, plus the unconditional `Esc: cancel · Ctrl+C: quit` appended after
  the whole switch. Setup appends its `Enter: confirm · Ctrl+C: quit`
  unconditionally too, but no step renders a second one — Setup shows exactly
  one line in every state.
- Two hints are embedded in other sentences: the Home timer line ends with
  `(c: manage)` (assembled in `app.go`, rendered at `home.go:145`), and
  `rates_view.go:64` says `press 'b' to browse the workspace`.
- **`full` is populated by 2 of the 12 `keysFor` constructors** (`homeKeys`,
  4 columns; `reportKeys`, 3), and both are already grouped semantically with
  the globals last. Ten constructors leave it nil. **The convention exists; it
  just has not been applied to the other ten.**
- **`short` is already curated**, not a dump of every binding: `reportKeys`
  advertises 6 of its 11. What it does not do yet is collapse key pairs — it
  lists `Up` and `Down` as two items.
- **`ForceQuit` (ctrl+c) is accepted on all 14 screens** — `app.go:610` checks
  it before routing — but it lives only in `keyDefaults`, never in `keyMap`, so
  no parity test declares it and no screen advertises it except Setup and Log,
  which say so by hand.
- **`Model.width`/`Model.height` are written by `WindowSizeMsg` and read by
  nobody.** They have been dead since v1.0.
- **10 input-forwarding contexts** across 5 screens (`setup` 3, `log` 2,
  `range` 2, `rates` 1, `entries` 2) pass unmatched keys to a `textinput`.
- **`Model.View()` delegates to a sub-model view for 10 screens**, stamps the
  clock first for Log, builds Loading and Error inline, and assembles the Home
  timer line inline before delegating. `screenEntries` is the one screen whose
  view is a `Model` method (`m.entriesView(th)`) rather than a sub-model's.
- `testdata/` holds **26 golden files**, produced by 21 test functions
  (`TestGoldenRatesTabs` alone writes four). **24 are screen goldens**; the
  other two are `palette_dark`/`palette_light` from tranche A and have nothing
  to do with this work.
- **Only 5 of the 24 screen goldens go through `m.View()`** (entries,
  entries_edit, entries_confirm_delete, error, loading). The other 19 call a
  sub-model view directly.
- **`screenLoading` and `screenError` share one `keysFor` branch**
  (`keys.go:264`). This tranche needs them to differ, so the branch splits.

## 4. Decisions

### 4.1 The footer is a pure function, not a sub-model

```go
func footerView(th theme, width int, showAll bool, k keyMap) string
```

It builds a `help.Model` from the theme on every call. `help.Model` is
configuration plus two state fields, so constructing it per render costs
nothing and avoids the one hazard this codebase keeps hitting: a sub-model that
must be written back explicitly and silently zeroes itself when someone forgets.

The only new state on the root is `helpAll bool`.

`help.New()` is **not** used: it derives its styles from lipgloss's default
renderer, which would bypass the injected-renderer discipline tranche A
established. The `help.Styles` struct is built from the theme instead —
`th.Help.Bold(true)` for keys, `th.Help` for descriptions and separators — and
`ShortSeparator` is set to `" · "` to match today's lines rather than bubbles'
default `" • "`.

### 4.2 One render site, with one exception

`Model.View()` appends the footer to whatever the screen returned. A new screen
cannot forget it, and it cannot drift per screen.

The exception is **`screenError`**, which keeps its inline
`press a key to return home`. That is not a binding — *any* key works — and
inventing a fake binding to describe it would be worse than the sentence.
`screenLoading` does get a footer; today it shows nothing at all, and it will
show `q quit`.

Consequence on Home: its view concatenates help line → error text → update
notice → timer line. With the help line gone and the footer appended centrally,
the notice and timer line move above the footer. That is the intended shape —
the footer belongs at the bottom on every screen, not in the middle of one.

### 4.3 `Enabled()` is acceptance; `short`/`full` are advertisement

B1 established that a disabled binding cannot match. This tranche uses the
other half of the same mechanism: a binding may be **enabled and unadvertised**.
Two things fall out of it.

**ctrl+c.** `ForceQuit` moves into `keyMap`, enabled on all 14 screens, which
is simply true. It appears in `short`/`full` only on the six screens where
`Quit` is disabled. That covers Setup's and Log's hand-written
`Ctrl+C: quit` — the footer words it `ctrl+c force quit`, from the binding's
own help text — and gives Rates, Range, the list browser and the time
entries a quit hint they have never had. Every screen's parity test gains
`ctrl+c` — a correction, not an expansion: the key was always accepted and
never declared.

**Key pairs.** `short` lists `Up` and `Down` separately today, which would
render as `↑/k move up · ↓/j move down` where the hand-written line says
`↑/↓ select`. Instead one representative per pair goes into `short`/`full`, and
its help text names the whole pair: `↑/↓/j/k` + `move`. The partner binding
stays enabled and matches as before — only the advertisement collapses. This is
also the first time the vim aliases are documented anywhere.

The four pairs are `Up`/`Down`, `NextField`/`PrevField` (tab / shift+tab),
`NextSection`/`PrevSection` on Rates, and `PrevMonth`/`NextMonth` — the last of
which is advertised only in `homeKeys.full`.

**Display-only bindings.** A pair item is a `key.Binding` that exists solely to
be rendered — it is never passed to `key.Matches`, because matching only ever
reads `keyMap`'s own fields. That makes it the right tool for one more case the
generated footer would otherwise lose: the delete confirmation currently reads
`y: delete · any other key: cancel`, and "any other key" is the *absence* of a
match, which no real binding can express. It becomes a display-only item so the
footer keeps saying it.

### 4.4 Full help is columns, globals last

`help.FullHelpView` renders **one column per group**, bindings stacked
vertically inside it. `homeKeys` and `reportKeys` already do this well; the
convention they embody is extended to the other ten rather than invented:

- one column per coherent group of actions, in the order a user would look for
  them (movement first where a screen has it, then what the screen *does*);
- **the last column is always the globals** — `?`, back, quit or force-quit;
- a screen with few bindings gets one column plus the globals. Empty groups are
  skipped by bubbles itself (`shouldRenderColumn`), so no special case is
  needed.

The two existing slices keep their **grouping and order**. They gain `Help` in
the last column, and `homeKeys`' first column collapses its two month items
into one pair item (§4.3) — a change of item count, not of shape.

### 4.5 `?` toggles, and `esc` is not involved

`?` is a new binding, enabled everywhere except **12 contexts**:

- the **10 input-forwarding contexts** — typing `?` into a note or a task ID
  must insert the character. `keysFor` already branches on exactly those modes,
  so the gate has a home.
- **`screenError`**, whose handler returns Home on *any* key. Enabling `?`
  there would swallow the only way out.
- **`entriesConfirmDelete`**, whose `default` arm cancels on *any other key*.
  Enabling `?` there would turn a cancel into a help toggle.

The last two matter more than they look: on both screens every key is already
spoken for, so a new global binding does not add a feature, it removes one.

`?` is handled in `Update`, next to `Quit` and `ForceQuit` and before
`routeKey`, so it works identically on every screen that enables it — including
`screenLoading`, which routes no keys at all.

`?` toggles `m.helpAll`. It persists across screens for the session.
**`esc` does not close it**: `Back` keeps meaning `pop()` and nothing else.
Making the back key depend on hidden state is precisely what B1 removed.

### 4.6 Width, finally read

`footerView` receives `m.width` and passes it to `help.Model.Width`, so short
help truncates with `…` instead of wrapping — the Report's line is already
~130 characters. Width `0` (before the first `WindowSizeMsg`, and in tests)
disables truncation, so rendering stays deterministic either way.

This is the only width-awareness this tranche introduces. Making the screen
bodies responsive stays out of scope.

### 4.7 Goldens split in two

Deleting the inline hint changes every screen body, so the existing goldens
move whatever else happens. The split keeps each diff readable:

| Kind | Count | What moves |
|---|---|---|
| body goldens (sub-model view) | 19 | lose exactly one hint line |
| composed goldens (`m.View()`) | 5 | lose the hint line, gain the footer |
| `palette_light` / `palette_dark` | 2 | **nothing** — unrelated to this work |
| **footer goldens (new)** | two per distinct label set | the actual new behavior |

Every body golden loses exactly *one* line: Log renders two hint lines only in
states no golden captures (`newLog` starts at `logModeSelect`, and `log_form`
captures `logForm`), and Setup never renders two at all.

The footer goldens come in pairs, short and full. **The full ones are the only
check on the column assignments** — a binding named in a `full` group but never
assigned in that branch is a zero `Binding`, which `FullHelpView` silently
drops. Nothing else in this tranche renders `FullHelp()`.

"One per distinct label set" is the same granularity the per-screen parity
tests already use, so the two nets cover the same ground: parity asserts which
keys a state accepts, the footer golden asserts how that state advertises them.

Converting the 16 sub-model goldens to `m.View()` was considered and rejected:
it is broad churn for little gain, and the dedicated footer goldens test the
new code in isolation, which the composed ones cannot do as precisely.

## 5. File structure

- **Create** `internal/tui/footer.go` — `footerView`, the theme→`help.Styles`
  mapping, and the pair-representative helpers.
- **Create** `internal/tui/footer_test.go` and `testdata/footer_*.golden`.
- **Modify** `internal/tui/keys.go` — `ForceQuit` and `Help` into `keyMap`;
  `full` for all 12 constructors; curated `short`.
- **Modify** `internal/tui/app.go` — the footer render site, `helpAll`, the `?`
  arm, and the timer line's `(c: manage)` suffix.
- **Modify** the 14 files holding a hint line, deleting it: `home`, `report`,
  `export`, `budget`, `members`, `filters`, `range`, `listbrowser`, `setup`,
  `history`, `entries`, `log`, `rates`, `rates_view`.
- **Modify** the per-screen parity tests (`ctrl+c`, `?`).

## 6. Task order — the first two move no goldens

1. **The renderer.** `footerView`, its unit tests, and two footer goldens — one
   short, one full — built from a hand-made `keyMap` rather than from
   `keysFor`, so the renderer is pinned before the data it will render exists.
   Nothing wired.
2. **Display data.** `full` groups for every branch, curated `short`,
   `ForceQuit` and `Help` into `keyMap`, `?` gated off in the 12 contexts, the
   Loading/Error branch split. Parity tests updated. Nothing renders it yet.
3. **Footer goldens**, two per label set — short and full. The full ones are
   the only thing that shows a `full` group was mis-assigned: an unassigned
   binding is a zero `Binding`, which `FullHelpView` drops in silence.
4. **`?`.** `m.helpAll`, the toggle arm in `Update`, and the gating tests.
   Nothing renders it yet, so nothing moves.
5. **Wiring.** `Model.View()` appends the footer; the 25 hint lines are
   deleted; `(c: manage)` is stripped. All 21 goldens regenerate **once**, and
   every diff is read.

The toggle deliberately lands *before* the wiring. The other order would ship
one task in which the footer advertises `? help` and pressing `?` does nothing.
6. **Docs.** `CHANGELOG`, both READMEs.

**A golden that moves before task 5 is a bug, not an update.**

## 7. Testing

- **Footer content tests, per screen and per mode**: assert the rendered footer
  against a golden. This is where the tranche's correctness lives — the 16 body
  goldens only prove a line was removed.
- **`?` gating**: assert `keysFor(m).Help.Enabled()` is false in each of the 12
  contexts; that a `?` keypress reaches the `textinput` in the 10 input ones;
  that it still returns Home from `screenError`; and that it still cancels a
  pending delete.
- **Advertisement vs acceptance**: assert that `ForceQuit` is enabled on all 14
  screens but present in `short` on exactly the six where `Quit` is disabled.
- **Truncation**: assert that a narrow width truncates with `…` and that width
  `0` does not.
- Existing per-screen label-parity tests gain `ctrl+c` and `?`.
- Pre-commit gate: `gofmt -l .`, `go vet ./...`,
  `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`,
  `go test ./... -race` — all clean/green.

## 8. Constraints

- `internal/report` and `internal/duration` stay pure and are untouched.
- No new `go.mod` dependencies. `bubbles/help` is a subpackage of a module
  that is already direct.
- bubbletea value receivers; explicit write-back before every return.
- The 30 non-hint uses of `th.Help` are not touched.
- Everything in the repo in ENGLISH except `README.it.md` and
  `CONTRIBUTING.it.md`; Conventional Commits; **no `Co-Authored-By` trailer**.
- Demo mode keeps working identically — it renders through the same views.

## 9. Out of scope

The overlay compositor and the command palette (#71, tranche B3). The report
table, gauges and the `rates.go` index refactor (tranche C). YAML themes,
configurable keybindings, mouse support and the Filters viewport (tranche D).
Responsive screen bodies: only the footer becomes width-aware.
