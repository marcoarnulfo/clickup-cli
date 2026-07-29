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
