package tui

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/fuzzy"
	"github.com/marcoarnulfo/clickup-cli/internal/themes"
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

	// Through m.Update, not openPaletteOn: that helper bypasses the Update
	// switch entirely, so a test built on it can never notice ctrl+p itself
	// failing to reach openPalette (see Update's tea.KeyMsg branch).
	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = got.(Model)
	if m.overlay != overlayPalette {
		t.Fatal("ctrl+p did not open the palette")
	}
	if m.screen != screenReport {
		t.Errorf("screen = %v, want screenReport — an overlay is not a place you navigated to", m.screen)
	}
	if len(m.nav) != len(before) {
		t.Errorf("nav = %v, want %v unchanged", m.nav, before)
	}

	got, _ = m.updateOverlay(keyMsg("esc"))
	m = got.(Model)
	if m.overlay != overlayNone {
		t.Error("esc did not close the palette")
	}
	if m.screen != screenReport || len(m.nav) != len(before) {
		t.Errorf("closing changed the screen or nav: screen=%v nav=%v", m.screen, m.nav)
	}
}

// openPalette's doc comment explains why: the expanded footer is several
// lines tall, and paletteRows subtracts a fixed two rows for the blank line
// plus footer, so a surviving helpAll would misdraw the box on top of it.
func TestPaletteOpenClearsHelpAll(t *testing.T) {
	t.Parallel()
	m := newTestModelOnReport()
	m.helpAll = true
	m = m.openPalette()
	if m.helpAll {
		t.Error("openPalette left helpAll set")
	}
}

// A regression test, NOT a red-green one. Moving the overlay check below the
// Quit check does not break it: with the palette open, keysFor answers with
// paletteKeys, where Quit is unassigned, so q reaches the query either way.
// The ordering's real teeth are in TestPaletteCtrlPClosesRatherThanReopening.
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

// THIS is what makes Update's ordering load-bearing. paletteKeys assigns
// Palette (ctrl+p has to close), so with the overlay check below the Palette
// check, a ctrl+p would call openPalette a second time and wipe the query
// instead of closing.
func TestPaletteCtrlPClosesRatherThanReopening(t *testing.T) {
	t.Parallel()
	m := typeInto(openPaletteOn(newTestModelOnReport()), "exp")
	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	after := got.(Model)
	if after.overlay != overlayNone {
		t.Errorf("ctrl+p left the overlay open with query %q; it reopened instead of closing", after.palette.query)
	}
}

// An overlay belongs to the screen it was raised over. A 401 relaunches the
// setup wizard, and a palette that survived would be drawn over — and would own
// the keyboard on — the one screen ctrl+p is not allowed on at all.
func TestPaletteDissolvesWhenAnErrorSwapsTheScreen(t *testing.T) {
	t.Parallel()
	t.Run("401 relaunches setup", func(t *testing.T) {
		t.Parallel()
		m := openPaletteOn(newTestModelOnReport())
		got, _ := m.Update(errMsg{err: clickup.ErrUnauthorized})
		after := got.(Model)
		if after.screen != screenSetup {
			t.Fatalf("screen = %v, want screenSetup", after.screen)
		}
		if after.overlay != overlayNone {
			t.Error("the palette survived onto the setup wizard")
		}
	})
	t.Run("a retryable error lands on the error screen", func(t *testing.T) {
		t.Parallel()
		m := openPaletteOn(newTestModelOnReport())
		got, _ := m.Update(retryableErrMsg{origin: screenRates, err: errors.New("boom")})
		after := got.(Model)
		if after.screen != screenError {
			t.Fatalf("screen = %v, want screenError", after.screen)
		}
		// View returns early for screenError, so a surviving palette would be
		// invisible AND would swallow the "press a key to return home" keypress.
		if after.overlay != overlayNone {
			t.Error("the palette survived onto the error screen, where it is invisible but still eats keys")
		}
	})
}

// A same-screen replace is not the screen swap the invariant above is about.
// Several async handlers call replace(<the screen they are already on>):
// statusesMsg on screenFilters, membersMsg on screenMembers,
// entriesReloadedMsg/entriesErrMsg on screenEntries, logDoneMsg/timerMsg on
// screenLog. openFilters launches the status enrichment that produces
// statusesMsg, so a user can open the palette on screenFilters while that
// fetch is still in flight — and its return must not silently wipe the query
// out from under them.
func TestPaletteSurvivesReplaceOnTheSameScreen(t *testing.T) {
	t.Parallel()
	m := newTestModelOnReport()
	m.screen = screenFilters
	m = typeInto(openPaletteOn(m), "go")

	after := m.replace(screenFilters)
	if after.overlay != overlayPalette {
		t.Error("a same-screen replace dissolved the palette")
	}
	if after.palette.query != "go" {
		t.Errorf("a same-screen replace wiped the query: got %q, want %q", after.palette.query, "go")
	}
}

