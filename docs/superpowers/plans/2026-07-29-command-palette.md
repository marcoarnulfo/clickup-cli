# Command palette + overlay compositor — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the TUI a `ctrl+p` fuzzy command palette that floats over the current
screen, and the line compositor that makes floating possible (#71, #59's last checkbox).

**Architecture:** A pure `internal/fuzzy` package scores the query. A pure `composite()`
splices a rendered box into the body's lines, cell by cell, ANSI-aware. An `overlayKind`
field on `Model` sits orthogonal to `m.screen` and takes the keyboard while open. The
palette's rows come from two sources — the current screen's keymap, derived not
duplicated, and an explicit `"Go to X"` navigation list whose targets share one set of
`openX()` methods with the key handlers.

**Tech Stack:** Go 1.26.5, bubbletea v1.3.10, bubbles v1.0.0, lipgloss v1.1.0,
`github.com/charmbracelet/x/ansi` v0.11.6 (promoted from indirect to direct).

**Spec:** `docs/superpowers/specs/2026-07-29-command-palette-overlay-design.md`. When this
plan and the spec disagree, stop and ask — do not pick one.

## Global Constraints

- **Everything committed to the repo is in English**, American spelling — code,
  identifiers, comments, UI strings, test names and messages, commit messages. The spec
  is in Italian; that is the historical exception for `docs/superpowers/`.
- **Never add a `Co-Authored-By` trailer to a commit message.** Not once, not anywhere.
- **Conventional Commits** for every commit message.
- **No new dependency** beyond promoting `github.com/charmbracelet/x/ansi` from indirect
  to direct in `go.mod`. Do not add `sahilm/fuzzy` or any other module.
- `internal/report`, `internal/duration` and the new `internal/fuzzy` stay **pure**: no
  I/O, nothing outside the standard library, no import of `internal/config`,
  `internal/clickup` or `internal/tui`.
- **Never call the real ClickUp API.** No credentials exist here. Use `httptest` and set
  `client.BaseURL` (see `internal/tui/app_test.go:86-96` for the pattern).
- **No style function calls `lipgloss.NewStyle()`.** Every style comes from the `theme`
  argument, which carries the injected renderer. `th.Cell` exists precisely so a plain
  cell has a style to use.
- **Never use `th.Title` inside a box row.** It carries `MarginBottom(1)` and would inject
  a blank line into the middle of the overlay.
- **Pre-commit gate, all five, all clean, every commit:**
  `gofmt -l .` (must print nothing) · `go vet ./...` ·
  `go run honnef.co/go/tools/cmd/staticcheck@latest ./...` · `go build ./...` ·
  `go test ./... -race`
- **Goldens are regenerated with `go test ./internal/tui -update`, never hand-edited.**
- **Goldens are blind to color.** `TestMain` (`golden_test.go:26`) pins the default
  renderer to `termenv.Ascii`, which strips backgrounds *and* bold. Every color decision
  needs a direct assertion on the returned `lipgloss.Style` or on real escape bytes from a
  locally built renderer.
- **Display width ≠ rune count.** They agree only in ASCII. Truncate through
  `shaveToWidth` (`internal/tui/report_table.go:250`), never by slicing runes.
- **A test written against a specific bug must be verified failing against that bug.**
  Where a step says so, break the implementation the stated way, run the test, paste the
  failing output into your report, then restore. Four tests in the previous tranche were
  caught passing against the very bug they existed to catch.

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/fuzzy/fuzzy.go` (new) | `Match` — case-insensitive subsequence scoring with matched rune indices. Pure. |
| `internal/tui/overlay.go` (new) | `composite` — splice a rendered box into a body's lines, ANSI-aware. |
| `internal/tui/open.go` (new) | The six screen-opening `Model` methods shared by the key handlers and the palette. |
| `internal/tui/actions.go` (new) | `action`, `screenActions`, `globalActions`, `paletteActions`, `keyMsgFor`, `capitalize`. |
| `internal/tui/palette.go` (new) | `paletteModel`, its update loop, its geometry and its rendering. |
| `internal/tui/keys.go` | The `Palette`/`PaletteUp`/`PaletteDown` defaults, `paletteKeys`, `paletteBindings`, and the `keysFor`/`screenKeys` split. |
| `internal/tui/app.go` | `overlayKind`, the two `Model` fields, the `Update` key ordering, `View()` composition. |

`openListBrowser` (`app.go:533`), `openEntries` (`entries.go:133`) and `openBudgetView`
(`report.go:135`) already exist in this shape and **stay where they are**. Moving them into
`open.go` would inflate the diff for no behavioural gain; `open.go`'s doc comment names
them instead. `openEntries` is the one exception — its signature changes in Task 3.

---

## Task 1: `internal/fuzzy`

**Files:**
- Create: `internal/fuzzy/fuzzy.go`
- Create: `internal/fuzzy/fuzzy_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `func Match(query, target string) (score int, idx []int, ok bool)`. `idx` holds
  **rune** indices into `target`, ascending, one per query rune. An empty query returns
  `(0, nil, true)`.

- [ ] **Step 1: Write the failing tests**

Create `internal/fuzzy/fuzzy_test.go`:

```go
package fuzzy

import (
	"slices"
	"testing"
)

func TestMatch(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		query   string
		target  string
		wantOK  bool
		wantIdx []int
	}{
		{"empty query matches anything", "", "Export report", true, nil},
		{"empty query matches an empty target", "", "", true, nil},
		{"query longer than target", "exports", "exp", false, nil},
		{"no match at all", "zz", "Export report", false, nil},
		{"exact prefix", "exp", "Export report", true, []int{0, 1, 2}},
		{"query upper, target lower", "EXP", "export report", true, []int{0, 1, 2}},
		{"query lower, target upper", "exp", "EXPORT REPORT", true, []int{0, 1, 2}},
		{"subsequence with gaps", "ert", "Export report", true, []int{0, 4, 5}},
		// Greedy matching returns [0, 5] here: r at 0, then the first t, the one
		// inside "report" — highlighting repor[t]. The word-start bonus is what
		// makes [0, 7] win instead: [r]eport [t]able. This row is the reason
		// Match is a dynamic program and not a left-to-right scan.
		{"prefers a word start over the nearest rune", "rt", "report table", true, []int{0, 7}},
		// idx is in RUNES. With byte offsets this row would be [0, 5].
		{"multibyte target", "cà", "città", true, []int{0, 4}},
		{"whole target", "abc", "abc", true, []int{0, 1, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, idx, ok := Match(tc.query, tc.target)
			if ok != tc.wantOK {
				t.Fatalf("Match(%q, %q) ok = %v, want %v", tc.query, tc.target, ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if !slices.Equal(idx, tc.wantIdx) {
				t.Errorf("Match(%q, %q) idx = %v, want %v", tc.query, tc.target, idx, tc.wantIdx)
			}
		})
	}
}

// TestMatchScoreIsExact pins the arithmetic on the smallest case that exercises
// both bonuses, so the weights cannot drift unnoticed while the orderings in
// TestMatchRanks still happen to hold.
func TestMatchScoreIsExact(t *testing.T) {
	t.Parallel()
	// a at 0: word start (+8), no lead penalty. b at 1: consecutive (+10).
	score, _, ok := Match("ab", "ab")
	if !ok {
		t.Fatal(`Match("ab", "ab") did not match`)
	}
	if score != 18 {
		t.Errorf("score = %d, want 18", score)
	}
}

// TestMatchRanks pins the orderings the palette depends on. These survive a
// tweak to the weights; TestMatchScoreIsExact does not. That split is
// deliberate: the constants are tunable, the orderings are not.
func TestMatchRanks(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		query  string
		better string
		worse  string
	}{
		{"a screen action beats the navigation row", "exp", "Export report", "Go to export"},
		{"a prefix beats a match buried inside", "rep", "Report", "Go to prep"},
		{"consecutive beats scattered", "ab", "ab c", "a x b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, _, ok := Match(tc.query, tc.better)
			if !ok {
				t.Fatalf("Match(%q, %q) did not match", tc.query, tc.better)
			}
			w, _, ok := Match(tc.query, tc.worse)
			if !ok {
				t.Fatalf("Match(%q, %q) did not match", tc.query, tc.worse)
			}
			if b <= w {
				t.Errorf("Match(%q, %q) = %d, want more than Match(%q, %q) = %d",
					tc.query, tc.better, b, tc.query, tc.worse, w)
			}
		})
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/fuzzy/`
Expected: FAIL — the package has no non-test file, so `Match` is undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/fuzzy/fuzzy.go`:

```go
// Package fuzzy scores a query against a candidate string the way a command
// palette needs it: case-insensitive subsequence matching that also reports
// which runes matched, so the caller can highlight them.
//
// The package is pure — no I/O, nothing outside the standard library — for the
// same reason internal/report and internal/duration are: the interesting part
// is an algorithm, and an algorithm is worth testing without a Model.
package fuzzy

import (
	"math"
	"unicode"
)

// Scoring weights. See the spec's section 7.2.
const (
	// consecutiveBonus rewards a rune that immediately follows the previous match.
	consecutiveBonus = 10
	// boundaryBonus rewards a match that starts a word.
	boundaryBonus = 8
	// maxLeadPenalty caps "how far into the target the match starts". Without a
	// cap, a long label loses to a worse match that merely starts further left.
	maxLeadPenalty = 10
)

// noMatch marks an unreachable cell of the table. It is halved so that adding a
// bonus to it can never wrap around into a plausible score.
const noMatch = math.MinInt / 2

// Match reports whether query matches target as a case-insensitive
// subsequence. When ok, score ranks the match and idx holds the rune indices of
// target that were matched, in ascending order, one per query rune.
//
// An empty query always matches, with score 0 and a nil idx.
//
// The search is exhaustive rather than greedy because idx is not only a ranking
// input: it decides which runes the caller highlights. Greedy matching of "rt"
// against "report table" yields [0, 5] — repor[t] — where the right answer is
// [0, 7], [r]eport [t]able.
func Match(query, target string) (score int, idx []int, ok bool) {
	q := lowerRunes(query)
	if len(q) == 0 {
		return 0, nil, true
	}
	t := lowerRunes(target)
	if len(q) > len(t) {
		return 0, nil, false
	}

	// best[j][i] is the highest score for matching q[:j+1] with q[j] landing on
	// t[i]. from[j][i] is the target index q[j-1] landed on along that path,
	// which is what lets idx be reconstructed at the end.
	best := make([][]int, len(q))
	from := make([][]int, len(q))
	for j := range best {
		best[j] = make([]int, len(t))
		from[j] = make([]int, len(t))
		for i := range best[j] {
			best[j][i] = noMatch
			from[j][i] = -1
		}
	}

	for i := range t {
		if t[i] != q[0] {
			continue
		}
		best[0][i] = boundaryScore(t, i) - min(i, maxLeadPenalty)
	}
	for j := 1; j < len(q); j++ {
		for i := j; i < len(t); i++ {
			if t[i] != q[j] {
				continue
			}
			for p := j - 1; p < i; p++ {
				if best[j-1][p] == noMatch {
					continue
				}
				s := best[j-1][p] + boundaryScore(t, i)
				if p == i-1 {
					s += consecutiveBonus
				}
				// Strictly greater keeps the leftmost path on a tie, which is
				// what makes the result deterministic.
				if s > best[j][i] {
					best[j][i], from[j][i] = s, p
				}
			}
		}
	}

	last := len(q) - 1
	end := -1
	score = noMatch
	for i := range t {
		if best[last][i] > score {
			end, score = i, best[last][i]
		}
	}
	if end < 0 {
		return 0, nil, false
	}
	idx = make([]int, len(q))
	for j, i := last, end; j >= 0; j-- {
		idx[j] = i
		i = from[j][i]
	}
	return score, idx, true
}

// boundaryScore is the word-start bonus for t[i]: index 0, or a rune that
// follows a separator.
func boundaryScore(t []rune, i int) int {
	if i == 0 {
		return boundaryBonus
	}
	switch t[i-1] {
	case ' ', '-', '_', '/', '.':
		return boundaryBonus
	}
	return 0
}

// lowerRunes decomposes s into lower-cased runes. Indices into the result are
// rune indices, which is what Match reports and what a caller highlighting
// characters needs — byte offsets would be wrong for any accented label.
func lowerRunes(s string) []rune {
	r := []rune(s)
	for i := range r {
		r[i] = unicode.ToLower(r[i])
	}
	return r
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/fuzzy/ -v`
Expected: PASS, every subtest.

- [ ] **Step 5: Verify the greedy-discrimination test actually discriminates**

The `"rt"` / `"report table"` row exists to catch greedy matching. Prove it does.

Temporarily replace the body of `Match` (keeping the empty-query and length guards) with a
greedy scan:

```go
	idx = make([]int, 0, len(q))
	i := 0
	for _, qr := range q {
		for i < len(t) && t[i] != qr {
			i++
		}
		if i == len(t) {
			return 0, nil, false
		}
		idx = append(idx, i)
		i++
	}
	return 0, idx, true
```

Run: `go test ./internal/fuzzy/ -run 'TestMatch/prefers_a_word_start' -v`
Expected: **FAIL**, reporting `idx = [0 5], want [0 7]`.

Paste that failing output into your report, then restore the real implementation and
re-run `go test ./internal/fuzzy/` to confirm it is green again.

- [ ] **Step 6: Gate and commit**

```bash
gofmt -l .
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go build ./...
go test ./... -race
git add internal/fuzzy
git commit -m "feat(fuzzy): add a pure subsequence matcher with matched-rune indices"
```

---

## Task 2: The overlay compositor

**Files:**
- Create: `internal/tui/overlay.go`
- Create: `internal/tui/overlay_test.go`
- Modify: `go.mod` (promote `github.com/charmbracelet/x/ansi` to a direct require)

**Interfaces:**
- Consumes: nothing from Task 1.
- Produces: `func composite(body, box string, x, y int) string`. `x`/`y` are terminal
  cells. Returns the body with the box's lines spliced in, the body preserved verbatim
  outside the box's rectangle.

**Measured behaviour of `ansi.Cut` you must rely on** (probed against x/ansi v0.11.6 —
these are facts, not guesses, and the implementation below depends on all four):

| Call on `"\x1b[31mHELLO\x1b[0m world"` | Result |
|---|---|
| `Cut(s, 0, 3)` | `"\x1b[31mHEL\x1b[0m"` — closes the style it opened |
| `Cut(s, 3, 11)` | `"\x1b[31mLO\x1b[0m world"` — re-emits the active style at the cut |
| `Cut(s, 8, 3)` | `""` — `left > right` is not a panic |
| `Cut(s, 20, 30)` | `"\x1b[31m\x1b[0m"` — **not empty** past the end: a zero-width escape pair |
| `Cut("\x1b[31mHELLO world", 0, 3)` | `"\x1b[31mHEL"` — on **unterminated** input it does NOT close |

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/overlay_test.go`:

```go
package tui

import (
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestCompositeSplicesTheBoxIntoTheBody(t *testing.T) {
	t.Parallel()
	body := "abcdefghij\nklmnopqrst\nuvwxyz1234"
	got := composite(body, "[]\n[]", 3, 1)
	want := "abcdefghij\nklm[]pqrst\nuvw[]z1234"
	if got != want {
		t.Errorf("composite =\n%q\nwant\n%q", got, want)
	}
}

// A body line shorter than x must gain spaces. Without the padding the box
// slides left on that line alone, which reads as a broken border.
func TestCompositePadsShortBodyLines(t *testing.T) {
	t.Parallel()
	got := composite("abcdefghij\nkl", "[]", 5, 1)
	want := "abcdefghij\nkl   []"
	if got != want {
		t.Errorf("composite =\n%q\nwant\n%q", got, want)
	}
}

// A body shorter than y+height must gain blank lines, or the box is clipped at
// the bottom — the Home screen's body is three lines and the palette is ten.
func TestCompositeExtendsAShortBody(t *testing.T) {
	t.Parallel()
	got := composite("abc", "[]\n[]", 0, 2)
	want := "abc\n\n[]\n[]"
	if got != want {
		t.Errorf("composite =\n%q\nwant\n%q", got, want)
	}
}

func TestCompositeBoxWiderThanBody(t *testing.T) {
	t.Parallel()
	got := composite("ab", "XXXXXX", 0, 0)
	if got != "XXXXXX" {
		t.Errorf("composite = %q, want %q", got, "XXXXXX")
	}
}

func TestCompositeKeepsEveryLineWidth(t *testing.T) {
	t.Parallel()
	body := strings.Repeat("x", 40) + "\n" + strings.Repeat("y", 40)
	for _, l := range strings.Split(composite(body, "AAAA\nBBBB", 10, 0), "\n") {
		if w := lipgloss.Width(l); w != 40 {
			t.Errorf("line %q is %d cells wide, want 40", l, w)
		}
	}
}

// The one failure mode a golden can never see. TestMain pins the DEFAULT
// renderer to termenv.Ascii, so golden output carries no escapes at all; this
// builds its own renderer with a real profile and asserts on the bytes.
//
// The trailing reset is stripped on purpose: an unterminated style is the one
// shape ansi.Cut does not close on its own, so it is the only shape whose color
// can bleed into the box.
func TestCompositeDoesNotLeakStyleIntoTheBox(t *testing.T) {
	t.Parallel()
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.ANSI)
	styled := r.NewStyle().Foreground(lipgloss.Color("1")).Render("HELLO world")
	open := strings.TrimSuffix(styled, "\x1b[0m")
	if !strings.Contains(open, "\x1b[") {
		t.Fatalf("the fixture carries no escape sequence (%q); this test would pass vacuously", open)
	}

	got := composite(open, "[]", 3, 0)
	i := strings.Index(got, "[]")
	if i < 0 {
		t.Fatalf("the box is missing from the composited line: %q", got)
	}
	if !strings.HasSuffix(got[:i], ansi.ResetStyle) {
		t.Errorf("the box is not preceded by a style reset, so the body's color bleeds into it: %q", got)
	}
}

// Past the end of a line ansi.Cut returns a zero-width escape pair rather than
// "", so the right-hand segment must be skipped by construction instead of
// being cut and discovered empty.
func TestCompositeAddsNoTrailingEscapes(t *testing.T) {
	t.Parallel()
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.ANSI)
	body := r.NewStyle().Foreground(lipgloss.Color("2")).Render("abcd")
	got := composite(body, "XX", 2, 0)
	if !strings.HasSuffix(got, "XX") {
		t.Errorf("composite = %q, want it to end with the box, with nothing appended", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/tui/ -run TestComposite`
Expected: FAIL to build — `composite` is undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/tui/overlay.go`:

```go
package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// composite draws box over body with the box's top-left cell at (x, y) and
// returns the combined text. Cells of body outside the box's rectangle survive
// verbatim, styling included — that layering is the whole point of #59, and it
// is what separates an overlay from a screen that replaced another one.
//
// x and y are terminal cells, not bytes and not runes. ansi.Cut measures in
// cells and never splits an escape sequence, which is what makes this safe to
// run over output lipgloss has already styled.
//
// Two shapes need help. A body line shorter than x gains spaces, or the box
// slides left on that line alone. A body shorter than y+height gains blank
// lines, or the box is clipped at the bottom — the Home screen's body is three
// lines and the palette is ten.
func composite(body, box string, x, y int) string {
	if box == "" {
		return body
	}
	x = max(x, 0)
	y = max(y, 0)

	boxLines := strings.Split(box, "\n")
	boxW := 0
	for _, l := range boxLines {
		boxW = max(boxW, lipgloss.Width(l))
	}

	lines := strings.Split(body, "\n")
	for len(lines) < y+len(boxLines) {
		lines = append(lines, "")
	}

	for i, bl := range boxLines {
		row := lines[y+i]
		rowW := lipgloss.Width(row)

		left := ansi.Cut(row, 0, x)
		if pad := x - lipgloss.Width(left); pad > 0 {
			left += strings.Repeat(" ", pad)
		}
		// ansi.Cut closes a style it had to reopen, but leaves an already-open,
		// unterminated one open (measured against x/ansi v0.11.6). Without this
		// reset such a line paints the box in its own color.
		if strings.ContainsRune(left, '\x1b') {
			left += ansi.ResetStyle
		}

		// Skipped rather than cut-and-discarded when the box reaches the end of
		// the line: past the end ansi.Cut returns a zero-width escape pair, not
		// an empty string, and every composited line would collect one.
		right := ""
		if x+boxW < rowW {
			right = ansi.Cut(row, x+boxW, rowW)
		}

		lines[y+i] = left + bl + right
	}
	return strings.Join(lines, "\n")
}
```

- [ ] **Step 4: Promote x/ansi to a direct dependency**

Run: `go mod tidy`
Expected: `github.com/charmbracelet/x/ansi v0.11.6` moves out of the `// indirect` block in
`go.mod`. No version changes, no new modules, `go.sum` unchanged. If `go mod tidy` wants to
add or bump anything else, stop and report it.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/tui/ -run TestComposite -v`
Expected: PASS, every test.

- [ ] **Step 6: Verify three tests fail against the bug each one exists to catch**

Run each break, capture the failing output, restore, and paste all three transcripts into
your report.

1. Delete the `if pad := ...` block.
   Run: `go test ./internal/tui/ -run TestCompositePadsShortBodyLines -v` → must FAIL.
2. Delete the `for len(lines) < y+len(boxLines)` loop.
   Run: `go test ./internal/tui/ -run TestCompositeExtendsAShortBody -v` → must FAIL
   (panic on the out-of-range index is an acceptable failure here — say so in the report).
3. Delete the `if strings.ContainsRune(left, '\x1b')` block.
   Run: `go test ./internal/tui/ -run TestCompositeDoesNotLeakStyleIntoTheBox -v` → must FAIL.

Restore the implementation and re-run `go test ./internal/tui/ -run TestComposite`.

- [ ] **Step 7: Gate and commit**

```bash
gofmt -l .
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go build ./...
go test ./... -race
git add internal/tui/overlay.go internal/tui/overlay_test.go go.mod
git commit -m "feat(tui): add an ANSI-aware line compositor for overlays (#59)"
```

---

## Task 3: Extract the screen-opening methods

**Files:**
- Create: `internal/tui/open.go`
- Create: `internal/tui/open_test.go`
- Modify: `internal/tui/report.go:92-125` (the `Export`, `Rates`, `LogHours`, `Filters`,
  `OpenEntries` cases of `updateReport`)
- Modify: `internal/tui/home.go:54-87` (the `Range`, `LogHours`, `Members` cases of
  `updateHome`)
- Modify: `internal/tui/entries.go:133-137` (`openEntries`' signature)
- Modify: `internal/tui/app.go:504` (`loadMembersCmd`'s doc comment)

**Interfaces:**
- Consumes: nothing from Tasks 1-2.
- Produces:
  - `func (m Model) openExport() Model`
  - `func (m Model) openRates() Model`
  - `func (m Model) openLog() Model`
  - `func (m Model) openRange() Model`
  - `func (m Model) openFilters() (Model, tea.Cmd)`
  - `func (m Model) openMembers(origin screen) (Model, tea.Cmd)`
  - `func (m Model) openEntries() (Model, tea.Cmd)` — **signature change**, was `Model`

This is a behaviour-preserving refactor. Task 5's global actions call these methods, and
without the extraction they would have to copy the handler bodies — the second source of
truth the whole design avoids.

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/open_test.go`:

```go
package tui

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
)

// openBase is a Model on the report screen with entries and a built report —
// the state every one of these methods is reached from in the real app.
func openBase() Model {
	m := newTestModelOnReport()
	m.report = goldenReport()
	return m
}

func TestOpenMethodsBuildTheirScreen(t *testing.T) {
	t.Parallel()
	t.Run("export", func(t *testing.T) {
		t.Parallel()
		m := openBase().openExport()
		if m.screen != screenExport {
			t.Errorf("screen = %v, want screenExport", m.screen)
		}
		if reflect.DeepEqual(m.export, exportModel{}) {
			t.Error("openExport left the export sub-model at its zero value")
		}
		if len(m.nav) == 0 || m.nav[len(m.nav)-1] != screenReport {
			t.Errorf("nav = %v, want screenReport on top so esc goes back", m.nav)
		}
	})
	t.Run("rates", func(t *testing.T) {
		t.Parallel()
		m := openBase().openRates()
		if m.screen != screenRates {
			t.Errorf("screen = %v, want screenRates", m.screen)
		}
		if reflect.DeepEqual(m.ratesScreen, ratesModel{}) {
			t.Error("openRates left the rates sub-model at its zero value")
		}
	})
	t.Run("log", func(t *testing.T) {
		t.Parallel()
		m := openBase().openLog()
		if m.screen != screenLog {
			t.Errorf("screen = %v, want screenLog", m.screen)
		}
		if reflect.DeepEqual(m.logScreen, logModel{}) {
			t.Error("openLog left the log sub-model at its zero value")
		}
	})
	t.Run("range", func(t *testing.T) {
		t.Parallel()
		m := openBase().openRange()
		if m.screen != screenRange {
			t.Errorf("screen = %v, want screenRange", m.screen)
		}
		if reflect.DeepEqual(m.rangeScreen, rangeModel{}) {
			t.Error("openRange left the range sub-model at its zero value")
		}
	})
	t.Run("entries", func(t *testing.T) {
		t.Parallel()
		m, _ := openBase().openEntries()
		if m.screen != screenEntries {
			t.Errorf("screen = %v, want screenEntries", m.screen)
		}
	})
}

// openEntries absorbed the lazy user fetch that used to live in updateReport's
// OpenEntries case. Without it here, the palette's "Go to entries" would have
// to copy that branch.
func TestOpenEntriesFetchesTheUserWhenUnknown(t *testing.T) {
	t.Parallel()
	m := openBase()
	m.userID = 0
	if _, cmd := m.openEntries(); cmd == nil {
		t.Error("openEntries returned no command with an unknown user; ownership gating stays off forever")
	}
	m.userID = 42
	if _, cmd := m.openEntries(); cmd != nil {
		t.Error("openEntries re-fetched a user it already knows")
	}
}

func TestOpenFiltersOpensImmediatelyWhenStatusesAreCached(t *testing.T) {
	t.Parallel()
	m := openBase()
	m.taskStatus = map[string]string{}
	for _, e := range m.entries {
		m.taskStatus[e.TaskID] = "open"
	}
	got, cmd := m.openFilters()
	if got.screen != screenFilters {
		t.Errorf("screen = %v, want screenFilters", got.screen)
	}
	if cmd != nil {
		t.Error("openFilters issued an enrichment command although every status was cached")
	}
}

// openMembers takes an origin because that value decides where a load failure
// lands. Pinned to screenHome, a failure raised from the palette on another
// screen would teleport the user Home and blame a screen they were not on.
func TestOpenMembersRoutesFailuresBackToItsOrigin(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	m := openBase()
	m.client = clickup.New("tok")
	m.client.BaseURL = srv.URL
	m.teamMembers = nil // force the fetch

	_, cmd := m.openMembers(screenRates)
	if cmd == nil {
		t.Fatal("openMembers issued no command with no members cached")
	}
	msg, ok := cmd().(retryableErrMsg)
	if !ok {
		t.Fatalf("command produced %T, want retryableErrMsg", cmd())
	}
	if msg.origin != screenRates {
		t.Errorf("origin = %v, want screenRates — the caller's screen, not a hardcoded Home", msg.origin)
	}
}

// The key handlers must go through the same methods, or the palette and the
// keyboard drift apart.
func TestReportAndHomeKeysReachTheSameScreens(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		key  string
		from func() Model
		want screen
	}{
		{"report export", "e", openBase, screenExport},
		{"report rates", "p", openBase, screenRates},
		{"report log hours", "n", openBase, screenLog},
		{"home range", "d", func() Model { m := newTestModel(); m.screen = screenHome; return m }, screenRange},
		{"home log hours", "n", func() Model { m := newTestModel(); m.screen = screenHome; return m }, screenLog},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := tc.from()
			var got tea.Model
			if m.screen == screenReport {
				got, _ = m.updateReport(keyMsg(tc.key))
			} else {
				got, _ = m.updateHome(keyMsg(tc.key))
			}
			if s := got.(Model).screen; s != tc.want {
				t.Errorf("screen = %v, want %v", s, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestOpen|TestReportAndHome'`
Expected: FAIL to build — `openExport`, `openRates`, `openLog`, `openRange`, `openFilters`
and `openMembers` are undefined, and `openEntries` returns one value.

- [ ] **Step 3: Create `open.go`**

```go
package tui

import tea "github.com/charmbracelet/bubbletea"

// The screen-opening surface: every method here builds a screen's sub-model and
// navigates to it, so a key handler and the command palette (#71) open a screen
// the same way instead of each knowing how.
//
// Three older members of this family live elsewhere and stay there — moving
// them would inflate a diff for no behavioural gain: openListBrowser in app.go,
// openEntries in entries.go, openBudgetView in report.go.

// openExport builds the export screen for the current report.
func (m Model) openExport() Model {
	m.export = newExport(m.report)
	return m.goTo(screenExport)
}

// openRates builds the per-list/per-member rates editor.
func (m Model) openRates() Model {
	m.ratesScreen = newRates(m.entries, m.cfg)
	return m.goTo(screenRates)
}

// openLog builds the hour-logging flow at its first step.
func (m Model) openLog() Model {
	m.logScreen = newLog(m.entries, m.cfg)
	return m.goTo(screenLog)
}

// openRange builds the range/preset picker.
func (m Model) openRange() Model {
	m.rangeScreen = newRange(m.preset)
	return m.goTo(screenRange)
}

// openFilters opens the client-side filter screen, enriching task statuses
// first when any are missing. The demo branch keeps the zero-I/O rule.
func (m Model) openFilters() (Model, tea.Cmd) {
	missing := m.tasksMissingStatus()
	if len(missing) == 0 {
		m.assignStatuses()
		m.filtersScreen = newFilters(m.entries, m.filterLists, m.filterTags, m.filterStatuses, m.filterBillable)
		return m.goTo(screenFilters), nil
	}
	m.filtersScreen = filtersModel{loadingStatuses: true}
	m = m.goTo(screenFilters)
	if m.demo {
		return m, demoStatusEnrichCmd(m.entries)
	}
	return m, statusEnrichCmd(m.client, missing)
}

// openMembers opens the member selector, fetching the workspace roster when it
// is not cached yet.
//
// origin is where a load failure returns to. It is a parameter rather than a
// constant screenHome because the command palette can open this screen from
// anywhere: attributing a failure to Home while the user was on Rates would
// both lose their place and misname the culprit.
func (m Model) openMembers(origin screen) (Model, tea.Cmd) {
	if len(m.teamMembers) > 0 {
		m.membersScreen = newMembers(m.teamMembers, m.selectedMembers)
		return m.goTo(screenMembers), nil
	}
	m.membersScreen = membersModel{loading: true}
	m = m.goTo(screenMembers)
	if m.demo {
		return m, demoMembersCmd()
	}
	return m, loadMembersCmd(m.client, m.cfg.WorkspaceID, origin)
}
```

- [ ] **Step 4: Change `openEntries`' signature**

In `internal/tui/entries.go:133-137`, replace the whole function with:

```go
// openEntries opens the time-entry browser over the currently visible entries.
//
// It also issues the lazy currentUserCmd retry when the authenticated user is
// still unknown: ownership gating (which entries can be edited or deleted)
// stays off until that lands, and this is the only screen that needs it. That
// retry used to live in updateReport's OpenEntries case; it moved here so the
// command palette's "Go to entries" cannot forget it.
func (m Model) openEntries() (Model, tea.Cmd) {
	m.entriesScreen = entriesModel{entries: sortEntriesByStartDesc(m.visibleEntries())}
	m = m.goTo(screenEntries)
	if m.userID == 0 {
		return m, m.currentUserCmd()
	}
	return m, nil
}
```

- [ ] **Step 5: Point the handlers at the new methods**

In `internal/tui/report.go`, inside `updateReport`, replace these five cases (match by
text, not by line number — earlier edits in this task shift them):

```go
	case key.Matches(msg, k.Export):
		m = m.openExport()
	case key.Matches(msg, k.Rates):
		m = m.openRates()
	case key.Matches(msg, k.LogHours):
		m = m.openLog()
	case key.Matches(msg, k.Filters):
		return m.openFilters()
	case key.Matches(msg, k.Budget):
		if !m.openBudgetView() {
			return m, nil
		}
	case key.Matches(msg, k.OpenEntries):
		return m.openEntries()
```

`openFilters` and `openEntries` return `(Model, tea.Cmd)` while the enclosing function
returns `(tea.Model, tea.Cmd)`; Go does not convert a multi-value return, so write them out:

```go
	case key.Matches(msg, k.Filters):
		mm, cmd := m.openFilters()
		return mm, cmd
	case key.Matches(msg, k.OpenEntries):
		mm, cmd := m.openEntries()
		return mm, cmd
```

In `internal/tui/home.go`, inside `updateHome`, replace three cases:

```go
	case key.Matches(msg, k.Range):
		m = m.openRange()
		return m, nil
	...
	case key.Matches(msg, k.LogHours):
		m = m.openLog()
	...
	case key.Matches(msg, k.Members):
		mm, cmd := m.openMembers(screenHome)
		return mm, cmd
```

Leave the `Timer` case alone: it builds a log screen in a *different* step
(`logTimerRunning`), so it is not the same open.

Update `loadMembersCmd`'s doc comment in `internal/tui/app.go:504`: it says "It's Home-only
today, so origin is always screenHome at the call site", which stops being true here.
Replace that sentence with: "origin is the screen to return a failure to — Home for the
key binding, the caller's own screen when the command palette opens it."

- [ ] **Step 6: Find every stale call site**

Run: `go vet ./...`
Expected: the compiler names any remaining single-value use of `openEntries`. `go build
./...` alone will not point at test files; `go vet` does. Fix each one it names.

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test ./internal/tui/ -run 'TestOpen|TestReportAndHome' -v`
Expected: PASS.

Run: `go test ./... -race`
Expected: PASS. This refactor is behaviour-preserving, so the pre-existing suite is its
real safety net — a single failure here means the extraction changed something.

- [ ] **Step 8: Gate and commit**

```bash
gofmt -l .
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go build ./...
go test ./... -race
git add internal/tui/open.go internal/tui/open_test.go internal/tui/report.go internal/tui/home.go internal/tui/entries.go internal/tui/app.go
git commit -m "refactor(tui): extract the screen-opening methods the palette will share"
```

---

## Task 4: Keymap plumbing

**Files:**
- Modify: `internal/tui/app.go` (add `overlayKind` and two `Model` fields)
- Modify: `internal/tui/keys.go` (three new defaults, one new `keyMap` field,
  `paletteKeys`, `paletteBindings`, the `keysFor`/`screenKeys` split)
- Modify: `internal/tui/keys_test.go` (extend the existing parity tests)
- Modify: `internal/tui/testdata/footer_*_full.golden` (regenerated)

**Interfaces:**
- Consumes: nothing from Tasks 1-3.
- Produces:
  - `type overlayKind int`, with `overlayNone` and `overlayPalette`
  - `Model.overlay overlayKind` and `Model.palette paletteModel` — **`paletteModel` is
    defined in Task 6.** For this task declare it as an empty placeholder struct in
    `palette.go`; Task 6 fills it in.
  - `func screenKeys(m Model) keyMap` — today's `keysFor` body, verbatim
  - `func keysFor(m Model) keyMap` — the overlay-aware wrapper
  - `func paletteKeys(d keyDefaults) keyMap`
  - `func (k keyMap) paletteBindings() []key.Binding`

At the end of this task `ctrl+p` is **advertised but inert**: nothing opens the palette
until Task 6. That is expected and is not a defect to report.

- [ ] **Step 1: Add the overlay type and the Model fields**

In `internal/tui/app.go`, after the `screen` constant block:

```go
// overlayKind is the floating layer drawn over the current screen, orthogonal
// to m.screen: opening one does not touch m.nav and closing one is not a pop().
// An overlay is not a place you navigated to.
//
// There are two values because there is one client. The third value arrives
// with the third client, not before.
type overlayKind int

const (
	overlayNone overlayKind = iota
	overlayPalette
)
```

Add to the `Model` struct, right after the `helpAll` field:

```go
	// overlay is the floating layer over m.screen, and palette is its state
	// when overlay == overlayPalette (#71). While an overlay is open it owns
	// the keyboard: see Update's tea.KeyMsg branch.
	overlay overlayKind
	palette paletteModel
```

Create `internal/tui/palette.go` with only the placeholder for now:

```go
package tui

// paletteModel is the command palette's state (#71). Task 6 fills it in.
type paletteModel struct{}
```

- [ ] **Step 2: Write the failing tests**

Append to `internal/tui/keys_test.go`:

```go
// The palette must not inherit d.Up/d.Down: those are "up"/"k" and "down"/"j",
// so typing j or k into the query would move the cursor instead of entering a
// letter. paletteKeys uses the arrow-only PaletteUp/PaletteDown defaults.
func TestPaletteKeysAreArrowOnly(t *testing.T) {
	t.Parallel()
	k := paletteKeys(defaultKeys())
	for _, tc := range []struct {
		name    string
		binding key.Binding
	}{
		{"up", k.Up},
		{"down", k.Down},
	} {
		for _, bad := range []string{"j", "k"} {
			if slices.Contains(tc.binding.Keys(), bad) {
				t.Errorf("palette %s binding accepts %q, which the query needs as a character", tc.name, bad)
			}
		}
	}
}

// While the palette is open it owns the keyboard, so q must not quit and ? must
// not toggle help. A zero key.Binding has nil keys, which Enabled() reports as
// false and key.Matches never fires on.
func TestPaletteKeysLeaveQuitAndHelpUnassigned(t *testing.T) {
	t.Parallel()
	k := paletteKeys(defaultKeys())
	if k.Quit.Enabled() {
		t.Error("the palette assigned Quit; q would close the program while typing")
	}
	if k.Help.Enabled() {
		t.Error("the palette assigned Help; ? would toggle full help instead of filtering")
	}
}

// keysFor is what the footer reads, so it must switch to the palette's bindings
// while the overlay is open. screenKeys is what screenActions reads, and it must
// keep answering for the screen underneath — see the spec's section 5.2b.
func TestKeysForFollowsTheOverlayAndScreenKeysDoesNot(t *testing.T) {
	t.Parallel()
	m := newTestModelOnReport()
	if !keysFor(m).GroupBy.Enabled() {
		t.Fatal("the report screen lost its GroupBy binding; the fixture is wrong")
	}
	m.overlay = overlayPalette
	if keysFor(m).GroupBy.Enabled() {
		t.Error("keysFor still returns the report's bindings with the palette open")
	}
	if !screenKeys(m).GroupBy.Enabled() {
		t.Error("screenKeys stopped answering for the screen underneath the overlay")
	}
}

// Every binding the palette offers must be replayable through routeKey, which
// means its first key round-trips through keyMsgFor. Anything else would build
// a KeyMsg whose String() does not match the binding, and the action would
// silently do nothing.
func TestEveryPaletteBindingIsReplayable(t *testing.T) {
	t.Parallel()
	for _, b := range defaultKeys().paletteDefaults() {
		keys := b.Keys()
		if len(keys) == 0 {
			t.Errorf("a palette default has no keys: %+v", b.Help())
			continue
		}
		if _, ok := keyMsgFor(keys[0]); !ok {
			t.Errorf("binding %q (%s) has a first key keyMsgFor cannot rebuild", keys[0], b.Help().Desc)
		}
	}
}
```

`paletteDefaults()` and `keyMsgFor` are named here but built in Task 5 — this task's run of
these tests stops at the two that compile. Add the first three tests now and the fourth in
Task 5; note in your report which you deferred.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestPaletteKeys|TestKeysForFollows'`
Expected: FAIL to build — `paletteKeys`, `screenKeys` and `overlayPalette` are undefined.

- [ ] **Step 4: Add the three defaults and the keyMap field**

In `internal/tui/keys.go`, add to `keyDefaults` (after the `Help` field):

```go
	// Palette opens the command palette (#71) and, while it is open, closes it
	// again. Unlike Help its enablement varies by screen only in that three
	// screens switch it off entirely: see keysFor.
	Palette key.Binding

	// PaletteUp/PaletteDown are the palette's cursor keys, arrows ONLY. The
	// generic Up/Down also accept k and j, which the palette needs as query
	// characters — a filter you cannot type "kanban" into is not a filter.
	PaletteUp   key.Binding
	PaletteDown key.Binding
```

In `defaultKeys()`:

```go
		Palette:     key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "command palette")),
		PaletteUp:   key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "move up")),
		PaletteDown: key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "move down")),
```

Add **one** field to `keyMap`, next to `Help`:

```go
	Palette   key.Binding
```

`PaletteUp`/`PaletteDown` deliberately get **no** `keyMap` field: `paletteKeys` assigns
them to the existing `Up`/`Down` fields. That keeps `keyMap`'s field count — and therefore
`TestAllBindingsCoversEveryField` — moving by exactly one.

Add `k.Palette` to `allBindings()`, in the first group beside `k.Help`.

- [ ] **Step 5: Split `keysFor` and add `paletteKeys` + `paletteBindings`**

Rename today's `keysFor` to `screenKeys` (body unchanged) and add the wrapper above it:

```go
// keysFor returns the bindings the given Model state accepts right now, and is
// what both Update and the footer read.
//
// An open overlay owns the keyboard, so it answers before the screen does.
// screenActions deliberately calls screenKeys instead: it derives the palette's
// own rows from the screen underneath, and asking keysFor there would hand it
// the palette's four keys and empty the list on the first keystroke.
func keysFor(m Model) keyMap {
	if m.overlay == overlayPalette {
		return paletteKeys(defaultKeys())
	}
	return screenKeys(m)
}
```

In `screenKeys`, switch off `Palette` where it must not open — add this to the three
existing branches:

- `screenSetup`: `setupKeys` never assigns `Palette` (the wizard must finish first).
- `screenLoading`: the inline `keyMap` literal never assigns `Palette` (a navigation while
  an `entriesMsg` is in flight is undone when it lands).
- `screenError`: the inline `keyMap` literal never assigns `Palette` (any key returns Home).

For **every other** per-screen constructor (`homeKeys`, `reportKeys`, `exportKeys`,
`ratesKeys`, `logKeys`, `membersKeys`, `rangeKeys`, `filtersKeys`, `listBrowserKeys`,
`budgetKeys`, `entriesKeys`), assign `Palette: d.Palette` in every `keyMap` literal it
returns, and append `k.Palette` to the **last group of `full` only**. Do **not** put it in
`short`: the `footer_*_short` goldens and `TestShortFootersFitEightyColumns` must stay
untouched.

Add `paletteKeys` at the end of the file:

```go
// paletteKeys is the binding set while the command palette is open (#71). It is
// not a screen: m.screen still names what is underneath, and this set simply
// takes precedence for as long as the overlay lives.
//
// Quit and Help stay unassigned, which is the second, independent guard on top
// of Update's ordering: even if the overlay check were moved below them, q and
// ? could not fire while the user is typing a query.
func paletteKeys(d keyDefaults) keyMap {
	k := keyMap{
		Up: d.PaletteUp, Down: d.PaletteDown,
		Confirm: d.Confirm, Back: d.Back,
		Palette: d.Palette, ForceQuit: d.ForceQuit,
	}
	k.Confirm.SetHelp("enter", "run")
	k.Back.SetHelp("esc", "close")
	pair := pairHelp(k.Up, k.Down, "↑/↓", "move")
	// ctrl+p closes as well as opens, but esc is the close every other screen
	// already teaches, so only esc is advertised.
	k.short = []key.Binding{pair, k.Confirm, k.Back, k.ForceQuit}
	k.full = [][]key.Binding{{pair, k.Confirm}, {k.Back, k.ForceQuit}}
	return k
}

// paletteBindings is the subset of a screen's bindings the command palette
// offers as commands. It is allBindings' curated twin: cursor mechanics (move,
// confirm, toggle, switch section, go back) are not commands and would only
// bury the ones that are.
//
// Generate is here despite being Enter rather than a rune: it is the most-used
// action in the program and keyMsgFor rebuilds it exactly. Confirm is not, even
// though it shares that key — the selection is by field, not by keystroke.
func (k keyMap) paletteBindings() []key.Binding {
	return []key.Binding{
		k.Generate, k.GroupBy, k.ChangeRange, k.Reload, k.Export, k.Rates,
		k.Filters, k.Budget, k.OpenEntries, k.LogHours, k.Timer, k.Members,
		k.Range, k.ToggleScope, k.ToggleWeek, k.PrevMonth, k.NextMonth,
		k.ListCurrency, k.ListBudget, k.NewOverride, k.ClearValue, k.BrowseList,
		k.Save, k.Delete, k.Edit, k.History, k.Tags, k.NewTag, k.StopTimer,
	}
}

// paletteDefaults mirrors paletteBindings over keyDefaults, so a test can check
// every palette-eligible key without having to reach a screen that enables it.
func (d keyDefaults) paletteDefaults() []key.Binding {
	return keyMap{
		Generate: d.Generate, GroupBy: d.GroupBy, ChangeRange: d.ChangeRange,
		Reload: d.Reload, Export: d.Export, Rates: d.Rates, Filters: d.Filters,
		Budget: d.Budget, OpenEntries: d.OpenEntries, LogHours: d.LogHours,
		Timer: d.Timer, Members: d.Members, Range: d.Range,
		ToggleScope: d.ToggleScope, ToggleWeek: d.ToggleWeek,
		PrevMonth: d.PrevMonth, NextMonth: d.NextMonth,
		ListCurrency: d.ListCurrency, ListBudget: d.ListBudget,
		NewOverride: d.NewOverride, ClearValue: d.ClearValue,
		BrowseList: d.BrowseList, Save: d.Save, Delete: d.Delete, Edit: d.Edit,
		History: d.History, Tags: d.Tags, NewTag: d.NewTag, StopTimer: d.StopTimer,
	}.paletteBindings()
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/tui/ -run 'TestPaletteKeys|TestKeysForFollows|TestAllBindings' -v`
Expected: PASS.

- [ ] **Step 7: Verify the arrow-only test catches the bug it exists for**

Temporarily change `paletteKeys` to `Up: d.Up, Down: d.Down`.
Run: `go test ./internal/tui/ -run TestPaletteKeysAreArrowOnly -v`
Expected: **FAIL**, naming both `j` and `k`. Paste the output into your report, then
restore `d.PaletteUp`/`d.PaletteDown`.

- [ ] **Step 8: Regenerate the full-help footer goldens**

Run: `go test ./internal/tui -update`
Then: `git diff --stat internal/tui/testdata/`

Expected: **only** `footer_*_full.golden` files changed, each gaining `ctrl+p command
palette`. If any `footer_*_short.golden` changed, `Palette` reached a `short` slice —
remove it and regenerate. If `footer_setup_*`, `footer_loading_*` or `footer_error_*`
changed, `Palette` reached a screen it must not be on.

Run: `go test ./internal/tui/ -run 'TestGoldenFooters|TestShortFootersFitEightyColumns'`
Expected: PASS.

- [ ] **Step 9: Gate and commit**

```bash
gofmt -l .
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go build ./...
go test ./... -race
git add internal/tui/keys.go internal/tui/keys_test.go internal/tui/app.go internal/tui/palette.go internal/tui/testdata
git commit -m "feat(tui): add the palette keymap and split keysFor from screenKeys (#71)"
```

---

## Task 5: The action registry

**Files:**
- Create: `internal/tui/actions.go`
- Create: `internal/tui/actions_test.go`
- Modify: `internal/tui/keys_test.go` (add `TestEveryPaletteBindingIsReplayable`, deferred
  from Task 4 Step 2)

**Interfaces:**
- Consumes: `screenKeys`, `paletteBindings`, `paletteDefaults` (Task 4); the `openX()`
  methods (Task 3).
- Produces:
  - `type action struct { label, hint string; run func(Model) (tea.Model, tea.Cmd) }`
  - `func paletteActions(m Model) []action`
  - `func keyMsgFor(s string) (tea.KeyMsg, bool)`

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/actions_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
)

func labels(as []action) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.label
	}
	return out
}

