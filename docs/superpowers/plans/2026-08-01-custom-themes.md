# Temi personalizzati — piano di implementazione

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** chiudere le caselle 1 e 3 della **#82** (tema da YAML, due temi
built-in) e il **punto 2 della #138** (colore del bordo di `th.Box`).

**Architecture:** un package foglia nuovo, `internal/themes`, possiede la
tavolozza, i temi built-in, il formato YAML e la validazione. `internal/config`
ne tiene solo il grezzo, `internal/cli` risolve e fa fallire l'avvio su un tema
invalido, `internal/tui` riceve la tavolozza già risolta.

**Tech Stack:** Go 1.26, lipgloss v1.1.0, termenv v0.16.0, yaml.v3.

**Spec:** `docs/superpowers/specs/2026-08-01-custom-themes-design.md`

## Global Constraints

- Tutto ciò che vive nel repo è in **inglese**: codice, identificatori,
  commenti, stringhe UI, nomi e messaggi dei test, messaggi di commit. Eccezioni:
  `README.it.md`, `CONTRIBUTING.it.md` e i doc di design sotto
  `docs/superpowers/`.
- **MAI** un trailer `Co-Authored-By`. Conventional Commits.
- `internal/report` e `internal/duration` restano **puri**: solo stdlib, nessun
  I/O, nessun import di `internal/config`, `internal/clickup`, `internal/tui`.
- **`internal/themes` è foglia**: importa solo `lipgloss`, `termenv`, `yaml.v3` e
  la stdlib. Non importa `config`, `tui`, `clickup`, `service`, e non fa I/O.
- **Mai chiamare l'API ClickUp vera.** Non esistono credenziali in questo
  ambiente. Il comportamento di rete si esercita solo con `httptest`.
- Nessuna funzione di stile in produzione chiama `lipgloss.NewStyle()`.
- **Ogni numero e ogni sequenza di escape scritta in un commento va misurata
  eseguendo il codice**, mai calcolata a mente.
- **Un test scritto contro un difetto non vale finché non lo si è visto fallire
  contro quel difetto**, con il transcript allegato al report.
- I golden si rigenerano solo con `go test ./internal/tui -update`, mai a mano, e
  si **guardano** dopo. In questa tranche si muovono **soltanto** `palette_dark`
  e `palette_light`; qualunque altro golden si muova è un difetto da indagare,
  non un golden da rigenerare.
- Gate prima di ogni commit, tutti e cinque, tutti puliti:
  `gofmt -l .` · `go vet ./...` ·
  `go run honnef.co/go/tools/cmd/staticcheck@latest ./...` · `go build ./...` ·
  `go test ./... -race`

---

### Task 1: il nucleo di `internal/themes`

**Files:**
- Create: `internal/themes/themes.go`
- Create: `internal/themes/themes_test.go`

**Interfaces:**
- Consuma: niente.
- Produce: `Palette`, `Value`, `Spec`, `TokenNames`, `validColor`, `shadeFor`,
  `adaptive`, `subtle`. I task 2 e 3 costruiscono su questi.

- [ ] **Step 1: scrivere i test che falliscono**

`internal/themes/themes_test.go`:

```go
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
```

- [ ] **Step 2: eseguirli e vederli fallire**

Run: `go test ./internal/themes -v`
Expected: il package non esiste ancora — errore di build.

- [ ] **Step 3: implementare**

`internal/themes/themes.go`:

