# Accessibility base — piano di implementazione

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** chiudere la **#74** — la rotella del mouse scorre le liste, e la
zebratura del report sopravvive su un terminale a 16 colori.

**Architecture:** la rotella viene tradotta in un `tea.KeyMsg` su/giù e rigiocata
dentro l'instradamento che già esiste (`updateOverlay` / `routeKey`), così ogni
schermata la eredita senza modifiche. Il colore della zebratura passa da
`AdaptiveColor` (conversione automatica) a `CompleteAdaptiveColor` (un valore
dichiarato per profilo) **solo** per il token `Subtle`.

**Tech Stack:** Go 1.26, bubbletea v1.3.10, lipgloss v1.1.0, termenv v0.16.0.

**Spec:** `docs/superpowers/specs/2026-08-01-accessibility-base-design.md`

## Global Constraints

- Tutto ciò che vive nel repo è in **inglese**: codice, identificatori,
  commenti, stringhe UI, nomi e messaggi dei test, messaggi di commit. Unica
  eccezione: i doc di design sotto `docs/superpowers/`.
- **MAI** un trailer `Co-Authored-By` in un messaggio di commit.
- Conventional Commits.
- `internal/report` e `internal/duration` restano **puri**: nessun I/O, solo
  stdlib, nessun import di `internal/config`, `internal/clickup`, `internal/tui`.
- **Mai chiamare l'API ClickUp vera.** Non esistono credenziali in questo
  ambiente. Il comportamento di rete si esercita solo con `httptest`.
- Nessuna funzione di stile in produzione chiama `lipgloss.NewStyle()`: gli
  stili vengono dal `theme` costruito su un renderer iniettato.
- **Ogni numero e ogni sequenza di escape scritta in un commento va misurata
  eseguendo il codice**, mai calcolata a mente.
- **Un test scritto contro un difetto non vale finché non lo si è visto fallire
  contro quel difetto**, con il transcript allegato al report.
- I golden si rigenerano solo con `go test ./internal/tui -update`, mai a mano, e
  si **guardano** dopo averli rigenerati.
- Gate prima di ogni commit, tutti e cinque, tutti puliti:
  `gofmt -l .` · `go vet ./...` ·
  `go run honnef.co/go/tools/cmd/staticcheck@latest ./...` · `go build ./...` ·
  `go test ./... -race`.

---

### Task 1: la chiave di config `mouse`

**Files:**
- Modify: `internal/config/config.go` (struct `Config`, accanto a `UpdateCheck`)
- Test: `internal/config/config_test.go` (in fondo, nuova sezione)

**Interfaces:**
- Consuma: niente.
- Produce: `Config.Mouse *bool` e `func (c Config) MouseEnabled() bool`. Il
  Task 3 chiama `MouseEnabled()`.

- [ ] **Step 1: scrivere i test che falliscono**

In fondo a `internal/config/config_test.go`:

```go
// --- optional mouse key (#74) ---

func TestMouseAbsentIsNil(t *testing.T) {
	// A config file without the key must load as nil — meaning "enabled".
	// With a plain bool the absent key would decode as false, and mouse
	// support would be born disabled in every config written so far.
	isolateConfig(t)
	if err := Save(Config{Token: "t", WorkspaceID: "1"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mouse != nil {
		t.Fatalf("Mouse = %v, want nil for an absent key", *cfg.Mouse)
	}
}

func TestMouseFalseRoundTrips(t *testing.T) {
	isolateConfig(t)
	no := false
	if err := Save(Config{Token: "t", WorkspaceID: "1", Mouse: &no}); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Mouse == nil || *got.Mouse {
		t.Fatalf("Mouse = %v, want an explicit false", got.Mouse)
	}
}

func TestMouseNilIsNotWrittenToDisk(t *testing.T) {
	// omitempty matters: without it Save writes "mouse: null" into every
	// config file it touches.
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
	if strings.Contains(string(raw), "mouse") {
		t.Fatalf("saved config mentions mouse:\n%s", raw)
	}
}

func TestMouseEnabled(t *testing.T) {
	t.Parallel()
	yes, no := true, false
	for _, tc := range []struct {
		name string
		in   *bool
		want bool
	}{
		{"absent", nil, true},
		{"explicit true", &yes, true},
		{"explicit false", &no, false},
	} {
		if got := (Config{Mouse: tc.in}).MouseEnabled(); got != tc.want {
			t.Errorf("%s: MouseEnabled() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: eseguirli e vederli fallire**

Run: `go test ./internal/config -run 'TestMouse' -v`
Expected: FAIL in compilazione — `unknown field Mouse` e
`cfg.MouseEnabled undefined`.

- [ ] **Step 3: implementare**

In `internal/config/config.go`, dentro `Config`, subito sotto `UpdateCheck`:

```go
	// Mouse enables mouse reporting, which turns the wheel into scrolling.
	// nil means enabled, for the same reason as UpdateCheck above: a plain
	// bool would make an absent key decode as false and silently disable a
	// default-on feature in every config written before this field existed.
	//
	// The escape hatch exists because mouse reporting takes the terminal's
	// native text selection away from the user: with it on, drag-to-select
	// needs Shift in most terminals.
	Mouse *bool `yaml:"mouse,omitempty"`
