package tui

import (
	"fmt"
	"io"
	"strings"
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

// paletteTheme renders with real colors, so the palette goldens capture the
// exact escape sequences each token produces.
func paletteTheme(dark bool) theme {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.TrueColor)
	r.SetHasDarkBackground(dark)
	return newTheme(r, defaultPalette())
}

// paletteSample renders one labeled line per style, so a single golden pins
// every token's color.
func paletteSample(th theme) string {
	var b strings.Builder
	for _, row := range []struct {
		name string
		st   lipgloss.Style
	}{
		{"Title", th.Title}, {"Help", th.Help}, {"Err", th.Err},
		{"OK", th.OK}, {"Accent", th.Accent}, {"Box", th.Box},
		{"Border", th.Border}, {"Zebra", th.Zebra},
	} {
		fmt.Fprintf(&b, "%-8s%s\n", row.name, row.st.Render(row.name))
	}
	return b.String()
}

func TestGoldenPaletteDark(t *testing.T) {
	t.Parallel()
	golden(t, "palette_dark", paletteSample(paletteTheme(true)))
}

func TestGoldenPaletteLight(t *testing.T) {
	t.Parallel()
	golden(t, "palette_light", paletteSample(paletteTheme(false)))
}

// The four tokens that are unreadable on white must differ between
// backgrounds. Muted must NOT: 240 already clears 7:1 on white, so changing it
// would be churn dressed up as accessibility.
func TestPaletteIsAdaptive(t *testing.T) {
	t.Parallel()
	p := defaultPalette()
	for _, tc := range []struct {
		name string
		c    lipgloss.AdaptiveColor
	}{
		{"Primary", p.Primary}, {"Accent", p.Accent},
		{"Danger", p.Danger}, {"Success", p.Success},
	} {
		if tc.c.Light == tc.c.Dark {
			t.Errorf("%s is not adaptive: Light == Dark == %q", tc.name, tc.c.Dark)
		}
	}
	if p.Muted.Light != p.Muted.Dark {
		t.Errorf("Muted was adapted (%q/%q) but reads fine on both backgrounds",
			p.Muted.Light, p.Muted.Dark)
	}

	// Subtle is checked separately: it is a CompleteAdaptiveColor, so it does
	// not fit the AdaptiveColor loop above. The requirement is the same one —
	// light and dark must differ — carried over to the two slots that survived
	// the type change. (The TrueColor slot holds the same 256-color index as
	// ANSI256 by design, so checking it too would assert the same thing twice.)
	if s := defaultPalette().Subtle; s.Light.ANSI256 == s.Dark.ANSI256 || s.Light.ANSI == s.Dark.ANSI {
		t.Errorf("Subtle is not adaptive: light %+v, dark %+v", s.Light, s.Dark)
	}
}

// NO_COLOR is honored by termenv inside lipgloss's renderer; this pins that
// contract so a future lipgloss bump cannot silently break it.
//
// io.Discard is never a TTY, so termenv.WithUnsafe() is required to make
// EnvColorProfile() consult the environment at all instead of short-circuiting
// to Ascii regardless of NO_COLOR. termenv.WithUnsafe() bypasses the TTY check.
func TestNoColorProducesNoEscapes(t *testing.T) {
	// Neutralize the ambient environment: a developer or CI runner that
	// exports NO_COLOR (or CLICOLOR=0) must not make the baseline guard
	// below fire spuriously. The test controls its own environment.
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR", "")
	t.Setenv("COLORTERM", "truecolor")

	// Guard: without NO_COLOR the profile must resolve to a colored one, or
	// the assertion below would pass no matter what NO_COLOR does. Removing
	// this guard re-opens the hole this test exists to close.
	base := lipgloss.NewRenderer(io.Discard, termenv.WithUnsafe())
	if base.Output().EnvColorProfile() == termenv.Ascii {
		t.Fatal("baseline profile is already Ascii; the NO_COLOR assertion below would be vacuous")
	}

	t.Setenv("NO_COLOR", "1")
	r := lipgloss.NewRenderer(io.Discard, termenv.WithUnsafe())
	r.SetColorProfile(r.Output().EnvColorProfile())
	th := newTheme(r, defaultPalette())
	out := paletteSample(th)
	if strings.Contains(out, "\x1b") {
		t.Errorf("NO_COLOR=1 still produced escape sequences:\n%q", out)
	}
}

// The zebra style must actually carry a background: it is the only thing that
// distinguishes an odd row, and the package goldens run under termenv.Ascii,
// which strips backgrounds — so nothing else in the suite would notice it
// going missing.
func TestZebraCarriesBackground(t *testing.T) {
	t.Parallel()
	th := paletteTheme(true)
	if th.Zebra.GetBackground() == th.Cell.GetBackground() {
		t.Error("Zebra has the same background as a plain cell, so odd rows are indistinguishable")
	}
}

// The zebra stripe is the one palette token whose automatic downconvert lands
// on the wrong color. Measured, with Subtle as a plain AdaptiveColor: 236
// becomes "\x1b[40m" (black) on a dark background and 254 becomes "\x1b[107m"
// (bright white) on a light one — on a normal terminal, exactly the background
// color, so the stripe disappears. CompleteAdaptiveColor names the 16-color
// value instead of letting the nearest-color search pick it.
//
// The second half of this test is the one that keeps the change surgical: at
// TrueColor and ANSI256 the output must stay byte-identical to what the old
// AdaptiveColor produced. Putting a hex triple in the TrueColor slot would
// break that half — and would render as 227;227;227, not 228, because of a
// rounding step in lipgloss's hex path.
func TestZebraSurvivesSixteenColors(t *testing.T) {
	t.Parallel()
	old := lipgloss.AdaptiveColor{Light: "254", Dark: "236"}
	for _, tc := range []struct {
		name    string
		dark    bool
		profile termenv.Profile
		want    string // "" means: identical to the old AdaptiveColor
	}{
		{"dark 16-color", true, termenv.ANSI, "\x1b[100m"},
		{"light 16-color", false, termenv.ANSI, "\x1b[47m"},
		{"dark truecolor", true, termenv.TrueColor, ""},
		{"light truecolor", false, termenv.TrueColor, ""},
		{"dark 256-color", true, termenv.ANSI256, ""},
		{"light 256-color", false, termenv.ANSI256, ""},
	} {
		r := lipgloss.NewRenderer(io.Discard)
		r.SetColorProfile(tc.profile)
		r.SetHasDarkBackground(tc.dark)
		got := newTheme(r, defaultPalette()).Zebra.Render("x")

		if tc.want == "" {
			if want := r.NewStyle().Background(old).Render("x"); got != want {
				t.Errorf("%s: Zebra = %q, want %q — this profile must not move",
					tc.name, got, want)
			}
			continue
		}
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: Zebra = %q, want it to contain %q", tc.name, got, tc.want)
		}
	}
}

// CLICOLOR_FORCE is the force-color variable termenv implements (FORCE_COLOR,
// the npm convention, is deliberately not supported).
func TestCliColorForceKeepsColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR", "")
	t.Setenv("CLICOLOR_FORCE", "1")
	r := lipgloss.NewRenderer(io.Discard)
	if got := r.Output().EnvColorProfile(); got == termenv.Ascii {
		t.Error("CLICOLOR_FORCE=1 resolved to the Ascii profile, expected color")
	}
}