```go
// Package themes owns the six semantic colors the TUI is built from, the
// built-in palettes, and the resolution of a user-written theme.
//
// It is a leaf by choice, not by necessity: internal/config could hold these
// types without an import cycle, since it imports no internal package of its
// own. They live here so that internal/config stays free of a rendering
// library, so that color tests run without internal/tui's TestMain pinning
// termenv.Ascii for the whole package, and so that validation stays a unit
// small enough to hold in one's head.
//
// The package name is plural because internal/tui already declares a type
// called `theme`, and Go forbids the same identifier in a file block (an
// import) and a package block (that type). An aliased import would compile;
// the plural avoids needing one at every import site.
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
	return yamlPair{Light: v.Light, Dark: v.Dark}, nil
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
func adaptive(light, dark string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
}

// subtle builds the zebra background, naming its 16-color value per side. The
// 256-color index sits in the TrueColor slot on purpose, so that only the
// 16-color profile differs from what the automatic conversion would produce.
func subtle(light, dark string) lipgloss.CompleteAdaptiveColor {
	return lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: light, ANSI256: light, ANSI: shadeFor(light)},
		Dark:  lipgloss.CompleteColor{TrueColor: dark, ANSI256: dark, ANSI: shadeFor(dark)},
	}
}
```

- [ ] **Step 4: eseguirli e vederli passare**

Run: `go test ./internal/themes -v`
Expected: PASS (4 test).

- [ ] **Step 5: prova per mutazione, obbligatoria**

Due mutazioni, una alla volta, ognuna con il transcript nel report:

1. In `shadeFor`, invertire il confronto (`if d7 > d8`). `TestShadeFor` **deve**
   fallire su `236` e `254`.
2. Cancellare `MarshalYAML`. `TestValueMarshalsABareStringBack` **deve** fallire.

Ripristinare dopo ognuna e verificare con `git diff`.

- [ ] **Step 6: gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/themes/
git commit -m "feat(themes): add the leaf package that owns the palette (#82)"
```

---

### Task 2: i temi built-in e il registro

**Files:**
- Create: `internal/themes/builtin.go`
- Create: `internal/themes/builtin_test.go`

**Interfaces:**
- Consuma: `Palette`, `adaptive`, `subtle` dal Task 1.
- Produce: `Default()`, `Dracula()`, `Nord()`, `Names()`, e la mappa `builtins`
  che il Task 3 consulta.

**Contesto che il brief non può sapere.** I valori di `Default()` sono
esattamente quelli che `internal/tui/theme.go` ha oggi in `defaultPalette()`, e
il Task 5 cancellerà quella funzione. Non vanno «migliorati»: il test di questo
task esiste apposta per inchiodarli.

- [ ] **Step 1: scrivere i test che falliscono**

`internal/themes/builtin_test.go`:

```go
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
```

- [ ] **Step 2: eseguirli e vederli fallire**

Run: `go test ./internal/themes -run 'TestDefault|TestEveryBuiltin|TestNames' -v`
Expected: FAIL in compilazione — `undefined: Default`, `builtins`, `Names`.

- [ ] **Step 3: implementare**

`internal/themes/builtin.go`:

```go
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
//	205 -> 127   205 (#FF5FAF) is ~1.9:1 on white; 127 (#AF00AF) is ~7.5:1
//	196 -> 124   196 (#FF0000) is ~4:1, under the 4.5:1 floor; 124 (#AF0000) ~8:1
//	 42 ->  28    42 (#00D787) is ~1.8:1 on white;  28 (#008700) is ~6.5:1
//
// Muted (240, #585858) is left alone: it already clears 7:1 on white. Adaptive
// means legible on both backgrounds, not different on both.
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
```

- [ ] **Step 4: eseguirli e vederli passare**

Run: `go test ./internal/themes -v`
Expected: PASS (9 test).

- [ ] **Step 5: prova per mutazione, obbligatoria**

Due mutazioni, con transcript:

1. Cambiare un carattere di un hex di `Dracula` in qualcosa di non valido (per
   esempio `#BD93FG`). `TestEveryBuiltinTokenRendersAColor` **deve** fallire. È
   la mutazione che dimostra che la trappola del §2.1 è davvero coperta.
2. In `Default()`, cambiare `subtle("254", "236")` in `subtle("254", "235")`.
   **Devono** fallire sia `TestDefaultKeepsTheShippedValues` sia
   `TestDefaultRendersLikeTheShippedPalette` — se solo il primo cade, il secondo
   non sta rendendo quello che dice.

Ripristinare dopo ognuna e verificare con `git diff`.

