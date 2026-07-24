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
- 12 sub-model `view()` methods take no styling parameter. Styles are also read
  outside a `view()` by `historyLine`, `browserRow`, `ratesModel.tabs`,
  `billingRow`, `entriesEditView`, `entriesHistoryView` and the five
  `ratesModel` section renderers (`listsView`, `membersView`, `overridesView`,
  `rulesView`, `draftView`). The compiler finds them all once a signature
  changes, but they are named here so a task's "Produces" list can be checked.
- `report.go:225` builds an ad-hoc `lipgloss.NewStyle().Bold(true)` inline. It
  carries no color, so no color-var grep finds it, but it is package-level
  style state inside a view and must be folded into the theme.
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
    Title, Help, Err, OK, Accent, Box, Header lipgloss.Style
}

func newTheme(r *lipgloss.Renderer, p palette) theme
```

Six styles are exactly today's six; `Header` is the seventh and is not new
either — it is the bare `lipgloss.NewStyle().Bold(true)` that `report.go`
builds inline today, given a name so it stops bypassing the theme. The refactor
therefore stays verifiable as a no-op.

The theme does **not** keep the palette as a field: nothing in this tranche
would read it, and a write-only field is dead weight staticcheck is right to
flag. Tranche D adds it back when the YAML loader gives it a consumer.

Styles that tranche C will need (`TableHeader`, `Zebra`, `Total`, gauges) are
**not** added now, for the same reason.

The palette ships the 5 tokens that have a consumer today. `Warning` and
`Border` from #63's wish list are deliberately omitted for the same reason;
they arrive with the code that uses them.

### 4.3 Renderer injection

`newTheme` takes a `*lipgloss.Renderer` and **forces background detection while
building** (it calls `r.HasDarkBackground()` before returning). This matters:
lipgloss resolves an `AdaptiveColor` lazily, at the first `Render`, by querying
the terminal over OSC-11. By then bubbletea owns the terminal and its input
reader races termenv for the reply — termenv times out and falls back to
"dark", so light-terminal users would never actually see the light palette.
`New()` runs before `tea.NewProgram`, so detecting there is safe; tests set the
background explicitly, so the query never fires under test.

Production passes `lipgloss.DefaultRenderer()`. Tests pass
`lipgloss.NewRenderer(io.Discard)` with a pinned profile and background, so:

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

**Known limit of the Ascii goldens.** termenv returns the string untouched
under the Ascii profile, stripping bold as well as color. A screen golden
therefore proves that text, layout, borders and margins survived the
refactor — but a swapped *role* (rendering with `th.Err` where `th.OK` belongs)
renders byte-identically and slips through. The mitigation is procedural, not
mechanical: the 146-site substitution is 1:1 and mechanical, so the task
reviewer checks the mapping on the diff. The palette goldens then catch any
role that reaches production with the wrong color.

### 4.6 `NO_COLOR` already works

lipgloss builds its default renderer through `termenv.EnvColorProfile()`, which
already honors `NO_COLOR` and `CLICOLOR`/`CLICOLOR_FORCE`. This half of #74
therefore needs **a regression test and documentation, not code**.

`FORCE_COLOR` (the npm convention, named in #74) is **not** implemented:
`CLICOLOR_FORCE` is the standard termenv supports, and adding a second
force-color variable by hand is unjustified surface area.

### 4.7 Adaptive values stay indexed, and only where contrast demands it

The `Dark` side keeps today's **xterm indices verbatim** (`205`, `240`, `196`,
`42`) rather than being rewritten as hex. Two reasons: an indexed color follows
a user's customized 256-color terminal palette while a hex triple overrides it,
and hand-converting indices to hex is exactly the kind of silent off-by-one this
tranche cannot afford (205 is `#FF5FAF`, not `#FF5FD7` — one digit apart from
206). Keeping the index means dark-terminal users provably see no change.

Only the tokens that are genuinely unreadable on white get a `Light` variant,
measured as WCAG contrast against `#FFFFFF`:

| Token | Dark | Light | Why |
|---|---|---|---|
| Primary, Accent | `205` | `127` | `205` on white is ~1.9:1 — unreadable |
| Danger | `196` | `124` | `196` is ~4:1, below the 4.5:1 floor |
| Success | `42` | `28` | `42` on white is ~1.8:1 — unreadable |
| Muted | `240` | `240` | already ~7:1 on white; changing it would be churn |

`Muted` staying identical is deliberate: "adaptive" means *legible on both*, not
*different on both*.

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
2. **`theme.go` + the 8 small files** (`app.go`, `home`, `setup`, `export`,
   `budget`, `members`, `range`, `listbrowser`). The initial palette sets
   `Light == Dark` to today's values (205/240/196/42), so the task-1 goldens
   must stay **byte-identical**.
3. **The 7 remaining files** (`report`, `entries`, `log`, `filters`, `rates`,
   `rates_view`, `history`), then `styles.go` is deleted. Goldens still
   byte-identical.
   `history.go` belongs here, not in task 2, even though it is small: it holds
   `entriesHistoryView`, which `entries.go` calls — splitting them across tasks
   would leave task 2 unable to compile within its own file list.
4. **The real adaptive palette.** The four tokens of §4.7 gain their `Light`
   variants; `palette_light` / `palette_dark` goldens are added, plus regression
   tests for `NO_COLOR` and `CLICOLOR_FORCE`. Screen goldens remain
   byte-identical — layout does not move.
5. **Docs.** `CHANGELOG`, both READMEs (color environment variables),
   `CONTRIBUTING` (how to regenerate goldens with `-update`, and that the
   resulting diff must be read, not rubber-stamped).

**A golden that changes during tasks 2 or 3 is a bug, not an update.**

## 7. Testing

- The existing suite is the primary net: it must stay green unchanged through
  tasks 2 and 3 (only `.view()` call sites gain an argument).
- Screen goldens cover: home (plain and with every notice line), report, entries
  browser, rates (each of the four tabs), log, filters, budget (populated and
  empty), range, members, export, setup, list browser and history — 18 files.
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
  (it is already in the build graph via lipgloss) — this needs an explicit
  `go mod tidy`, and the resulting `go.mod`/`go.sum` diff is expected, not
  accidental.
- bubbletea value receivers; explicit write-back before every return.
- Demo mode keeps working identically (it renders through the same views).
- Everything in the repo in ENGLISH; Conventional Commits; **no
  `Co-Authored-By` trailer**.

## 9. Out of scope

Overlays, keymaps and navigation (tranche B). Tables, gauges and the `rates.go`
index refactor (tranche C). YAML themes, configurable keybindings, mouse support
and the Filters viewport (tranche D). Responsive layout: views ignore terminal
width today and continue to; making them width-aware is a tranche-C concern.
