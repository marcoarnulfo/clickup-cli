# Keymap registry and navigation stack (v1.9, tranche B1) — Design

> Spec for the second tranche of the **v1.9 TUI design system** milestone.
> Covers #131 (deeper goldens), and the keymap-registry and navigation-stack
> halves of #59. Branch: `feat/tui-keymap-nav`.

## 1. Goal

Replace ad-hoc key handling and scattered screen assignments with **one
declared keymap per screen** and **one navigation mechanism**, so that the
footer (#69), the command palette (#71) and user-configurable keybindings
(#82) each have a single place to plug into.

Nothing user-facing changes except one thing, named deliberately in §7: the
Report screen gains `esc`.

## 2. Why this is B1, not all of tranche B

Tranche B was split so the **overlay compositor is born with its first
consumer** rather than as dead code — the same discipline that kept `Warning`,
`Border` and the table styles out of tranche A.

| | Ships | Issues |
|---|---|---|
| **B1 (this spec)** | Deeper goldens, keymap registry, handler migration, navigation stack | #131, #59 (part) |
| B2 | Overlay compositor + footer key-hint bar + `?` help overlay | #59 (rest), #69 |
| B3 | Command palette | #71 |

## 3. Current state (measured, not assumed)

Counts exclude `_test.go` files.

- **97 `case "key":` clauses** across twelve handlers: `log` 17, `entries` 16,
  `rates` 16, `report` 9, `home` 9, `filters` 8, `members` 6, `export` 4,
  `range` 4, `listbrowser` 4, `setup` 3, `budget` 1. (`demo.go`'s two `case`
  clauses match space IDs, not keys.)
- **12 `msg.Type == tea.Key…` comparisons** — `log` 5, `setup` 3, `entries` 2,
  `app` 1, `rates` 1 — mostly inside text-input steps. #82 cannot remap
  `esc`/`enter` inside a form unless these migrate too. One of them,
  `rates.go:472`, is a `KeyRunes` **class** filter for numeric fields, not a key
  match: it cannot become a binding and is carved out explicitly.
- **7 switch-style `case tea.KeyEnter:` arms** — `entries` 2, `range` 3,
  `rates` 2 — which no `msg.Type ==` grep finds. They are hardcoded key dispatch
  for the same reason and migrate too.
- **66 screen transitions**, not 62: 62 literal `.screen = screen…`
  assignments across 13 files (`app` 22, `report` 8, `entries` 6, `home` 6,
  `log` 6, `range` 3, `filters` 2, `members` 2, `listbrowser` 2, `rates` 2,
  `budget` 1, `export` 1, `setup` 1) **plus 4 indirect ones** —
  `listbrowser.go:78` (`= bs.origin`) and `log.go:276`, `:354`, `:483`
  (`= lg.origin`) — which a literal grep misses and which are exactly the
  origin-field reads this tranche removes.
- **Two screens already carry an `origin` field**: `listBrowserModel.origin`
  (entered from Log or Rates) and **`logModel.origin`** (entered from Home or
  Report). Both are ad-hoc navigation state.
- The global quit check (`app.go:601`) is six negative exclusions on one line.
- The test suite is 5,141 lines.

## 4. Decisions

### 4.1 One keymap per screen, derived by a pure function

`keys.go` holds the bindings and one entry point:

```go
func keysFor(m Model) keyMap
```

`keyMap` is a single flat type. Most fields are zero for any given screen, and
that is correct: a zero `key.Binding` has `Enabled() == false` and matches
nothing, which is the same mechanism `keysFor` uses to switch contextual keys
off.

Rejected: a `keymap` field on the `Model` (a second copy of the truth that every
state change must re-sync — the failure mode tranche A avoided by refusing to
put the theme on each sub-model), and per-sub-model `keys()` methods (the
contextual state lives on the root `Model`, so they would need it anyway,
spread over twelve files).

**`keysFor` covers all 14 screens, not the 12 with handlers.** `screenLoading`
and `screenError` have no key handler but do accept `q` today, and once the
global quit check reads `keysFor(m).Quit` a missing case would silently disable
it — leaving Loading, which swallows every other key, with no exit at all.

**Modes are part of the screen's identity.** A screen's accepted labels depend on
its mode or step: `entriesModel.mode` and `tagNewMode`, `logModel.step` and its
`formField` sub-steps, `ratesModel.editing` and the three-step `draft`,
`rangeModel` editing, `setupModel` step, `entriesModel.editStep`. All of that
state is reachable from the root `Model`. One exception is structural:
`ratesModel.updateDraft` has a `ratesModel` receiver and cannot call
`keysFor(m)` — its keymap must be passed in as a parameter.

### 4.2 The keymap carries its own help ordering, from day one

B2 needs `bubbles/help`, whose `KeyMap` interface is `ShortHelp() []key.Binding`
and `FullHelp() [][]key.Binding` — **ordered and grouped per screen**, which a
flat struct of mostly-zero fields cannot derive. So `keyMap` carries the order
explicitly and implements the interface now:

```go
type keyMap struct {
    // every binding this screen may use, contextually enabled
    Quit, Back, Report, Log /* … */ key.Binding

    short []key.Binding   // footer order, this screen
    full  [][]key.Binding // grouped, for the ? overlay
}

func (k keyMap) ShortHelp() []key.Binding  { return k.short }
func (k keyMap) FullHelp() [][]key.Binding { return k.full }
```

Every binding gets its `WithHelp` text **in B1**. The strings already exist —
they are the inline help lines each view renders today. Deferring this to B2
would mean touching all ~100 bindings a second time.

Bindings are defined in a **screen-independent defaults table** and then
contextually enabled by `keysFor`, so #82's config overrides later mutate the
defaults table alone.

### 4.3 Enablement is load-bearing, so it is the only gate

Once `keysFor` disables a binding, `key.Matches` fails and the handler's guard
becomes unreachable. Enablement therefore controls **behavior**, not just what
the footer displays.

Each contextual guard consequently lives in **exactly one place**: the
binding's `Enabled()` state. The inline `if` that used to sit in the handler is
removed when its condition moves into `keysFor`, and the shared predicates —
`canEdit(e, userID)`, team scope, non-empty list, current rates section — are
called from there. A test asserts enablement ⇔ guard for every contextually
gated key.

> **Amended after implementation.** This section originally said handlers would
> keep their guards as defense in depth. They do not, and that is deliberate:
> two copies of a condition are two things to keep in sync, and the duplicate
> is unreachable by construction, so it can drift without any test noticing.
> The `enablement ⇔ guard` tests are what makes one copy safe — they pin the
> predicate against the behavior it gates. Contextual logic that selects
> between two *live* behaviors (Home's `f` branching on whether the roster is
> already loaded, for instance) is not a guard and stays in the handler.

### 4.4 One navigation mechanism, with a truncating stack

`nav []screen` on the `Model`, holding the **parent chain only** — the current
screen stays in `m.screen` and is *not* the top of `nav`. An empty `nav` means
"nowhere to go back to", which is exactly Home's state and makes `pop()` on Home
a safe no-op rather than a special case.

The stack is reached only through a small transition API:

| Call | Meaning |
|---|---|
| `goTo(s)` | forward navigation; pushes |
| `replace(s)` | same logical step, different screen — `screenLoading`, `screenError`, and the async handlers that land on a result screen |
| `pop()` | return to the parent — every `esc`, and every success/apply return |
| `resetTo(s)` | clear the stack — Home, and the 401 setup relaunch |

**Truncating push:** `goTo(s)` where `s` is already in `nav` truncates the stack
above it instead of appending. `nav` therefore never holds a duplicate, its
depth is bounded by the 14 screen constants, and unbounded growth is impossible
by construction — not by remembering to clear at the right moment. It is a
structural invariant, not a rule anyone has to apply.

**The report screen re-roots.** Report is reachable three ways — Home `enter`,
Report `r`, and the logDone `r` reload — and all three arrive through
`entriesMsg`, a `replace` from Loading. Classified naively, they would leave
`nav` in three different states, including empty (making Report's own back keys
no-ops) and `[Home, Report]` with `screen == Report` (making the first `esc` a
visible dead key that returns Report to itself). The `entriesMsg` handler
therefore re-roots explicitly: `resetTo(screenHome).goTo(screenReport)`. That is
exactly today's semantics — Report's back target is unconditionally Home — and
it makes every arrival path converge on the same chain.

**Both `origin` fields are deleted**, because keeping one would reproduce the
two-mechanism ambiguity this decision exists to remove. But
`listBrowserModel.origin` is not purely navigation state: `selectBrowsedList`
(`listbrowser.go:135`) branches on it to decide **which sub-model receives the
chosen list** — a rates row or the log task-pick flow. That routing decision
needs an explicit replacement, not just deletion; the parent screen at the top
of `nav` is the natural discriminator.

### 4.5 Return edges are not only `esc`

"`esc` becomes pop" covers half the return edges. Screens also return on
**success**, by assignment: `filters.go:161` (apply), `members.go:67` (confirm),
`range.go:75,136`, `rates.go:870` (save), `listbrowser.go:151,157` (list chosen),
`log.go:483` (logDone), `budget.go:22` (`b` toggles back), `report.go:72`
(`m`/`s`). Every one becomes `pop()`.

**The plan carries a table classifying all 62 assignment sites** as
goTo / replace / pop / resetTo. A fresh implementer that sees only "esc becomes
pop, Home clears" will classify the async and loading sites wrong.

### 4.6 `screenLoading` and `screenError` are replaces, never pushes

`routeKey` has **no `screenLoading` case**: a screen popped back into Loading
swallows every key except the global quit. Loading is entered from 10+ sites and
left only by async messages. It — and `screenError`, a dead-end whose any-key
handler returns Home — must always `replace`, never `goTo`.

There is no global `esc → pop` interception in `Update`. `screenSetup` has no
`esc` handling at all, and five screens forward unmatched keys into a
`textinput`; a global intercept would break both. `pop()` is wired per screen.

### 4.7 Migration must preserve the input-forwarding order

Five screens forward unmatched keys into a `textinput`, across nine input
contexts (`log` ID-input and form, `rates` editing, `range` editing, `setup`'s
three steps, `entries` edit form and new-tag mode). If a handler is
restructured so `key.Matches` runs
*before* the input-forwarding branch, typed characters start firing actions —
typing "s" into a note field would stop the timer.

The migration therefore **preserves each handler's step/mode dispatch structure
exactly** and substitutes labels in place. `keysFor` disables action bindings in
input modes.

Two patterns stay as `default` clauses and never become bindings:
`entriesConfirmDelete`'s "any other key cancels" and `screenError`'s "press any
key".

### 4.8 The goldens do not protect this work

Goldens verify rendering; this tranche changes input. A wrong `key.Matches`
makes a key silently mute and no golden notices. Green goldens here prove only
that **no layout moved** — which is worth having, and is why #131 comes first,
but it is not evidence the migration is correct.

The real net is:

1. **A per-screen label-parity test.** For each screen, the exact set of key
   labels its bindings declare, asserted against the pre-migration `case` labels.
   This catches transcription errors — the actual failure mode of a mechanical
   migration — at a fraction of the cost of behavioral tests.
2. **~11 targeted transition tests** for action keys with no test today: `report`
   `m`/`s`/`r`/`e`; `entries` list `esc`; `range` `esc` in list mode (→ Home)
   and in editing mode (→ *stays* on Range, closing the editor — a mode change,
   not a navigation); `export` `esc`; `budget` `esc` and `b`-return;
   `logDone` `r`.
3. **A review rule:** every removed `case` label must appear verbatim in a
   `WithKeys`.

Writing a transition test for each of the ~60 untested key labels was
considered and rejected: it is two to three times the migration itself, and most
of it would assert that `j` moves a cursor.

## 5. Task order

1. **Deeper goldens (#131).** Log form, entries edit form, `screenError`, plus
   `t.Run` subtests in `TestGoldenRatesTabs`. No production change.
2. **`keys.go` + the global keys.** The defaults table, `keysFor`, and the quit
   hack replaced by a declared property.
3. **Handler group A** — `home`, `report`, `filters`, `members`, `export`,
   `range`, `listbrowser`, `setup`, `budget`: 48 cases.
4. **Handler group B** — `log`, `entries`, `rates`: 49 cases, plus 9 of the 12
   `msg.Type == tea.Key…` sites (including `app.go`'s `ctrl+c`) and 4 of the 7
   switch-style `case tea.Key…` arms. The remaining 3 and 3 belong to `setup`
   and `range`, which are group A.
5. **Navigation stack.** The transition API, all 66 sites classified, both
   `origin` fields deleted, Report's new `esc`.
6. **Docs.** `CHANGELOG`, both READMEs. The `q` row already lists the full
   exclusion set correctly; what the tables lack is Report's new `esc`, and
   every other row must be re-verified against `keys.go` once it is the single
   source of truth.

Keymap before navigation is deliberate: the mechanical migration is then
provable against unchanged navigation semantics (tests green, goldens
byte-identical), and the semantic change lands afterward on one already
centralized `Back` site per screen.

## 6. Known defects to fix deliberately, not mechanically

- **`case "space"` never fires.** bubbletea maps the space rune to `KeySpace`,
  whose `String()` is `" "`. The `"space"` arm in `filters.go:133`,
  `members.go:54` and `entries.go:234` is dead. Migrate as `WithKeys(" ")` with
  the help text "space", and drop the dead label knowingly so the parity check
  does not flag it.
- **`log.go:314-316` is unreachable**: the guard at `log.go:275` catches `esc`
  for every non-input step first. Delete it, with a comment — migrating it
  mechanically preserves a latent wrong-destination bug.
- **Three async handlers get a staleness guard** — `membersMsg`
  (`app.go:768`), `statusesMsg` (`app.go:780`), `historyMsg` (`app.go:701`) —
  unlike `tagsMsg` and `spacesMsg`, which already have one. Other handlers
  force a screen unguarded too (`logErrMsg`, `logDoneMsg`, `timerStoppedMsg`,
  `entriesReloadedMsg`, `entriesErrMsg`), but they can only arrive from a
  Loading state that swallows keys, so no competing navigation can have
  happened meanwhile. **Guarding these three is the scope; it is not a claim
  that every other handler is already guarded.**
  **The guard is not the same shape for all three.** `tagsMsg`'s works because
  its fetch is dispatched while staying on `screenEntries`; `h` instead
  dispatches from `screenLoading` (`entries.go:167`), so a
  `if m.screen != screenEntries` guard on `historyMsg` would drop every reply
  and strand the user on Loading — which swallows all keys but quit. The
  history guard must accept `screenLoading`.
- **The quit exclusion set must be reproduced exactly.** `screenListBrowser` is
  excluded from `q`-quits despite having no text input, and `entries` is
  excluded even in list mode. "Screens with text inputs" is *not* the rule; an
  implementer inferring it would change behavior. `q` also currently quits from
  `screenError` before its any-key handler runs.

## 7. The one intended behavior change

Report has no `esc` today — going back is `m` or `s`. Under "`esc` = pop
everywhere" it gains `esc` → Home. This is an improvement and is called out
here, in the CHANGELOG, and in the README key table, rather than arriving as a
side effect.

It does **not** arrive by itself: a plan that only classifies existing
transition sites would ship without it, because Report has no `esc` site to
classify. The binding, the handler arm, the parity-test update and a transition
test are an explicit deliverable of the navigation task.

Report's on-screen help line cannot mention the new key: help strings are
rendered, and every golden must stay byte-identical through this tranche. `esc`
becomes discoverable when B2 replaces the inline help lines with the generated
footer — which is precisely what the keymap registry exists to feed.

## 8. Testing and constraints

- `internal/report` and `internal/duration` stay pure and are untouched.
- No new `go.mod` dependencies: `bubbles/key` is already available
  (`bubbles v1.0.0` is a direct dependency).
- All 21 existing goldens plus the new deep ones stay **byte-identical**
  through tasks 2–5. A moved golden means a layout changed, which this tranche
  must not do.
- Demo mode needs no parity work: `demo.go` holds fixtures and commands only, no
  key handlers and no screen transitions.
- Every transition routes through the API. The plan's verification step greps
  for surviving `\.screen = screen` assignments outside `nav.go`; the result
  must be empty.
- bubbletea value receivers; explicit write-back before every return.
- Pre-commit gate: `gofmt -l .`, `go vet ./...`,
  `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`,
  `go test ./... -race` — all clean/green.
- Everything in the repo in ENGLISH except `README.it.md` and
  `CONTRIBUTING.it.md`; Conventional Commits; **no `Co-Authored-By` trailer**.

## 9. Out of scope

The overlay compositor, the footer key-hint bar and the `?` overlay (B2, #69).
The command palette (B3, #71). User-configurable keybindings and YAML themes
(tranche D, #82) — B1 only shapes the defaults table so that work stays cheap.
Mouse support (#74, tranche D). Making views width-aware: `help.Model` needs the
terminal width, but that dependency arrives with B2.