```

E, accanto a `Valid()`:

```go
// MouseEnabled reports whether the TUI should turn on mouse reporting.
func (c Config) MouseEnabled() bool { return c.Mouse == nil || *c.Mouse }
```

- [ ] **Step 4: eseguirli e vederli passare**

Run: `go test ./internal/config -run 'TestMouse' -v`
Expected: PASS (4 test).

- [ ] **Step 5: gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add the optional mouse key (#74)"
```

---

### Task 2: la rotella scorre

**Files:**
- Create: `internal/tui/mouse.go`
- Create: `internal/tui/mouse_test.go`
- Modify: `internal/tui/app.go` (type-switch di `Update`, accanto a `case tea.KeyMsg`)

**Interfaces:**
- Consuma: `routeKey`, `updateOverlay`, `Model.overlay` (già esistenti).
- Produce: `func wheelKey(msg tea.MouseMsg) (tea.KeyMsg, bool)`.

**Contesto che il brief non può sapere.** `keyMsgFor` (`internal/tui/actions.go:149`)
esiste già e costruisce `KeyMsg` sintetici, ma **non va usata né allargata**: il
suo commento dichiara l'insieme chiuso di proposito, per la command palette.
`wheelKey` è una funzione separata con la propria motivazione.

- [ ] **Step 1: scrivere i test che falliscono**

Nuovo file `internal/tui/mouse_test.go`:

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWheelKey(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   tea.MouseMsg
		want tea.KeyType
		ok   bool
	}{
		{"wheel up", tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress}, tea.KeyUp, true},
		{"wheel down", tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress}, tea.KeyDown, true},
		{"wheel left", tea.MouseMsg{Button: tea.MouseButtonWheelLeft, Action: tea.MouseActionPress}, 0, false},
		{"wheel right", tea.MouseMsg{Button: tea.MouseButtonWheelRight, Action: tea.MouseActionPress}, 0, false},
		{"left click", tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}, 0, false},
		{"release", tea.MouseMsg{Button: tea.MouseButtonNone, Action: tea.MouseActionRelease}, 0, false},
		{"motion", tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion}, 0, false},
	} {
		got, ok := wheelKey(tc.in)
		if ok != tc.ok {
			t.Errorf("%s: ok = %v, want %v", tc.name, ok, tc.ok)
			continue
		}
		if ok && got.Type != tc.want {
			t.Errorf("%s: key type = %v, want %v", tc.name, got.Type, tc.want)
		}
	}
}

// The wheel must reach the active screen through the same path a key takes, so
// every screen that already handles up/down scrolls without being modified.
func TestWheelScrollsTheActiveScreen(t *testing.T) {
	m := filtersScreenFixture()
	if m.filtersScreen.row != 0 {
		t.Fatalf("fixture starts at row %d, want 0", m.filtersScreen.row)
	}

	u, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	m = u.(Model)
	if m.filtersScreen.row != 1 {
		t.Fatalf("row = %d after one wheel-down notch, want 1", m.filtersScreen.row)
	}

	u, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	m = u.(Model)
	if m.filtersScreen.row != 0 {
		t.Fatalf("row = %d after wheel-up, want back to 0", m.filtersScreen.row)
	}
}

// A click carries no meaning yet (#74 ships the wheel only), and must not be
// mistaken for a keystroke by the screen underneath.
func TestClickChangesNothing(t *testing.T) {
	m := filtersScreenFixture()
	u, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if got := u.(Model).filtersScreen.row; got != 0 {
		t.Errorf("row = %d after a click, want 0 — clicks are ignored", got)
	}
}

