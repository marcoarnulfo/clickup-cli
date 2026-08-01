package themes

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Resolve turns a configured theme name and the user's own themes into the
// palette the TUI renders with.
//
// Every failure is an error rather than a fallback, for the reason validColor's
// comment records: a bad color degrades invisibly — colorless text, or an
// escape sequence no terminal reads — so a silent fallback would leave the user
// staring at a wrong-looking TUI with nothing to go on. The same rule
// billing.rounding.increment follows.
func Resolve(name string, custom map[string]Spec) (Palette, error) {
	// EVERY user theme is validated, not only the selected one: a typo in a
	// theme you are not using today is still a typo, and the day you switch to
	// it is the worst possible moment to find out. Sorted so that a config with
	// two broken themes always reports the same one.
	for _, n := range slices.Sorted(maps.Keys(custom)) {
		if _, clash := builtins[n]; clash {
			return Palette{}, fmt.Errorf("theme %q is built-in; choose another name for your own", n)
		}
		if err := validateSpec(n, custom[n]); err != nil {
			return Palette{}, err
		}
	}
	if name == "" {
		name = "default"
	}
	if spec, ok := custom[name]; ok {
		return applySpec(spec), nil
	}
	if p, ok := builtins[name]; ok {
		return p(), nil
	}
	return Palette{}, fmt.Errorf("unknown theme %q; available: %s",
		name, strings.Join(allNames(custom), ", "))
}

// allNames is every theme the user could have named, sorted, for the error
// above: the built-ins plus whatever they defined themselves.
func allNames(custom map[string]Spec) []string {
	out := Names()
	for n := range custom {
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}

// validateSpec checks one user theme's tokens and colors. Separate from
// applySpec because Resolve validates every theme but applies only one.
func validateSpec(name string, spec Spec) error {
	for _, token := range slices.Sorted(maps.Keys(spec)) {
		if !slices.Contains(TokenNames, token) {
			return fmt.Errorf("theme %q: unknown token %q; valid tokens: %s",
				name, token, strings.Join(TokenNames, ", "))
		}
		v := spec[token]
		for _, side := range []string{v.Light, v.Dark} {
			if err := validColor(side); err != nil {
				return fmt.Errorf("theme %q, token %q, value %q: %w", name, token, side, err)
			}
		}
	}
	return nil
}

// applySpec starts from Default and overrides only the tokens the spec names.
// It assumes validateSpec has already passed.
func applySpec(spec Spec) Palette {
	p := Default()
	for token, v := range spec {
		switch token {
		case "primary":
			p.Primary = adaptive(v.Light, v.Dark)
		case "accent":
			p.Accent = adaptive(v.Light, v.Dark)
		case "muted":
			p.Muted = adaptive(v.Light, v.Dark)
		case "danger":
			p.Danger = adaptive(v.Light, v.Dark)
		case "success":
			p.Success = adaptive(v.Light, v.Dark)
		case "subtle":
			p.Subtle = subtle(v.Light, v.Dark)
		}
	}
	return p
}
