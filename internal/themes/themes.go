// Package themes owns the six semantic colors the TUI is built from, the
// built-in palettes, and the resolution of a user-written theme.
//
// It is a leaf on purpose. Both internal/config (which decodes a theme from
// YAML) and internal/tui (which turns one into lipgloss styles) need this type,
// and internal/tui already imports internal/config — so the type cannot live in
// either without creating an import cycle. The package name is plural because
// internal/tui already declares a type called `theme`, and Go forbids the same
// identifier in a file block (an import) and a package block (that type).
package themes

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"gopkg.in/yaml.v3"
)

// Palette is the six semantic colors the whole TUI is built from. It is the
// only place a color literal belongs: the TUI's styles derive from these.
//
// Subtle is a CompleteAdaptiveColor while the other five are AdaptiveColor
// because it is the one token whose automatic downconvert lands on the wrong
// color: at 16 colors there is no mid-grey, so the nearest-color search picks
// the background itself and the zebra stripe disappears. shadeFor names the
// 16-color value instead of letting termenv guess it.
type Palette struct {
	Primary lipgloss.AdaptiveColor
	Accent  lipgloss.AdaptiveColor
	Muted   lipgloss.AdaptiveColor
	Danger  lipgloss.AdaptiveColor
	Success lipgloss.AdaptiveColor
	Subtle  lipgloss.CompleteAdaptiveColor
}

// TokenNames are the YAML keys a theme may set, in the order error messages
// list them.
var TokenNames = []string{"primary", "accent", "muted", "danger", "success", "subtle"}

// Value is one token as written in YAML: either a bare color, meaning the same
// on both backgrounds, or a light/dark pair.
type Value struct{ Light, Dark string }

// UnmarshalYAML accepts both forms. internal/clickup's tagName does the same
// for ClickUp's JSON, where a tag arrives as either a string or an object.
func (v *Value) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		var s string
		if err := n.Decode(&s); err != nil {
			return err
		}
		v.Light, v.Dark = s, s
		return nil
	}
	var pair struct{ Light, Dark string }
	if err := n.Decode(&pair); err != nil {
		return err
	}
	v.Light, v.Dark = pair.Light, pair.Dark
	return nil
}

// yamlPair is the marshaled form of a genuine light/dark pair. It is a struct
// rather than a map because a map[string]string marshals its keys
// alphabetically — measured — which would silently reorder a user's
// `{light: …, dark: …}` into `{dark: …, light: …}` on the first save.
type yamlPair struct {
	Light string `yaml:"light"`
	Dark  string `yaml:"dark"`
}

// MarshalYAML collapses an equal pair back to a bare string. Without it,
// config.Save would rewrite a user's `muted: "240"` as a light/dark map the
// first time anything saves the config.
func (v Value) MarshalYAML() (any, error) {
	if v.Light == v.Dark {
		return v.Light, nil
	}
	return yamlPair(v), nil
}

// Spec is one user-written theme: token name -> value. A theme names only the
// tokens it changes; the rest come from Default.
type Spec map[string]Value

// validColor reports whether s is a color this package will accept: a #RGB or
// #RRGGBB hex, or a decimal index in 0-255.
//
// The range check is not defensive politeness. Measured: lipgloss renders an
// invalid color as no color at all — no error, no panic — so a typo produces a
// colorless TUI and no message; and it renders "999" as "\x1b[38;5;999m", a
// sequence no terminal understands. Nothing downstream will ever complain, so
// this is the only place a bad color can be caught.
func validColor(s string) error {
	if s == "" {
		// Measured: an unquoted #hex is a YAML comment, so `muted: #fff`
		// arrives here as "". Saying only "empty color" would send the user
		// looking for a key they wrote correctly.
		return fmt.Errorf("empty color (an unquoted #hex is a YAML comment: write it in quotes)")
	}
	if h, ok := strings.CutPrefix(s, "#"); ok {
		if len(h) != 3 && len(h) != 6 {
			return fmt.Errorf("hex color %q must be #RGB or #RRGGBB", s)
		}
		if strings.TrimLeft(strings.ToLower(h), "0123456789abcdef") != "" {
			return fmt.Errorf("hex color %q has a non-hex digit", s)
		}
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("color %q is neither a #hex nor a number", s)
	}
	if n < 0 || n > 255 {
		return fmt.Errorf("color index %q is outside 0-255", s)
	}
	return nil
}

// shadeFor picks the 16-color background a Subtle value degrades to.
//
// There is no threshold constant, deliberately: at 16 colors the only two
// shades that are neither a hue nor the background are 8 (#808080) and 7
// (#C0C0C0), so the rule is simply whichever of those two is perceptually
// closer. Measured, that reproduces the values the default palette carried
// before this package existed — 236 -> 8, 254 -> 7 — so moving the palette here
// does not change a single byte of what the default theme renders.
//
// Only ever called on a color validColor has already accepted.
func shadeFor(c string) string {
	col := termenv.ConvertToRGB(termenv.TrueColor.Color(c))
	d8 := col.DistanceLab(termenv.ConvertToRGB(termenv.TrueColor.Color("8")))
	d7 := col.DistanceLab(termenv.ConvertToRGB(termenv.TrueColor.Color("7")))
	if d7 < d8 {
		return "7"
	}
	return "8"
}

// adaptive builds an ordinary two-background color.
//
//lint:ignore U1000 not called yet: task 2 wires it into the built-in palettes
func adaptive(light, dark string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
}

// subtle builds the zebra background, naming its 16-color value per side. The
// 256-color index sits in the TrueColor slot on purpose, so that only the
// 16-color profile differs from what the automatic conversion would produce.
//
//lint:ignore U1000 not called yet: task 2 wires it into the built-in palettes
func subtle(light, dark string) lipgloss.CompleteAdaptiveColor {
	return lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: light, ANSI256: light, ANSI: shadeFor(light)},
		Dark:  lipgloss.CompleteColor{TrueColor: dark, ANSI256: dark, ANSI: shadeFor(dark)},
	}
}
