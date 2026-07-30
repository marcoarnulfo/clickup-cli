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

// The same property with wide glyphs, which is where it actually breaks.
// ansi.Cut treats the two edges differently: on the left it DROPS a cluster
// that straddles the limit, on the right it KEEPS it whole — so the right-hand
// segment starts one column early and the line comes out a cell too wide.
//
// An ASCII-only width test passes straight over this. That is exactly how a
// width test in the previous tranche passed over a 78-column table rendered
// into a 60-column terminal.
func TestCompositeHandlesWideGlyphsOnBothEdges(t *testing.T) {
	t.Parallel()
	// 20 double-width glyphs: 40 cells, one glyph per two columns, so a box at
	// an odd x straddles a glyph at both edges.
	body := strings.Repeat("漢", 20)
	if w := lipgloss.Width(body); w != 40 {
		t.Fatalf("the fixture is %d cells, want 40; this test assumes double-width glyphs", w)
	}
	got := composite(body, "AAAAAA", 5, 0)
	if w := lipgloss.Width(got); w != 40 {
		t.Errorf("composited line is %d cells, want 40: %q", w, got)
	}
	if !strings.Contains(got, "AAAAAA") {
		t.Errorf("the box was mangled: %q", got)
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

// TestCompositeDoesNotClipTheReportTableBesideANarrowPalette pins, rather than
// fixes, a choice: at a 40-column terminal the palette box does not span the
// full width, and the one column of body it leaves exposed on its right is
// legitimate content — the report table's own right border — not an
// artifact to crop.
//
// Measured from goldenPaletteModel at width 40 (screenBody + palette.layout,
// the real render path, not a hand-built fixture): the box sits at x=2 with
// boxW=36, so its footprint ends at column 38. Row 7, the table's top border,
// is the widest row the box actually covers (39 cells, against 38 for every
// other covered row) and is one cell wider than that footprint, so column 38
// survives — the same "╮" that closes the table's own border. At 50, 60, 80,
// and 120 columns the box's footprint reaches at least that 39th column, so
// nothing is left exposed there; only 40 shows this column at all.
//
// Why it is there rather than cropped: overlay.go:11-13 documents that cells
// of body outside the box's rectangle survive verbatim, "that layering is the
// whole point of #59" — the same contract that already leaves the body
// visible to the LEFT of the box on this very screen. An overlay that instead
// cropped the body to the box's rectangle would fail this test, and would
// also fail three tests that already pin the splice this relies on:
// TestCompositeSplicesTheBoxIntoTheBody (the box lands inside the body rather
// than replacing it), TestCompositeKeepsEveryLineWidth (a crop changes line
// widths, which that test forbids), and
// TestCompositeHandlesWideGlyphsOnBothEdges (same property against wide
// glyphs).
func TestCompositeDoesNotClipTheReportTableBesideANarrowPalette(t *testing.T) {
	t.Parallel()
	m := goldenPaletteModel()
	m.width = 40
	body := m.screenBody()
	box, x, y := m.palette.layout(m.theme, m.width, m.height, strings.Count(body, "\n")+1)
	got := composite(body, box, x, y)

	boxW := 0
	for _, l := range strings.Split(box, "\n") {
		boxW = max(boxW, lipgloss.Width(l))
	}

	// Guards the "row 7" index above, not just the measurement: if the report
	// screen ever gains or loses a line above the table, row 7 stops being the
	// table's top border and this fails with a numeric delta that hides the
	// real cause. Fail here instead, naming it, so a future reader lands on
	// "the index moved" rather than "the count is off by one."
	const tableTopBorderRow = 7
	if r := strings.Split(body, "\n")[tableTopBorderRow]; !strings.HasPrefix(r, "╭") || !strings.HasSuffix(r, "╮") {
		t.Fatalf("row %d of the body is no longer the report table's top border (got %q); "+
			"the report screen's layout above the table changed, so this test's row index needs "+
			"updating, not its numbers", tableTopBorderRow, r)
	}

	row := strings.Split(got, "\n")[tableTopBorderRow] // the table's top border, ╭───...───╮
	exposed := ansi.Cut(row, x+boxW, lipgloss.Width(row))
	if w := lipgloss.Width(exposed); w != 1 {
		t.Fatalf("column exposed beside the box is %d cells wide, want 1: %q", w, exposed)
	}
	if exposed != "╮" {
		t.Errorf("exposed cell = %q, want the table's own top-right corner %q", exposed, "╮")
	}
}

// Past the end of a line the right-hand segment is skipped by construction
// (composite's x+boxW < rowW guard), not cut and discovered empty: without
// that guard the straddle correction would still run against Cut's own empty
// result and could append a stray character.
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