func hasLabel(as []action, want string) bool {
	for _, a := range as {
		if a.label == want {
			return true
		}
	}
	return false
}

func TestKeyMsgFor(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in     string
		wantOK bool
		want   string
	}{
		{"g", true, "g"},
		{"enter", true, "enter"},
		{"tab", false, ""},
		{"shift+tab", false, ""},
		{"up", false, ""},
		{"", false, ""},
	} {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			msg, ok := keyMsgFor(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("keyMsgFor(%q) ok = %v, want %v", tc.in, ok, tc.wantOK)
			}
			// The round trip is the whole point: key.Matches compares
			// msg.String() against the binding's key strings.
			if ok && msg.String() != tc.want {
				t.Errorf("keyMsgFor(%q).String() = %q, want %q", tc.in, msg.String(), tc.want)
			}
		})
	}
}

func TestScreenActionsComeFromTheScreenKeymap(t *testing.T) {
	t.Parallel()
	got := labels(screenActions(newTestModelOnReport()))
	for _, want := range []string{"Grouping", "Export", "Budgets", "Filters", "Entries"} {
		if !hasLabel(screenActions(newTestModelOnReport()), want) {
			t.Errorf("the report screen offers no %q action; got %v", want, got)
		}
	}
	// Cursor mechanics are not commands.
	for _, unwanted := range []string{"Move up", "Move down", "Confirm", "Back", "Help", "Quit"} {
		if hasLabel(screenActions(newTestModelOnReport()), unwanted) {
			t.Errorf("%q leaked into the palette; it is cursor mechanics, not a command", unwanted)
		}
	}
}

