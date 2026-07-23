# Avviso di aggiornamento (#104) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dire all'utente che esiste una release più recente — su stderr nella CLI e con una riga discreta nella TUI — senza rallentare nulla, senza rumore negli script e senza mai scaricare codice.

**Architecture:** Confronto e forma delle versioni in un package puro `internal/version`; versione corrente, cache su disco e chiamata HTTP in `internal/service/update.go` (il livello impuro già stabilito); le due superfici consumano il risultato. Il controllo parte solo per versioni che sono release pubblicate, al massimo ogni 24 ore, ed è silenzioso in ogni caso di errore.

**Tech Stack:** Go 1.26, stdlib (`net/http`, `encoding/json`, `runtime/debug`, `os`), cobra per la CLI, bubbletea per la TUI. **Nessuna nuova dipendenza.**

**Spec:** `docs/superpowers/specs/2026-07-23-update-notice-design.md` (leggerla; questo piano la implementa).

## Global Constraints

- **Purezza:** `internal/version` non fa I/O, non chiama `time.Now`, non importa `config`/`clickup`/`tui`/`service`. Tutto l'I/O sta in `internal/service`. Qualsiasi violazione è un difetto.
- **Regola della versione (positiva):** il controllo parte **solo** se la versione corrente è `vMAJOR.MINOR.PATCH` con tutte e tre le componenti numeriche, **senza** prerelease e **senza** build metadata. Da Go 1.24 una build locale produce una pseudo-version (`v1.6.1-0.20260723143812-50d39f8`), non `(devel)`: elencare i casi da escludere sarebbe incompleto.
- **Avviso solo se strettamente più recente:** `latest > current`, mai `latest != current`.
- **Silenzio totale sugli errori:** rete, timeout, 4xx/5xx, rate limit, JSON malformato, cache corrotta — nessun avviso, nessun messaggio, nessun exit code diverso.
- **stdout è intoccabile:** l'avviso della CLI va su **stderr**. `clup report --json` deve restare parsabile.
- **Nessun header `Authorization`** sulla chiamata a GitHub; `User-Agent` obbligatorio.
- **Opt-out:** `CLUP_NO_UPDATE_CHECK` (vince su tutto) → config `update_check: false` → altrimenti attivo.
- **Demo: solo la TUI.** In `CLICKUP_DEMO=1` la TUI non controlla mai (vincolo "zero I/O" della demo). Il comando headless **ignora deliberatamente** `CLICKUP_DEMO` — lo dichiara un commento in `internal/cli/report.go` ed è bloccato da `TestReportIgnoresDemoEnv`: il percorso headless passa sempre dalla config e dall'API vere, quindi controlla anche in quella condizione. Perciò il Task 6 passa `false` come modalità demo, e non è una svista.
- **Config additiva:** `UpdateCheck *bool` con tag `yaml:"update_check,omitempty"`. Il puntatore evita che l'assenza della chiave valga `false`; `omitempty` evita che `Save` scriva `update_check: null` ovunque.
- **Niente self-update:** il tool non scarica né sostituisce mai il proprio binario.
- **Processo:** Conventional Commits, **MAI** `Co-Authored-By`. Testo in-repo in inglese (i doc di design restano in italiano). Prima di ogni commit: `gofmt -l .` (vuoto), `go vet ./...`, `go build ./...`, `go test ./... -race` (verde).

## File Structure

- `internal/version/version.go` (nuovo) — puro: `IsRelease`, `Newer`, `Resolve`, costante `Dev`.
- `internal/version/version_test.go` (nuovo) — table-driven.
- `internal/service/update.go` (nuovo) — `CurrentVersion`, `Enabled`, cache, `CheckForUpdate`.
- `internal/service/update_test.go` (nuovo) — `httptest` + `t.TempDir()`.
- `internal/config/config.go` (modifica) — campo `UpdateCheck *bool`.
- `internal/cli/cli.go` (modifica) — `const version` → resolver.
- `internal/cli/report.go` (modifica) — controllo concorrente + avviso su stderr.
- `internal/tui/app.go` (modifica) — `Init` lancia il comando, msg tipizzato, campo sul Model.
- `internal/tui/home.go` (modifica) — riga dell'avviso.
- `README.md`, `README.it.md` (modifica) — documentazione della funzione e dell'opt-out.

---

### Task 1: `internal/version` — forma e confronto delle versioni

**Files:**
- Create: `internal/version/version.go`, `internal/version/version_test.go`

**Interfaces:**
- Produces:
```go
const Dev = "dev"
func IsRelease(v string) bool                       // true solo per "vX.Y.Z" puro
func Newer(current, latest string) bool             // true se latest > current; false se uno dei due non è release
func Resolve(ldflagsVersion, mainVersion string) string
```

- [ ] **Step 1: scrivere i test che falliscono** in `internal/version/version_test.go`:

