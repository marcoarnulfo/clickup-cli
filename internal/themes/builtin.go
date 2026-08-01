package themes

import (
	"maps"
	"slices"

	"github.com/charmbracelet/lipgloss"
)

// Default is the built-in palette. Dark keeps the xterm indices the TUI has
// always shipped; Light overrides only the tokens that are illegible on white,
// measured as WCAG contrast against #FFFFFF:
//
//	205 -> 127   205 (#FF5FAF) is 2.80:1 on white; 127 (#AF00AF) is 6.10:1
//	196 -> 124   196 (#FF0000) is 4.00:1, under the 4.5:1 floor; 124 (#AF0000) is 7.44:1
//	 42 ->  28    42 (#00D787) is 1.90:1 on white;  28 (#008700) is 4.70:1 — over the
//	              4.5:1 floor, but not by much
//
// Muted (240, #585858) is left alone: it already clears 7.11:1 on white.
// Adaptive means legible on both backgrounds, not different on both.
//
// Subtle is a background, judged by a different rule than the five foregrounds:
// 236 on dark and 254 on light are chosen so the DEFAULT FOREGROUND still
// clears contrast painted on top of them. A zebra stripe that swallows its own
// text is worse than no stripe at all.
func Default() Palette {
	return Palette{
		Primary: adaptive("127", "205"),
		Accent:  adaptive("127", "205"),
		Muted:   adaptive("240", "240"),
		Danger:  adaptive("124", "196"),
		Success: adaptive("28", "42"),
		Subtle:  subtle("254", "236"),
	}
}

// mono is a color that does not change with the terminal background. The two
// palettes below are built from it because both are designed for a dark
// terminal and are shipped faithfully: see the design doc's §2.4 for the
// measured contrast ratios on a light background, and the README for the
// warning users get.
func mono(c string) lipgloss.AdaptiveColor { return adaptive(c, c) }

// Dracula is draculatheme.com's published palette, mapped onto the six tokens
// as Primary=Purple, Accent=Pink, Muted=Comment, Danger=Red, Success=Green,
// Subtle=Current Line.
func Dracula() Palette {
	return Palette{
		Primary: mono("#BD93F9"),
		Accent:  mono("#FF79C6"),
		Muted:   mono("#6272A4"),
		Danger:  mono("#FF5555"),
		Success: mono("#50FA7B"),
		Subtle:  subtle("#44475A", "#44475A"),
	}
}

// Nord is nordtheme.com's published palette, mapped onto the six tokens as
// Primary=nord8, Accent=nord9 (both Frost), Muted=nord3 and Subtle=nord1 (both
// Polar Night), Danger=nord11 and Success=nord14 (both Aurora).
func Nord() Palette {
	return Palette{
		Primary: mono("#88C0D0"),
		Accent:  mono("#81A1C1"),
		Muted:   mono("#4C566A"),
		Danger:  mono("#BF616A"),
		Success: mono("#A3BE8C"),
		Subtle:  subtle("#3B4252", "#3B4252"),
	}
}

// builtins is the registry Resolve consults.
var builtins = map[string]func() Palette{
	"default": Default,
	"dracula": Dracula,
	"nord":    Nord,
}

// Names lists the built-in themes, sorted, for error messages.
func Names() []string {
	return slices.Sorted(maps.Keys(builtins))
}