// A disabled binding must not appear. This is what makes the palette's gating
// free: the same Enabled() that decides the footer decides the palette.
func TestScreenActionsRespectEnablement(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	m.screen = screenHome
	m.scope = "me"
	if hasLabel(screenActions(m), "Select members") {
		t.Error("the members action appears in the me scope, where its binding is disabled")
	}
	m.scope = "team"
	if !hasLabel(screenActions(m), "Select members") {
		t.Error("the members action is missing in the team scope, where its binding is enabled")
	}
}

// Labels are the footer's own words with the first rune upper-cased, so the two
// surfaces cannot drift. ToUpper on the first BYTE would corrupt any label that
// starts with a multibyte rune.
func TestCapitalizeHandlesMultibyte(t *testing.T) {
	t.Parallel()
	if got := capitalize("èxport"); got != "Èxport" {
		t.Errorf("capitalize(%q) = %q, want %q", "èxport", got, "Èxport")
	}
	if got := capitalize(""); got != "" {
		t.Errorf("capitalize(%q) = %q, want %q", "", got, "")
	}
}

func TestGlobalActionsAreGated(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	m.screen = screenHome
	m.entries = nil
	for _, unwanted := range []string{"Go to report", "Go to budgets", "Go to filters", "Go to entries", "Go to export"} {
		if hasLabel(globalActions(m), unwanted) {
			t.Errorf("%q is offered with no entries loaded; there is no report to show", unwanted)
		}
	}
	m.entries = goldenEntries()
	for _, want := range []string{"Go to report", "Go to budgets", "Go to filters", "Go to entries", "Go to export"} {
		if !hasLabel(globalActions(m), want) {
			t.Errorf("%q is missing although entries are loaded", want)
		}
	}
}

