# Keybinding configurabili — piano di implementazione

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development
> (recommended) or superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** chiudere la **#82** — la sua terza casella, gli override delle
keybinding dal config. Con questa la milestone v1.9 si chiude.

**Architecture:** i nomi delle chiavi YAML si derivano per reflection dai campi
di `keyDefaults`, quindi ogni binding tranne `force_quit` è rimappabile e nessuno
futuro richiede lavoro. `internal/config` tiene il grezzo, `internal/tui` risolve
e valida accanto alla tabella che valida, `internal/cli` fa fallire l'avvio su
una configurazione che non può onorare.

**Tech Stack:** Go 1.26, bubbles/key v1.0.0, bubbletea v1.3.10, yaml.v3.

**Spec:** `docs/superpowers/specs/2026-08-01-configurable-keys-design.md`

## Global Constraints

- Tutto ciò che vive nel repo è in **inglese**: codice, identificatori,
  commenti, stringhe UI, nomi e messaggi dei test, messaggi di commit. Eccezioni:
  `README.it.md`, `CONTRIBUTING.it.md` e i doc di design sotto
  `docs/superpowers/`.
- **MAI** un trailer `Co-Authored-By`. Conventional Commits.
- `internal/report` e `internal/duration` restano **puri**: solo stdlib, nessun
  I/O, nessun import di `internal/config`, `internal/clickup`, `internal/tui`,
  `internal/themes`.
- `internal/themes` resta **foglia**: solo lipgloss, termenv, yaml.v3 e stdlib.
- **Mai chiamare l'API ClickUp vera.** Non esistono credenziali in questo
  ambiente. Il comportamento di rete si esercita solo con `httptest`.
- Nessuna funzione di stile in produzione chiama `lipgloss.NewStyle()`.
- **Ogni numero scritto in un commento va misurato eseguendo il codice**, mai
  calcolato a mente.
- **Un test scritto contro un difetto non vale finché non lo si è visto fallire
  contro quel difetto**, con il transcript allegato al report.
- I golden si rigenerano solo con `go test ./internal/tui -update`, mai a mano.
  **In questa tranche nessun golden deve muoversi**: nessun default cambia.
- Gate prima di ogni commit, tutti e cinque, tutti puliti:
  `gofmt -l .` · `go vet ./...` ·
  `go run honnef.co/go/tools/cmd/staticcheck@latest ./...` · `go build ./...` ·
  `go test ./... -race`

---

### Task 1: la chiave `keys` nel config

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consuma: niente.
- Produce: `config.KeySpec` e `Config.Keys map[string]KeySpec`. Il Task 3 li
  legge.

- [ ] **Step 1: scrivere i test che falliscono**

In fondo a `internal/config/config_test.go`:

```go
// --- optional keys map (#82) ---

func TestKeysRoundTrip(t *testing.T) {
	isolateConfig(t)
	in := Config{Token: "t", WorkspaceID: "1", Keys: map[string]KeySpec{
		"log_hours": {"L"},
		"up":        {"up", "ctrl+u"},
	}}
	if err := Save(in); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Keys["log_hours"]) != 1 || got.Keys["log_hours"][0] != "L" {
		t.Errorf("log_hours = %v, want [L]", got.Keys["log_hours"])
	}
	if len(got.Keys["up"]) != 2 || got.Keys["up"][0] != "up" || got.Keys["up"][1] != "ctrl+u" {
		t.Errorf("up = %v, want [up ctrl+u]", got.Keys["up"])
	}
}

func TestKeySpecAcceptsAScalarAndAList(t *testing.T) {
	t.Parallel()
	var got map[string]KeySpec
	src := "log_hours: \"L\"\nup: [up, ctrl+u]\n"
	if err := yaml.Unmarshal([]byte(src), &got); err != nil {
		t.Fatal(err)
	}
	if len(got["log_hours"]) != 1 || got["log_hours"][0] != "L" {
		t.Errorf("scalar form gave %v, want [L]", got["log_hours"])
	}
	if len(got["up"]) != 2 || got["up"][0] != "up" || got["up"][1] != "ctrl+u" {
		t.Errorf("list form gave %v, want [up ctrl+u]", got["up"])
	}
}

// Save serializes the whole Config, so without MarshalYAML a single key written
// as a scalar would come back as a one-element list the first time anything
// saves the config. This test is the only thing that would notice.
func TestSaveKeepsASingleKeyScalar(t *testing.T) {
	isolateConfig(t)
	if err := Save(Config{Token: "t", WorkspaceID: "1", Keys: map[string]KeySpec{
		"log_hours": {"L"},
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
	if !strings.Contains(string(raw), `log_hours: L`) {
		t.Fatalf("saved config did not keep the scalar form:\n%s", raw)
	}
}

func TestKeysAbsentIsNotWritten(t *testing.T) {
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
	if strings.Contains(string(raw), "keys:") {
		t.Fatalf("saved config mentions keys:\n%s", raw)
	}
}
```

`config_test.go` importa già `os` e `strings`; aggiungere `gopkg.in/yaml.v3`.

- [ ] **Step 2: eseguirli e vederli fallire**

Run: `go test ./internal/config -run 'TestKey|TestSaveKeepsASingle' -v`
Expected: FAIL in compilazione — `undefined: KeySpec`.

- [ ] **Step 3: implementare**

In `internal/config/config.go`, accanto agli altri tipi:

```go
// KeySpec is one binding's keys as written in YAML: a bare key, or a list of
// them. It lives here rather than in internal/tui because nothing outside this
// package needs the type — internal/tui takes plain strings.
type KeySpec []string

// UnmarshalYAML accepts both forms, so the common case is `log_hours: L`
// rather than `log_hours: [L]`.
func (k *KeySpec) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		var s string
		if err := n.Decode(&s); err != nil {
			return err
		}
		*k = KeySpec{s}
		return nil
	}
	var list []string
	if err := n.Decode(&list); err != nil {
		return err
	}
	*k = list
	return nil
}

// MarshalYAML collapses a single key back to a scalar. Without it, Save — which
// marshals the whole Config — would rewrite a user's `log_hours: L` as a
// one-element list the first time anything saved the config.
func (k KeySpec) MarshalYAML() (any, error) {
	if len(k) == 1 {
		return k[0], nil
	}
	return []string(k), nil
}
```

e dentro `Config`, sotto `Themes`:

```go
	// Keys remaps the TUI's bindings by name. The names are the ones
	// internal/tui derives from its binding table; this package holds the YAML
	// as written and validates nothing.
	Keys map[string]KeySpec `yaml:"keys,omitempty"`
```

- [ ] **Step 4: eseguirli e vederli passare**

Run: `go test ./internal/config -run 'TestKey|TestSaveKeepsASingle' -v`
Expected: PASS (4 test).

- [ ] **Step 5: prova per mutazione, obbligatoria**

Cancellare `MarshalYAML`. `TestSaveKeepsASingleKeyScalar` **deve** fallire, e il
suo output deve mostrare la forma a lista. Ripristinare e verificare con
`git diff`.