// With an overlay open the overlay owns the wheel, exactly as it owns the
// keyboard: the screen underneath must not scroll behind it.
func TestWheelGoesToTheOverlayWhenOneIsOpen(t *testing.T) {
	m := openPaletteOn(newTestModelOnReport())
	m.height = 24
	if len(m.palette.items) < 2 {
		t.Fatalf("the fixture has %d actions; this test needs at least 2", len(m.palette.items))
	}

	u, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	got := u.(Model)
	if got.palette.idx != 1 {
		t.Errorf("palette idx = %d after wheel-down, want 1", got.palette.idx)
	}
}
```

- [ ] **Step 2: eseguirli e vederli fallire**

Run: `go test ./internal/tui -run 'TestWheel|TestClick' -v`
Expected: FAIL in compilazione — `undefined: wheelKey`.

- [ ] **Step 3: implementare**

Nuovo file `internal/tui/mouse.go`:

```go
package tui

import tea "github.com/charmbracelet/bubbletea"

// wheelKey translates a wheel notch into the arrow key it stands for, and
// reports false for every other mouse event.
//
// Going through a synthetic KeyMsg is what makes the wheel work on every
// screen without touching any of them: the message is replayed through the
// same routing a real key takes, so each screen's existing cursor handling —
// and the tests that cover it — apply unchanged. actions.go does the same for
// the command palette.
//
// It deliberately does not go through keyMsgFor: that function's key set is
// closed on purpose for the palette's benefit (see its doc comment), and
// widening it for a different caller would betray that.
//
// There is no check on msg.Action, and it would be dead code: measured against
// bubbletea v1.3.10, a wheel notch always arrives as a single press. The SGR
// parser excludes IsWheel() from the release branch (mouse.go:186-191, with
// the comment "Wheel buttons don't have release events"), and the X10 parser
// assigns a release only in the ordinary-button branch and never applies the
// motion bit to a wheel event (mouse.go:249-257).
//
// Horizontal wheel and the side buttons are dropped: nothing in the TUI scrolls
// sideways, and a key that means nothing is worse than no key at all.
func wheelKey(msg tea.MouseMsg) (tea.KeyMsg, bool) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return tea.KeyMsg{Type: tea.KeyUp}, true
	case tea.MouseButtonWheelDown:
		return tea.KeyMsg{Type: tea.KeyDown}, true
	}
	return tea.KeyMsg{}, false
}
```

In `internal/tui/app.go`, dentro il type-switch di `Update`, **subito sotto il
blocco `case tea.KeyMsg:`** (che finisce con `return m.routeKey(msg)`):

```go
	case tea.MouseMsg:
		k, ok := wheelKey(msg)
		if !ok {
			return m, nil
		}
		// The global bindings the KeyMsg case checks first — ForceQuit, Quit,
		// Help, Palette — are all keystrokes, so a wheel notch cannot match
		// any of them and they are skipped. The overlay check is not optional:
		// with the palette open the wheel must move its selection, not scroll
		// the screen behind it.
		if m.overlay != overlayNone {
			return m.updateOverlay(k)
		}
		return m.routeKey(k)
```

- [ ] **Step 4: eseguirli e vederli passare**

Run: `go test ./internal/tui -run 'TestWheel|TestClick' -v`
Expected: PASS (4 test).

- [ ] **Step 5: prova per mutazione, obbligatoria**

Due mutazioni, ognuna eseguita e con il transcript nel report:

1. Togliere le tre righe `if m.overlay != overlayNone { ... }` dal caso nuovo.
   `TestWheelGoesToTheOverlayWhenOneIsOpen` **deve** fallire. Se passa lo stesso,
   il test non prova quello che dice e va riscritto.
2. In `wheelKey`, far ritornare `true` anche per `MouseButtonLeft`.
   `TestClickChangesNothing` **deve** fallire.

Ripristinare entrambe le mutazioni prima di proseguire, e verificare con
`git diff` che l'albero sia tornato quello di prima.

- [ ] **Step 6: gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui/mouse.go internal/tui/mouse_test.go internal/tui/app.go
git commit -m "feat(tui): scroll lists with the mouse wheel (#74)"
```

---

### Task 3: accendere il mouse nel programma

**Files:**
- Modify: `internal/cli/cli.go` (`runTUI`)
- Test: `internal/cli/cli_test.go`

**Interfaces:**
- Consuma: `config.Config.MouseEnabled()` dal Task 1.
- Produce: `func programOptions(cfg config.Config) []tea.ProgramOption`.

- [ ] **Step 1: scrivere il test che fallisce**

In fondo a `internal/cli/cli_test.go` (aggiungere gli import
`"github.com/marcoarnulfo/clickup-cli/internal/config"`):

```go
// tea.ProgramOption is an opaque function: this test can prove that an extra
// option is appended when the mouse is on, not WHICH option it is. That is the
// honest limit of what is checkable here — the real coverage of the decision
// lives on config.MouseEnabled, which is pure. This test exists to catch the
// forgotten append, and nothing more.
func TestProgramOptionsAddsOneWhenMouseIsEnabled(t *testing.T) {
	t.Parallel()
	no := false
	off := len(programOptions(config.Config{Mouse: &no}))
	on := len(programOptions(config.Config{}))
	if on != off+1 {
		t.Errorf("programOptions: %d options with the mouse on, %d with it off; want exactly one more", on, off)
	}
}
```