func TestGlobalActionsSkipTheCurrentScreen(t *testing.T) {
	t.Parallel()
	m := newTestModelOnReport()
	if hasLabel(globalActions(m), "Go to report") {
		t.Error(`"Go to report" is offered while already on the report screen`)
	}
	if !hasLabel(globalActions(m), "Go to rates") {
		t.Error(`"Go to rates" is missing from the report screen`)
	}
}

func TestGlobalMembersActionIsTeamOnly(t *testing.T) {
	t.Parallel()
	m := newTestModelOnReport()
	m.scope = "me"
	if hasLabel(globalActions(m), "Go to members") {
		t.Error(`"Go to members" is offered in the me scope`)
	}
	m.scope = "team"
	if !hasLabel(globalActions(m), "Go to members") {
		t.Error(`"Go to members" is missing in the team scope`)
	}
}

// Screen actions come first: "what can I do here" before "take me elsewhere".
func TestPaletteActionsPutScreenActionsFirst(t *testing.T) {
	t.Parallel()
	got := labels(paletteActions(newTestModelOnReport()))
	firstGlobal := -1
	for i, l := range got {
		if strings.HasPrefix(l, "Go to ") {
			firstGlobal = i
			break
		}
	}
	if firstGlobal <= 0 {
		t.Fatalf("expected screen actions before the first navigation row; got %v", got)
	}
	for _, l := range got[firstGlobal:] {
		if !strings.HasPrefix(l, "Go to ") && l != "Log hours" && l != "Quit" {
			t.Errorf("screen action %q appears after the navigation block: %v", l, got)
		}
	}
}