// The box is positioned in body coordinates, but bubbletea keeps the LAST
// height lines of an overflowing view. Without the correction the box scrolls
// off the top of a long report.
func TestPaletteBoxStaysVisibleUnderALongBody(t *testing.T) {
	t.Parallel()
	p := openPaletteOn(newTestModelOnReport()).palette
	_, _, shortY := p.layout(testTheme(true), 90, 30, 10)
	if shortY != paletteTopY {
		t.Errorf("y = %d on a body that fits, want %d", shortY, paletteTopY)
	}
	_, _, longY := p.layout(testTheme(true), 90, 30, 60)
	if longY <= shortY {
		t.Errorf("y = %d on a 60-line body in a 30-row terminal, want more than %d — the box would be scrolled away", longY, shortY)
	}
	// The box must land inside the window bubbletea will actually show.
	if longY < 60+2-30 {
		t.Errorf("y = %d is still above the visible window, which starts at body line %d", longY, 60+2-30)
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

// A query typed with the cursor sitting near the bottom of a longer list can
// shrink the list out from under it. Without refreshPalette resetting idx and
// top on every keystroke, idx would stay past the end of the new, shorter
// items slice, and Confirm's m.palette.items[m.palette.idx] would panic with
// an index out of range.
func TestPaletteRefreshResetsCursorWhenQueryShrinksTheList(t *testing.T) {
	t.Parallel()
	m := openPaletteOn(newTestModelOnReport())
	m.height = 24
	if len(m.palette.items) <= paletteMaxRows {
		t.Fatalf("the fixture has only %d actions; this test needs more than %d", len(m.palette.items), paletteMaxRows)
	}

	for range len(m.palette.items) - 1 {
		got, _ := m.updateOverlay(tea.KeyMsg{Type: tea.KeyDown})
		m = got.(Model)
	}
	if i, n := m.palette.idx, len(m.palette.items); i != n-1 {
		t.Fatalf("idx = %d, want %d — the cursor did not reach the bottom of the full list", i, n-1)
	}

	m = typeInto(m, "export")
	if n := len(m.palette.items); n == 0 || n >= paletteMaxRows {
		t.Fatalf(`query "export" left %d items; this test needs a short list for the old idx to land past its end`, n)
	}
	if i, n := m.palette.idx, len(m.palette.items); i < 0 || i >= n {
		t.Fatalf("idx = %d out of range for %d items after the query shrank the list", i, n)
	}
	if m.palette.top < 0 || m.palette.top > m.palette.idx {
		t.Errorf("top = %d, idx = %d — the scroll window was not reset with the cursor", m.palette.top, m.palette.idx)
	}

	// Confirm must not panic indexing into the shrunk list.
	got, _ := m.updateOverlay(keyMsg("enter"))
	if after := got.(Model); after.screen != screenExport {
		t.Errorf("screen = %v, want screenExport", after.screen)
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
	box, x, y := m.palette.layout(testTheme(true), m.width, m.height, 12)
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

// A query longer than the box's inner width must be shaved like every other
// line box builds, or it breaks the box's own right border with nothing
// noticing.
func TestPaletteLongQueryStaysInsideTheBox(t *testing.T) {
	t.Parallel()
	m := openPaletteOn(newTestModelOnReport())
	m.width, m.height = 100, 30
	m = typeInto(m, strings.Repeat("x", paletteWidth*2))

	box, _, _ := m.palette.layout(testTheme(true), m.width, m.height, 12)
	lines := strings.Split(box, "\n")
	want := lipgloss.Width(lines[0])
	for i, l := range lines {
		if w := lipgloss.Width(l); w != want {
			t.Errorf("box line %d is %d cells, want %d: %q", i, w, want, l)
		}
	}
}

// colorTheme builds a theme on a renderer with a real color profile. testTheme
// pins termenv.Ascii, which strips every escape, so a style assertion made
// through it cannot fail no matter what highlight does.
func colorTheme() theme {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.ANSI)
	r.SetHasDarkBackground(true)
	return newTheme(r, themes.Default())
}

func TestPaletteHighlightUsesTheAccentStyle(t *testing.T) {
	t.Parallel()
	th := colorTheme()
	if probe := th.Accent.Render("x"); !strings.Contains(probe, "\x1b[") {
		t.Fatalf("the accent style renders no escape (%q); this test would pass vacuously", probe)
	}
	got := highlight(th, "export", []int{0, 1, 2})
	if w := lipgloss.Width(got); w != 6 {
		t.Errorf("highlight changed the label's width: %d cells, %q", w, got)
	}
	if !strings.HasPrefix(got, th.Accent.Render("exp")) {
		t.Errorf("the matched prefix is not rendered in the accent style: %q", got)
	}
	if !strings.HasSuffix(got, th.Cell.Render("ort")) {
		t.Errorf("the unmatched tail picked up the accent style: %q", got)
	}
}

// fuzzy.Match indexes the FULL label, so a truncated label must drop any
// index landing at or past the cut — and the ellipsis must never light up.
//
// The query is chosen so its match lands EXACTLY on the ellipsis column, not
// merely "somewhere past the cut": "t" is label's 20th rune (index 19), and
// truncateWidth(label, 20) replaces that very column with "…". That is the only
// boundary this test can actually observe. An index further out (the original
// version of this test used "z9", matching at columns 25 and 35) is never even
// read: the render loop below only walks the truncated string's own 20 runes,
// so such an index sits in the hit map but nothing ever looks it up — dropping
// it or not produces byte-identical output, and the test would pass against a
// highlight that drops nothing at all.
func TestPaletteHighlightSurvivesTruncation(t *testing.T) {
	t.Parallel()
	th := colorTheme()
	label := "abcdefghijklmnopqrstuvwxyz0123456789"
	_, idx, ok := fuzzy.Match("t", label)
	if !ok {
		t.Fatal(`Match("t", label) did not match`)
	}
	if want := []int{19}; !slices.Equal(idx, want) {
		t.Fatalf("idx = %v, want %v — this test needs the match to land exactly on the ellipsis column truncateWidth(label, 20) produces", idx, want)
	}

	short := truncateWidth(label, 20)
	got := highlight(th, short, idx)
	if w := lipgloss.Width(got); w != 20 {
		t.Errorf("highlighted label is %d cells, want 20: %q", w, got)
	}
	// The one match landed on the ellipsis itself, so nothing may be accented.
	if want := th.Cell.Render(short); got != want {
		t.Errorf("highlight lit up the ellipsis:\n got %q\nwant %q", got, want)
	}
}