- [ ] **Step 2: eseguirlo e vederlo fallire**

Run: `go test ./internal/cli -run TestProgramOptions -v`
Expected: FAIL in compilazione — `undefined: programOptions`.

- [ ] **Step 3: implementare**

In `internal/cli/cli.go`:

```go
// programOptions builds the bubbletea options for a config. Extracted from
// runTUI so the mouse decision is reachable from a test: runTUI itself blocks
// on a terminal.
//
// WithMouseCellMotion (DEC mode 1002) and not WithMouseAllMotion (1003): the
// latter reports pointer motion even with no button held, a stream of messages
// this TUI would discard. Neither is "wheel only" — no such DEC mode exists —
// which is why enabling the wheel costs the terminal's native text selection,
// and why config.Mouse exists to turn it back off.
func programOptions(cfg config.Config) []tea.ProgramOption {
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if cfg.MouseEnabled() {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	return opts
}
```

e in `runTUI` sostituire la riga 40 con:

```go
	p := tea.NewProgram(tui.New(cfg), programOptions(cfg)...)
```

- [ ] **Step 4: eseguirlo e vederlo passare**

Run: `go test ./internal/cli -run TestProgramOptions -v`
Expected: PASS.

- [ ] **Step 5: prova per mutazione**

Togliere l'`if cfg.MouseEnabled()` (appendere sempre). Il test **deve**
fallire. Ripristinare e verificare con `git diff`.

- [ ] **Step 6: gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/cli/cli.go internal/cli/cli_test.go
git commit -m "feat(cli): enable mouse reporting unless the config turns it off (#74)"
```

---

### Task 4: la zebratura sopravvive a 16 colori

**Files:**
- Modify: `internal/tui/theme.go` (campo `Subtle` della `palette`, e
  `defaultPalette`)
- Modify: `internal/tui/theme_test.go` (`TestPaletteIsAdaptive`, che **non
  compila più**, + il test nuovo)

**Interfaces:**
- Consuma: niente.
- Produce: `palette.Subtle` di tipo `lipgloss.CompleteAdaptiveColor`.

**Due cose che il brief non può sapere, e che vanno gestite in questo task:**

1. `TestPaletteIsAdaptive` (`theme_test.go:93`) itera uno slice di
   `lipgloss.AdaptiveColor` e ci mette dentro `Subtle`. Cambiare il tipo del
   campo **rompe la compilazione del package di test**. `Subtle` va tolto da
   quel loop e messo in una propria asserzione, che deve continuare a
   verificare la stessa cosa: chiaro e scuro devono differire.
2. `TestGoldenPaletteDark` / `TestGoldenPaletteLight` rendono con
   `termenv.TrueColor` (`theme_test.go:58-63`). Il cambiamento è misurato come
   byte-identico a TrueColor, quindi **quei due golden non devono muoversi**. Se
   si muovono, gli slot sono sbagliati: non rigenerarli, correggere il codice.

- [ ] **Step 1: scrivere il test che fallisce**

In `internal/tui/theme_test.go`, accanto a `TestZebraCarriesBackground`:

```go
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
```

- [ ] **Step 2: eseguirlo e vederlo fallire**

Run: `go test ./internal/tui -run TestZebraSurvivesSixteenColors -v`
Expected: FAIL su `dark 16-color` (`Zebra = "\x1b[40mx\x1b[0m"`, voleva
`"\x1b[100m"`) e su `light 16-color` (`"\x1b[107m…"`, voleva `"\x1b[47m"`). I
quattro casi con `want == ""` passano già: è giusto così, sono la guardia.

**Allegare questo transcript al report:** è la prova che il test vede il
difetto.

- [ ] **Step 3: implementare**

In `internal/tui/theme.go`, il campo:

```go
	// Subtle is the only token whose automatic downconvert lands on the wrong
	// color, so it names a value per profile instead of letting termenv pick
	// the nearest one. The other five stay AdaptiveColor: measured, their
	// nearest-color conversion already arrives where it should, and making
	// them explicit would be five times the surface for no visible change.
	Subtle lipgloss.CompleteAdaptiveColor // zebra row background
