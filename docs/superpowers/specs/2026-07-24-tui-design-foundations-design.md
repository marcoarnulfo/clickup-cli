# TUI design-system foundations (v1.9, tranche A) — Design

> Spec for the first of four tranches of the **v1.9 TUI design system** milestone.
> Covers #75 (golden harness), #54 (theme registry), #63 (semantic palette) and
> the color-profile half of #74. Branch: `feat/tui-design-foundations`.

## 1. Goal

Replace the package-level style vars with a **theme carried on the root Model
and passed explicitly to every view**, built from a **semantic adaptive
palette**, and land a **golden-file regression harness first** so the refactor
is provably behavior-preserving.

Nothing user-facing changes except one thing: colors adapt to a light or dark
terminal background.

## 2. Why this is a tranche, not the milestone

v1.9 is four independent subsystems with a hard dependency order. Splitting
them keeps each spec, plan and branch reviewable:

| Tranche | Issues | Ships |
|---|---|---|
| **A (this spec)** | #75, #54, #63, #74 (color profile) | Golden harness, theme on the Model, adaptive palette |
| B | #59, #69, #71 | Keymap registry, navigation stack, overlay compositor, `?` help, command palette |
| C | #66, #80, #117 | Report table, gauges/sparklines, `rates.go` selection-index refactor |
| D | #82, #74 (mouse), #28 | YAML themes, configurable keybindings, mouse, Filters viewport |

Every later tranche is written against the golden harness and the theme this
one establishes.

## 3. Current state (measured, not assumed)

- `internal/tui` is ~10k lines over 37 files.
- `styles.go` is 17 lines and holds *nothing but* vars: 4 colors (`colAccent`
  205, `colDim` 240, `colErr` 196, `colOK` 42) and 6 styles (`styleTitle`,
  `styleHelp`, `styleErr`, `styleAccent`, `styleOK`, `styleBox`). The
  `newTextInput` / `newNumberInput` helpers live in `setup.go` and are not
  touched by this work.
- Those styles are read at **146 sites across 16 files**.
- 12 sub-model `view()` methods take no styling parameter; 4 render helpers
  (`historyLine`, `browserRow`, `ratesModel.tabs`, `billingRow`) use styles
  outside a `view()`.
- Views never read `Model.width`/`Model.height` — the fields are stored on
  `tea.WindowSizeMsg` and never consumed. **Rendering is therefore already
  independent of terminal size**, so golden files need no width pinning.
- 17 test call sites invoke `.view()` / `.View()`.

## 4. Decisions

### 4.1 The theme is an explicit parameter

`Model.theme` is the single source of truth; `view()` and the 4 render helpers
take it as an argument:

```go
func (rm reportModel) view(th theme) string
```

Rejected: a `th theme` field on each sub-model (the codebase's mandatory
explicit write-back means one forgotten copy yields a zero-value theme —
empty styles, no compile error, silent bug), and a mutable package var
(exactly what #54 exists to remove; also unsafe for parallel tests under
`-race`).

### 4.2 Two levels: palette → styles

```go
type palette struct {
    Primary, Accent, Muted, Danger, Success lipgloss.AdaptiveColor
}

type theme struct {
    p palette // unexported: views consume styles, never raw colors
    Title, Help, Err, OK, Accent, Box lipgloss.Style
}

func newTheme(r *lipgloss.Renderer, p palette) theme
```

The 6 styles are exactly today's 6, so the refactor is verifiable as a no-op.
Styles that tranche C will need (`TableHeader`, `Zebra`, `Total`, gauges) are
**not** added now: they would be dead code and staticcheck would flag them.

The palette ships the 5 tokens that have a consumer today. `Warning` and
`Border` from #63's wish list are deliberately omitted for the same reason;
they arrive with the code that uses them.

### 4.3 Renderer injection

`newTheme` takes a `*lipgloss.Renderer`. Production passes
`lipgloss.DefaultRenderer()`. Tests pass `lipgloss.NewRenderer(io.Discard)`
with a pinned profile and background, so:

- golden output is deterministic regardless of the terminal running the suite,
- light and dark themes coexist in one test binary,
- no global `lipgloss.SetColorProfile`, so tests stay parallel-safe under
  `-race`.

### 4.4 Golden files: an internal helper, not teatest

A ~25-line helper in `internal/tui` renders a view, compares it against
`testdata/<name>.golden`, and rewrites it under `-update`.

