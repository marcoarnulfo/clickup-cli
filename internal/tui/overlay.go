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
// Three shapes need help. A body line shorter than x gains spaces, or the box
// slides left on that line alone. A body shorter than y+height gains blank
// lines, or the box is clipped at the bottom — the Home screen's body is three
// lines and the palette is ten. And a wide glyph straddling either edge gets
// dropped rather than kept, because half a glyph cannot be drawn and staying
// aligned matters more than one character under the box's border.
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
			// A wide glyph straddling the box's right edge is KEPT WHOLE by Cut
			// (the left edge drops it instead), which would start this segment a
			// column early and push the line one cell too wide. Half a glyph
			// cannot be drawn, so drop it and hand its column back as a space.
			// At most one cluster can straddle, so one correction is enough.
			if lipgloss.Width(left)+lipgloss.Width(bl)+lipgloss.Width(right) > rowW {
				right = " " + ansi.Cut(row, x+boxW+1, rowW)
			}
		}

		lines[y+i] = left + bl + right
	}
	return strings.Join(lines, "\n")
}