// A screen action must do exactly what pressing its key does. This is the whole
// justification for deriving them instead of writing a second registry.
func TestScreenActionRunMatchesTheKeypress(t *testing.T) {
	t.Parallel()
	m := newTestModelOnReport()
	m.report = goldenReport()

	var run func(Model) (tea.Model, tea.Cmd)
	for _, a := range screenActions(m) {
		if a.label == "Export" {
			run = a.run
		}
	}
	if run == nil {
		t.Fatal(`no "Export" action on the report screen`)
	}
	viaAction, _ := run(m)
	viaKey, _ := m.updateReport(keyMsg("e"))
	if viaAction.(Model).screen != viaKey.(Model).screen {
		t.Errorf("action landed on %v, the keypress on %v",
			viaAction.(Model).screen, viaKey.(Model).screen)
	}
	if viaAction.(Model).screen != screenExport {
		t.Errorf("screen = %v, want screenExport", viaAction.(Model).screen)
	}
}

// Quit is the one global action that is not navigation, so it is appended
// outside the target-skipping loop rather than given a fake target screen.
func TestQuitActionIsAlwaysOffered(t *testing.T) {
	t.Parallel()
	for _, s := range []screen{screenHome, screenReport, screenRates} {
		m := newTestModelOnReport()
		m.screen = s
		if !hasLabel(globalActions(m), "Quit") {
			t.Errorf("Quit is missing on %v", s)
		}
	}
}

func TestGlobalMembersActionUsesTheCallersScreenAsOrigin(t *testing.T) {
	t.Parallel()
	m := newTestModelOnReport()
	m.scope = "team"
	m.screen = screenRates
	m.teamMembers = nil
	m.client = clickup.New("tok")
	m.client.BaseURL = "http://127.0.0.1:1" // refused instantly, no test server needed

	var run func(Model) (tea.Model, tea.Cmd)
	for _, a := range globalActions(m) {
		if a.label == "Go to members" {
			run = a.run
		}
	}
	if run == nil {
		t.Fatal(`no "Go to members" action`)
	}
	_, cmd := run(m)
	if cmd == nil {
		t.Fatal("no command issued with no members cached")
	}
	msg, ok := cmd().(retryableErrMsg)
	if !ok {
		t.Fatalf("command produced %T, want retryableErrMsg", cmd())
	}
	if msg.origin != screenRates {
		t.Errorf("origin = %v, want screenRates", msg.origin)
	}
}
```

Also add `TestEveryPaletteBindingIsReplayable` to `internal/tui/keys_test.go` — the exact
code is in Task 4, Step 2.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/tui/ -run 'TestKeyMsgFor|TestScreenActions|TestGlobal|TestPaletteActions|TestCapitalize|TestQuitAction|TestEveryPaletteBinding'`
Expected: FAIL to build — nothing in `actions.go` exists yet.

- [ ] **Step 3: Write the implementation**

Create `internal/tui/actions.go`:

```go
package tui

import (
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
)

// action is one row of the command palette (#71).
//
// run returns tea.Model rather than Model on purpose: that is routeKey's own
// signature, so a screen action is a direct forward with no type assertion in
// between.
type action struct {
	label string // "Export", "Go to rates"
	hint  string // the key that also does this, "" for a navigation row
	run   func(Model) (tea.Model, tea.Cmd)
}

// paletteActions is everything ctrl+p offers right now: the current screen's
// commands first, then the cross-screen navigation. With an empty query every
// score is 0, so this order survives into the rendered list — "what can I do
// here" above "take me elsewhere".
func paletteActions(m Model) []action {
	screen := screenActions(m)
	global := globalActions(m)
	out := make([]action, 0, len(screen)+len(global))
	out = append(out, screen...)
	return append(out, global...)
}

// screenActions derives the current screen's commands from its keymap rather
// than from a second list that would have to be kept in step with it.
//
// Three things fall out of that for free: the gating (a disabled binding simply
// is not here), the wording (the label is the footer's own description, so the
// two surfaces cannot drift), and the behaviour (run replays the key, so there
// is no duplicated handler to diverge).
//
// screenKeys, not keysFor: with the palette open keysFor answers for the
// palette, and this list would empty itself on the first keystroke.
func screenActions(m Model) []action {
	var out []action
	for _, b := range screenKeys(m).paletteBindings() {
		if !b.Enabled() {
			continue
		}
		keys := b.Keys()
		if len(keys) == 0 {
			continue
		}
		msg, ok := keyMsgFor(keys[0])
		if !ok {
			// Unreachable today, pinned by TestEveryPaletteBindingIsReplayable.
			// Dropping beats firing a KeyMsg that matches nothing.
			continue
		}
		out = append(out, action{
			label: capitalize(b.Help().Desc),
			hint:  b.Help().Key,
			run:   func(m Model) (tea.Model, tea.Cmd) { return m.routeKey(msg) },
		})
	}
	return out
}

// globalActions is the cross-screen navigation ctrl+p offers from anywhere.
//
// The rule this list obeys, and the reason it is short: global actions are
// navigation only. Anything that changes state stays a screen action, where the
// footer teaches it and the palette repeats it.
//
// Each row opens its screen through the same openX method the key handler uses
// (see open.go), so the two can never build a screen differently.
func globalActions(m Model) []action {
	hasReport := len(m.entries) > 0
	rows := []struct {
		label   string
		target  screen
		enabled bool
		run     func(Model) (tea.Model, tea.Cmd)
	}{
		{"Go to home", screenHome, true, func(m Model) (tea.Model, tea.Cmd) {
			return m.resetTo(screenHome), nil
		}},
		{"Go to report", screenReport, hasReport, func(m Model) (tea.Model, tea.Cmd) {
			return m.goTo(screenReport), nil
		}},
		{"Go to rates", screenRates, true, func(m Model) (tea.Model, tea.Cmd) {
			return m.openRates(), nil
		}},
		{"Go to budgets", screenBudget, hasReport, func(m Model) (tea.Model, tea.Cmd) {
			// A false return means the config's pricing or timezone failed to
			// parse, and openBudgetView has already routed to screenError —
			// which is the right landing either way, so there is nothing to add.
			m.openBudgetView()
			return m, nil
		}},
		{"Go to filters", screenFilters, hasReport, func(m Model) (tea.Model, tea.Cmd) {
			mm, cmd := m.openFilters()
			return mm, cmd
		}},
		{"Go to entries", screenEntries, hasReport, func(m Model) (tea.Model, tea.Cmd) {
			mm, cmd := m.openEntries()
			return mm, cmd
		}},
		{"Go to export", screenExport, hasReport, func(m Model) (tea.Model, tea.Cmd) {
			return m.openExport(), nil
		}},
		{"Go to range", screenRange, true, func(m Model) (tea.Model, tea.Cmd) {
			return m.openRange(), nil
		}},
		{"Go to members", screenMembers, m.scope == "team", func(m Model) (tea.Model, tea.Cmd) {
			mm, cmd := m.openMembers(m.screen)
			return mm, cmd
		}},
		{"Log hours", screenLog, true, func(m Model) (tea.Model, tea.Cmd) {
			return m.openLog(), nil
		}},
	}

	var out []action
	for _, r := range rows {
		if !r.enabled || r.target == m.screen {
			continue
		}
		out = append(out, action{label: r.label, run: r.run})
	}
	// Quit is the one row that is not navigation, so it has no target screen to
	// compare against and is appended rather than given a fake one.
	return append(out, action{label: "Quit", run: func(m Model) (tea.Model, tea.Cmd) {
		return m, tea.Quit
	}})
}

// keyMsgFor rebuilds the tea.KeyMsg a binding's first key would produce.
//
// The set is closed on purpose: key.Matches compares msg.String() against the
// binding's key strings, and only these two shapes round-trip exactly. "tab",
// "up" and "shift+tab" would not, which is why the palette does not offer the
// bindings that use them. ok is false for anything else, so an action that
// cannot be executed faithfully is dropped rather than mis-fired.
func keyMsgFor(s string) (tea.KeyMsg, bool) {
	if s == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}, true
	}
	if r := []rune(s); len(r) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: r}, true
	}
	return tea.KeyMsg{}, false
}

// capitalize upper-cases the first rune. strings.ToUpper on s[:1] would corrupt
// any label whose first character is multibyte.
func capitalize(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/tui/ -run 'TestKeyMsgFor|TestScreenActions|TestGlobal|TestPaletteActions|TestCapitalize|TestQuitAction|TestEveryPaletteBinding' -v`
Expected: PASS.