Rejected: `github.com/charmbracelet/x/exp/teatest`. It has no tagged releases
(it would enter `go.mod` as a pseudo-version of an explicitly experimental
package), and its `WaitFor`-with-timeout model introduces flakiness risk into
the very safety net every later tranche depends on. The house pattern is
already direct `Update()` + `view()`; the helper fits it. Full-program testing
is not needed to lock rendered output.

### 4.5 Two kinds of golden file

| Kind | Profile | Captures |
|---|---|---|
| screen goldens | `termenv.Ascii` | text, alignment, borders, margins, truncation |
| `palette_light` / `palette_dark` | `termenv.TrueColor` | the exact value of every token |

Screen goldens stay human-readable in PR diffs; a color change fails in exactly
one place with a clear message, instead of turning every file into a wall of
escape codes.

### 4.6 `NO_COLOR` already works

lipgloss builds its default renderer through `termenv.EnvColorProfile()`, which
already honors `NO_COLOR` and `CLICOLOR`/`CLICOLOR_FORCE`. This half of #74
therefore needs **a regression test and documentation, not code**.

`FORCE_COLOR` (the npm convention, named in #74) is **not** implemented:
`CLICOLOR_FORCE` is the standard termenv supports, and adding a second
force-color variable by hand is unjustified surface area.

## 5. File structure

- **Create** `internal/tui/theme.go` — `palette`, `theme`, `newTheme`,
  `defaultPalette()`.
- **Create** `internal/tui/golden_test.go` — the `golden` helper, the `-update`
  flag, and `testTheme(dark bool) theme`.
- **Create** `internal/tui/testdata/*.golden`.
- **Delete** `internal/tui/styles.go` outright — it holds only the vars that
  `theme.go` replaces.
- **Modify** `internal/tui/app.go` — `theme` field on `Model`, built in `New()`;
  every `view()` call site passes it.
- **Modify** the 15 remaining files that read styles.

## 6. Task order — each verifiable as a no-op

1. **Golden harness.** Helper, `-update`, `testTheme`, and screen goldens
   generated **against the current code**, before anything is touched. No
   production change at all.
2. **`theme.go` + the 9 small files** (`app.go`, `home`, `setup`, `export`,
   `budget`, `history`, `members`, `range`, `listbrowser`). The initial palette
   sets `Light == Dark` to today's values (205/240/196/42), so the task-1
   goldens must stay **byte-identical**.
3. **The 6 large files** (`report`, `entries`, `log`, `filters`, `rates`,
   `rates_view`), then `styles.go` is deleted. Goldens still byte-identical.
4. **The real adaptive palette.** `Light` values diverge from `Dark`;
   `palette_light` / `palette_dark` goldens are added, plus regression tests for
   `NO_COLOR` and `CLICOLOR_FORCE`. Screen goldens remain byte-identical —
   layout does not move.
5. **Docs.** `CHANGELOG`, both READMEs (color environment variables),
   `CONTRIBUTING` (how to regenerate goldens with `-update`, and that the
   resulting diff must be read, not rubber-stamped).

**A golden that changes during tasks 2 or 3 is a bug, not an update.**

## 7. Testing

- The existing suite is the primary net: it must stay green unchanged through
  tasks 2 and 3 (only `.view()` call sites gain an argument).
- Screen goldens cover, at minimum: home, report, entries browser, rates
  (each tab), log, filters, budget, range, members, export, setup, list browser,
  history — one representative state each.
- Golden tests are parallel-safe: each builds its own renderer.
- `TestNoColorEnvDisablesColor` asserts that a theme built from a renderer whose
  profile resolves under `NO_COLOR=1` emits no escape sequences.
- Pre-commit gate: `gofmt -l .`, `go vet ./...`,
  `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`,
  `go test ./... -race` — all clean/green. (staticcheck is in CI and broke
  v1.8's first run; it is not optional locally.)

## 8. Constraints

- `internal/report` and `internal/duration` stay pure and are untouched.
- No new `go.mod` dependencies. `termenv` is promoted from indirect to direct
  (it is already in the build graph via lipgloss).
- bubbletea value receivers; explicit write-back before every return.
- Demo mode keeps working identically (it renders through the same views).
- Everything in the repo in ENGLISH; Conventional Commits; **no
  `Co-Authored-By` trailer**.

## 9. Out of scope

Overlays, keymaps and navigation (tranche B). Tables, gauges and the `rates.go`
index refactor (tranche C). YAML themes, configurable keybindings, mouse support
and the Filters viewport (tranche D). Responsive layout: views ignore terminal
width today and continue to; making them width-aware is a tranche-C concern.