- [ ] **Step 6: gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/config/
git commit -m "feat(config): add the keys map for binding overrides (#82)"
```

---

### Task 2: i nomi dei binding, derivati e inchiodati

**Files:**
- Create: `internal/tui/keyname.go`
- Create: `internal/tui/keyname_test.go`

**Interfaces:**
- Consuma: `keyDefaults` (già esistente, `keys.go:14`).
- Produce: `bindingName(field string) string` e `BindingNames() []string`. I
  Task 3 e 4 li usano.

- [ ] **Step 1: scrivere i test che falliscono**

`internal/tui/keyname_test.go`:

```go
package tui

import (
	"slices"
	"testing"
)

func TestBindingName(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ in, want string }{
		{"Quit", "quit"},
		{"LogHours", "log_hours"},
		{"PaletteUp", "palette_up"},
		{"ForceQuit", "force_quit"},
		// An acronym run stays together. The naive rule — underscore before
		// every capital — turns this into "pick_by_i_d", which would have
		// shipped as a config key.
		{"PickByID", "pick_by_id"},
		{"ID", "id"},
		{"HTTPServer", "http_server"},
	} {
		if got := bindingName(tc.in); got != tc.want {
			t.Errorf("bindingName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// This is the test that matters most in this task. The config keys a user
// writes are derived from Go field names, so renaming a field silently renames
// a key in every config file in the wild. Spelling the list out here means the
// rename fails a test instead of failing a user.
func TestBindingNamesArePinned(t *testing.T) {
	t.Parallel()
	want := []string{
		"back", "browse_list", "budget", "change_range", "clear_value",
		"confirm", "confirm_delete", "delete", "down", "edit",
		"export", "filters", "force_quit", "generate", "group_by",
		"help", "history", "list_budget", "list_currency", "log_hours",
		"members", "new_override", "new_tag", "next_field", "next_month",
		"next_section", "no", "open_entries", "palette", "palette_down",
		"palette_up", "pick_by_id", "pick_guided", "pick_timer", "prev_field",
		"prev_month", "prev_section", "quit", "range", "rates",
		"reload", "save", "select_all", "stop_timer", "tags",
		"timer", "toggle_item", "toggle_scope", "toggle_week", "up",
		"yes",
	}
	got := BindingNames()
	if !slices.Equal(got, want) {
		t.Errorf("BindingNames() = %v\nwant %v", got, want)
	}
}

func TestBindingNamesAreUnique(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, n := range BindingNames() {
		if seen[n] {
			t.Errorf("duplicate derived name %q — two fields collapse to the same config key", n)
		}
		seen[n] = true
	}
}
```

- [ ] **Step 2: eseguirli e vederli fallire**

Run: `go test ./internal/tui -run TestBindingName -v`
Expected: FAIL in compilazione — `undefined: bindingName`, `undefined: BindingNames`.

- [ ] **Step 3: implementare**

`internal/tui/keyname.go`:

```go
package tui

import (
	"reflect"
	"slices"
	"strings"
	"unicode"
)

// bindingName turns a keyDefaults field name into the key a user writes in
// their config: LogHours -> log_hours.
//
// An acronym run stays together — PickByID becomes pick_by_id, not
// pick_by_i_d — because the underscore goes in only where a case boundary is a
// word boundary: after a lowercase letter, or before the last capital of a run.
// Measured against every field in keyDefaults; the naive rule produced
// "pick_by_i_d", which would have shipped as a config key.
func bindingName(field string) string {
	r := []rune(field)
	var b strings.Builder
	for i, c := range r {
		if !unicode.IsUpper(c) {
			b.WriteRune(c)
			continue
		}
		prevLower := i > 0 && unicode.IsLower(r[i-1])
		nextLower := i+1 < len(r) && unicode.IsLower(r[i+1])
		if i > 0 && (prevLower || nextLower) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(c))
	}
	return b.String()
}

// BindingNames lists every binding name, sorted, for validation errors and for
// the docs. It includes force_quit so the pinned list covers every field, even
// though ResolveKeys rejects that one explicitly.
//
// Derived rather than written out so that a binding added to keyDefaults is
// remappable with no further work unless it is explicitly fixed like force_quit
// — the maintenance multiplier #82 warned about does not exist. The cost is
// moved instead: renaming a Go field would rename a user's config key, which is
// what TestBindingNamesArePinned exists to catch.
func BindingNames() []string {
	ty := reflect.TypeOf(keyDefaults{})
	out := make([]string, 0, ty.NumField())
	for i := range ty.NumField() {
		out = append(out, bindingName(ty.Field(i).Name))
	}
	slices.Sort(out)
	return out
}
```

- [ ] **Step 4: eseguirli e vederli passare**

Run: `go test ./internal/tui -run TestBindingName -v`
Expected: PASS (3 test).

- [ ] **Step 5: prova per mutazione, obbligatoria**

Due mutazioni, con transcript:

1. In `bindingName`, togliere la condizione `nextLower` (cioè tornare alla
   regola ingenua). `TestBindingName` **deve** fallire su `PickByID` e
   `HTTPServer`, e `TestBindingNamesArePinned` su `pick_by_id`.
2. Rinominare il campo `Timer` di `keyDefaults` in `TimerToggle`.
   `TestBindingNamesArePinned` **deve** fallire. È la prova che il test protegge
   davvero i config degli utenti da un rename.

Ripristinare dopo ognuna e verificare con `git diff`.

- [ ] **Step 6: gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui/keyname.go internal/tui/keyname_test.go
git commit -m "feat(tui): derive the config name of every binding (#82)"
```

---

### Task 3: `KeyTable` e `ResolveKeys`

**Files:**
- Create: `internal/tui/keytable.go`
- Create: `internal/tui/keytable_test.go`
- Modify: `internal/tui/actions.go` (`canonicalKeyTypes`, `keyNameAliases`,
  `parseKeyName`)
- Test: `internal/tui/actions_test.go`

**Interfaces:**
- Consuma: `bindingName`, `BindingNames` dal Task 2; `config.KeySpec` dal Task 1.
- Produce: `KeyTable`, `(KeyTable).bindings()`, `DefaultKeyTable()`,
  `ResolveKeys(map[string]config.KeySpec) (KeyTable, error)`,
  `parseKeyName(string) (tea.KeyMsg, error)` e `keyLabel([]string) string`. Il
  Task 4 aggiunge il controllo sulle collisioni dentro `ResolveKeys`; il Task 5
  riusa lo stesso parser per la command palette.

**Contesto che il brief non può sapere.** La `KeyTable` zero **deve** valere
come i default: i test costruiscono 108 letterali `Model{…}` senza passare da
`New`, misurati, e leggere una tabella zero li disabiliterebbe tutti.

- [ ] **Step 1: scrivere i test che falliscono**

`internal/tui/keytable_test.go`:

```go
package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/marcoarnulfo/clickup-cli/internal/config"
)

// The zero KeyTable is what a Model built by hand in a test carries, and 108
// such Models exist. It must behave as the built-in defaults.
func TestZeroKeyTableIsTheDefaults(t *testing.T) {
	t.Parallel()
	var zero KeyTable
	if got, want := zero.bindings().Quit.Keys(), defaultKeys().Quit.Keys(); len(got) != len(want) || got[0] != want[0] {
		t.Errorf("zero table Quit = %v, want the default %v", got, want)
	}
}

func TestResolveKeysOverridesAndKeepsTheRest(t *testing.T) {
	t.Parallel()
	kt, err := ResolveKeys(map[string]config.KeySpec{"log_hours": {"L"}})
	if err != nil {
		t.Fatal(err)
	}
	d := kt.bindings()
	if got := d.LogHours.Keys(); len(got) != 1 || got[0] != "L" {
		t.Errorf("LogHours = %v, want [L]", got)
	}
	if got, want := d.Quit.Keys(), defaultKeys().Quit.Keys(); got[0] != want[0] {
		t.Errorf("Quit = %v, want the untouched default %v", got, want)
	}
}

// The help string carries the key inside it, so an overridden binding that kept
// its old help would lie in the footer and in the ? overlay.
// The keys are chosen so the override stays collision-free once Task 4's rule
// lands: "k" is already Up's own, and "ctrl+u" is claimed by nothing. An
// earlier draft used "w", which toggle_week claims — Task 4 would then have
// rejected it and this test could never have passed.
func TestResolveKeysRegeneratesTheHelp(t *testing.T) {
	t.Parallel()
	kt, err := ResolveKeys(map[string]config.KeySpec{"up": {"k", "ctrl+u"}})
	if err != nil {
		t.Fatal(err)
	}
	h := kt.bindings().Up.Help()
	if h.Key != "k/ctrl+u" {
		t.Errorf("help key = %q, want %q", h.Key, "k/ctrl+u")
	}
	if want := defaultKeys().Up.Help().Desc; h.Desc != want {
		t.Errorf("help desc = %q, want the original %q", h.Desc, want)
	}
}

func TestResolveKeysErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   map[string]config.KeySpec
		says []string
	}{
		{
			name: "unknown binding lists the valid names",
			in:   map[string]config.KeySpec{"log_hourz": {"L"}},
			says: []string{"log_hourz", "log_hours"},
		},
		{
			name: "force_quit is not remappable",
			in:   map[string]config.KeySpec{"force_quit": {"ctrl+x"}},
			says: []string{"force_quit", "ctrl+c"},
		},
		{
			name: "an empty list is rejected",
			in:   map[string]config.KeySpec{"quit": {}},
			says: []string{"quit", "at least one key"},
		},
		{
			name: "an empty key inside the list is rejected",
			in:   map[string]config.KeySpec{"quit": {"Q", ""}},
			says: []string{"quit", "empty"},
		},
		{
			// Without its own check this reaches the collision rule, which
			// reports "already claimed by " with an empty list — the only
			// other claimant being the binding itself.
			name: "the same key twice in one list is rejected",
			in:   map[string]config.KeySpec{"quit": {"Q", "Q"}},
			says: []string{"quit", "\"Q\"", "twice"},
		},
	} {
		_, err := ResolveKeys(tc.in)
		if err == nil {
			t.Errorf("%s: ResolveKeys = nil error, want one", tc.name)
			continue
		}
		for _, s := range tc.says {
			if !strings.Contains(err.Error(), s) {
				t.Errorf("%s: error %q does not mention %q", tc.name, err, s)
			}
		}
	}
}

// ctrl+c must survive a config that remaps everything else: it is the only way
// out of a TUI whose Quit the user has moved somewhere they cannot reach.
//
// Each binding gets a distinct private-use Unicode rune so the sweep is both
// canonically producible as one KeyMsg and collision-free. A sweep built from,
// say, "ctrl+"+name[:1] would collide (back, budget and browse_list would all
// want ctrl+b) and this test would quietly turn into one that skips forever.
func TestForceQuitSurvivesEveryOverride(t *testing.T) {
	t.Parallel()
	over := map[string]config.KeySpec{}
	for i, n := range BindingNames() {
		if n == forceQuitName {
			continue
		}
		over[n] = config.KeySpec{string(rune(0xE000 + i))}
	}
	kt, err := ResolveKeys(over)
	if err != nil {
		t.Fatalf("a collision-free sweep was rejected: %v", err)
	}
	if got, want := kt.bindings().ForceQuit.Keys(), defaultKeys().ForceQuit.Keys(); got[0] != want[0] {
		t.Errorf("ForceQuit = %v, want %v", got, want)
	}
	// And the sweep really did land, so the assertion above is not vacuous.
	if got := kt.bindings().Quit.Keys(); len(got) != 1 || got[0] == "q" {
		t.Errorf("Quit = %v, want the swept key — the sweep did not take effect", got)
	}
}
```

In `internal/tui/actions_test.go`, aggiungere tre test table-driven:

- `TestParseKeyNameAcceptsCanonicalNames`: rune singole ASCII/Unicode, spazio
  letterale, `alt+ `, control key canoniche e relative forme `alt+`; ogni
  `tea.KeyMsg` deve fare round-trip esatto con `String()`;
- `TestParseKeyNameAcceptsEveryCanonicalSpecialKey`: tutte le special key
  esportate da Bubble Tea che rappresentano un singolo input terminale — frecce,
  home/end, page, insert/delete, modificatori e `f1`..`f20` — sia normali sia
  `alt+`;
- `TestParseKeyNameRejectsAliasesAndUnreachableNames`: stringa vuota, sequenze
  multi-runa (`g g`, `gg`), nomi inesistenti (`f21`, `runes`, `alt+`) e alias
  non canonici. Gli alias devono suggerire la forma di `KeyMsg.String()`:
  `space` → spazio letterale, control-backtick → `ctrl+@`, `ctrl+i` → `tab`,
  `ctrl+m` → `enter`, `ctrl+[` → `esc`, `ctrl+?` → `backspace`, inclusa la
  forma `alt+`.

In `internal/tui/keytable_test.go`, aggiungere
`TestResolveKeysRejectsUnreachableKeyNamesAtStartup`: `ResolveKeys` deve
rifiutare sequenze, nomi fuori insieme e alias, nominando binding, valore e,
quando esiste, spelling canonico. Iniziare inoltre
`TestLiteralSpaceOverrideRoutesAndRendersAsSpace` con le asserzioni disponibili
in questo task: il tasto grezzo resta `" "`, mentre help e label mostrano
`space`; `alt+ ` mostra `alt+space`. Il Task 5 completa lo stesso test con routing
e palette.

- [ ] **Step 2: eseguirli e vederli fallire**

Run: `go test ./internal/tui -run 'TestZeroKeyTable|TestResolveKeys|TestForceQuit|TestParseKeyName|TestLiteralSpace|TestAltSpace' -v`
Expected: FAIL in compilazione — `undefined: KeyTable`, `undefined: ResolveKeys`,
`undefined: parseKeyName` e `undefined: keyLabel`.

- [ ] **Step 3: implementare**

`internal/tui/keytable.go`:

```go
package tui

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
)

// forceQuitName is the one binding a config may not remap: ctrl+c is the way
// out of a TUI whose other keys the user has moved somewhere unreachable.
const forceQuitName = "force_quit"

// KeyTable is the resolved binding table the TUI routes and renders with.
//
// The zero value means the built-in defaults, which is what a Model built by
// hand carries — the tests construct 108 of those without going through New,
// and a zero table read literally would leave every binding disabled.
type KeyTable struct {
	d   keyDefaults
	set bool
}

// DefaultKeyTable is the built-in table, for callers that want it explicitly.
func DefaultKeyTable() KeyTable { return KeyTable{d: defaultKeys(), set: true} }

func (kt KeyTable) bindings() keyDefaults {
	if !kt.set {
		return defaultKeys()
	}
	return kt.d
}

// keyLabel renders the canonical literal-space names visibly without changing
// the raw strings used by key.Matches and collision claims.
func keyLabel(keys []string) string {
	labels := slices.Clone(keys)
	for i, k := range labels {
		switch k {
		case " ":
			labels[i] = "space"
		case "alt+ ":
			labels[i] = "alt+space"
		}
	}
	return strings.Join(labels, "/")
}

// ResolveKeys applies a config's overrides to the built-in table.
//
// Every failure is an error rather than a fallback: a key that cannot be
// honored would leave the user in front of a TUI where a command simply does
// not answer, with nothing to go on. Same rule as billing.rounding.increment
// and as the theme resolution.
func ResolveKeys(overrides map[string]config.KeySpec) (KeyTable, error) {
	d := defaultKeys()
	v := reflect.ValueOf(&d).Elem()
	ty := v.Type()

	index := map[string]int{}
	for i := range ty.NumField() {
		index[bindingName(ty.Field(i).Name)] = i
	}

	for _, name := range slices.Sorted(maps.Keys(overrides)) {
		if name == forceQuitName {
			return KeyTable{}, fmt.Errorf(
				"binding %q cannot be remapped: ctrl+c is the way out of a TUI whose other keys have moved", name)
		}
		i, ok := index[name]
		if !ok {
			// force_quit is filtered out of the suggestion list: offering the
			// one name that is then rejected would be a message that argues
			// with itself.
			valid := slices.DeleteFunc(BindingNames(), func(s string) bool { return s == forceQuitName })
			return KeyTable{}, fmt.Errorf("unknown binding %q; valid names: %s",
				name, strings.Join(valid, ", "))
		}
		ks := overrides[name]
		if len(ks) == 0 {
			return KeyTable{}, fmt.Errorf("binding %q needs at least one key", name)
		}
		seen := map[string]bool{}
		for _, k := range ks {
			if k == "" {
				return KeyTable{}, fmt.Errorf("binding %q has an empty key in its list", name)
			}
			if _, err := parseKeyName(k); err != nil {
				return KeyTable{}, fmt.Errorf("binding %q uses invalid key %q: %w", name, k, err)
			}
			// Caught here rather than by the collision rule, which would report
			// the binding colliding with itself and name no other claimant.
			if seen[k] {
				return KeyTable{}, fmt.Errorf("binding %q lists key %q twice", name, k)
			}
			seen[k] = true
		}
		old := v.Field(i).Interface().(key.Binding)
		// The help string carries the key inside it, so it has to be rebuilt or
		// the footer would advertise a key that no longer does anything. The
		// typographic arrows the defaults use (↑/k, tab/▸) are lost for
		// remapped bindings: the user chose these keys, and inventing a
		// prettier rendering for them would be guessing.
		v.Field(i).Set(reflect.ValueOf(key.NewBinding(
			key.WithKeys(ks...),
			key.WithHelp(keyLabel(ks), old.Help().Desc),
		)))
	}
	return KeyTable{d: d, set: true}, nil
}
```

In `internal/tui/actions.go`, implementare un solo parser canonico:

- `canonicalKeyTypes` enumera le `tea.KeyType` che Bubble Tea può produrre come
  singolo input, senza affidarsi a range numerici non contigui;
- `parseKeyName` separa l'eventuale prefisso `alt+`, rifiuta gli alias sopra con
  il suggerimento canonico, prova le special key e poi una sola runa stampabile.
  Per `tea.KeySpace` imposta anche `Runes: []rune{' '}`, come fa il parser del
  terminale;
- accetta un nome solo se il `tea.KeyMsg` ricostruito restituisce **esattamente**
  quel nome da `String()`. Non normalizza e non accetta sequenze che nessun
  singolo `KeyMsg` può produrre.

`ResolveKeys` chiama il parser prima di costruire il binding, così una chiave
irraggiungibile ferma l'avvio invece di diventare un comando morto. Aggiungere
`"maps"` al blocco import di `keytable.go`.

- [ ] **Step 4: eseguirli e vederli passare**

Run: `go test ./internal/tui -run 'TestZeroKeyTable|TestResolveKeys|TestForceQuit|TestParseKeyName|TestLiteralSpace|TestAltSpace' -v`
Expected: PASS per tutti i test e sottotest selezionati.

- [ ] **Step 5: prova per mutazione, obbligatoria**

Sei mutazioni, con transcript:

1. Togliere la rigenerazione dell'aiuto (passare `old.Help().Key` invece di
   `keyLabel(ks)`). `TestResolveKeysRegeneratesTheHelp` **deve**
   fallire.
2. Togliere il controllo su `forceQuitName`. Il caso «force_quit is not
   remappable» **deve** fallire.
3. Far ritornare `KeyTable{d: d}` senza `set: true`.
   `TestResolveKeysOverridesAndKeepsTheRest` **deve** fallire, perché la tabella
   ricadrebbe sui default e `LogHours` tornerebbe `n`.
4. Permettere più di una runa nel fallback di `parseKeyName`.
   `TestParseKeyNameRejectsAliasesAndUnreachableNames` **deve** fallire su `gg`
   e `g g`.
5. Togliere `keyNameAliases` e lasciare che gli alias passino o cadano nell'errore
   generico. I sottotest su `space`, `ctrl+i`, `ctrl+m`, `ctrl+[` e `ctrl+?`
   **devono** fallire perché manca il rifiuto con suggerimento canonico.
6. In `keyLabel`, ritornare `strings.Join(keys, "/")` senza rendere gli spazi.
   I test su spazio e alt-spazio **devono** fallire mostrando label invisibili.

Ripristinare dopo ognuna e verificare con `git diff`.

- [ ] **Step 6: gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui/keytable.go internal/tui/keytable_test.go internal/tui/actions.go internal/tui/actions_test.go
git commit -m "feat(tui): resolve binding overrides from the config (#82)"
```

---

### Task 4: la regola sulle collisioni

**Files:**
- Modify: `internal/tui/keytable.go` (`ResolveKeys`, più `claims` e `checkCollisions`)
- Modify: `internal/tui/keytable_test.go`

**Interfaces:**
- Consuma: `bindingName`, `keyDefaults`.
- Produce: nessuna API nuova esportata; `ResolveKeys` guadagna un controllo.

**Contesto che il brief non può sapere, e che è il cuore del task.** I default
contengono **20 tasti rivendicati da più di un binding**, misurati — `n` da
quattro binding, `enter` da tre, `h` da tre. Convivono perché `screenKeys` ne
abilita solo un sottoinsieme per schermata. Una regola «due binding non possono
condividere un tasto» rifiuterebbe quindi i default stessi: **non implementarla**.

La regola è: per ogni tasto rivendicato da **due o più binding dopo gli
override**, l'insieme dei binding che lo rivendicano dopo deve essere un
sottoinsieme di quello che lo rivendicava prima. Un tasto con un solo
rivendicante dopo gli override non è soggetto al controllo.

- [ ] **Step 1: scrivere i test che falliscono**

In `internal/tui/keytable_test.go`:

```go
// The defaults are heavily overloaded on purpose — measured, 20 physical keys
// are claimed by more than one binding, because screenKeys enables only a
// subset per screen. A rule that forbade any sharing would reject what we
// ship, so the rule is narrower: for a key with two or more post-override
// claimants, those claimants must be a subset of the defaults' claimants.
func TestCollisionRule(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   map[string]config.KeySpec
		ok   bool
		says []string
	}{
		{
			// n is shared by log_hours, new_override, new_tag and no. Moving
			// log_hours away leaves a subset behind, which is fine.
			name: "moving a binding off a shared key is allowed",
			in:   map[string]config.KeySpec{"log_hours": {"L"}},
			ok:   true,
		},
		{
			name: "taking a free key is allowed",
			in:   map[string]config.KeySpec{"export": {"ctrl+e"}},
			ok:   true,
		},
		{
			name: "adding a claimant to a contested key is rejected",
			in:   map[string]config.KeySpec{"export": {"n"}},
			says: []string{"\"n\"", "export"},
		},
		{
			name: "adding a claimant to a singly claimed key is rejected too",
			in:   map[string]config.KeySpec{"export": {"q"}},
			says: []string{"\"q\"", "export", "quit"},
		},
		{
			// A clean swap IS allowed, and this case exists to keep anyone
			// from "fixing" the rule into rejecting it. quit owns "q" and
			// reload owns "r" outright. After trading them, each destination
			// still has exactly one claimant, so the two-or-more clause never
			// applies even though the claimant at each key changed. Measured
			// against the real defaults.
			name: "a clean swap between two bindings is allowed",
			in:   map[string]config.KeySpec{"quit": {"r"}, "reload": {"q"}},
			ok:   true,
		},
		{
			// This is the declared cost from the design doc's §2.2. It is not
			// about swaps: taking a key someone else still claims is rejected
			// even when the two never share a screen.
			// export lives on the report, list_budget on the rates screen, and
			// the rule cannot know that. The user picks another key; the
			// alternative is a table of every screen state, kept in sync
			// forever.
			name: "taking a key another binding still claims is rejected — the declared cost",
			in:   map[string]config.KeySpec{"export": {"g"}},
			says: []string{"export", "\"g\"", "group_by", "list_budget"},
		},
	} {
		_, err := ResolveKeys(tc.in)
		if tc.ok {
			if err != nil {
				t.Errorf("%s: ResolveKeys = %v, want nil", tc.name, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: ResolveKeys = nil error, want one", tc.name)
			continue
		}
		for _, s := range tc.says {
			if !strings.Contains(err.Error(), s) {
				t.Errorf("%s: error %q does not mention %s", tc.name, err, s)
			}
		}
	}
}

// The rule must never reject the table we ship.
func TestDefaultsPassTheCollisionRule(t *testing.T) {
	t.Parallel()
	if _, err := ResolveKeys(nil); err != nil {
		t.Fatalf("the built-in defaults do not satisfy the collision rule: %v", err)
	}
}
```

- [ ] **Step 2: eseguirli e vederli fallire**

Run: `go test ./internal/tui -run 'TestCollisionRule|TestDefaultsPass' -v`
Expected: FAIL sui tre casi che si aspettano un errore — oggi `ResolveKeys` non
controlla niente e li accetta tutti. `TestDefaultsPassTheCollisionRule` e i tre
casi `ok: true` passano già: è giusto così, sono la guardia.

- [ ] **Step 3: implementare**

In `internal/tui/keytable.go`:

```go
// claims maps every physical key to the sorted names of the bindings that want
// it. The defaults are deliberately overloaded — measured, 20 keys have more
// than one claimant — because screenKeys enables only a subset per screen.
func claims(d keyDefaults) map[string][]string {
	out := map[string][]string{}
	v := reflect.ValueOf(d)
	ty := v.Type()
	for i := range v.NumField() {
		b := v.Field(i).Interface().(key.Binding)
		name := bindingName(ty.Field(i).Name)
		for _, k := range b.Keys() {
			out[k] = append(out[k], name)
		}
	}
	for k := range out {
		slices.Sort(out[k])
	}
	return out
}

// checkCollisions rejects a post-override key with two or more claimants unless
// all of them already claimed that key in the defaults.
//
// Detecting real conflicts would mean asking, per screen, which bindings are
// enabled at once — 14 screens plus the sub-modes of entries, log, rates and
// setup, a table to keep in sync with every screen ever added. This rule is
// computed from the key table alone, and is deliberately conservative: it
// refuses taking a key another binding still claims, even when no screen would
// ever activate both. Clean swaps remain allowed; otherwise the user picks
// another key, and the message says exactly which collision they created.
func checkCollisions(before, after keyDefaults) error {
	was, now := claims(before), claims(after)
	for _, k := range slices.Sorted(maps.Keys(now)) {
		names := now[k]
		if len(names) < 2 {
			continue
		}
		for _, n := range names {
			if slices.Contains(was[k], n) {
				continue
			}
			others := slices.DeleteFunc(slices.Clone(names), func(s string) bool { return s == n })
			return fmt.Errorf("binding %q cannot take key %q: it is already claimed by %s",
				n, k, strings.Join(others, ", "))
		}
	}
	return nil
}
```

e in `ResolveKeys`, subito prima del `return` finale:

```go
	if err := checkCollisions(defaultKeys(), d); err != nil {
		return KeyTable{}, err
	}
	return KeyTable{d: d, set: true}, nil
```

- [ ] **Step 4: eseguirli e vederli passare**

Run: `go test ./internal/tui -run 'TestCollisionRule|TestDefaultsPass|TestResolveKeys|TestForceQuit' -v`
Expected: PASS.

Poi la suite intera: `go test ./... -race`. Expected: PASS, e **nessun golden
modificato** (`git status --short internal/tui/testdata/` vuoto).

- [ ] **Step 5: prova per mutazione, obbligatoria**

Due mutazioni, con transcript:

1. Sostituire il corpo di `checkCollisions` con `return nil`. I tre casi di
   `TestCollisionRule` che si aspettano un errore **devono** fallire, e
   `TestDefaultsPassTheCollisionRule` **no**.
2. Rendere la regola assoluta: far ritornare errore appena `len(names) >= 2`,
   senza il confronto con `was[k]`. `TestDefaultsPassTheCollisionRule` **deve**
   fallire — è la prova che la regola stretta rifiuterebbe i default che
   spediamo.

Ripristinare dopo ognuna e verificare con `git diff`.

- [ ] **Step 6: gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui/
git commit -m "feat(tui): enforce the conservative key collision rule (#82)"
```

---

### Task 5: la tabella arriva fino a `Update`

**Files:**
- Modify: `internal/tui/keys.go` (`keysFor`, `screenKeys`)
- Modify: `internal/tui/app.go` (`Model`, `New`)
- Modify: `internal/tui/actions.go` (`screenActions` usa `parseKeyName`)
- Modify: `internal/cli/cli.go` (`runTUI`, più `resolveKeys`)
- Create: `internal/tui/helpers_test.go`
- Modify: i file di test che chiamano `New` (il compilatore li elenca)
- Test: `internal/tui/actions_test.go`, `internal/tui/keys_test.go`,
  `internal/tui/keytable_test.go`
- Test: `internal/cli/cli_test.go`

**Interfaces:**
- Consuma: `KeyTable`, `ResolveKeys` e `parseKeyName` dai Task 3-4;
  `config.Keys` dal Task 1.
- Produce: `New(cfg config.Config, pal themes.Palette, kt KeyTable) Model`,
  `Model.keys`, `testModel(cfg) Model`, `cli.resolveKeys`; la command palette
  può riprodurre ogni chiave canonica accettata all'avvio.

**Contesto che il brief non può sapere.**

1. `defaultKeys()` ha **tre** call site in produzione: `keys.go:293`
   (`keysFor` con la palette aperta), `keys.go:305` (`screenKeys`) e
   `app.go:663`. I primi due leggono la tabella dal Model; **il terzo non si
   tocca**, ed è voluto: `ForceQuit` non è rimappabile, quindi il controllo di
   `ctrl+c` legge dai default non modificabili e nessuna configurazione può
   toglierlo.
2. `New` ha **44 call site**, misurati nella tranche precedente: `app_test.go`
   33, `demo_test.go` 3, `log_test.go` 2, `report_test.go` 2, `golden_test.go` 1,
   `home_test.go` 1, `palette_demo_test.go` 1, `internal/cli/cli.go` 1.
   **Non cercarli con un grep letterale**: la stringa `New(cfg` ne trova nove.
   Cambiare la firma e lasciare che sia `go build ./... && go vet ./...` a
   elencare quelle rimaste.
3. I 43 call site dei test vanno convertiti a `testModel(cfg)`, non a
   `New(cfg, themes.Default(), DefaultKeyTable())`: è la stessa quantità di
   modifiche oggi e una sola la prossima volta che la firma cresce.

- [ ] **Step 1: scrivere i test che falliscono**

`internal/tui/helpers_test.go`:

```go
package tui

import (
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/themes"
)

// testModel builds a Model the way production does, with the built-in palette
// and bindings. It exists so that growing New's signature touches one line
// instead of forty-three.
func testModel(cfg config.Config) Model {
	return New(cfg, themes.Default(), DefaultKeyTable())
}
```

In `internal/tui/keytable_test.go`, il test che chiude il giro:

```go
// The end-to-end guard for the zero-means-defaults decision: an override has to
// survive all the way into Update's routing. If cli ever stopped passing the
// table, the zero value would quietly fall back to the defaults and only this
// test would notice.
func TestAnOverrideReachesUpdate(t *testing.T) {
	kt, err := ResolveKeys(map[string]config.KeySpec{"log_hours": {"L"}})
	if err != nil {
		t.Fatal(err)
	}
	m := New(config.Config{Token: "t", WorkspaceID: "team1"}, themes.Default(), kt)
	m.screen = screenReport
	m.nav = []screen{screenHome}

	if got, _ := m.Update(keyMsg("L")); got.(Model).screen != screenLog {
		t.Errorf("L did not open the log screen; screen = %v", got.(Model).screen)
	}
	if got, _ := m.Update(keyMsg("n")); got.(Model).screen == screenLog {
		t.Error("n still opens the log screen, so the override did not take effect")
	}
}
```

In `internal/cli/cli_test.go`:

```go
func TestResolveKeysRejectsAnUnknownBinding(t *testing.T) {
	t.Parallel()
	_, err := resolveKeys(config.Config{Keys: map[string]config.KeySpec{"nope": {"x"}}})
	if err == nil {
		t.Fatal("resolveKeys of an unknown binding = nil error, want one")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q does not name the binding the user asked for", err)
	}
	if !strings.HasPrefix(err.Error(), "keys:") {
		t.Errorf("error %q is not prefixed with the config section it comes from", err)
	}
}
```

In `internal/tui/actions_test.go`:

- `TestScreenActionsReplayAControlKeyOverride` rimappa Export sul solo `ctrl+e`
  e confronta il risultato dell'azione con il `tea.KeyMsg` fisico;
- `TestScreenActionsReplaySpecialKeyOverrides` fa lo stesso per rappresentanti
  delle famiglie speciali (`f1`, `home`, `delete`, `ctrl+shift+up`, `alt+f20`).
  Il parser esaustivo del Task 3 e il suo riuso diretto in `screenActions` fanno
  valere lo stesso contratto per tutte le special key accettate, non solo per i
  rappresentanti.

In `internal/tui/keys_test.go`, aggiungere
`TestEveryPaletteBindingIsReplayable`, che passa ogni tasto di ogni binding in
`defaultKeys().paletteDefaults()` a `parseKeyName`: anche i default devono
rispettare il contratto imposto agli override. Completare infine
`TestLiteralSpaceOverrideRoutesAndRendersAsSpace` in `keytable_test.go`: la hint
della palette e il footer mostrano `space`, e l'azione produce lo stesso routing
di `tea.KeySpace`.

- [ ] **Step 2: eseguirli e vederli fallire**

Run: `go test ./internal/tui -run 'TestAnOverrideReachesUpdate|TestScreenActionsReplay|TestEveryPaletteBindingIsReplayable|TestLiteralSpace' -v`
e `go test ./internal/cli -run TestResolveKeys -v`.
Expected: i test di wiring non compilano — `New` prende ancora due argomenti e
`resolveKeys` non esiste — mentre i test della palette espongono il replay
limitato del vecchio helper sulle special key.

- [ ] **Step 3: implementare**

In `internal/tui/keys.go`:

```go
func keysFor(m Model) keyMap {
	if m.overlay == overlayPalette {
		return paletteKeys(m.keys.bindings())
	}
	return screenKeys(m)
}
```

e in `screenKeys`, la prima riga del corpo:

```go
	d := m.keys.bindings()
```

In `internal/tui/app.go`, il campo nel `Model`:

```go
	// keys is the resolved binding table. The zero value means the built-in
	// defaults, so a Model built by hand in a test routes normally.
	keys KeyTable
```

e la firma:

```go
func New(cfg config.Config, pal themes.Palette, kt KeyTable) Model {
```

con `keys: kt,` nel letterale `Model{…}`. **`app.go:663` non si tocca.**

In `internal/tui/actions.go`, `screenActions` non ricostruisce più il primo tasto
come runa né mantiene un secondo elenco parziale di control/special key. Usa il
parser del Task 3:

```go
configured := b.Keys()[0]
msg, err := parseKeyName(configured)
if err != nil {
	// ResolveKeys validates overrides; TestEveryPaletteBindingIsReplayable
	// validates defaults. Reaching this point is an internal invariant failure.
	panic(fmt.Sprintf("invalid key %q reached the command palette: %v", configured, err))
}
out = append(out, action{
	label: capitalize(b.Help().Desc),
	hint:  b.Help().Key,
	run:   func(m Model) (tea.Model, tea.Cmd) { return m.routeKey(msg) },
})
```

Lo stesso `tea.KeyMsg` canonico usato per validare viene così riprodotto
dall'azione: nessuna chiave accettata da `ResolveKeys` può sparire dalla palette
o perdere `Type`/`Alt` durante il replay.

In `internal/cli/cli.go`, accanto a `resolveTheme`:

```go
// resolveKeys builds the binding table the TUI will route with, and refuses to
// start on a configuration it cannot honor. Extracted from runTUI for the same
// reason resolveTheme and programOptions were: runTUI blocks on a terminal.
func resolveKeys(cfg config.Config) (tui.KeyTable, error) {
	kt, err := tui.ResolveKeys(cfg.Keys)
	if err != nil {
		return tui.KeyTable{}, fmt.Errorf("keys: %w", err)
	}
	return kt, nil
}
```

e in `runTUI`:

```go
	kt, err := resolveKeys(cfg)
	if err != nil {
		return err
	}
	p := tea.NewProgram(tui.New(cfg, pal, kt), programOptions(cfg)...)
```

Poi far elencare al compilatore i call site rimasti e convertirli a
`testModel(cfg)`.

- [ ] **Step 4: eseguire tutto**

Run: `go test ./... -race`
Expected: PASS, e **nessun golden modificato**. Verificare con
`git status --short internal/tui/testdata/`: deve essere vuoto.

- [ ] **Step 5: prova per mutazione, obbligatoria**

Quattro mutazioni, con transcript:

1. In `screenKeys`, rimettere `d := defaultKeys()`.
   `TestAnOverrideReachesUpdate` **deve** fallire.
2. In `resolveKeys`, ignorare l'errore e ritornare `tui.KeyTable{}, nil`.
   `TestResolveKeysRejectsAnUnknownBinding` **deve** fallire.
3. In `app.go`, cambiare il controllo di ForceQuit da `defaultKeys().ForceQuit` a
   `m.keys.bindings().ForceQuit`. **Nessun test deve fallire** — è il caso in cui
   la mutazione *non* rompe niente, e va **riportato come tale**: oggi le due
   espressioni coincidono perché `force_quit` non è rimappabile, quindi la riga
   è protetta dal divieto e non da un test. Dichiararlo nel report invece di
   fingere una copertura che non c'è.
4. In `screenActions`, ripristinare un helper che accetta solo una runa e
   `ctrl+a`..`ctrl+z`, saltando gli altri nomi. I casi speciali devono fallire
   perché Export scompare dalla palette.

Ripristinare dopo ognuna e verificare con `git diff`.

- [ ] **Step 6: gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui/ internal/cli/
git commit -m "feat(tui): route with the configured binding table (#82)"
```

---

### Task 6: il footer dice la verità

**Files:**
- Modify: `internal/tui/keytable.go` (`KeyTable` ricorda i nomi sovrascritti,
  più `label` e `setHelp`)
- Modify: `internal/tui/keys.go` (i costruttori per schermata ricevono la
  `KeyTable`; i 16 `pairHelp` e gli 8 `SetHelp`)
- Modify: `internal/tui/rates.go`, `internal/tui/rates_view.go` (le due stringhe
  di prosa che nominano un tasto)
- Test: `internal/tui/keytable_test.go`

**Interfaces:**
- Consuma: `KeyTable` dai Task 3-5.
- Produce: `(KeyTable).label`, `(KeyTable).setHelp`, `(KeyTable).keysOf`.

**Il difetto che questo task chiude, misurato.** Rigenerare l'aiuto dentro
`ResolveKeys` non basta: i costruttori per schermata riscrivono l'etichetta con
stringhe letterali in **24 punti** — 16 `pairHelp` e 8 `SetHelp` — perché una
riga di footer copre spesso una coppia di binding. Con una tabella che rimappa
`confirm→ctrl+j` e `back→ctrl+q`, il footer della palette resta
`↑/↓ move · enter run · esc close`. Il test del Task 3 guarda la `KeyTable` e non
un footer reso, quindi passa mentendo.

**La regola**: l'etichetta letterale si usa finché **nessuno** dei binding che
copre è stato rimappato. Così il footer di default non cambia di un byte — e
**nessun golden si muove**, che è la verifica principale di questo task.

- [ ] **Step 1: scrivere i test che falliscono**

In `internal/tui/keytable_test.go`:

```go
// The footer is where a remapped binding would lie: the per-screen builders
// write the key label as a literal, so regenerating it inside ResolveKeys is
// not enough on its own.
func TestFooterShowsRemappedKeys(t *testing.T) {
	kt, err := ResolveKeys(map[string]config.KeySpec{"up": {"ctrl+u"}})
	if err != nil {
		t.Fatal(err)
	}
	m := New(config.Config{Token: "t", WorkspaceID: "team1"}, themes.Default(), kt)
	m.screen = screenReport
	m.nav = []screen{screenHome}
	m.width = 120

	foot := m.footerView()
	if !strings.Contains(foot, "ctrl+u") {
		t.Errorf("footer does not advertise the remapped key:\n%s", foot)
	}
	if strings.Contains(foot, "↑/↓/j/k") {
		t.Errorf("footer still shows the default label for a remapped binding:\n%s", foot)
	}
}

// And the other half: with nothing remapped the label is untouched, which is
// what keeps every footer golden where it is.
func TestFooterKeepsItsLabelsWhenNothingIsRemapped(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "team1"}, themes.Default(), DefaultKeyTable())
	m.screen = screenReport
	m.nav = []screen{screenHome}
	m.width = 120

	if foot := m.footerView(); !strings.Contains(foot, "↑/↓/j/k") {
		t.Errorf("footer lost its default label:\n%s", foot)
	}
}
```

**Il nome esatto del metodo che rende il footer va verificato prima di scrivere
il test** — leggere `internal/tui/footer.go` e usare quello vero, non
`footerView` se si chiama diversamente.

- [ ] **Step 2: eseguirli e vederli fallire**

Run: `go test ./internal/tui -run TestFooter -v`
Expected: `TestFooterShowsRemappedKeys` FAIL (il footer mostra ancora
`↑/↓/j/k`), `TestFooterKeepsItsLabelsWhenNothingIsRemapped` PASS — è la guardia.

- [ ] **Step 3: implementare**

In `internal/tui/keytable.go`, la tabella ricorda cosa è stato toccato:

```go
type KeyTable struct {
	d    keyDefaults
	over map[string]bool // names the config remapped
	set  bool
}
```

`ResolveKeys` popola `over[name] = true` a ogni override e lo mette nel valore
di ritorno. `DefaultKeyTable()` lo lascia nil: niente è stato rimappato.

```go
// keysOf returns the keys currently bound to a binding, by config name.
func (kt KeyTable) keysOf(name string) []string { … }

