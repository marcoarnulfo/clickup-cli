package themes

import (
	"io"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Default must keep the exact values internal/tui carried before this package
// existed. Moving code must not change a color.
func TestDefaultKeepsTheShippedValues(t *testing.T) {
	t.Parallel()
	p := Default()
	for _, tc := range []struct {
		name        string
		got         lipgloss.AdaptiveColor
		light, dark string
	}{
		{"Primary", p.Primary, "127", "205"},
		{"Accent", p.Accent, "127", "205"},
		{"Muted", p.Muted, "240", "240"},
		{"Danger", p.Danger, "124", "196"},
		{"Success", p.Success, "28", "42"},
	} {
		if tc.got.Light != tc.light || tc.got.Dark != tc.dark {
			t.Errorf("%s = {%q, %q}, want {%q, %q}",
				tc.name, tc.got.Light, tc.got.Dark, tc.light, tc.dark)
		}
	}
	want := lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: "254", ANSI256: "254", ANSI: "7"},
		Dark:  lipgloss.CompleteColor{TrueColor: "236", ANSI256: "236", ANSI: "8"},
	}
	if p.Subtle != want {
		t.Errorf("Subtle = %+v, want %+v — shadeFor must reproduce the hand-picked slots", p.Subtle, want)
	}
}

// The four tokens that are unreadable on white differ between backgrounds;
// Muted must not, since 240 already clears 7:1 on white.
func TestDefaultIsAdaptive(t *testing.T) {
	t.Parallel()
	p := Default()
	for _, tc := range []struct {
		name string
		c    lipgloss.AdaptiveColor
	}{{"Primary", p.Primary}, {"Accent", p.Accent}, {"Danger", p.Danger}, {"Success", p.Success}} {
		if tc.c.Light == tc.c.Dark {
			t.Errorf("%s is not adaptive: Light == Dark == %q", tc.name, tc.c.Dark)
		}
	}
	if p.Muted.Light != p.Muted.Dark {
		t.Errorf("Muted was adapted (%q/%q) but reads fine on both backgrounds",
			p.Muted.Light, p.Muted.Dark)
	}
	if s := Default().Subtle; s.Light.ANSI256 == s.Dark.ANSI256 || s.Light.ANSI == s.Dark.ANSI {
		t.Errorf("Subtle is not adaptive: light %+v, dark %+v", s.Light, s.Dark)
	}
}

// Measured: lipgloss renders an invalid color as no color at all, silently. A
// typo in a hex we ship would therefore produce a colorless token and no error
// anywhere. This is the only thing that would catch it.
func TestEveryBuiltinTokenRendersAColor(t *testing.T) {
	t.Parallel()
	for _, name := range Names() {
		p, ok := builtins[name]
		if !ok {
			t.Fatalf("Names() returned %q, which is not in builtins", name)
		}
		pal := p()
		for _, dark := range []bool{true, false} {
			r := lipgloss.NewRenderer(io.Discard)
			r.SetColorProfile(termenv.TrueColor)
			r.SetHasDarkBackground(dark)
			for label, c := range map[string]lipgloss.TerminalColor{
				"Primary": pal.Primary, "Accent": pal.Accent, "Muted": pal.Muted,
				"Danger": pal.Danger, "Success": pal.Success, "Subtle": pal.Subtle,
			} {
				if out := r.NewStyle().Foreground(c).Render("x"); out == "x" {
					t.Errorf("%s/%s (dark=%v) rendered no escape at all: %q", name, label, dark, out)
				}
			}
		}
	}
}

// The other half of the guard: Default must RENDER exactly like the palette
// internal/tui carried before this package existed, at every profile and on
// both backgrounds. Matching values alone would not prove it — a slot the
// renderer consults could still differ.
func TestDefaultRendersLikeTheShippedPalette(t *testing.T) {
	t.Parallel()
	// The palette as internal/tui/theme.go declared it before the move.
	shipped := Palette{
		Primary: lipgloss.AdaptiveColor{Light: "127", Dark: "205"},
		Accent:  lipgloss.AdaptiveColor{Light: "127", Dark: "205"},
		Muted:   lipgloss.AdaptiveColor{Light: "240", Dark: "240"},
		Danger:  lipgloss.AdaptiveColor{Light: "124", Dark: "196"},
		Success: lipgloss.AdaptiveColor{Light: "28", Dark: "42"},
		Subtle: lipgloss.CompleteAdaptiveColor{
			Light: lipgloss.CompleteColor{TrueColor: "254", ANSI256: "254", ANSI: "7"},
			Dark:  lipgloss.CompleteColor{TrueColor: "236", ANSI256: "236", ANSI: "8"},
		},
	}
	got := Default()
	profiles := []termenv.Profile{termenv.TrueColor, termenv.ANSI256, termenv.ANSI, termenv.Ascii}
	for _, profile := range profiles {
		for _, dark := range []bool{true, false} {
			r := lipgloss.NewRenderer(io.Discard)
			r.SetColorProfile(profile)
			r.SetHasDarkBackground(dark)
			for _, tc := range []struct {
				name string
				a, b lipgloss.TerminalColor
			}{
				{"Primary", got.Primary, shipped.Primary},
				{"Accent", got.Accent, shipped.Accent},
				{"Muted", got.Muted, shipped.Muted},
				{"Danger", got.Danger, shipped.Danger},
				{"Success", got.Success, shipped.Success},
				{"Subtle", got.Subtle, shipped.Subtle},
			} {
				x := r.NewStyle().Foreground(tc.a).Render("x")
				y := r.NewStyle().Foreground(tc.b).Render("x")
				if x != y {
					t.Errorf("profile=%v dark=%v %s: rendered %q, want %q",
						profile, dark, tc.name, x, y)
				}
			}
		}
	}
}

func TestNamesAreSortedAndComplete(t *testing.T) {
	t.Parallel()
	got := Names()
	want := []string{"default", "dracula", "nord"}
	if len(got) != len(want) {
		t.Fatalf("Names() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Names() = %v, want %v (sorted)", got, want)
		}
	}
}
