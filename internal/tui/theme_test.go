package tui

import (
	"io"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// testTheme builds a theme on its own renderer, so tests never touch the
// global default renderer and stay parallel-safe.
func testTheme(dark bool) theme {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.Ascii)
	r.SetHasDarkBackground(dark)
	return newTheme(r, defaultPalette())
}

// The default palette must keep today's colors on a dark background, so the
// refactor is a no-op for the terminals most users are on.
func TestDefaultPaletteKeepsCurrentDarkColors(t *testing.T) {
	t.Parallel()
	p := defaultPalette()
	for _, tc := range []struct {
		name string
		got  string
		want string
	}{
		{"Primary", p.Primary.Dark, "205"},
		{"Accent", p.Accent.Dark, "205"},
		{"Muted", p.Muted.Dark, "240"},
		{"Danger", p.Danger.Dark, "196"},
		{"Success", p.Success.Dark, "42"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s dark = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

// Task 2 ships Light == Dark on purpose: the adaptive values land in Task 4,
// so this task cannot change a single rendered byte. Task 4 deletes this test.
func TestDefaultPaletteIsNotYetAdaptive(t *testing.T) {
	t.Parallel()
	p := defaultPalette()
	for _, tc := range []struct {
		name string
		c    lipgloss.AdaptiveColor
	}{
		{"Primary", p.Primary}, {"Accent", p.Accent}, {"Muted", p.Muted},
		{"Danger", p.Danger}, {"Success", p.Success},
	} {
		if tc.c.Light != tc.c.Dark {
			t.Errorf("%s: Light %q != Dark %q; the adaptive split belongs to Task 4",
				tc.name, tc.c.Light, tc.c.Dark)
		}
	}
}

// A theme built on a private renderer must not disturb the package default.
func TestThemeRendererIsIsolated(t *testing.T) {
	t.Parallel()
	before := lipgloss.ColorProfile()
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.TrueColor)
	_ = newTheme(r, defaultPalette())
	if after := lipgloss.ColorProfile(); after != before {
		t.Errorf("building a theme changed the default color profile: %v -> %v", before, after)
	}
}