// label returns lit unless one of the named bindings was remapped, in which
// case it derives the label from the keys actually bound.
//
// The literal labels the screens pass carry typography the defaults earned —
// "↑/↓/j/k", "tab/⇧tab" — and deriving them unconditionally would replace that
// with something longer and uglier for every user, remapped or not, and move
// every footer golden. So the literal wins until it would be a lie.
func (kt KeyTable) label(lit string, names ...string) string {
	remapped := false
	var keys []string
	for _, n := range names {
		if kt.over[n] {
			remapped = true
		}
		keys = append(keys, kt.keysOf(n)...)
	}
	if !remapped {
		return lit
	}
	return keyLabel(keys)
}

// setHelp is label's counterpart for the single-binding SetHelp call sites.
func (kt KeyTable) setHelp(b *key.Binding, name, lit, desc string) {
	b.SetHelp(kt.label(lit, name), desc)
}
```

In `internal/tui/keys.go`, i costruttori per schermata ricevono la `KeyTable`
invece della sola `keyDefaults` — è una **semplificazione** di firma, non una
crescita, perché `d` si ricava dentro:

```go
func homeKeys(m Model, kt KeyTable) keyMap {
	d := kt.bindings()
	…
}
```

e i 24 punti diventano:

```go
pair := pairHelp(k.Up, k.Down, kt.label("↑/↓/j/k", "up", "down"), "move")
kt.setHelp(&k.ClearValue, "clear_value", "d", "use the default rate")
```

`pairHelp` **non cambia firma**: cambia solo il terzo argomento nei suoi 16 call
site.

In `internal/tui/rates.go:618` e `internal/tui/rates_view.go:55`, le due
stringhe che nominano un tasto vanno costruite dal binding vivo invece che dalla
lettera — per esempio interpolando `k.ListBudget.Help().Key` e
`k.BrowseList.Help().Key` al posto di `'g'` e `'b'`.

- [ ] **Step 4: eseguire tutto**

Run: `go test ./... -race`
Expected: PASS, e **nessun golden modificato**. Verificare con
`git status --short internal/tui/testdata/`: deve essere vuoto. Se un golden si
muove, l'etichetta letterale non sta più vincendo quando dovrebbe, ed è un
difetto: non rigenerarlo.

- [ ] **Step 5: prova per mutazione, obbligatoria**

Due mutazioni, con transcript:

1. In `label`, ritornare sempre `lit`. `TestFooterShowsRemappedKeys` **deve**
   fallire.
2. In `label`, ritornare sempre la forma derivata. `TestFooterKeepsItsLabelsWhenNothingIsRemapped`
   **deve** fallire, **e con essa i golden dei footer** — è la prova che la
   guardia protegge davvero il rendering di default. Riportare quanti golden
   falliscono.

Ripristinare dopo ognuna e verificare con `git diff`.

- [ ] **Step 6: gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui/
git commit -m "fix(tui): show remapped keys in the footer and the help (#82)"
```