```go
package version

import "testing"

func TestIsRelease(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"v1.7.0", true},
		{"v0.0.0", true},
		{"v1.10.3", true},
		{"dev", false},
		{"(devel)", false},
		{"", false},
		{"1.7.0", false},                                  // manca la v
		{"v1.7", false},                                   // due componenti
		{"v1.7.0.1", false},                               // quattro componenti
		{"v1.7.0-rc1", false},                             // prerelease
		{"v1.7.0+dirty", false},                           // build metadata
		{"v1.6.1-0.20260723143812-50d39f89c2fe", false},   // pseudo-version (go build da Go 1.24)
		{"v1.7.x", false},
		{"v1.+7.0", false},                                // Atoi accetterebbe "+7": va rifiutato
		{"v1..0", false},
		{"v-1.7.0", false},
	}
	for _, c := range cases {
		if got := IsRelease(c.in); got != c.want {
			t.Errorf("IsRelease(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		name             string
		current, latest  string
		want             bool
	}{
		{"patch", "v1.7.0", "v1.7.1", true},
		{"minor", "v1.7.9", "v1.8.0", true},
		{"major", "v1.9.9", "v2.0.0", true},
		{"equal", "v1.7.0", "v1.7.0", false},
		{"older", "v1.8.0", "v1.7.0", false},
		{"numeric not lexicographic", "v1.9.0", "v1.10.0", true},
		{"numeric not lexicographic patch", "v1.0.9", "v1.0.10", true},
		{"current not a release", "dev", "v1.8.0", false},
		{"current is a pseudo-version", "v1.6.1-0.20260723143812-50d39f8", "v1.8.0", false},
		{"latest not a release", "v1.7.0", "garbage", false},
		{"both empty", "", "", false},
	}
	for _, c := range cases {
		if got := Newer(c.current, c.latest); got != c.want {
			t.Errorf("%s: Newer(%q, %q) = %v, want %v", c.name, c.current, c.latest, got, c.want)
		}
	}
}

func TestResolve(t *testing.T) {
	cases := []struct {
		name             string
		ldflags, mainVer string
		want             string
	}{
		{"ldflags wins", "v1.8.0", "v1.7.0", "v1.8.0"},
		{"build info when no ldflags", "", "v1.7.0", "v1.7.0"},
		{"pseudo-version passes through", "", "v1.6.1-0.2026-abc", "v1.6.1-0.2026-abc"},
		{"nothing available", "", "", Dev},
	}
	for _, c := range cases {
		if got := Resolve(c.ldflags, c.mainVer); got != c.want {
			t.Errorf("%s: Resolve(%q, %q) = %q, want %q", c.name, c.ldflags, c.mainVer, got, c.want)
		}
	}
}
```

- [ ] **Step 2: verificare che falliscano** — `go test ./internal/version/ -v` → FAIL (package inesistente).

- [ ] **Step 3: implementare** `internal/version/version.go`:

```go
// Package version reports and compares clup's build version. It is pure: no
// I/O, no time, no dependencies beyond the standard library.
package version

import "strings"

// Dev is the version reported when no build version is available.
const Dev = "dev"

// parseRelease parses a published release version — "vMAJOR.MINOR.PATCH" with
// no prerelease segment and no build metadata — into its three components.
//
// The shape test is deliberately positive rather than a list of exclusions.
// Since Go 1.24 a local `go build` stamps the version from VCS state, so a
// source build past a tag reports a pseudo-version such as
// "v1.6.1-0.20260723143812-50d39f8" (and "+dirty" on a dirty tree), not
// "(devel)". Enumerating the forms to reject would be incomplete by
// construction; requiring the release shape is not.
func parseRelease(v string) (major, minor, patch int, ok bool) {
	rest, found := strings.CutPrefix(v, "v")
	if !found {
		return 0, 0, 0, false
	}
	parts := strings.Split(rest, ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	var out [3]int
	for i, p := range parts {
		n, ok := atoiStrict(p)
		if !ok {
			return 0, 0, 0, false
		}
		out[i] = n
	}
	return out[0], out[1], out[2], true
}

// atoiStrict parses a run of ASCII digits. Unlike strconv.Atoi it rejects
// signs, so "+7" (which Atoi would happily read as 7) is not a version
// component.
func atoiStrict(s string) (int, bool) {
	if s == "" || len(s) > 9 { // 9 digits keeps the accumulation far from overflow
		return 0, false
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

// IsRelease reports whether v is a published release version. Only these
// versions take part in an update check: a source build must never be told to
// "update".
func IsRelease(v string) bool {
	_, _, _, ok := parseRelease(v)
	return ok
}

// Newer reports whether latest is strictly newer than current. It is false
// unless both are release versions — and false when they are equal, or when
// latest is older, so that a locally built pre-release tag is never told to
// downgrade.
func Newer(current, latest string) bool {
	cMaj, cMin, cPatch, ok := parseRelease(current)
	if !ok {
		return false
	}
	lMaj, lMin, lPatch, ok := parseRelease(latest)
	if !ok {
		return false
	}
	if lMaj != cMaj {
		return lMaj > cMaj
	}
	if lMin != cMin {
		return lMin > cMin
	}
	return lPatch > cPatch
}

// Resolve picks the version to report, preferring a value injected at build
// time, then the version the Go toolchain stamped into the binary, then Dev.
// It takes both inputs as parameters so it can be tested: debug.ReadBuildInfo
// inside a test binary reports the test module, not the built program.
func Resolve(ldflagsVersion, mainVersion string) string {
	if ldflagsVersion != "" {
		return ldflagsVersion
	}
	if mainVersion != "" {
		return mainVersion
	}
	return Dev
}
```

- [ ] **Step 4: verificare che passino** — `go test ./internal/version/ -race` → PASS.

- [ ] **Step 5: commit** — `feat(version): pure release-shape check and version comparison (#104)`

---

### Task 2: versione corrente reale in `clup --version`

