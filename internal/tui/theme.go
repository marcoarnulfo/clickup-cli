package tui

import "github.com/charmbracelet/lipgloss"

// palette holds the semantic colors the whole TUI is built from. It is the
// only place a color literal belongs: styles derive from these, and a future
// user-supplied theme (#82) will override these six values, not the styles.
type palette struct {
	Primary lipgloss.AdaptiveColor // titles and headings
	Accent  lipgloss.AdaptiveColor // selection, highlighted values
	Muted   lipgloss.AdaptiveColor // help lines and secondary text
	Danger  lipgloss.AdaptiveColor // errors
	Success lipgloss.AdaptiveColor // confirmations

	// Subtle is the only token whose automatic downconvert lands on the wrong
	// color, so it names a value per profile instead of letting termenv pick
	// the nearest one. The other five stay AdaptiveColor: measured, their
	// nearest-color conversion already arrives where it should, and making
	// them explicit would be five times the surface for no visible change.
	Subtle lipgloss.CompleteAdaptiveColor // zebra row background
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
	Cell   lipgloss.Style // a plain report-table cell: no color, just the renderer
	Border lipgloss.Style // the report table's frame
	Zebra  lipgloss.Style // alternate report-table row
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
		Cell:   r.NewStyle(),
		Border: r.NewStyle().Foreground(p.Muted),
		Zebra:  r.NewStyle().Background(p.Subtle),
	}
}

// defaultPalette is the built-in palette. Dark keeps the xterm indices the TUI
// has always shipped, so a dark terminal renders exactly as before and a user's
// customized 256-color palette is still honored (a hex triple would override
// it). Light overrides only the tokens that are illegible on white, measured as
// WCAG contrast against #FFFFFF:
//
//	205 -> 127   205 (#FF5FAF) is ~1.9:1 on white; 127 (#AF00AF) is ~7.5:1
//	196 -> 124   196 (#FF0000) is ~4:1, under the 4.5:1 floor; 124 (#AF0000) ~8:1
//	 42 ->  28    42 (#00D787) is ~1.8:1 on white;  28 (#008700) is ~6.5:1
//
// Muted (240, #585858) is left alone: it already clears 7:1 on white. Adaptive
// means legible on both backgrounds, not different on both.
//
// Subtle is a background, so it is judged by a different rule than the five
// foregrounds above: 236 (#303030) on dark and 254 (#E4E4E4) on light are
// chosen so the DEFAULT FOREGROUND still clears contrast when painted on top
// of them, not so they contrast with the terminal's own background. A zebra
// stripe that swallows its own text is worse than no stripe at all.
func defaultPalette() palette {
	return palette{
		Primary: lipgloss.AdaptiveColor{Light: "127", Dark: "205"},
		Accent:  lipgloss.AdaptiveColor{Light: "127", Dark: "205"},
		Muted:   lipgloss.AdaptiveColor{Light: "240", Dark: "240"},
		Danger:  lipgloss.AdaptiveColor{Light: "124", Dark: "196"},
		Success: lipgloss.AdaptiveColor{Light: "28", Dark: "42"},

		// The 256-color index sits in the TrueColor slot on purpose, so only
		// the 16-color profile moves: measured, TrueColor and ANSI256 stay
		// byte-identical to the AdaptiveColor this replaces. At 16 colors the
		// only choices that are a shade rather than a hue are 0/8 (black,
		// bright black) and 7/15 (white, bright white); on a dark background 0
		// and 15 are the background and the text, leaving 8, and on a light one
		// the same argument leaves 7. Indices 0-15 belong to the user's
		// terminal theme, so the exact contrast is not knowable from here —
		// this is the best choice by construction, not a measured ratio like
		// the five foregrounds above.
		Subtle: lipgloss.CompleteAdaptiveColor{
			Light: lipgloss.CompleteColor{TrueColor: "254", ANSI256: "254", ANSI: "7"},
			Dark:  lipgloss.CompleteColor{TrueColor: "236", ANSI256: "236", ANSI: "8"},
		},
	}
}