If `TestScreenActionsComeFromTheScreenKeymap` fails on a label, read the failure before
changing the test: the expected strings are `capitalize()` applied to the help
descriptions in `defaultKeys()` (`"grouping"` → `"Grouping"`, `"export"` → `"Export"`,
`"budgets"` → `"Budgets"`, `"filters"` → `"Filters"`, `"entries"` → `"Entries"`,
`"select members"` → `"Select members"`). If a description has changed since this plan was
written, fix the test to the real string and say so in your report.

- [ ] **Step 5: Verify the screenKeys choice is load-bearing**

Temporarily change `screenActions` to call `keysFor(m)` instead of `screenKeys(m)`, and add
this temporary check to confirm the difference is real:

```go
	m := newTestModelOnReport()
	m.overlay = overlayPalette
	if len(screenActions(m)) == 0 {
		t.Fatal("screenActions is empty with the overlay open")
	}
```

Run it: with `keysFor` it must FAIL (empty list); with `screenKeys` it must PASS. Paste
both outputs into your report, then remove the temporary check — Task 6 covers this
permanently with `TestPaletteKeepsScreenActionsWhileTyping`.

- [ ] **Step 6: Gate and commit**

```bash
gofmt -l .
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go build ./...
go test ./... -race
git add internal/tui/actions.go internal/tui/actions_test.go internal/tui/keys_test.go
git commit -m "feat(tui): derive the palette's actions from the keymap and the openX methods (#71)"
```

---

## Task 6: The palette itself

**Files:**
- Modify: `internal/tui/palette.go` (replace the Task 4 placeholder with the real model)
- Create: `internal/tui/palette_test.go`
- Modify: `internal/tui/app.go` (the `tea.KeyMsg` branch of `Update`, and `View()`)
- Create: `internal/tui/testdata/palette_report.golden`, `palette_filtered.golden`,
  `palette_no_match.golden`, `palette_narrow.golden`

**Interfaces:**
- Consumes: `fuzzy.Match` (Task 1); `composite` (Task 2); `paletteActions` (Task 5);
  `paletteKeys`, `overlayPalette` (Task 4).
- Produces: `func (m Model) openPalette() Model`,
  `func (m Model) updateOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd)`,
  `func (p paletteModel) layout(th theme, width, height int) (box string, x, y int)`.

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/palette_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func openPaletteOn(m Model) Model { return m.openPalette() }

func typeInto(m Model, s string) Model {
	for _, r := range s {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		if r == ' ' {
			msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
		}
		got, _ := m.updateOverlay(msg)
		m = got.(Model)
	}
	return m
}

func TestPaletteOpensAndClosesWithoutTouchingNav(t *testing.T) {
	t.Parallel()
	m := newTestModelOnReport()
	before := append([]screen(nil), m.nav...)

	m = openPaletteOn(m)
	if m.overlay != overlayPalette {
		t.Fatal("ctrl+p did not open the palette")
	}
	if m.screen != screenReport {
		t.Errorf("screen = %v, want screenReport — an overlay is not a place you navigated to", m.screen)
	}
	if len(m.nav) != len(before) {
		t.Errorf("nav = %v, want %v unchanged", m.nav, before)
	}

	got, _ := m.updateOverlay(keyMsg("esc"))
	m = got.(Model)
	if m.overlay != overlayNone {
		t.Error("esc did not close the palette")
	}
	if m.screen != screenReport || len(m.nav) != len(before) {
		t.Errorf("closing changed the screen or nav: screen=%v nav=%v", m.screen, m.nav)
	}
}

// The overlay owns the keyboard. If Update checked Quit before the overlay,
// typing "q" would end the program mid-query.
func TestPaletteQueryAcceptsQ(t *testing.T) {
	t.Parallel()
	m := openPaletteOn(newTestModelOnReport())
	got, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil {
		t.Fatal("typing q issued a command; the quit path fired inside the palette")
	}
	if q := got.(Model).palette.query; q != "q" {
		t.Errorf("query = %q, want %q", q, "q")
	}
}

// Same shape for ?: inside the palette it is a character, not the help toggle.
func TestPaletteQueryAcceptsQuestionMark(t *testing.T) {
	t.Parallel()
	m := openPaletteOn(newTestModelOnReport())
	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if got.(Model).helpAll {
		t.Error("? toggled full help while the palette was open")
	}
	if q := got.(Model).palette.query; q != "?" {
		t.Errorf("query = %q, want %q", q, "?")
	}
}

// j and k are cursor keys everywhere else in this program. In a query they are
// letters, and a filter you cannot type "kanban" into is not a filter.
func TestPaletteTypesJAndK(t *testing.T) {
	t.Parallel()
	m := typeInto(openPaletteOn(newTestModelOnReport()), "jk")
	if q := m.palette.query; q != "jk" {
		t.Errorf("query = %q, want %q — j/k moved the cursor instead of typing", q, "jk")
	}
}

// bubbletea reclassifies a lone space as KeySpace while leaving it in Runes.
// Without a branch for that type the query could never contain a space.
func TestPaletteQueryAcceptsSpace(t *testing.T) {
	t.Parallel()
	m := typeInto(openPaletteOn(newTestModelOnReport()), "log h")
	if q := m.palette.query; q != "log h" {
		t.Errorf("query = %q, want %q", q, "log h")
	}
	if len(m.palette.items) == 0 {
		t.Error(`"log h" matched nothing; the space broke the filter`)
	}
}

// The permanent form of Task 5's temporary check: screenActions must keep
// answering for the screen underneath once the overlay owns keysFor.
func TestPaletteKeepsScreenActionsWhileTyping(t *testing.T) {
	t.Parallel()
	m := typeInto(openPaletteOn(newTestModelOnReport()), "e")
	for _, it := range m.palette.items {
		if it.a.hint != "" {
			return // a screen action survived; that is all this test needs
		}
	}
	t.Errorf("every row lost its key hint, so no screen action survived typing: %v", m.palette.items)
}

func TestPaletteBackspace(t *testing.T) {
	t.Parallel()
	m := typeInto(openPaletteOn(newTestModelOnReport()), "exp")
	got, _ := m.updateOverlay(tea.KeyMsg{Type: tea.KeyBackspace})
	m = got.(Model)
	if m.palette.query != "ex" {
		t.Errorf("query = %q, want %q", m.palette.query, "ex")
	}
	m.palette.query = ""
	got, _ = m.updateOverlay(tea.KeyMsg{Type: tea.KeyBackspace})
	if q := got.(Model).palette.query; q != "" {
		t.Errorf("backspace on an empty query produced %q", q)
	}
}

func TestPaletteCursorClampsAndScrolls(t *testing.T) {
	t.Parallel()
	m := openPaletteOn(newTestModelOnReport())
	m.height = 24
	if len(m.palette.items) <= paletteMaxRows {
		t.Fatalf("the fixture has only %d actions; this test needs more than %d", len(m.palette.items), paletteMaxRows)
	}

	up, _ := m.updateOverlay(tea.KeyMsg{Type: tea.KeyUp})
	if i := up.(Model).palette.idx; i != 0 {
		t.Errorf("idx = %d after up at the top, want 0", i)
	}

	for range len(m.palette.items) + 3 {
		got, _ := m.updateOverlay(tea.KeyMsg{Type: tea.KeyDown})
		m = got.(Model)
	}
	if i, n := m.palette.idx, len(m.palette.items); i != n-1 {
		t.Errorf("idx = %d, want %d — down ran past the last row", i, n-1)
	}
	rows := paletteRows(m.height)
	if m.palette.idx < m.palette.top || m.palette.idx >= m.palette.top+rows {
		t.Errorf("cursor %d is outside the visible window [%d, %d)", m.palette.idx, m.palette.top, m.palette.top+rows)
	}
}

func TestPaletteEnterRunsTheSelectedActionAndCloses(t *testing.T) {
	t.Parallel()
	m := newTestModelOnReport()
	m.report = goldenReport()
	m = typeInto(openPaletteOn(m), "export")

	got, _ := m.updateOverlay(keyMsg("enter"))
	after := got.(Model)
	if after.overlay != overlayNone {
		t.Error("the palette stayed open after running an action; it would render over the new screen")
	}
	if after.screen != screenExport {
		t.Errorf("screen = %v, want screenExport", after.screen)
	}
}

func TestPaletteEnterOnAnEmptyListDoesNothing(t *testing.T) {
	t.Parallel()
	m := typeInto(openPaletteOn(newTestModelOnReport()), "zzzzzz")
	if len(m.palette.items) != 0 {
		t.Fatalf("expected no matches for %q, got %d", "zzzzzz", len(m.palette.items))
	}
	got, cmd := m.updateOverlay(keyMsg("enter"))
	if cmd != nil {
		t.Error("enter on an empty list issued a command")
	}
	if got.(Model).screen != screenReport {
		t.Errorf("screen = %v, want screenReport unchanged", got.(Model).screen)
	}
}

func TestPaletteDoesNotOpenOnSetupLoadingOrError(t *testing.T) {
	t.Parallel()
	for _, s := range []screen{screenSetup, screenLoading, screenError} {
		m := newTestModel()
		m.screen = s
		got, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
		if got.(Model).overlay != overlayNone {
			t.Errorf("ctrl+p opened the palette on %v", s)
		}
	}
}