---

### Task 7: documentazione

**Files:**
- Modify: `README.md` (elenco delle chiavi di config, dopo il blocco `themes`)
- Modify: `README.it.md` (stesso punto)
- Modify: `CHANGELOG.md` (sezione `## [Unreleased]`)

**Interfaces:** nessuna.

- [ ] **Step 1: `README.md`**

Dopo il punto elenco `themes`:

```markdown
- `keys` (optional): remap any binding except `force_quit` by name. A value is one
  key or a list of them, and every binding you do not name keeps its default. The
  names are listed by `clup` in the error it prints when one is wrong; they are the
  binding names in snake_case — `quit`, `log_hours`, `prev_month`, `palette_up`,
  and so on.

    ```yaml
    keys:
      log_hours: "L"
      up: ["up", "ctrl+u"]
    ```

  - **`force_quit` cannot be remapped.** `ctrl+c` is the way out of a TUI whose
    other keys you have moved somewhere you cannot reach.
  - **An override is refused when it adds a claimant to a key that still has two
    or more claimants after all overrides**, and the message names the binding that
    has it. The built-in bindings deliberately share keys — `n` alone serves four
    of them, on screens where only one is ever active — so the rule is not "no
    sharing". Moving a binding off a shared key and clean swaps are allowed. The
    check cannot tell which screens two bindings share, so it errs toward refusing:
    if it rejects something you believe is safe, pick another key.
  - A remapped binding shows its new keys in the footer and in the `?` help.
```