- [ ] **Step 6: gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/themes/
git commit -m "feat(themes): ship the dracula and nord palettes (#82)"
```

---

### Task 3: `Resolve` e i suoi errori

**Files:**
- Create: `internal/themes/resolve.go`
- Create: `internal/themes/resolve_test.go`

**Interfaces:**
- Consuma: `Palette`, `Spec`, `Value`, `TokenNames`, `validColor`, `adaptive`,
  `subtle`, `builtins`, `Names`.
- Produce: `func Resolve(name string, custom map[string]Spec) (Palette, error)`.
  Il Task 5 lo chiama da `internal/cli`.

- [ ] **Step 1: scrivere i test che falliscono**

`internal/themes/resolve_test.go`:

```go
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
```

- [ ] **Step 2: eseguirli e vederli fallire**

Run: `go test ./internal/themes -run TestResolve -v`
Expected: FAIL in compilazione — `undefined: Resolve`.

- [ ] **Step 3: implementare**

`internal/themes/resolve.go`:

```go
package themes

import (
	"fmt"
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
```

Il blocco import di `resolve.go` è quindi `fmt`, `maps`, `slices`, `strings`.

- [ ] **Step 4: eseguirli e vederli passare**

Run: `go test ./internal/themes -v`
Expected: PASS (13 test).

- [ ] **Step 5: prova per mutazione, obbligatoria**

Tre mutazioni, una alla volta, con transcript:

1. In `validateSpec`, togliere il ciclo `validColor`. I casi «invalid color»,
   «out-of-range» ed «empty half» di `TestResolveErrors` **devono** fallire.
2. In `applySpec`, far partire `p` da `Palette{}` invece che da `Default()`.
   `TestResolveCustomInheritsTheRest` **deve** fallire.
3. In `Resolve`, validare solo il tema selezionato invece di tutti (spostare la
   chiamata a `validateSpec` dentro il ramo `if spec, ok := custom[name]`). Il
   caso «a bad color in a theme that is not selected» **deve** fallire, e gli
   altri no.

Ripristinare dopo ognuna e verificare con `git diff`.

- [ ] **Step 6: gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/themes/
git commit -m "feat(themes): resolve a configured theme, refusing bad input (#82)"
```

---

### Task 4: le chiavi `theme` e `themes` nel config

**Files:**
- Modify: `internal/config/config.go` (struct `Config`)
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consuma: `themes.Spec` dal Task 1.
- Produce: `Config.Theme string`, `Config.Themes map[string]themes.Spec`. Il
  Task 5 li legge da `internal/cli`.

- [ ] **Step 1: scrivere i test che falliscono**

In fondo a `internal/config/config_test.go` (import di
`"github.com/marcoarnulfo/clickup-cli/internal/themes"`):

```go
// --- theme keys (#82) ---

func TestThemeKeysRoundTrip(t *testing.T) {
	isolateConfig(t)
	in := Config{Token: "t", WorkspaceID: "1", Theme: "mine", Themes: map[string]themes.Spec{
		"mine": {
			"muted":  {Light: "240", Dark: "240"},
			"accent": {Light: "127", Dark: "205"},
		},
	}}
	if err := Save(in); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Theme != "mine" {
		t.Errorf("Theme = %q, want %q", got.Theme, "mine")
	}
	if got.Themes["mine"]["muted"] != (themes.Value{Light: "240", Dark: "240"}) {
		t.Errorf("muted = %+v, want 240 on both sides", got.Themes["mine"]["muted"])
	}
	if got.Themes["mine"]["accent"] != (themes.Value{Light: "127", Dark: "205"}) {
		t.Errorf("accent = %+v, want the pair", got.Themes["mine"]["accent"])
	}
}

// A token the user wrote as a bare string must come back as a bare string:
// Save serializes the whole Config, so without Value.MarshalYAML every save
// would rewrite their file into a shape they did not choose.
func TestSaveKeepsABareColorBare(t *testing.T) {
	isolateConfig(t)
	if err := Save(Config{Token: "t", WorkspaceID: "1", Themes: map[string]themes.Spec{
		"mine": {"muted": {Light: "240", Dark: "240"}},
	}}); err != nil {
		t.Fatal(err)
	}
	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `muted: "240"`) {
		t.Fatalf("saved config did not keep the bare form:\n%s", raw)
	}
}

func TestThemeKeysAbsentAreZero(t *testing.T) {
	isolateConfig(t)
	if err := Save(Config{Token: "t", WorkspaceID: "1"}); err != nil {
		t.Fatal(err)
	}
	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "theme") {
		t.Fatalf("saved config mentions theme:\n%s", raw)
	}
}
```

- [ ] **Step 2: eseguirli e vederli fallire**

Run: `go test ./internal/config -run 'TestTheme|TestSaveKeeps' -v`
Expected: FAIL in compilazione — `unknown field Theme`.

- [ ] **Step 3: implementare**

In `internal/config/config.go`, dentro `Config`, sotto `Mouse`:

```go
	// Theme names the palette the TUI renders with: a built-in name, or one of
	// the Themes below. Empty means the built-in default. The names are
	// resolved by internal/themes, not here — this package holds the YAML as
	// written and nothing more.
	Theme string `yaml:"theme,omitempty"`
	// Themes are the user's own palettes, by name. A theme names only the
	// tokens it changes; internal/themes fills the rest from the default.
	Themes map[string]themes.Spec `yaml:"themes,omitempty"`
```

più l'import di `"github.com/marcoarnulfo/clickup-cli/internal/themes"`.

- [ ] **Step 4: eseguirli e vederli passare**

Run: `go test ./internal/config -run 'TestTheme|TestSaveKeeps' -v`
Expected: PASS (3 test).

- [ ] **Step 5: gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/config/
git commit -m "feat(config): add the theme and themes keys (#82)"
```

---

### Task 5: la TUI riceve la tavolozza risolta

**Files:**
- Modify: `internal/tui/theme.go` (cancella `palette` e `defaultPalette`)
- Modify: `internal/tui/app.go` (`New`)
- Modify: `internal/cli/cli.go` (`runTUI`, più la nuova `resolveTheme`)
- Modify: `internal/tui/theme_test.go`, `internal/tui/palette_test.go`,
  `internal/tui/app_test.go`, `internal/tui/log_test.go`,
  `internal/tui/report_test.go`, `internal/tui/demo_test.go`,
  `internal/tui/golden_test.go`, `internal/tui/home_test.go`,
  `internal/tui/palette_demo_test.go`
- Test: `internal/cli/cli_test.go`

**Interfaces:**
- Consuma: `themes.Palette`, `themes.Default()`, `themes.Resolve` dai Task 1-3;
  `config.Theme`/`config.Themes` dal Task 4.
- Produce: `func New(cfg config.Config, pal themes.Palette) Model`.

**Contesto che il brief non può sapere.**

1. `defaultPalette()` è referenziata in **11 punti**: la definizione
   (`theme.go:86-87`), `app.go:210`, `palette_test.go:399`, e `theme_test.go` ×8
   (righe 19, 26, 50, 62, 97, 119, 149, 199). Tutte diventano `themes.Default()`,
   tranne la definizione che sparisce.
2. **Due test di `theme_test.go` sono già stati riscritti nel Task 2 dentro
   `internal/themes` e vanno cancellati qui**, non adattati:
   `TestDefaultPaletteKeepsCurrentDarkColors` (riga ~24) è ora
   `TestDefaultKeepsTheShippedValues`, e `TestPaletteIsAdaptive` (riga ~93) è ora
   `TestDefaultIsAdaptive`. Tenerli in due package è duplicazione, non copertura.
3. Ogni altro test di `theme_test.go` resta dov'è: verifica come `newTheme`
   costruisce gli **stili**, che è responsabilità di `internal/tui`.
4. **`New` ha 44 call site su 8 file**, misurati: `app_test.go` 33,
   `demo_test.go` 3, `log_test.go` 2, `report_test.go` 2, `golden_test.go` 1,
   `home_test.go` 1, `palette_demo_test.go` 1, `internal/cli/cli.go:57` 1. Nei
   test si passa `themes.Default()`.

   **Non cercarli con `grep 'New(cfg)'`**: quella stringa ne trova nove. Le altre
   chiamate sono `New(config.Config{…})` (34 nel solo `app_test.go`),
   `New(demoConfig())` e `New(realCfg)`. La prima versione di questo piano diceva
   «nove», e chi l'avesse seguita avrebbe lasciato `internal/tui` non compilabile
   a metà del task. Il modo affidabile è compilare: `go build ./... && go vet ./...`
   dopo il cambio di firma elenca ogni chiamata rimasta.

- [ ] **Step 1: scrivere il test che fallisce**

In `internal/cli/cli_test.go`:

```go
// An unresolvable theme must stop the launch with an error the user can act on,
// not start a TUI with the wrong colors. The check is on resolveTheme, which is
// the production code path runTUI takes: runTUI itself blocks on a terminal and
// cannot be called from a test, the same reason programOptions was extracted.
func TestResolveThemeRejectsAnUnknownName(t *testing.T) {
	t.Parallel()
	_, err := resolveTheme(config.Config{Theme: "nope"})
	if err == nil {
		t.Fatal("resolveTheme of an unknown theme = nil error, want one")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q does not name the theme the user asked for", err)
	}
	// The prefix is what tells the user which part of their config is at fault
	// once Execute prints it as "error: …".
	if !strings.HasPrefix(err.Error(), "theme:") {
		t.Errorf("error %q is not prefixed with the config section it comes from", err)
	}
}

func TestResolveThemeAcceptsABuiltin(t *testing.T) {
	t.Parallel()
	if _, err := resolveTheme(config.Config{Theme: "dracula"}); err != nil {
		t.Errorf("resolveTheme(dracula) = %v, want nil", err)
	}
	if _, err := resolveTheme(config.Config{}); err != nil {
		t.Errorf("resolveTheme with no theme set = %v, want nil (the default)", err)
	}
}
```

- [ ] **Step 2: eseguirli e vederli fallire**

Run: `go test ./internal/cli -run TestResolveTheme -v`
Expected: FAIL in compilazione — `undefined: resolveTheme`.

- [ ] **Step 3: implementare**

In `internal/tui/theme.go`: cancellare il tipo `palette` e la funzione
`defaultPalette`, e cambiare la firma:

```go
func newTheme(r *lipgloss.Renderer, p themes.Palette) theme {
```

Il corpo non cambia: i campi di `themes.Palette` hanno gli stessi nomi.

In `internal/tui/app.go`:

```go
func New(cfg config.Config, pal themes.Palette) Model {
```

e alla riga del campo `theme:`

```go
		theme:  newTheme(lipgloss.DefaultRenderer(), pal),
```

In `internal/cli/cli.go`, una funzione nuova accanto a `programOptions`:

```go
// resolveTheme builds the palette the TUI will render with, and refuses to
// start on a configuration it cannot honor.
//
// Extracted from runTUI for the same reason programOptions was: runTUI blocks
// on a terminal and cannot be called from a test, while the decision it makes
// can. Called before the program starts because this is the last place a
// configuration error still reaches stderr — Execute prints it as "error: …" —
// instead of appearing inside an already-running TUI.
func resolveTheme(cfg config.Config) (themes.Palette, error) {
	pal, err := themes.Resolve(cfg.Theme, cfg.Themes)
	if err != nil {
		return themes.Palette{}, fmt.Errorf("theme: %w", err)
	}
	return pal, nil
}
```

e dentro `runTUI`, subito dopo `config.Load()`:

```go
	pal, err := resolveTheme(cfg)
	if err != nil {
		return err
	}
	p := tea.NewProgram(tui.New(cfg, pal), programOptions(cfg)...)
```

Nei test di `internal/tui`, sostituire `defaultPalette()` con `themes.Default()`
e `New(cfg)` con `New(cfg, themes.Default())`, e cancellare i due test elencati
nel punto 2 del contesto.

- [ ] **Step 4: eseguire tutto**

Run: `go test ./... -race`
Expected: PASS, e **nessun golden modificato** — questa task non cambia un
colore, sposta soltanto da dove arrivano. Verificare con
`git status --short internal/tui/testdata/`: deve essere vuoto. Se un golden si
muove, è un difetto: non rigenerarlo.

- [ ] **Step 5: prova per mutazione, obbligatoria**

Due mutazioni, con transcript:

1. In `resolveTheme`, ignorare l'errore e ritornare `themes.Default(), nil`.
   `TestResolveThemeRejectsAnUnknownName` **deve** fallire.
2. In `resolveTheme`, togliere il `fmt.Errorf("theme: %w", …)` e ritornare `err`
   nudo. Il test **deve** fallire sull'asserzione del prefisso — se passa lo
   stesso, quell'asserzione non serve a niente e va tolta o riscritta.

Un limite da **dichiarare nel report** invece di nasconderlo: nessuno dei due
test prova che `runTUI` *chiami* `resolveTheme`, perché `runTUI` blocca su un
terminale. Mostrare con `git diff` che la chiamata c'è e che è l'unico percorso.

- [ ] **Step 6: gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui/ internal/cli/
git commit -m "refactor(tui): take the palette from internal/themes (#82)"
```

---

### Task 6: il bordo di `th.Box` (#138 punto 2)

**Files:**
- Modify: `internal/tui/theme.go` (`newTheme`, campo `Box`)
- Modify: `internal/tui/testdata/palette_dark.golden`,
  `internal/tui/testdata/palette_light.golden` (rigenerati, mai a mano)
- Test: `internal/tui/theme_test.go`

**Interfaces:** nessuna nuova.

- [ ] **Step 1: scrivere il test che fallisce**

In `internal/tui/theme_test.go`:

```go
// The report screen draws its table frame with th.Border and its empty-state
// box with th.Box. Until th.Box carried a border color the two frames rendered
// at different weights on the same screen, which is #138's second point: a user
// whose report has no hours saw a different frame from one whose report does.
func TestBoxAndBorderShareTheirColor(t *testing.T) {
	t.Parallel()
	for _, dark := range []bool{true, false} {
		th := paletteTheme(dark)
		if got, want := th.Box.GetBorderTopForeground(), th.Border.GetForeground(); got != want {
			t.Errorf("dark=%v: Box border = %+v, Border = %+v — the two frames must weigh the same",
				dark, got, want)
		}
	}
}
```

- [ ] **Step 2: eseguirlo e vederlo fallire**

Run: `go test ./internal/tui -run TestBoxAndBorderShareTheirColor -v`
Expected: FAIL su entrambi i fondi. Misurato, i due valori a confronto sono
`lipgloss.NoColor{}` (quello che un bordo senza colore ritorna) contro
`lipgloss.AdaptiveColor{Light:"240", Dark:"240"}`. **Incollare il transcript.**

- [ ] **Step 3: implementare**

In `internal/tui/theme.go`, dentro `newTheme`:

```go
		Box: r.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(p.Muted).Padding(0, 1),
```

- [ ] **Step 4: rigenerare e GUARDARE i due golden**

```bash
go test ./internal/tui -update
git status --short internal/tui/testdata/
```

Expected: esattamente **due** file modificati, `palette_dark.golden` e
`palette_light.golden`. Se ne compaiono altri, fermarsi e riportare: sarebbe un
difetto dell'implementazione, non un golden stantio.

Poi **aprire i due file e guardarli**: la riga `Box` deve portare
`\x1b[38;5;240m` sul bordo, e nient'altro deve essere cambiato. Riportare cosa
si è visto, non che si è eseguito il comando.

- [ ] **Step 5: eseguire tutto**

Run: `go test ./... -race`
Expected: PASS.

- [ ] **Step 6: gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui/
git commit -m "fix(tui): give the box border the same color as the table frame (#138)"
```

---

### Task 7: documentazione

**Files:**
- Modify: `README.md` (elenco delle chiavi di config)
- Modify: `README.it.md` (stesso punto)
- Modify: `CHANGELOG.md` (sezione `## [Unreleased]`)

**Interfaces:** nessuna.

- [ ] **Step 1: `README.md`**

Dopo il punto elenco `- \`mouse\` (optional): …`:

```markdown
- `theme` (optional): the palette the TUI renders with. Built-in names are
  `default`, `dracula` and `nord`; you can also name one of your own from `themes`
  below. Omitting the key uses `default`. An unknown name stops `clup` from starting
  rather than silently falling back.
  - **`dracula` and `nord` are designed for a dark terminal and are shipped
    faithfully.** On a light background parts of the UI become hard or impossible
    to read — measured against white, dracula's green sits at 1.37:1 and nord's
    zebra stripe at 2.09:1, both far below the 3:1 floor. `default` is the
    adaptive one: it changes with your terminal background.
- `themes` (optional): your own palettes, by name. A theme names only the tokens it
  changes; the rest come from `default`. The six tokens are `primary`, `accent`,
  `muted`, `danger`, `success` and `subtle` (the report's zebra stripe). A value is
  either one color, used on both backgrounds, or a `{light: …, dark: …}` pair. A
  color is a `#RGB`/`#RRGGBB` hex or a number from 0 to 255. Anything else stops
  startup with a message naming the theme, the token and the value — and every
  theme you define is checked, not just the one you selected.
  - **Quote your hex values.** In YAML an unquoted `#` starts a comment, so
    `muted: #fff` sets `muted` to nothing at all. `muted: "#fff"` is the color.
    Plain numbers need no quotes.

    ```yaml
    theme: mine
    themes:
      mine:
        accent: {light: "127", dark: "205"}
        muted: "240"
    ```
```

- [ ] **Step 2: `README.it.md`**

Lo stesso contenuto, in italiano, dopo il punto elenco `- \`mouse\` (opzionale): …`.
Mantenere i due file equivalenti nel significato, compresi i numeri misurati.

- [ ] **Step 3: `CHANGELOG.md`**

In `### Added` sotto `## [Unreleased]`:

```markdown
- Themes: `theme: dracula` (or `nord`) picks a built-in palette, and `themes:` in
  the config defines your own, naming only the tokens you want to change. An
  unknown theme or an invalid color stops startup with a message naming the
  theme, the token and the value, because lipgloss renders a bad color as no
  color at all and would otherwise fail silently (#82).
```

In `### Fixed`:

```markdown
- The report's empty-state box is drawn with the same border color as the table
  frame, so a report with no hours no longer shows a differently-weighted frame
  from one with hours (#138).
```

- [ ] **Step 4: rileggere quello che si è scritto**

Aprire i tre file e **guardare** le sezioni modificate in contesto: che i punti
elenco stiano nella lista giusta, che l'indentazione combaci con i vicini, che il
blocco YAML sia annidato correttamente sotto il suo punto elenco, e che le due
voci di changelog siano nella sezione giusta. Riportare cosa si è visto.

- [ ] **Step 5: gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add README.md README.it.md CHANGELOG.md
git commit -m "docs: document the theme keys and the box border fix (#82, #138)"
```

---

## Note per il controllore

- La **#82 non si chiude** con questa tranche: resta la casella 2, gli override
  delle keybinding, che è la tranche successiva. Chiudere la issue qui sarebbe
  sbagliato.
- La **#138 si chiude**: il punto 1 era già stato risolto dalla tranche D
  (`reportNumWidths` misurato dal contenuto ha sostituito la costante), il punto
  2 è il Task 6.
- Parità demo-mode: `cli` risolve dal config vero e passa la tavolozza a `New`,
  che sostituisce solo `cfg` con `demoConfig()`. Quindi `CLICKUP_DEMO=1` onora il
  tema dell'utente. In CI, dove un config non esiste, resta il default e il GIF
  del README non cambia.
