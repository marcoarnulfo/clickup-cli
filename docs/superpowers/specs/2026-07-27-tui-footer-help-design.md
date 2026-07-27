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

- **56 uses of `th.Help`** across 15 files. Only **25 are key-hint lines**. 30
  are empty-state notices, table headers, breadcrumbs, subtotals, progress
  messages and the update notice. The 56th is the Home timer status line, which
  is not a hint but carries one inside it. **This tranche touches the 25 plus
  that one suffix, and leaves the other 30 alone.**
- Two screens render **two** hint lines at once: `log.go:573` and
  `setup.go:166` append an unconditional `"… · Ctrl+C: quit"` *after* whatever
  the current step already rendered.
- Two hints are embedded in other sentences: the Home timer line ends with
  `(c: manage)` (assembled in `app.go`, rendered at `home.go:145`), and
  `rates_view.go:64` says `press 'b' to browse the workspace`.
- **`full` is populated by 2 of the 12 `keysFor` constructors** (`homeKeys`,
  `reportKeys`), and both are shaped one-binding-per-group — which
  `help.FullHelpView` renders as one wide row, not as columns. Ten constructors
  leave it nil.
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
- **Only 5 of the 21 goldens go through `m.View()`** (entries, entries_edit,
  entries_confirm_delete, error, loading). The other 16 call a sub-model view
  directly.

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
`Quit` is disabled. That reproduces Setup's and Log's hand-written
`Ctrl+C: quit` exactly, and gives Rates, Range, the list browser and the time
entries a quit hint they have never had. Every screen's parity test gains
`ctrl+c` — a correction, not an expansion: the key was always accepted and
never declared.

**Key pairs.** Listing every binding separately turns today's `↑/↓ select` into
`↑/k move up · ↓/j move down` on eight screens. Instead one representative per
pair goes into `short`/`full`, and its help text names the whole pair:
`↑/↓/j/k` + `move`. The partner binding stays enabled and matches as before.
This is also the first time the vim aliases are documented anywhere.

### 4.4 Full help is three columns

`help.FullHelpView` renders **one column per group**, bindings stacked
vertically inside it. All 12 constructors adopt the same grouping:

| Column | Contents |
|---|---|
| Movement | cursor, section/field switching, month navigation |
| Actions | what this screen does |
| Global | `?`, back, quit / force-quit |

Empty groups are skipped by bubbles itself (`shouldRenderColumn`), so a screen
with no movement keys needs no special case.

The two existing `full` slices are reshaped: today they are one binding per
group, which renders as a single wide row.

### 4.5 `?` toggles, and `esc` is not involved

`?` is a new binding, enabled everywhere **except the 10 input-forwarding
contexts** — typing `?` into a note or a task ID must insert the character.
`keysFor` already branches on exactly those modes, so the gate has a home.

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
| body goldens (sub-model view) | 16 | lose one hint line — two for Log and Setup |
| composed goldens (`m.View()`) | 5 | lose the hint line, gain the footer |
| **footer goldens (new)** | one per distinct label set | the actual new behavior |

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
2. **Display data.** `full` groups for all 12 constructors, curated `short`,
   `ForceQuit` and `Help` into `keyMap`, `?` gated off in the 10 input
   contexts. Parity tests updated. **The per-label-set footer goldens land
   here** — this is the task where `short`/`full` become real, and a golden of
   the real footer is the only thing that shows a group was mis-assigned.
   Nothing renders them on screen yet.
3. **Wiring.** `Model.View()` appends the footer; the 25 hint lines are
   deleted; `(c: manage)` is stripped. All 21 goldens regenerate **once**, and
   every diff is read.
4. **`?`.** `helpAll`, the toggle arm, and tests that full help renders columns
   and that `?` still types into the inputs where it is gated off.
5. **Docs.** `CHANGELOG`, both READMEs.

**A golden that moves during tasks 1 or 2 is a bug, not an update.**

## 7. Testing

- **Footer content tests, per screen and per mode**: assert the rendered footer
  against a golden. This is where the tranche's correctness lives — the 16 body
  goldens only prove a line was removed.
- **`?` gating**: assert `keysFor(m).Help.Enabled()` is false in each of the 10
  input contexts, and that a `?` keypress there reaches the `textinput`.
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
- The 31 non-hint uses of `th.Help` are not touched.
- Everything in the repo in ENGLISH except `README.it.md` and
  `CONTRIBUTING.it.md`; Conventional Commits; **no `Co-Authored-By` trailer**.
- Demo mode keeps working identically — it renders through the same views.

## 9. Out of scope

The overlay compositor and the command palette (#71, tranche B3). The report
table, gauges and the `rates.go` index refactor (tranche C). YAML themes,
configurable keybindings, mouse support and the Filters viewport (tranche D).
Responsive screen bodies: only the footer becomes width-aware.
