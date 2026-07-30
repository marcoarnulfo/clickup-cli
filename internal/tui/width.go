package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// This file is the only sanctioned way to fit a string into a column.
//
// Every layout in this package is measured in DISPLAY COLUMNS, but Go counts
// runes in two places that used to be relied on: a rune-sliced cut, and fmt's
// "%-Ns" padding (fmt's width is a rune count, so a 7-rune 14-column string
// padded to 10 comes out 17 columns wide). The two agree only for ASCII, and
// ClickUp list and task names are exactly where emoji and CJK live.
//
// Both helpers take a SINGLE LINE. ansi.Truncate lets a "\n" through, so a
// multi-line input would produce a "cell" whose padding applies to the last line
// only; no call site passes one.

// truncateWidth cuts s to at most cols display columns, the ellipsis included.
// cols <= 0 returns "".
func truncateWidth(s string, cols int) string {
	if cols <= 0 {
		return ""
	}
	return ansi.Truncate(s, cols, "…")
}

// cell renders s into exactly cols display columns: truncated when too wide,
// space-padded when too narrow. It replaces the truncate(s, N) + "%-Ns" pair,
// where both halves counted runes.
//
// The pad is not cosmetic. ansi.Truncate drops a wide cluster whole rather than
// splitting it, so a cut can land NARROWER than cols ("日本語のリスト" cut to 2
// gives "…", one column). Without the measured pad the column misaligns anyway,
// just in the other direction.
func cell(s string, cols int) string {
	s = truncateWidth(s, cols)
	if pad := cols - lipgloss.Width(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}
