package themes

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestResolveBuiltins(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"", "default", "dracula", "nord"} {
		if _, err := Resolve(name, nil); err != nil {
			t.Errorf("Resolve(%q, nil) = %v, want nil — an empty name means default", name, err)
		}
	}
	got, err := Resolve("dracula", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Primary != Dracula().Primary {
		t.Errorf("Resolve(\"dracula\") did not return the dracula palette")
	}
}

// A theme names only what it changes; everything else comes from Default.
func TestResolveCustomInheritsTheRest(t *testing.T) {
	t.Parallel()
	got, err := Resolve("mine", map[string]Spec{
		"mine": {"accent": {Light: "1", Dark: "2"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := (lipgloss.AdaptiveColor{Light: "1", Dark: "2"}); got.Accent != want {
		t.Errorf("Accent = %+v, want %+v", got.Accent, want)
	}
	if got.Primary != Default().Primary {
		t.Errorf("Primary = %+v, want the default %+v — unnamed tokens must be inherited",
			got.Primary, Default().Primary)
	}
}

// A custom Subtle must still get its 16-color slot derived, or every custom
// theme silently undoes the zebra-stripe fix.
func TestResolveDerivesSubtleForACustomTheme(t *testing.T) {
	t.Parallel()
	got, err := Resolve("mine", map[string]Spec{
		"mine": {"subtle": {Light: "#E4E4E4", Dark: "#303030"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Subtle.Dark.ANSI != "8" || got.Subtle.Light.ANSI != "7" {
		t.Errorf("Subtle ANSI slots = %q/%q, want 7/8", got.Subtle.Light.ANSI, got.Subtle.Dark.ANSI)
	}
}

// Every failure names the theme, the token and the value: an error that only
// says "invalid color" makes the user hunt through their config by hand.
func TestResolveErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		theme  string
		custom map[string]Spec
		says   []string
	}{
		{
			name:  "unknown theme lists what is available",
			theme: "nope",
			says:  []string{"nope", "default", "dracula", "nord"},
		},
		{
			name:   "a custom theme may not shadow a built-in",
			theme:  "default",
			custom: map[string]Spec{"dracula": {}},
			says:   []string{"dracula", "built-in"},
		},
		{
			name:   "unknown token lists the valid ones",
			theme:  "mine",
			custom: map[string]Spec{"mine": {"primry": {Light: "1", Dark: "1"}}},
			says:   []string{"mine", "primry", "primary", "subtle"},
		},
		{
			name:   "invalid color names theme, token and value",
			theme:  "mine",
			custom: map[string]Spec{"mine": {"accent": {Light: "bogus", Dark: "1"}}},
			says:   []string{"mine", "accent", "bogus"},
		},
		{
			// The Light half being valid must not let a bad Dark half slip
			// through: both sides of a pair are validated independently.
			name:   "invalid dark half names theme, token and value",
			theme:  "mine",
			custom: map[string]Spec{"mine": {"accent": {Light: "1", Dark: "bogus"}}},
			says:   []string{"mine", "accent", "bogus"},
		},
		{
			name:   "an out-of-range index is rejected",
			theme:  "mine",
			custom: map[string]Spec{"mine": {"accent": {Light: "999", Dark: "1"}}},
			says:   []string{"mine", "accent", "999", "0-255"},
		},
		{
			name:   "an empty half of a pair is rejected, and says why",
			theme:  "mine",
			custom: map[string]Spec{"mine": {"accent": {Light: "", Dark: "1"}}},
			says:   []string{"mine", "accent", "quotes"},
		},
		{
			// A typo in a theme you are not using is still a typo. Selecting
			// "default" here means only the all-themes validation can catch it.
			name:   "a bad color in a theme that is not selected still fails",
			theme:  "default",
			custom: map[string]Spec{"unused": {"accent": {Light: "bogus", Dark: "1"}}},
			says:   []string{"unused", "accent", "bogus"},
		},
	} {
		_, err := Resolve(tc.theme, tc.custom)
		if err == nil {
			t.Errorf("%s: Resolve = nil error, want one", tc.name)
			continue
		}
		for _, s := range tc.says {
			if !strings.Contains(err.Error(), s) {
				t.Errorf("%s: error %q does not mention %q", tc.name, err, s)
			}
		}
	}
}