```

e in `defaultPalette`, al posto della riga `Subtle:` attuale:

```go
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
```

In `theme_test.go`, togliere `{"Subtle", p.Subtle}` dal loop di
`TestPaletteIsAdaptive` e aggiungere in fondo alla stessa funzione:

```go
	// Subtle is checked separately: it is a CompleteAdaptiveColor, so it does
	// not fit the AdaptiveColor loop above. The requirement is the same one —
	// light and dark must differ — and it is checked on the two profiles that
	// actually render a stripe.
	if s := defaultPalette().Subtle; s.Light.ANSI256 == s.Dark.ANSI256 || s.Light.ANSI == s.Dark.ANSI {
		t.Errorf("Subtle is not adaptive: light %+v, dark %+v", s.Light, s.Dark)
	}
```

- [ ] **Step 4: eseguire tutto il package**

Run: `go test ./internal/tui -v -run 'TestZebra|TestPalette|TestGoldenPalette'`
Expected: PASS ovunque.

Poi la suite intera: `go test ./... -race`.
Expected: PASS, e **nessun golden modificato**. Verificare con
`git status --short internal/tui/testdata/`: deve essere vuoto. Se un golden è
cambiato, è un difetto dell'implementazione, non del golden — non rigenerarlo.

- [ ] **Step 5: prova per mutazione, obbligatoria**

Rimettere `Subtle: lipgloss.AdaptiveColor{Light: "254", Dark: "236"}` (con il
tipo del campo riportato ad `AdaptiveColor`).
`TestZebraSurvivesSixteenColors` **deve** fallire sui due casi a 16 colori.
Ripristinare e verificare con `git diff`.

- [ ] **Step 6: gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui/theme.go internal/tui/theme_test.go
git commit -m "fix(tui): keep the zebra stripe visible on a 16-color terminal (#74)"
```

---

### Task 5: documentazione

**Files:**
- Modify: `README.md` (elenco delle chiavi di config, dopo `update_check` alla
  riga ~438)
- Modify: `README.it.md` (stesso punto, riga ~467)
- Modify: `CHANGELOG.md` (sezione `## [Unreleased]`)

**Interfaces:** nessuna.

- [ ] **Step 1: `README.md`**

Subito dopo il punto elenco `- \`update_check\` (optional): …`:

```markdown
- `mouse` (optional): set to `false` to turn off mouse reporting. Omitting the key
  (or setting `true`) leaves it enabled, which makes the wheel scroll lists. Mouse
  reporting takes the terminal's native text selection away: with it on, hold
  **Shift** while dragging to select and copy, or set this key to `false` to give
  the mouse back to the terminal for good.
```

- [ ] **Step 2: `README.it.md`**

Subito dopo `- \`update_check\` (opzionale): …`:

```markdown
- `mouse` (opzionale): impostare `false` per disattivare il supporto mouse.
  Omettere la chiave (o impostare `true`) lo lascia attivo, e la rotella scorre le
  liste. Il mouse tracking toglie al terminale la selezione nativa del testo: con
  il mouse attivo, tenere **Shift** mentre si trascina per selezionare e copiare,
  oppure impostare questa chiave a `false` per restituire il mouse al terminale
  una volta per tutte.
```

- [ ] **Step 3: `CHANGELOG.md`**

In `### Added` sotto `## [Unreleased]`:

```markdown
- The mouse wheel scrolls every list screen, and the command palette when it is
  open. Mouse reporting can be turned off with `mouse: false` in the config,
  which gives the terminal its native text selection back (#74).
```

In `### Fixed`:

```markdown
- The report's zebra stripe no longer disappears on a 16-color terminal, where
  the automatic downconvert of its 256-color grey landed on the background color
  itself (#74).
```

- [ ] **Step 4: rileggere quello che si è scritto**

Aprire i tre file e **guardare** le sezioni modificate in contesto: che il
punto elenco stia nella lista giusta, che l'indentazione combaci con i vicini,
che le due voci di changelog siano nella sezione giusta.

- [ ] **Step 5: gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add README.md README.it.md CHANGELOG.md
git commit -m "docs: document the mouse key and the 16-color zebra fix (#74)"
```

---

## Note per il controllore

- La #74 si chiude quando entrambe le caselle rimaste — «Mouse support» e
  «truecolor→256→16 downconvert» — sono spuntate. Le altre due erano già chiuse
  dalla tranche A.
- La casella «downconvert» si chiude con il Task 4 **e con la misura**: la
  conversione automatica funzionava già per cinque tinte su sei, e la spec lo
  documenta. Nel chiudere la issue vale la pena scriverlo, così nessuno cerca
  una pipeline di conversione che non esiste.
- Parità demo-mode: gratis. La rotella passa per lo stesso `Update` del modello
  vero, e `CLICKUP_DEMO=1` non cambia né il tema né l'instradamento.