- [ ] **Step 2: `README.it.md`**

Lo stesso contenuto, in italiano, nella posizione corrispondente. Mantenere i due
file equivalenti nel significato. Usare il registro informale singolare che il
file usa altrove ("puoi", "tuo"), non il plurale di cortesia.

- [ ] **Step 3: `CHANGELOG.md`**

In `### Added` sotto `## [Unreleased]`:

```markdown
- Keybindings are configurable: `keys:` in the config remaps any binding except
  `force_quit` by name, with a single key or a list. `force_quit` (`ctrl+c`) stays
  fixed so there is always a way out. An unknown name, an empty list, or adding a
  claimant to a key that still has two or more claimants after all overrides stops
  startup with a message naming what collided; clean moves and swaps remain allowed
  (#82).
```

- [ ] **Step 4: rileggere quello che si è scritto**

Aprire i tre file e **guardare** le sezioni modificate in contesto: che i punti
elenco stiano nella lista giusta, che i blocchi YAML siano annidati sotto il
proprio punto, e che la voce di changelog sia sotto `## [Unreleased]`. Riportare
cosa si è visto.

- [ ] **Step 5: gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add README.md README.it.md CHANGELOG.md
git commit -m "docs: document the keys map (#82)"
```

---

## Note per il controllore

- Con questo task **la #82 si chiude**, e con lei la milestone v1.9.
- Nessun golden deve muoversi in nessun task: non cambia nessun default, e il
  Task 6 tiene apposta l'etichetta letterale finché niente è rimappato.
- `TestForceQuitSurvivesEveryOverride` (Task 3) usa una sweep di rune Unicode
  private-use, tutte producibili come singoli `KeyMsg` e deliberatamente prive di
  collisioni. Al Task 4 va comunque **rieseguito**: se cominciasse a fallire, la
  regola sta rifiutando qualcosa che non dovrebbe.
- **Il Task 6 è nato da una review, non dalla prima stesura.** La spec diceva che
  rigenerare l'aiuto dentro `ResolveKeys` bastava; eseguendo, il footer
  continuava a mostrare i tasti di default in 24 punti. Se un implementer trova
  che l'elenco dei 24 non torna, va misurato di nuovo e riportato, non aggirato.
