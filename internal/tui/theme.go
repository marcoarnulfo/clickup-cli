package tui

import "github.com/charmbracelet/lipgloss"

// palette holds the semantic colors the whole TUI is built from. It is the
// only place a color literal belongs: styles derive from these, and a future
// user-supplied theme (#82) will override these five values, not the styles.
type palette struct {
	Primary lipgloss.AdaptiveColor // titles and headings
	Accent  lipgloss.AdaptiveColor // selection, highlighted values
	Muted   lipgloss.AdaptiveColor // help lines and secondary text
	Danger  lipgloss.AdaptiveColor // errors
	Success lipgloss.AdaptiveColor // confirmations
}

// theme is the styled surface the views render through. It travels as an
// explicit argument rather than living in package state, so no view can render
// with a half-built theme and tests can hold two themes at once.
//
// The palette is deliberately not kept as a field: nothing reads it in this
// tranche, and a write-only field is dead weight. It comes back when a
// user-supplied theme (#82) needs to be inspected.
type theme struct {
	Title  lipgloss.Style
	Help   lipgloss.Style
	Err    lipgloss.Style
	OK     lipgloss.Style
	Accent lipgloss.Style
	Box    lipgloss.Style
	Header lipgloss.Style // bold, uncolored: the report's column header row
}

// newTheme builds the styles for a palette on a specific renderer. Production
// passes lipgloss.DefaultRenderer(); tests pass a renderer with a pinned color
// profile and background so output is deterministic.
func newTheme(r *lipgloss.Renderer, p palette) theme {
	// Resolve the terminal background now, while we still own the terminal.
	// lipgloss otherwise resolves AdaptiveColor lazily, at the first Render,
	// by querying the terminal over OSC-11 — by then bubbletea's input reader
	// is competing for the reply, termenv times out and falls back to "dark",
	// and a light terminal never gets the light palette.
	_ = r.HasDarkBackground()

	return theme{
		Title:  r.NewStyle().Bold(true).Foreground(p.Primary).MarginBottom(1),
		Help:   r.NewStyle().Foreground(p.Muted),
		Err:    r.NewStyle().Foreground(p.Danger).Bold(true),
		OK:     r.NewStyle().Foreground(p.Success),
		Accent: r.NewStyle().Foreground(p.Accent),
		Box:    r.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1),
		Header: r.NewStyle().Bold(true),
	}
}

// defaultPalette is the built-in palette. Light and Dark are deliberately
// identical here: this keeps the theme refactor byte-for-byte invisible. The
// adaptive Light values land with #63's second half.
func defaultPalette() palette {
	dup := func(c string) lipgloss.AdaptiveColor {
		return lipgloss.AdaptiveColor{Light: c, Dark: c}
	}
	return palette{
		Primary: dup("205"), // magenta, ClickUp-ish
		Accent:  dup("205"),
		Muted:   dup("240"),
		Danger:  dup("196"),
		Success: dup("42"),
	}
}