func TestPaletteBoxIsExactlyItsWidth(t *testing.T) {
	t.Parallel()
	m := openPaletteOn(newTestModelOnReport())
	m.width, m.height = 100, 30
	box, x, y := m.palette.layout(testTheme(true), m.width, m.height)
	if y != paletteTopY {
		t.Errorf("y = %d, want %d", y, paletteTopY)
	}
	lines := strings.Split(box, "\n")
	want := lipgloss.Width(lines[0])
	for i, l := range lines {
		if w := lipgloss.Width(l); w != want {
			t.Errorf("box line %d is %d cells, want %d: %q", i, w, want, l)
		}
	}
	if x+want > m.width {
		t.Errorf("the box overflows: x=%d width=%d terminal=%d", x, want, m.width)
	}
}

// Match returns indices into the FULL label; a truncated label must drop the
// ones that fell off, and the ellipsis must never light up.
func TestPaletteHighlightSurvivesTruncation(t *testing.T) {
	t.Parallel()
	th := testTheme(true)
	long := strings.Repeat("ab", 40)
	_, idx, ok := fuzzyMatchForTest(long)
	if !ok {
		t.Fatal("the fixture query did not match")
	}
	got := highlight(th, shaveToWidth(long, 20), idx)
	if w := lipgloss.Width(got); w != 20 {
		t.Errorf("highlighted label is %d cells, want 20: %q", w, got)
	}
}

// Under termenv.Ascii the goldens cannot see this, so assert on the style.
func TestPaletteHighlightUsesTheAccentStyle(t *testing.T) {
	t.Parallel()
	th := testTheme(true)
	if th.Accent.GetForeground() == th.Cell.GetForeground() {
		t.Fatal("the theme's Accent and Cell share a foreground; this test cannot discriminate")
	}
}
```

`fuzzyMatchForTest` is a two-line helper you write in the test file:

```go
func fuzzyMatchForTest(target string) (int, []int, bool) { return fuzzy.Match("aaaa", target) }
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/tui/ -run TestPalette`
Expected: FAIL to build — `openPalette`, `updateOverlay`, `layout`, `paletteRows`,
`paletteMaxRows`, `paletteTopY` and `highlight` are undefined.

- [ ] **Step 3: Write `palette.go`**

Replace the placeholder file entirely:

```go
package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/marcoarnulfo/clickup-cli/internal/fuzzy"
)

// Geometry. See the spec's section 8.2.
const (
	paletteWidth    = 52 // preferred box width, borders included
	paletteMinWidth = 24 // never narrower, even on a narrow terminal
	paletteMaxRows  = 8  // action rows visible at once
	paletteChrome   = 4  // top border + query row + separator + bottom border
	paletteTopY     = 2  // body row the top border sits on
	paletteTitle    = "Command palette"
)

// paletteItem is one filtered row: the action, the rune indices fuzzy.Match
// hit (which drive the highlighting, not just the ranking) and its score.
type paletteItem struct {
	a     action
	idx   []int
	score int
}

// paletteModel is the command palette's state (#71).
//
// The query is a plain string with append and backspace rather than a
// textinput.Model: that type derives its styles from lipgloss's default
// renderer — the same reason footer.go refuses help.New() — and would add three
// fields of editing state a twenty-item filter never uses.
//
// There is no cached copy of every action. refreshPalette rebuilds from
// paletteActions on each keystroke: a fuzzy match over twenty short strings
// costs nothing, and a copy that is never kept cannot go stale.
type paletteModel struct {
	query string
	items []paletteItem
	idx   int // selected row
	top   int // first visible row
}

// openPalette raises the overlay. It does not touch m.nav: an overlay is not a
// place you navigated to, so closing it is not a pop().
func (m Model) openPalette() Model {
	m.overlay = overlayPalette
	m.palette = paletteModel{}
	return m.refreshPalette()
}

func (m Model) closePalette() Model {
	m.overlay = overlayNone
	m.palette = paletteModel{}
	return m
}

// refreshPalette rebuilds the filtered, ranked list for the current query.
func (m Model) refreshPalette() Model {
	all := paletteActions(m)
	items := make([]paletteItem, 0, len(all))
	for _, a := range all {
		score, idx, ok := fuzzy.Match(m.palette.query, a.label)
		if !ok {
			continue
		}
		items = append(items, paletteItem{a: a, idx: idx, score: score})
	}
	// Stable, so an empty query (every score 0) keeps paletteActions' order:
	// screen commands above the navigation rows.
	sort.SliceStable(items, func(i, j int) bool { return items[i].score > items[j].score })
	m.palette.items = items
	m.palette.idx = 0
	m.palette.top = 0
	return m
}

// updateOverlay handles every key while an overlay is open. Update routes here
// before it checks Quit, so nothing below can be reached by a query character.
func (m Model) updateOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := keysFor(m) // == paletteKeys: the handler and the footer read one source
	switch {
	case key.Matches(msg, k.Back), key.Matches(msg, k.Palette):
		return m.closePalette(), nil

	case key.Matches(msg, k.Confirm):
		if len(m.palette.items) == 0 {
			return m, nil
		}
		run := m.palette.items[m.palette.idx].a.run
		// Close FIRST: the action changes screen, and a palette still raised
		// would be composited over the screen it just opened.
		return run(m.closePalette())

	case key.Matches(msg, k.Up):
		if m.palette.idx > 0 {
			m.palette.idx--
		}
		m.palette = scrollPalette(m.palette, paletteRows(m.height))
		return m, nil

	case key.Matches(msg, k.Down):
		if m.palette.idx < len(m.palette.items)-1 {
			m.palette.idx++
		}
		m.palette = scrollPalette(m.palette, paletteRows(m.height))
		return m, nil

	// Backspace is text editing, which none of the ten textinput screens
	// advertises either, so it is a type check rather than a keymap binding.
	case msg.Type == tea.KeyBackspace:
		if r := []rune(m.palette.query); len(r) > 0 {
			m.palette.query = string(r[:len(r)-1])
			m = m.refreshPalette()
		}
		return m, nil
	}

	// bubbletea reclassifies a lone space as KeySpace while still carrying it in
	// Runes (key.go:698-702), so a query could not contain a space without this
	// second type. Alt-modified runes are chords, not text.
	if (msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace) && !msg.Alt {
		m.palette.query += string(msg.Runes)
		m = m.refreshPalette()
	}
	return m, nil
}

// scrollPalette moves the visible window so idx is always inside it. This is the
// scrolling the Filters screen still lacks (#28); the palette gets it right from
// the start.
func scrollPalette(p paletteModel, rows int) paletteModel {
	if p.idx < p.top {
		p.top = p.idx
	}
	if p.idx >= p.top+rows {
		p.top = p.idx - rows + 1
	}
	return p
}

// paletteRows is how many action rows fit. The subtraction accounts for the box
// chrome, the rows above it, and the blank line plus footer View always appends.
// The floor of 3 says the palette shrinks on a short terminal but never vanishes.
func paletteRows(height int) int {
	if height <= 0 {
		return paletteMaxRows
	}
	return max(3, min(paletteMaxRows, height-paletteChrome-paletteTopY-2))
}

// paletteBoxWidth is the box's total width, borders included.
func paletteBoxWidth(width int) int {
	if width <= 0 {
		return paletteWidth
	}
	// On a terminal too narrow even for the floor, the box keeps its 24 columns
	// and overflows to the right: a box that spills is readable, a box squeezed
	// below its floor is not.
	return max(paletteMinWidth, min(paletteWidth, width-4))
}

// layout renders the box and returns the cell its top-left corner goes in.
func (p paletteModel) layout(th theme, width, height int) (string, int, int) {
	boxW := paletteBoxWidth(width)
	x := 0
	if width > boxW {
		x = (width - boxW) / 2
	}
	return p.box(th, boxW, paletteRows(height)), x, paletteTopY
}

// box builds the frame a line at a time rather than through th.Box.
//
// For an overlay the rendered width is not decoration: it is the number
// composite uses to decide where the body resumes. lipgloss's own border and
// padding arithmetic already cost this package one bug (the report table's
// amputated right border, #66), so here every line is boxW cells wide by
// construction and a test checks it.
func (p paletteModel) box(th theme, boxW, rows int) string {
	innerW := boxW - 4
	var b strings.Builder

	// th.Accent, never th.Title: Title carries MarginBottom(1) and would inject
	// a blank line into the middle of the overlay.
	dashes := boxW - lipgloss.Width(paletteTitle) - 5
	b.WriteString(th.Border.Render("╭─ ") + th.Accent.Render(paletteTitle) +
		th.Border.Render(" "+strings.Repeat("─", dashes)+"╮") + "\n")

	q := shaveToWidth("> "+p.query, innerW)
	b.WriteString(paletteLine(th, th.Cell.Render(q), innerW-lipgloss.Width(q)))
	b.WriteString(th.Border.Render("├"+strings.Repeat("─", boxW-2)+"┤") + "\n")

	if len(p.items) == 0 {
		msg := shaveToWidth("no matching action", innerW)
		b.WriteString(paletteLine(th, th.Help.Render(msg), innerW-lipgloss.Width(msg)))
	}
	for i := p.top; i < len(p.items) && i < p.top+rows; i++ {
		b.WriteString(paletteLine(th, paletteRow(th, p.items[i], i == p.idx, innerW), 0))
	}

	b.WriteString(th.Border.Render("╰" + strings.Repeat("─", boxW-2) + "╯"))
	return b.String()
}

// paletteLine wraps already-styled content in the side borders, padding it out
// by pad cells so every line is the same width.
func paletteLine(th theme, content string, pad int) string {
	if pad > 0 {
		content += th.Cell.Render(strings.Repeat(" ", pad))
	}
	return th.Border.Render("│ ") + content + th.Border.Render(" │") + "\n"
}

// paletteRow renders one action's inner content, exactly innerW cells wide.
func paletteRow(th theme, it paletteItem, selected bool, innerW int) string {
	cursor := "  "
	style := th.Cell
	if selected {
		cursor = "▸ " // the marker members.go, export.go and filters.go already use
		style = th.Accent
	}

	hintW := lipgloss.Width(it.a.hint)
	labelW := innerW - 2 - hintW
	if hintW > 0 {
		labelW -= 2 // two spaces between the label and the key
	}
	label := shaveToWidth(it.a.label, max(labelW, 1))
	gap := innerW - 2 - lipgloss.Width(label) - hintW

	out := style.Render(cursor) + highlight(th, label, it.idx)
	if gap > 0 {
		out += th.Cell.Render(strings.Repeat(" ", gap))
	}
	return out + th.Help.Render(it.a.hint)
}

// highlight renders label with the runes at idx in th.Accent and the rest in
// th.Cell.
//
// idx indexes the FULL label, because that is what fuzzy.Match was given.
// Truncation happens first (highlighting first would leave escape sequences to
// be cut in half), so indices past the shortened label are dropped here, and a
// trailing ellipsis never lights up.
func highlight(th theme, label string, idx []int) string {
	r := []rune(label)
	limit := len(r)
	if limit > 0 && r[limit-1] == '…' {
		limit--
	}
	hit := make(map[int]bool, len(idx))
	for _, i := range idx {
		if i < limit {
			hit[i] = true
		}
	}

	var b strings.Builder
	// Runs, not runes: one Render per stretch keeps the escape sequences down to
	// a handful per row instead of one pair per character.
	for i := 0; i < len(r); {
		j := i
		for j < len(r) && hit[j] == hit[i] {
			j++
		}
		seg := string(r[i:j])
		if hit[i] {
			b.WriteString(th.Accent.Render(seg))
		} else {
			b.WriteString(th.Cell.Render(seg))
		}
		i = j
	}
	return b.String()
}
```

- [ ] **Step 4: Wire the palette into `Update` and `View`**

In `internal/tui/app.go`, replace the whole `case tea.KeyMsg:` block with:

```go
	case tea.KeyMsg:
		// ForceQuit first and unconditionally: with an overlay open it is the
		// only way out that nothing else can intercept.
		if key.Matches(msg, defaultKeys().ForceQuit) {
			return m, tea.Quit
		}
		// An open overlay owns the keyboard, and this check MUST stay above
		// Quit: below it, typing "q" into the palette's query would end the
		// program. TestPaletteQueryAcceptsQ pins the ordering.
		if m.overlay != overlayNone {
			return m.updateOverlay(msg)
		}
		if key.Matches(msg, keysFor(m).Quit) {
			return m, tea.Quit
		}
		// Checked here, beside Quit/ForceQuit, rather than inside routeKey: that
		// makes '?' behave identically on every screen keysFor enables it for,
		// including screenLoading, which routeKey has no case for at all.
		// keysFor(m).Help is already unassigned (a no-op key.Binding, so
		// key.Matches never fires) on every screen where '?' must mean
		// something else — the ten textinput-forwarding contexts, screenError
		// (any key -> Home), and entriesConfirmDelete (any key but y cancels).
		if key.Matches(msg, keysFor(m).Help) {
			m.helpAll = !m.helpAll
			return m, nil
		}
		if key.Matches(msg, keysFor(m).Palette) {
			return m.openPalette(), nil
		}
		return m.routeKey(msg)
