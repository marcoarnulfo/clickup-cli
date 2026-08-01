package themes

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidColor(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in   string
		ok   bool
		says string // a substring the error must contain, when it errors
	}{
		{"#FF00AA", true, ""},
		{"#fff", true, ""},
		{"0", true, ""},
		{"255", true, ""},
		{"236", true, ""},
		// An unquoted #hex is a YAML comment, so it reaches validation as an
		// empty string. Measured: `muted: #fff` decodes to Value{"",""}. The
		// message has to say so, or the user hunts for a key they wrote right.
		{"", false, "quotes"},
		{"bogus", false, "neither"},
		{"#GGGGGG", false, "non-hex"},
		{"#FF00A", false, "#RGB or #RRGGBB"},
		{"256", false, "0-255"},
		{"999", false, "0-255"},
		{"-1", false, "0-255"},
	} {
		err := validColor(tc.in)
		if tc.ok {
			if err != nil {
				t.Errorf("validColor(%q) = %v, want nil", tc.in, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("validColor(%q) = nil, want an error", tc.in)
			continue
		}
		if !strings.Contains(err.Error(), tc.says) {
			t.Errorf("validColor(%q) = %q, want it to mention %q", tc.in, err, tc.says)
		}
	}
}

// shadeFor is what keeps the zebra stripe alive at 16 colors on a custom theme.
// The first two rows are the property that matters: they are the values the
// default palette had hand-written before this package existed, so the
// derivation must reproduce them exactly or the default theme changes color.
// Every number here was measured with termenv.ConvertToRGB + DistanceLab.
func TestShadeFor(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ in, want string }{
		{"236", "8"},     // #303030, d(8)=0.3372 d(7)=0.5784
		{"254", "7"},     // #E4E4E4, d(8)=0.3700 d(7)=0.1288
		{"#44475A", "8"}, // dracula Current Line
		{"#3B4252", "8"}, // nord1
		{"#808080", "8"}, // ANSI 8 itself, distance 0
		{"#A0A0A0", "7"}, // just past the crossover
		{"#000000", "8"},
		{"#FFFFFF", "7"},
	} {
		if got := shadeFor(tc.in); got != tc.want {
			t.Errorf("shadeFor(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestValueUnmarshalsBothForms(t *testing.T) {
	t.Parallel()
	var got Spec
	src := "accent: {light: \"127\", dark: \"205\"}\nmuted: \"240\"\n"
	if err := yaml.Unmarshal([]byte(src), &got); err != nil {
		t.Fatal(err)
	}
	if got["accent"] != (Value{Light: "127", Dark: "205"}) {
		t.Errorf("accent = %+v, want the pair", got["accent"])
	}
	// A bare string means the same color on both backgrounds.
	if got["muted"] != (Value{Light: "240", Dark: "240"}) {
		t.Errorf("muted = %+v, want 240 on both sides", got["muted"])
	}
}

// Without MarshalYAML a bare string comes back as a light/dark map, so the
// first thing that calls config.Save rewrites the user's file into a shape they
// did not choose. This test is the only thing that would notice.
func TestValueMarshalsABareStringBack(t *testing.T) {
	t.Parallel()
	out, err := yaml.Marshal(Spec{"muted": {Light: "240", Dark: "240"}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(out)), `muted: "240"`; got != want {
		t.Errorf("marshaled to %q, want %q", got, want)
	}
	pair, err := yaml.Marshal(Spec{"accent": {Light: "127", Dark: "205"}})
	if err != nil {
		t.Fatal(err)
	}
	// A real pair stays a pair, AND keeps light before dark. Measured: a
	// map[string]string marshals its keys alphabetically, so returning a map
	// would silently reorder what the user wrote.
	li := strings.Index(string(pair), "light:")
	di := strings.Index(string(pair), "dark:")
	if li < 0 || di < 0 {
		t.Fatalf("a genuine pair must stay a pair, got %q", pair)
	}
	if li > di {
		t.Errorf("marshaled %q — light must come before dark, as the user wrote it", pair)
	}
}