**Files:**
- Create: `internal/service/update.go` (solo `CurrentVersion` in questo task)
- Modify: `internal/cli/cli.go` (rimuove `const version = "dev"`)
- Test: `internal/cli/cli_test.go` (estende l'asserzione esistente)

**Interfaces:**
- Consumes: `version.Resolve`, `version.Dev` (Task 1).
- Produces: `func service.CurrentVersion() string`.

- [ ] **Step 1: nessun test nuovo — usare quello che c'è.** `TestRootCmdSettings` in
`internal/cli/cli_test.go` asserisce già `cmd.Version != ""`, che è esattamente
l'invariante da preservare: dopo la modifica il comando radice deve continuare a esporre
una versione non vuota.

**Non** aggiungere un test tipo "la versione non è più `dev` hardcoded": non potrebbe
funzionare. Sotto `go test` il binario porta le build info del *modulo di test*, quindi il
valore esatto non è fissabile, e un'asserzione di non-vuoto passerebbe identica anche con
la costante di prima — un test che non può fallire per la ragione che dichiara. La
copertura vera del resolver sta nei test puri di `version.Resolve` (Task 1).

- [ ] **Step 2: eseguire prima della modifica** — `go test ./internal/cli/ -run TestRootCmdSettings -v` → PASS. È la rete di sicurezza: deve continuare a passare dopo.

- [ ] **Step 3: implementare** `internal/service/update.go`:

```go
package service

import (
	"runtime/debug"

	"github.com/marcoarnulfo/clickup-cli/internal/version"
)

// ldflagsVersion can be set at build time with
//
//	-ldflags "-X github.com/marcoarnulfo/clickup-cli/internal/service.ldflagsVersion=v1.8.0"
//
// No tooling sets it today: it exists so a future release pipeline can stamp a
// version, and as the injection seam that keeps version.Resolve testable.
var ldflagsVersion string

// CurrentVersion reports the version of the running binary. For a `go install
// module/cmd@vX.Y.Z` build the Go toolchain stamps the resolved tag, so this
// is a real release version; for a local `go build` it is a pseudo-version,
// which version.IsRelease deliberately rejects.
func CurrentVersion() string {
	main := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		main = info.Main.Version
	}
	return version.Resolve(ldflagsVersion, main)
}
```

- [ ] **Step 4: sostituire la costante** in `internal/cli/cli.go`: eliminare `const version = "dev"` (riga 18 e il suo commento) e usare `service.CurrentVersion()` dentro `rootCmd()`, dove il comando imposta `Version:`. Aggiungere l'import di `internal/service` se manca.

- [ ] **Step 5: eseguire** — `go test ./internal/cli/ ./internal/service/ -race` → PASS; `go build ./...` pulito.

- [ ] **Step 6: commit** — `feat(cli): report the real build version instead of a hardcoded "dev" (#104)`

---

### Task 3: chiave di config `update_check`

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces: campo `Config.UpdateCheck *bool` con tag `yaml:"update_check,omitempty"`.

- [ ] **Step 1: scrivere i test che falliscono** in `internal/config/config_test.go`:

> **Isolamento obbligatorio.** Ogni test che chiama `Save` **deve** aprire con
> `isolateConfig(t)`, l'helper già presente in `config_test.go`. Impostare solo
> `XDG_CONFIG_HOME` **non isola nulla su macOS**, dove `os.UserConfigDir()` ignora quella
> variabile: un test così scriverebbe nel config reale dello sviluppatore
> (`~/Library/Application Support/clup/config.yml`) e glielo sovrascriverebbe.
> `isolateConfig` imposta sia `HOME` sia `XDG_CONFIG_HOME` proprio per questo.
> Nota anche che **`Path()` ritorna `(string, error)`**.

```go
func TestUpdateCheckAbsentIsNil(t *testing.T) {
	// A config file without the key must load as nil — meaning "enabled".
	// With a plain bool the absent key would decode as false and the update
	// check would be born disabled in every existing config.
	isolateConfig(t)
	if err := Save(Config{Token: "t", WorkspaceID: "1"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UpdateCheck != nil {
		t.Fatalf("UpdateCheck = %v, want nil for an absent key", *cfg.UpdateCheck)
	}
}

func TestUpdateCheckFalseRoundTrips(t *testing.T) {
	isolateConfig(t)
	no := false
	if err := Save(Config{Token: "t", WorkspaceID: "1", UpdateCheck: &no}); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.UpdateCheck == nil || *got.UpdateCheck {
		t.Fatalf("UpdateCheck = %v, want an explicit false", got.UpdateCheck)
	}
}

func TestUpdateCheckNilIsNotWrittenToDisk(t *testing.T) {
	// omitempty matters: without it Save writes "update_check: null" into
	// every config file it touches.
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
	if strings.Contains(string(raw), "update_check") {
		t.Fatalf("saved config mentions update_check:\n%s", raw)
	}
}
```

- [ ] **Step 2: verificare che falliscano** — `go test ./internal/config/ -run UpdateCheck -v` → FAIL (campo inesistente).

- [ ] **Step 3: implementare** — in `internal/config/config.go`, aggiungere al `Config` (in coda ai campi esistenti, senza spostare nulla):

```go
	// UpdateCheck enables the "a newer release exists" check. nil means
	// enabled: a plain bool would make an absent key decode as false and
	// disable the check in every config written before this field existed.
	UpdateCheck *bool `yaml:"update_check,omitempty"`
```

Nessuna modifica a `migrate`: l'aggiunta è additiva e uno schema v2 senza la chiave resta valido.

- [ ] **Step 4: eseguire** — `go test ./internal/config/ -race` → PASS.

- [ ] **Step 5: commit** — `feat(config): optional update_check key (#104)`

---

### Task 4: cache su disco

**Files:**
- Modify: `internal/service/update.go`
- Test: `internal/service/update_test.go` (nuovo)

**Interfaces:**
- Produces (non esportati, usati dal Task 5):
```go
type updateCache struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}
func defaultCachePath() (string, error)          // os.UserCacheDir()/clup/update.json
func readCache(path string) (updateCache, bool)  // ok=false se manca/illeggibile/corrotta
func writeCache(path string, c updateCache) error // atomica: temp + rename
func cacheFresh(c updateCache, now time.Time) bool
```

- [ ] **Step 1: scrivere i test che falliscono** in `internal/service/update_test.go`:

```go
package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update.json")
	now := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	if err := writeCache(path, updateCache{CheckedAt: now, Latest: "v1.8.0"}); err != nil {
		t.Fatal(err)
	}
	got, ok := readCache(path)
	if !ok || got.Latest != "v1.8.0" || !got.CheckedAt.Equal(now) {
		t.Fatalf("readCache = %+v, ok=%v", got, ok)
	}
}

func TestReadCacheMissingOrCorrupt(t *testing.T) {
	dir := t.TempDir()
	if _, ok := readCache(filepath.Join(dir, "nope.json")); ok {
		t.Error("missing cache must not report ok")
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := readCache(bad); ok {
		t.Error("corrupt cache must not report ok — it is treated as stale, never as an error")
	}
}

func TestCacheFresh(t *testing.T) {
	now := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		checkedAt time.Time
		want      bool
	}{
		{"just checked", now.Add(-time.Minute), true},
		{"23h ago", now.Add(-23 * time.Hour), true},
		{"25h ago", now.Add(-25 * time.Hour), false},
		{"exactly 24h ago", now.Add(-24 * time.Hour), false},
		{"in the future", now.Add(time.Hour), false}, // clock moved back: never "fresh forever"
		{"zero value", time.Time{}, false},
	}
	for _, c := range cases {
		if got := cacheFresh(updateCache{CheckedAt: c.checkedAt}, now); got != c.want {
			t.Errorf("%s: cacheFresh = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestWriteCacheLeavesNoTempFile(t *testing.T) {
	// Two concurrent clup invocations can write the cache at the same time; a
	// truncated file is exactly the corruption readCache has to tolerate.
	// Writing through a temp file plus rename means a reader sees either the
	// old file or the new one, never half of one.
	//
	// Note the limit of this test, and do not over-trust it: it only proves no
	// temp file is left behind. A plain os.WriteFile would pass it too.
	// Atomicity itself is not practically assertable here; the guarantee comes
	// from os.Rename, and the test guards the litter the technique produces.
	dir := t.TempDir()
	path := filepath.Join(dir, "update.json")
	if err := writeCache(path, updateCache{CheckedAt: time.Now(), Latest: "v1.8.0"}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "update.json" {
		t.Fatalf("temp file left behind: %v", entries)
	}
}
```

- [ ] **Step 2: verificare che falliscano** — `go test ./internal/service/ -run Cache -v` → FAIL (simboli inesistenti).

- [ ] **Step 3: implementare** — aggiungere a `internal/service/update.go`:

```go
// updateCheckInterval is how long a check result is reused before asking
// GitHub again.
const updateCheckInterval = 24 * time.Hour

type updateCache struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

// defaultCachePath returns the cache location. os.UserCacheDir honours
// XDG_CACHE_HOME on Linux and gives the right directory on macOS, mirroring
// how internal/config uses os.UserConfigDir rather than a hardcoded path.
func defaultCachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "clup", "update.json"), nil
}

// readCache reads the cached result. A missing, unreadable or malformed file
// reports ok == false: the caller treats that as stale and refetches. A cache
// problem is never surfaced to the user.
func readCache(path string) (updateCache, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return updateCache{}, false
	}
	var c updateCache
	if err := json.Unmarshal(raw, &c); err != nil {
		return updateCache{}, false
	}
	return c, true
}

// writeCache writes the cache atomically: a temp file in the same directory
// followed by a rename, so a concurrent reader sees either the old file or the
// new one and never a truncated one.
func writeCache(path string, c updateCache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".update-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once the rename succeeded
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// cacheFresh reports whether the cached result is still usable. A CheckedAt in
// the future counts as stale: without that, a clock moved backwards would keep
// the cache "fresh" forever.
func cacheFresh(c updateCache, now time.Time) bool {
	if c.CheckedAt.IsZero() || c.CheckedAt.After(now) {
		return false
	}
	return now.Sub(c.CheckedAt) < updateCheckInterval
}
```

Import da aggiungere: `encoding/json`, `os`, `path/filepath`, `time`.

- [ ] **Step 4: eseguire** — `go test ./internal/service/ -race` → PASS.

- [ ] **Step 5: commit** — `feat(service): atomic 24h cache for the update check (#104)`

---

### Task 5: opt-out e controllo vero e proprio

**Files:**
- Modify: `internal/service/update.go`
- Test: `internal/service/update_test.go`

**Interfaces:**
- Consumes: `version.IsRelease`, `version.Newer` (Task 1); `config.Config.UpdateCheck` (Task 3); la cache (Task 4).
- Produces:
```go
func UpdateCheckEnabled(cfg config.Config, demo bool) bool

type UpdateOptions struct {
	Current   string        // versione corrente; se non è una release il controllo non parte
	CachePath string        // "" => defaultCachePath()
	APIURL    string        // "" => endpoint GitHub
	Now       time.Time     // zero => time.Now()
}

// CheckForUpdate returns the latest release tag and true only when it is
// strictly newer than Current.
func CheckForUpdate(ctx context.Context, o UpdateOptions) (latest string, newer bool)
```

- [ ] **Step 1: scrivere i test che falliscono** in `internal/service/update_test.go`:

```go
func TestUpdateCheckEnabled(t *testing.T) {
	no, yes := false, true
	cases := []struct {
		name string
		env  string
		cfg  config.Config
		demo bool
		want bool
	}{
		{"default on", "", config.Config{}, false, true},
		{"config nil is on", "", config.Config{UpdateCheck: nil}, false, true},
		{"config true", "", config.Config{UpdateCheck: &yes}, false, true},
		{"config false", "", config.Config{UpdateCheck: &no}, false, false},
		{"env wins over config true", "1", config.Config{UpdateCheck: &yes}, false, false},
		{"env any value", "please-dont", config.Config{}, false, false},
		{"demo never checks", "", config.Config{UpdateCheck: &yes}, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("CLUP_NO_UPDATE_CHECK", c.env)
			if got := UpdateCheckEnabled(c.cfg, c.demo); got != c.want {
				t.Errorf("UpdateCheckEnabled = %v, want %v", got, c.want)
			}
		})
	}
}

// newTestAPI returns a server answering like GitHub's releases/latest, and a
// counter of how many requests it received.
func newTestAPI(t *testing.T, tag string, status int) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.Header.Get("User-Agent") == "" {
			t.Error("request has no User-Agent; GitHub rejects those")
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("update check must never send an Authorization header")
		}
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		fmt.Fprintf(w, `{"tag_name":%q}`, tag)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestCheckForUpdateFetchesAndReportsNewer(t *testing.T) {
	srv, calls := newTestAPI(t, "v1.8.0", http.StatusOK)
	latest, newer := CheckForUpdate(context.Background(), UpdateOptions{
		Current:   "v1.7.0",
		CachePath: filepath.Join(t.TempDir(), "update.json"),
		APIURL:    srv.URL,
		Now:       time.Now(),
	})
	if latest != "v1.8.0" || !newer {
		t.Fatalf("got (%q, %v), want (v1.8.0, true)", latest, newer)
	}
	if *calls != 1 {
		t.Fatalf("calls = %d, want 1", *calls)
	}
}

func TestCheckForUpdateUsesFreshCacheWithoutCallingTheServer(t *testing.T) {
	// Without the request counter this test would prove nothing: it would pass
	// whether or not the cache was consulted.
	srv, calls := newTestAPI(t, "v9.9.9", http.StatusOK)
	path := filepath.Join(t.TempDir(), "update.json")
	now := time.Now()
	if err := writeCache(path, updateCache{CheckedAt: now.Add(-time.Hour), Latest: "v1.8.0"}); err != nil {
		t.Fatal(err)
	}
	latest, newer := CheckForUpdate(context.Background(), UpdateOptions{
		Current: "v1.7.0", CachePath: path, APIURL: srv.URL, Now: now,
	})
	if latest != "v1.8.0" || !newer {
		t.Fatalf("got (%q, %v), want the cached (v1.8.0, true)", latest, newer)
	}
	if *calls != 0 {
		t.Fatalf("server was called %d times despite a fresh cache", *calls)
	}
}

func TestCheckForUpdateSkipsNonReleaseCurrent(t *testing.T) {
	srv, calls := newTestAPI(t, "v1.8.0", http.StatusOK)
	for _, current := range []string{"dev", "(devel)", "v1.6.1-0.20260723143812-50d39f8", "v1.7.0+dirty"} {
		latest, newer := CheckForUpdate(context.Background(), UpdateOptions{
			Current: current, CachePath: filepath.Join(t.TempDir(), "update.json"),
			APIURL: srv.URL, Now: time.Now(),
		})
		if newer || latest != "" {
			t.Errorf("current=%q: got (%q, %v), want no check at all", current, latest, newer)
		}
	}
	if *calls != 0 {
		t.Fatalf("a source build must not reach the network (calls=%d)", *calls)
	}
}

func TestCheckForUpdateOlderOrEqualIsSilent(t *testing.T) {
	for _, tag := range []string{"v1.7.0", "v1.6.0"} {
		srv, _ := newTestAPI(t, tag, http.StatusOK)
		_, newer := CheckForUpdate(context.Background(), UpdateOptions{
			Current: "v1.7.0", CachePath: filepath.Join(t.TempDir(), "update.json"),
			APIURL: srv.URL, Now: time.Now(),
		})
		if newer {
			t.Errorf("tag %q: reported an update over v1.7.0", tag)
		}
	}
}

func TestCheckForUpdateFailuresAreSilentAndStampTheCache(t *testing.T) {
	// Offline users must not pay the timeout on every single invocation, so a
	// failed attempt still records when it happened.
	srv, _ := newTestAPI(t, "", http.StatusInternalServerError)
	path := filepath.Join(t.TempDir(), "update.json")
	now := time.Now()
	if _, newer := CheckForUpdate(context.Background(), UpdateOptions{
		Current: "v1.7.0", CachePath: path, APIURL: srv.URL, Now: now,
	}); newer {
		t.Fatal("a 500 must not produce a notice")
	}
	c, ok := readCache(path)
	if !ok || !c.CheckedAt.Equal(now) {
		t.Fatalf("failed attempt not stamped: %+v ok=%v", c, ok)
	}
}

func TestCheckForUpdateFailureKeepsPreviousLatest(t *testing.T) {
	srv, _ := newTestAPI(t, "", http.StatusInternalServerError)
	path := filepath.Join(t.TempDir(), "update.json")
	now := time.Now()
	if err := writeCache(path, updateCache{CheckedAt: now.Add(-48 * time.Hour), Latest: "v1.8.0"}); err != nil {
		t.Fatal(err)
	}
	latest, newer := CheckForUpdate(context.Background(), UpdateOptions{
		Current: "v1.7.0", CachePath: path, APIURL: srv.URL, Now: now,
	})
	// An offline user who already learned about v1.8.0 keeps being told: it is
	// still true.
	if latest != "v1.8.0" || !newer {
		t.Fatalf("got (%q, %v), want the previously known (v1.8.0, true)", latest, newer)
	}
}

func TestCheckForUpdateMalformedBodyIsSilent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "{not json")
	}))
	defer srv.Close()
	if _, newer := CheckForUpdate(context.Background(), UpdateOptions{
		Current: "v1.7.0", CachePath: filepath.Join(t.TempDir(), "update.json"),
		APIURL: srv.URL, Now: time.Now(),
	}); newer {
		t.Fatal("malformed JSON must not produce a notice")
	}
}
```

- [ ] **Step 2: verificare che falliscano** — `go test ./internal/service/ -run 'Update|Check' -v` → FAIL.

- [ ] **Step 3: implementare** — aggiungere a `internal/service/update.go`:

```go
// githubLatestReleaseURL is the endpoint asked for the newest release.
//
// It deliberately excludes drafts and prereleases. That is what keeps the
// release flow quiet in the window where a tag is pushed but its notes are
// still a draft: the endpoint keeps returning the previous release. Do not
// swap it for the tags API, which would lose that property.
const githubLatestReleaseURL = "https://api.github.com/repos/marcoarnulfo/clickup-cli/releases/latest"

const updateCheckTimeout = 2 * time.Second

// UpdateCheckEnabled reports whether the update check may run.
// CLUP_NO_UPDATE_CHECK wins over the config; demo mode never checks, because a
// demo session performs no I/O at all.
func UpdateCheckEnabled(cfg config.Config, demo bool) bool {
	if demo {
		return false
	}
	if os.Getenv("CLUP_NO_UPDATE_CHECK") != "" {
		return false
	}
	if cfg.UpdateCheck != nil && !*cfg.UpdateCheck {
		return false
	}
	return true
}

// UpdateOptions configures a check. The zero value of each optional field
// selects the production default; tests fill them in.
type UpdateOptions struct {
	Current   string
	CachePath string
	APIURL    string
	Now       time.Time
}

// CheckForUpdate reports the latest published release and whether it is
// strictly newer than the running version.
//
// It never returns an error: every failure — no cache, a corrupt cache, no
// network, a timeout, a non-200, a malformed body — is silent, because a
// failed update check is not the user's problem.
func CheckForUpdate(ctx context.Context, o UpdateOptions) (string, bool) {
	// A source build has nothing meaningful to compare against, and must not
	// even reach the network.
	if !version.IsRelease(o.Current) {
		return "", false
	}
	now := o.Now
	if now.IsZero() {
		now = time.Now()
	}
	path := o.CachePath
	if path == "" {
		p, err := defaultCachePath()
		if err != nil {
			return "", false
		}
		path = p
	}

	cached, ok := readCache(path)
	if ok && cacheFresh(cached, now) {
		return cached.Latest, version.Newer(o.Current, cached.Latest)
	}

	latest, err := fetchLatestRelease(ctx, o)
	if err != nil || !version.IsRelease(latest) {
		// Keep whatever we knew, but record the attempt so an offline user
		// does not pay the timeout on every invocation.
		latest = cached.Latest
	}
	_ = writeCache(path, updateCache{CheckedAt: now, Latest: latest})
	return latest, version.Newer(o.Current, latest)
}

func fetchLatestRelease(ctx context.Context, o UpdateOptions) (string, error) {
	url := o.APIURL
	if url == "" {
		url = githubLatestReleaseURL
	}
	client := &http.Client{Timeout: updateCheckTimeout}
	ctx, cancel := context.WithTimeout(ctx, updateCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	// GitHub rejects requests without a User-Agent. The call carries no
	// Authorization header: it is anonymous, and the user's ClickUp token has
	// no business travelling to api.github.com.
	req.Header.Set("User-Agent", "clup/"+o.Current)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github returned %s", resp.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", err
	}
	return body.TagName, nil
}
```

Import da aggiungere: `context`, `fmt`, `io`, `net/http`, e `internal/config`.

- [ ] **Step 4: eseguire** — `go test ./internal/service/ -race` → PASS.

- [ ] **Step 5: commit** — `feat(service): update check with opt-out, silent failure and 24h cache (#104)`

---

### Task 6: avviso nella CLI, su stderr

**Files:**
- Modify: `internal/cli/report.go`
- Test: `internal/cli/report_test.go`

**Interfaces:**
- Consumes: `service.CurrentVersion`, `service.UpdateCheckEnabled`, `service.CheckForUpdate`.

- [ ] **Step 1: scrivere i test che falliscono** in `internal/cli/report_test.go`:

```go
func TestReportNoticeGoesToStderrAndStdoutStaysJSON(t *testing.T) {
	// The whole point of putting the notice on stderr: `clup report --json`
	// feeds scripts, and a line on stdout would break every jq downstream.
	// ... avviare il fake server del report già usato in questo file ...
	// ... impostare CLUP_NO_UPDATE_CHECK="" e puntare la cache e l'API di
	//     update a un server di test che risponde v99.0.0 ...
	// ... catturare stdout E stderr con os.Pipe, come già fa
	//     TestReportJSONSchemaGolden per il solo stdout ...

	var parsed map[string]any
	if err := json.Unmarshal(stdout, &parsed); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout)
	}
	if strings.Contains(string(stdout), "is available") {
		t.Error("the update notice leaked into stdout")
	}
	if !strings.Contains(string(stderr), "is available") {
		t.Errorf("no notice on stderr:\n%s", stderr)
	}
	if !strings.Contains(string(stderr), "CLUP_NO_UPDATE_CHECK") {
		t.Error("the notice must tell the user how to turn it off")
	}
}

func TestReportNoNoticeWhenDisabled(t *testing.T) {
	t.Setenv("CLUP_NO_UPDATE_CHECK", "1")
	// ... stesso setup; asserire che stderr NON contenga "is available"
	//     e che il server di update non sia mai stato chiamato ...
}
```

*(Per rendere il controllo pilotabile dai test, `runReport` legge l'endpoint e il percorso di cache da variabili di package non esportate — `updateAPIURL`, `updateCachePath` — vuote in produzione e valorizzate dal test. Non aggiungere flag pubbliche per questo. I test **devono** ripristinarle con `t.Cleanup`, sullo stesso schema dell'helper `withSeams` già presente in `report_test.go`: altrimenti un test lascia il proprio endpoint acceso per quelli successivi del package.)*

- [ ] **Step 2: verificare che falliscano** — `go test ./internal/cli/ -run Notice -v` → FAIL.

- [ ] **Step 3: implementare** — in `internal/cli/report.go`, dentro `runReport`.

**Collocazione esatta:** la chiamata a `startUpdateCheck` va **subito dopo che la config è
stata caricata e validata** (`cfg` non esiste prima di quel punto) e **prima** del recupero
delle entry, così il controllo gira in parallelo alla chiamata API del report. Aggiungere
`context` agli import del file se manca.

```go
	// Start the update check alongside the report fetch. Running it serially
	// would add up to two seconds to a `clup report` once a day.
	updates := startUpdateCheck(cmd.Context(), cfg)

	// ... corpo esistente di runReport, invariato, fino alla scrittura del report ...

	if latest, ok := <-updates; ok {
		fmt.Fprintf(os.Stderr, "\nclup %s is available (you have %s)\n"+
			"  go install github.com/marcoarnulfo/clickup-cli/cmd/clup@latest\n"+
			"  disable: CLUP_NO_UPDATE_CHECK=1\n", latest, service.CurrentVersion())
	}
```

e, in fondo al file:

```go
// updateAPIURL and updateCachePath are empty in production; tests set them to
// point the check at a local server and a temporary cache.
var (
	updateAPIURL    string
	updateCachePath string
)

// startUpdateCheck runs the update check in the background and returns a
// channel yielding the newer version, if any. The channel is always closed, so
// a receive never blocks the command from finishing.
func startUpdateCheck(ctx context.Context, cfg config.Config) <-chan string {
	out := make(chan string, 1)
	if !service.UpdateCheckEnabled(cfg, false) {
		close(out)
		return out
	}
	go func() {
		defer close(out)
		latest, newer := service.CheckForUpdate(ctx, service.UpdateOptions{
			Current:   service.CurrentVersion(),
			APIURL:    updateAPIURL,
			CachePath: updateCachePath,
		})
		if newer {
			out <- latest
		}
	}()
	return out
}
```

*(Il `false` passato a `UpdateCheckEnabled` è la modalità demo, ed è voluto: il percorso headless ignora deliberatamente `CLICKUP_DEMO` — vedi le Global Constraints e `TestReportIgnoresDemoEnv`. La demo riguarda solo la TUI.)*

- [ ] **Step 4: eseguire** — `go test ./internal/cli/ -race` → PASS. Verificare a mano che `go run ./cmd/clup report --json` non stampi nulla di estraneo su stdout.

- [ ] **Step 5: commit** — `feat(cli): report a newer release on stderr, never on stdout (#104)`

---

### Task 7: riga dell'avviso nella TUI

**Files:**
- Modify: `internal/tui/app.go`, `internal/tui/home.go`
- Test: `internal/tui/app_test.go`, `internal/tui/home_test.go`

**Interfaces:**
- Consumes: `service.UpdateCheckEnabled`, `service.CheckForUpdate`, `service.CurrentVersion`.
- Produces: `updateAvailableMsg`, campo `Model.latestVersion string`.

- [ ] **Step 1: scrivere i test che falliscono**:

```go
// internal/tui/app_test.go
func TestUpdateAvailableMsgSetsLatestVersion(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1"})
	next, _ := m.Update(updateAvailableMsg{latest: "v1.8.0"})
	got := next.(Model)
	if got.latestVersion != "v1.8.0" {
		t.Fatalf("latestVersion = %q, want v1.8.0", got.latestVersion)
	}
	if got.screen == screenError {
		t.Fatal("an update notice must never route to the error screen")
	}
}

func TestInitDoesNotCheckInDemoMode(t *testing.T) {
	m := New(config.Config{})
	m.demo = true
	if cmd := m.Init(); cmd != nil {
		t.Fatal("demo mode must issue no commands: the demo performs zero I/O")
	}
}

// internal/tui/home_test.go
func TestHomeShowsUpdateNotice(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1"})
	m.latestVersion = "v1.8.0"
	out := m.View()
	if !strings.Contains(out, "v1.8.0") {
		t.Fatalf("home view does not mention the new version:\n%s", out)
	}
}
```

- [ ] **Step 2: verificare che falliscano** — `go test ./internal/tui/ -run 'Update|Init|Notice' -v` → FAIL.

- [ ] **Step 3: implementare**.

Nota sulla view della home: la firma reale è
`func (m homeModel) view(rangeLabel, scope, membersNote string) string` — **non** riceve il
Model radice. Passare la versione come quarto argomento da `Model.View()`, dove lo stato
vive, seguendo lo schema già usato per gli altri dati mostrati in home.

In `app.go`, il msg tipizzato accanto agli altri:

```go
// updateAvailableMsg carries a newer published release. It is only ever sent
// when one exists: unlike every other command in this program, the update
// check never reports its failures — it emits no errMsg and never routes to
// screenError, because a failed update check is not the user's problem.
type updateAvailableMsg struct{ latest string }
```

Il campo sul Model radice (dove vive lo stato vero), accanto agli altri campi di stato:

```go
	latestVersion string // newer published release, "" when up to date or unknown
```

Il comando, guardato sulla demo:

```go
func (m Model) updateCheckCmd() tea.Cmd {
	if !service.UpdateCheckEnabled(m.cfg, m.demo) {
		return nil
	}
	return func() tea.Msg {
		latest, newer := service.CheckForUpdate(context.Background(), service.UpdateOptions{
			Current: service.CurrentVersion(),
		})
		if !newer {
			return nil
		}
		return updateAvailableMsg{latest: latest}
	}
}
```

`Init` (oggi ritorna `nil`) lo lancia:

```go
func (m Model) Init() tea.Cmd {
	return m.updateCheckCmd()
}
```

Il case nel type switch di `Update`:

```go
	case updateAvailableMsg:
		m.latestVersion = msg.latest
		return m, nil
```

In `home.go`, la riga con gli stili già esistenti di `styles.go` (nessuno stile nuovo):

```go
	if m.latestVersion != "" {
		b.WriteString(styleHelp.Render(fmt.Sprintf("clup %s available — go install …/cmd/clup@latest", m.latestVersion)) + "\n")
	}
```

*(Adattare l'accesso al campo alla firma reale della view della home: se riceve il Model radice usarlo direttamente, altrimenti passare la stringa come argomento, seguendo lo schema già usato dagli altri dati mostrati in home.)*

- [ ] **Step 4: eseguire** — `go test ./internal/tui/ -race` → PASS.

- [ ] **Step 5: commit** — `feat(tui): discreet update notice on the home screen (#104)`

---

### Task 8: documentazione bilingue

**Files:**
- Modify: `README.md`, `README.it.md`

- [ ] **Step 1: scrivere la sezione** in `README.md`, vicino alla configurazione:

Contenuto obbligatorio, in inglese: che `clup` controlla **una volta al giorno** se esiste una release più recente; che la chiamata è **anonima**, va a `api.github.com`, **non manda mai il token** e **non scarica nulla** (non esiste self-update); che il risultato è in cache in `os.UserCacheDir()/clup/update.json`; che si disattiva con `CLUP_NO_UPDATE_CHECK=1` **oppure** con `update_check: false` nel config; e che chi ha compilato da sorgente non riceve avvisi.

- [ ] **Step 2: rispecchiare in `README.it.md`** — stessa posizione, stessa struttura, prosa italiana. Chiavi di config, flag e nomi di variabili restano identici.

- [ ] **Step 3: verificare** — `go build ./...` e `go test ./... -race` (nessun codice cambia; è la rete di sicurezza).

- [ ] **Step 4: commit** — `docs: document the update check and how to disable it (#104)`

---

## Self-Review (autore)

- **Copertura della spec:** §3 architettura → T1/T4/T5; §4 versione corrente → T1/T2; §4.1 regola positiva → T1 (test sulle pseudo-version e su `+dirty`); §5.1 cache → T4; §5.2 freschezza e cache corrotta → T4; §5.3 chiamata, header, endpoint → T5; §5.4 scrittura atomica e stamp sul fallimento → T4/T5; §5.5 silenzio → T5; §6 strettamente più recente → T1/T5; §7 opt-out → T3/T5; §8 testo, collocazione, latenza, TUI, semantica d'errore → T6/T7; §9 test → in ogni task; §10 docs → T8. ✓
- **Coerenza dei tipi:** `UpdateOptions`, `CheckForUpdate`, `UpdateCheckEnabled`, `CurrentVersion`, `updateCache`, `updateAvailableMsg` usati con la stessa firma in tutti i task. ✓
- **Segnaposto:** nessuno. Nei tre punti in cui il codice dipende da come è scritto oggi il repo — il nome del costruttore del comando radice (T2), l'helper del percorso di config nei test (T3), la firma della view della home (T7) — il piano dice esplicitamente di allinearsi all'esistente invece di inventare un secondo modo.
- **YAGNI:** niente ordinamento dei prerelease, nessuna dipendenza nuova, nessun comando `clup version --check`, nessun self-update.