```

Replace `View()` with:

```go
func (m Model) View() string {
	body := m.screenBody()
	if m.screen == screenError {
		// Every key returns Home here, which is not a binding — the screen
		// says so in its own sentence instead.
		return body
	}
	if m.overlay == overlayPalette {
		// Composed over the BODY, not over the finished view: the footer stays
		// below and visible, and it advertises the palette's own keys because
		// keysFor follows the overlay.
		box, x, y := m.palette.layout(m.theme, m.width, m.height)
		body = composite(body, box, x, y)
	}
	// Screens differ on whether their body ends with a newline; trimming here
	// is what puts the footer the same distance below every one of them.
	return strings.TrimRight(body, "\n") + "\n\n" + footerView(m.theme, m.width, m.helpAll, keysFor(m))
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/tui/ -run TestPalette -v`
Expected: PASS.

Run: `go test ./... -race`
Expected: PASS.

- [ ] **Step 6: Verify three tests fail against the bugs they exist for**

Break, run, capture, restore. Paste all three transcripts into your report.

1. In `Update`, move the `if m.overlay != overlayNone` check **below** the `Quit` check.
   Run: `go test ./internal/tui/ -run TestPaletteQueryAcceptsQ -v` → must FAIL.
2. In `paletteKeys`, use `Up: d.Up, Down: d.Down`.
   Run: `go test ./internal/tui/ -run TestPaletteTypesJAndK -v` → must FAIL.
3. In `updateOverlay`, drop `|| msg.Type == tea.KeySpace` from the final condition.
   Run: `go test ./internal/tui/ -run TestPaletteQueryAcceptsSpace -v` → must FAIL.

- [ ] **Step 7: Add the goldens**

Append to `internal/tui/golden_test.go`:

```go
// goldenPaletteModel is a report screen with the palette open at a fixed size,
// so the box's position and contents are byte-stable.
func goldenPaletteModel() Model {
	m := goldenModel()
	m.theme = testTheme(true)
	m.entries = goldenEntries()
	m.report = goldenReport()
	m.rep = newReport(m.report, "", goldenDaily())
	m.screen = screenReport
	m.nav = []screen{screenHome}
	m.width, m.height = 90, 30
	return m.openPalette()
}

func TestGoldenPalette(t *testing.T) {
	t.Parallel()
	golden(t, "palette_report", goldenPaletteModel().View())
}

func TestGoldenPaletteFiltered(t *testing.T) {
	t.Parallel()
	m := goldenPaletteModel()
	m.palette.query = "exp"
	m = m.refreshPalette()
	golden(t, "palette_filtered", m.View())
}

func TestGoldenPaletteNoMatch(t *testing.T) {
	t.Parallel()
	m := goldenPaletteModel()
	m.palette.query = "zzzzzz"
	m = m.refreshPalette()
	golden(t, "palette_no_match", m.View())
}

// A terminal narrower than the preferred box, to pin the floor and the centring.
func TestGoldenPaletteNarrow(t *testing.T) {
	t.Parallel()
	m := goldenPaletteModel()
	m.width = 40
	golden(t, "palette_narrow", m.View())
}
```

Run: `go test ./internal/tui -update`
Then **read all four new golden files**. Check by eye, and say in your report that you did:
the box's frame is unbroken on every line; the report table is still visible to the left
and right of the box; the footer below reads `↑/↓ move · enter run · esc close · ctrl+c
force quit`; `palette_no_match` shows `no matching action`; `palette_narrow`'s box is not
wider than 40 columns.

Run: `git diff --stat internal/tui/testdata/`
Expected: only the four new files. **If any pre-existing golden changed, stop and report
it** — nothing in this task should alter another screen.

- [ ] **Step 8: Gate and commit**

```bash
gofmt -l .
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go build ./...
go test ./... -race
git add internal/tui/palette.go internal/tui/palette_test.go internal/tui/app.go internal/tui/golden_test.go internal/tui/testdata
git commit -m "feat(tui): add the ctrl+p command palette as a floating overlay (#71)"
```

---

## Task 7: Docs, demo parity and the GIF

**Files:**
- Modify: `README.md`, `README.it.md`, `CHANGELOG.md`
- Modify: `docs/demo.tape`, `docs/demo.gif`
- Create: `internal/tui/palette_demo_test.go`

**Interfaces:**
- Consumes: everything from Tasks 1-6.
- Produces: nothing code-facing.

- [ ] **Step 1: Write the demo-parity test**

Every feature in this project has to work identically under `CLICKUP_DEMO=1`, with no I/O.

Create `internal/tui/palette_demo_test.go`:

```go
package tui

import (
	"testing"
)

// Demo mode must offer the same palette, and open it without any I/O: the demo
// exists so the TUI can be tried without an account, and a palette that reached
// the network there would break that promise.
//
// t.Setenv makes this test non-parallel by construction, which is what keeps it
// from racing TestMain's os.Unsetenv of the same variable.
func TestPaletteWorksInDemoMode(t *testing.T) {
	t.Setenv("CLICKUP_DEMO", "1")
	m := New(demoConfigForTest())
	m.entries = goldenEntries()
	m.screen = screenReport
	m.nav = []screen{screenHome}

	m = m.openPalette()
	if len(m.palette.items) == 0 {
		t.Fatal("the palette is empty in demo mode")
	}
	if !hasLabel(paletteActions(m), "Go to rates") {
		t.Error("demo mode lost the navigation rows")
	}

	// The two navigation rows that issue commands must take their demo branch.
	for _, label := range []string{"Go to filters", "Go to members"} {
		m.scope = "team"
		m.teamMembers = nil
		for _, a := range paletteActions(m) {
			if a.label != label {
				continue
			}
			if _, cmd := a.run(m); cmd != nil {
				// Running the command is safe here precisely because the demo
				// branch never dials: if this hangs or errors, the branch was
				// missed.
				if msg := cmd(); msg == nil {
					t.Errorf("%s produced a nil message in demo mode", label)
				}
			}
		}
	}
}
```

`demoConfigForTest()` is a one-liner you add to the same file if `demoConfig()` is not
already reachable from tests — check `internal/tui/demo.go` first and reuse what is there
rather than inventing a second fixture. Say in your report which you used.

- [ ] **Step 2: Run it**

Run: `go test ./internal/tui/ -run TestPaletteWorksInDemoMode -v`
Expected: PASS.

- [ ] **Step 3: Update both READMEs**

`README.md` — in the feature list, beside the existing entries about the adaptive report
table and the per-day sparkline, add:

```markdown
- **Command palette** — `ctrl+p` opens a fuzzy launcher that floats over the current
  screen: type a few letters, press Enter. It offers the current screen's own commands
  (the same ones the footer advertises, with their keys) plus one-step navigation to any
  other screen.
```

`README.it.md` — the same passage in Italian, matching the surrounding style:

```markdown
- **Command palette** — `ctrl+p` apre un launcher fuzzy che galleggia sopra la schermata
  corrente: scrivi qualche lettera, premi Invio. Offre i comandi della schermata corrente
  (gli stessi che il footer annuncia, con i loro tasti) più la navigazione in un passo
  verso qualunque altra schermata.
```

Check the surrounding lines and match their bullet style and voice rather than pasting
these verbatim if the file has moved on.

- [ ] **Step 4: Update the CHANGELOG**

Under `## [Unreleased]`, in `### Added`:

```markdown
- Command palette (`ctrl+p`): a fuzzy action launcher that floats over the current screen,
  offering both that screen's commands and one-step navigation to any other (#71).
- An overlay compositor: `View()` can now layer a floating box over a screen instead of
  replacing it (#59).
```

- [ ] **Step 5: Add a palette beat to the demo tape**

`docs/demo.tape` drives `vhs`. Add a beat after the report is on screen — read the file
first and match its existing `Type`/`Sleep`/`Enter` rhythm:

```
Ctrl+P
Sleep 1s
Type "exp"
Sleep 1500ms
Escape
Sleep 500ms
```

Add this comment at the top of the tape if it is not already there, so nobody installs a
stray binary into their PATH again:

```
# Regenerate with a locally built binary, without installing anything:
#   go build -o /tmp/clupdemo/clup ./cmd/clup
#   PATH=/tmp/clupdemo:$PATH vhs docs/demo.tape
```

- [ ] **Step 6: Regenerate the GIF**

```bash
mkdir -p /tmp/clupdemo
go build -o /tmp/clupdemo/clup ./cmd/clup
PATH=/tmp/clupdemo:$PATH vhs docs/demo.tape
rm -rf /tmp/clupdemo
```

Then **look at the result**, do not just check the file changed. Extract three frames from
the palette section and read them:

```bash
ffmpeg -y -i docs/demo.gif -vf "select='eq(n\,120)+eq(n\,150)+eq(n\,180)'" -vsync 0 /tmp/frame%02d.png
```

Adjust the frame numbers until you land inside the palette beat. Confirm in your report:
the box's border is unbroken, the screen underneath is visible around it, and the query
text is legible. If `vhs` or `ffmpeg` is not installed, say so plainly and leave
`docs/demo.gif` untouched rather than committing a GIF you could not inspect.

- [ ] **Step 7: Gate and commit**

```bash
gofmt -l .
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go build ./...
go test ./... -race
git add README.md README.it.md CHANGELOG.md docs/demo.tape docs/demo.gif internal/tui/palette_demo_test.go
git commit -m "docs: document the command palette and refresh the demo GIF (#71)"
```

---

## Self-review (run by the plan's author, recorded here)

**Spec coverage.** §2.1 → Tasks 4-5. §2.2 → Task 2. §2.3 → nothing to build, recorded in
the spec. §2.4 → Task 1. §2.5 → Task 6 (`box` builds the frame by hand). §2.6 → Task 6
(`query string`). §4 → Task 2. §5 → Tasks 4 and 6. §5.2b → Task 4 Step 5, pinned by Task 6.
§6.1 → Task 5. §6.2 → Task 5. §6.3 → Task 3. §6.4 → accepted, no code. §6.5 → Task 6
(`refreshPalette`'s `SliceStable`). §7 → Task 1. §8 → Task 6. §9.5 → Task 7 Step 1.
§10 → nothing to build. §11 → the Global Constraints above.

**Two things the spec asks for that no task built, now added:** the demo-parity test
(§9.5) became Task 7 Step 1, and the `TestEveryPaletteBindingIsReplayable` deferral is
called out explicitly in both Task 4 and Task 5 so it cannot fall between them.

**Type consistency.** `openEntries` returns `(Model, tea.Cmd)` in Tasks 3, 5 and 6 alike.
`action.run` is `func(Model) (tea.Model, tea.Cmd)` in Tasks 5 and 6 alike. `layout` returns
`(string, int, int)` in Tasks 6's implementation and its test. `paletteModel` is declared
empty in Task 4 and replaced in Task 6 — Task 4 Step 1 says so.

**Known ordering hazard.** Task 4 leaves `ctrl+p` advertised in the footer but inert until
Task 6. Called out in Task 4's preamble so a reviewer does not file it.
