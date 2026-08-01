package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/marcoarnulfo/clickup-cli/internal/themes"
)

// theme is the styled surface the views render through. It travels as an
// explicit argument rather than living in package state, so no view can render
// with a half-built theme and tests can hold two themes at once.
//
// The palette itself is deliberately not kept as a field: nothing reads it
// back out of theme, and a write-only field is dead weight.
type theme struct {
	Title  lipgloss.Style
	Help   lipgloss.Style
	Err    lipgloss.Style
	OK     lipgloss.Style
	Accent lipgloss.Style
	Box    lipgloss.Style
	Header lipgloss.Style // bold, uncolored: the report's column header row
	Cell   lipgloss.Style // a plain report-table cell: no color, just the renderer
	Border lipgloss.Style // the report table's frame
	Zebra  lipgloss.Style // alternate report-table row
}

// newTheme builds the styles for a palette on a specific renderer. Production
// passes lipgloss.DefaultRenderer(); tests pass a renderer with a pinned color
// profile and background so output is deterministic.
func newTheme(r *lipgloss.Renderer, p themes.Palette) theme {
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
		Box:    r.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(p.Muted).Padding(0, 1),
		Header: r.NewStyle().Bold(true),
		Cell:   r.NewStyle(),
		Border: r.NewStyle().Foreground(p.Muted),
		Zebra:  r.NewStyle().Background(p.Subtle),
	}
}
