# Tariffe per lista (v1.1) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Aggiungere tariffe orarie per lista/progetto ClickUp (con fallback sulla tariffa di default), con nomi lista leggibili e una schermata TUI per assegnarle, aperta col tasto `p`.

**Architecture:** La logica pura (`report`) guadagna un tipo `Rates` (default + override per lista) e calcola l'importo **per entry** poi lo somma nei bucket; il client `clickup` risolve i nomi lista con cache; la TUI aggiunge una schermata tariffe che salva in config e ricalcola il report.

**Tech Stack:** Go 1.26 · Charm (bubbletea/bubbles/lipgloss) · net/http · yaml.v3. Nessuna nuova dipendenza esterna.

## Global Constraints

- **Go 1.26+** (go1.26.5). Module `github.com/marcoarnulfo/clickup-cli`. Binario `clickup`. Licenza MIT.
- **Nessuna nuova dipendenza esterna** oltre a quelle già presenti.
- **ClickUp API base** `https://api.clickup.com/api/v2`; auth header `Authorization: <token>` (no `Bearer`).
- **Retrocompatibilità:** una config v1.0 senza `rates` deve comportarsi esattamente come oggi (tutto alla tariffa `rate`).
- **`report` resta puro:** solo stdlib, nessun import di `config`/`clickup`. La conversione `config → report.Rates` vive in `tui`.
- **Tasto `p` documentato** (requisito esplicito): nella riga di help della vista report, nella schermata tariffe, e nel README.
- **Cambio di arrotondamento intenzionale:** l'importo passa da `round2(oreArrotondate × rate)` a `round2(Σ oreReali × tariffaLista)` (più accurato). Il test `TestRoundingTwoDecimals` va aggiornato (9.9 → 10.0).
- **TDD** su `config`, `report`, `clickup`; TUI testata via `Update()`.
- **Commit frequenti**, Conventional Commits in inglese. **Mai** `Co-Authored-By` né "Generated with" nei messaggi di commit.

---

## File Structure

```
internal/
  config/config.go            # + campo Rates map[string]float64
  config/config_test.go       # + round-trip di Rates
  report/model.go             # + tipo Rates{Default, ByList} + metodo For
  report/aggregate.go         # Build(rates Rates), importo per-entry, Report.Rate = Default
  report/aggregate_test.go    # firma Build aggiornata + test per-lista/misto + rounding
  clickup/client.go           # Client + cache listNames (mutex); New la inizializza
  clickup/list.go             # NEW: ListName(ctx, id) con cache + fallback
  clickup/clickup_test.go     # + test ListName (risoluzione, cache, fallback)
  tui/app.go                  # + screenRates, ratesScreen field, routing/view, ratesFromConfig, name-resolution in loadEntriesCmd
  tui/report.go               # tasto 'p' -> screenRates; help line con "p: tariffe"
  tui/rates.go                # NEW: ratesModel (schermata tariffe) + updateRates
  tui/app_test.go             # + test wiring/schermata
README.md                     # + "p: tariffe" e sezione config rates
```

---

## Task 1: `config` — campo `Rates`

**Files:**
- Modify: `internal/config/config.go`, `internal/config/config_test.go`

**Interfaces:**
- Produces: `Config.Rates map[string]float64` (yaml `rates,omitempty`) — mappa `list_id → tariffa`.

- [ ] **Step 1: Aggiorna il test round-trip (RED)**

In `internal/config/config_test.go`, sostituisci il corpo di `TestSaveThenLoadRoundTrip` con una versione che include `Rates`:

```go
func TestSaveThenLoadRoundTrip(t *testing.T) {
	isolateConfig(t)
	want := Config{
		Token: "tok_123", WorkspaceID: "900", Currency: "EUR", Rate: 45,
		Rates: map[string]float64{"111": 60, "222": 30},
	}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Token != want.Token || got.WorkspaceID != want.WorkspaceID ||
		got.Currency != want.Currency || got.Rate != want.Rate {
		t.Fatalf("scalar round-trip mismatch: got %+v want %+v", got, want)
	}
	if len(got.Rates) != 2 || got.Rates["111"] != 60 || got.Rates["222"] != 30 {
		t.Fatalf("rates round-trip mismatch: got %+v", got.Rates)
	}
}
```

(La vecchia versione confrontava `got != want`, che non compila con un campo map: va sostituita con i confronti campo-per-campo qui sopra.)

- [ ] **Step 2: Esegui (RED)**

Run: `go test ./internal/config/ -run TestSaveThenLoadRoundTrip -v`
Expected: FAIL a compilazione (`Config` non ha campo `Rates`) o assertion.

- [ ] **Step 3: Aggiungi il campo `Rates`**

In `internal/config/config.go`, aggiorna lo struct:

```go
type Config struct {
	Token       string             `yaml:"token"`
	WorkspaceID string             `yaml:"workspace_id"`
	Currency    string             `yaml:"currency"`
	Rate        float64            `yaml:"rate"`
	Rates       map[string]float64 `yaml:"rates,omitempty"` // list_id -> tariffa override
}
```

`Valid()`, `Load()`, `Save()`, `Path()` restano invariati (yaml gestisce la map automaticamente).

- [ ] **Step 4: Esegui (GREEN)**

Run: `go test ./internal/config/ -v`
Expected: PASS (tutti). `Config` con un campo map non è più comparabile con `==`, ma nessun altro test lo confronta così (verificato: gli altri usano confronti scalari o `Config{}` vuoto — `got != (Config{})` **non** compila più con una map).

⚠️ **Attenzione:** `TestLoadMissingFileReturnsEmpty` fa `got != (Config{})`. Con l'aggiunta della map, `Config` non è più comparabile e questo test **non compila**. Sostituisci quella riga con un controllo esplicito:

```go
	if got.Token != "" || got.WorkspaceID != "" || got.Currency != "" || got.Rate != 0 || got.Rates != nil {
		t.Fatalf("expected zero Config, got %+v", got)
	}
```

Applica anche questa modifica nello Step 3 (stesso commit), poi ri-esegui lo Step 4.

- [ ] **Step 5: Commit**

```bash
git add internal/config
git commit -m "feat: add per-list Rates map to config"
```

---

## Task 2: `report` — tipo `Rates`, `Build(rates)`, importo per-entry

**Files:**
- Modify: `internal/report/model.go`, `internal/report/aggregate.go`, `internal/report/aggregate_test.go`
- Modify (call sites): `internal/tui/app.go`, `internal/tui/report.go`

**Interfaces:**
- Consumes: `config.Config.Rates` (via il nuovo helper `tui.ratesFromConfig`).
- Produces:
  - `type Rates struct { Default float64; ByList map[string]float64 }` con `func (r Rates) For(listID string) float64`.
  - `func Build(entries []TimeEntry, groupBy string, rates Rates, currency string, year int, month time.Month) Report` (era `rate float64`).
  - `Report.Rate float64` = tariffa di default (valorizzato da `Build` con `rates.Default`).
  - `func ratesFromConfig(cfg config.Config) report.Rates` (in `internal/tui/app.go`).

- [ ] **Step 1: Aggiorna i test di `report` (RED)**

In `internal/report/aggregate_test.go`:

(a) Aggiorna **ogni** chiamata a `Build`: il 3° argomento passa da uno scalare a `Rates{Default: <scalare>}`. Precisamente:
- `Build(sampleEntries(), GroupByTotal, 50, ...)` → `Build(sampleEntries(), GroupByTotal, Rates{Default: 50}, ...)`
- `Build(sampleEntries(), GroupByTask, 0, ...)` → `Build(sampleEntries(), GroupByTask, Rates{Default: 0}, ...)`
- idem per `TestBuildByList`, `TestBuildByDayChronological` (rate 0 → `Rates{Default: 0}`)
- `Build(nil, GroupByTask, 50, ...)` → `Build(nil, GroupByTask, Rates{Default: 50}, ...)`
- In `TestRoundingTwoDecimals`: `Build(e, GroupByTask, 30, ...)` → `Build(e, GroupByTask, Rates{Default: 30}, ...)`

(b) In `TestRoundingTwoDecimals` aggiorna l'atteso dell'importo: con il calcolo per-entry, `1/3 h × 30 = 10.00` (esatto), non `9.90`. Cambia:

```go
	if r.TotalAmount != 9.9 { // 0.33 * 30
		t.Fatalf("amount should be 9.9, got %v", r.TotalAmount)
	}
```

in:

```go
	if r.TotalAmount != 10 { // 1/3 h * 30 = 10.00 esatto (importo per-entry da ore reali)
		t.Fatalf("amount should be 10, got %v", r.TotalAmount)
	}
```

(c) Aggiungi i nuovi test per le tariffe per lista e la tariffa mista:

```go
func TestRatesFor(t *testing.T) {
	r := Rates{Default: 30, ByList: map[string]float64{"1": 50}}
	if r.For("1") != 50 {
		t.Fatalf("override lista 1 dovrebbe essere 50, got %v", r.For("1"))
	}
	if r.For("999") != 30 {
		t.Fatalf("lista senza override dovrebbe usare default 30, got %v", r.For("999"))
	}
}

func TestBuildPerListRates(t *testing.T) {
	base := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	entries := []TimeEntry{
		{TaskName: "A", ListID: "1", ListName: "Cliente A", Start: base, Duration: d(2)},
		{TaskName: "B", ListID: "2", ListName: "Cliente B", Start: base, Duration: d(1)},
	}
	rates := Rates{Default: 30, ByList: map[string]float64{"1": 50}}
	r := Build(entries, GroupByList, rates, "EUR", 2026, time.July)
	amt := map[string]float64{}
	for _, b := range r.Buckets {
		amt[b.Label] = b.Amount
	}
	if amt["Cliente A"] != 100 { // 2h * 50
		t.Fatalf("Cliente A amount = %v, want 100", amt["Cliente A"])
	}
	if amt["Cliente B"] != 30 { // 1h * 30 (default)
		t.Fatalf("Cliente B amount = %v, want 30", amt["Cliente B"])
	}
	if r.TotalAmount != 130 {
		t.Fatalf("total amount = %v, want 130", r.TotalAmount)
	}
	if r.Rate != 30 {
		t.Fatalf("Report.Rate should carry the default rate, got %v", r.Rate)
	}
}

func TestBuildMixedRatePerTask(t *testing.T) {
	base := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	// stessa task, due liste con tariffe diverse
	entries := []TimeEntry{
		{TaskName: "X", ListID: "1", Start: base, Duration: d(2)},
		{TaskName: "X", ListID: "2", Start: base, Duration: d(1)},
	}
	rates := Rates{Default: 0, ByList: map[string]float64{"1": 50, "2": 30}}
	r := Build(entries, GroupByTask, rates, "EUR", 2026, time.July)
	if len(r.Buckets) != 1 {
		t.Fatalf("want 1 task bucket, got %d", len(r.Buckets))
	}
	if r.Buckets[0].Hours != 3 {
		t.Fatalf("hours = %v, want 3", r.Buckets[0].Hours)
	}
	if r.Buckets[0].Amount != 130 { // 2*50 + 1*30
		t.Fatalf("mixed-rate amount = %v, want 130", r.Buckets[0].Amount)
	}
}
```

- [ ] **Step 2: Esegui (RED)**

Run: `go test ./internal/report/ -v`
Expected: FAIL a compilazione (`Rates` non definito; `Build` firma vecchia).

- [ ] **Step 3: Aggiungi il tipo `Rates` in `model.go`**

In `internal/report/model.go`, aggiungi in fondo:

```go
// Rates risolve la tariffa oraria per lista, con fallback sulla tariffa di default.
type Rates struct {
	Default float64
	ByList  map[string]float64
}

// For ritorna la tariffa della lista se presente, altrimenti la tariffa di default.
func (r Rates) For(listID string) float64 {
	if v, ok := r.ByList[listID]; ok {
		return v
	}
	return r.Default
}
```

- [ ] **Step 4: Aggiorna `Build` in `aggregate.go`**

In `internal/report/aggregate.go`, sostituisci l'intera funzione `Build` con:

```go
// Build aggrega le entry in un Report secondo groupBy. L'importo di ogni bucket
// è la somma, sulle entry del bucket, di ore_reali × tariffa_della_lista (Rates.For),
// arrotondata a 2 decimali. Report.Rate riporta la tariffa di default (per l'export).
func Build(entries []TimeEntry, groupBy string, rates Rates, currency string, year int, month time.Month) Report {
	r := Report{
		Year:     year,
		Month:    month,
		GroupBy:  groupBy,
		Currency: currency,
		Rate:     rates.Default,
	}

	sumsH := map[string]float64{}
	sumsA := map[string]float64{}
	var order []string
	for _, e := range entries {
		k := keyFor(e, groupBy)
		if _, seen := sumsH[k]; !seen {
			order = append(order, k)
		}
		h := e.Duration.Hours()
		sumsH[k] += h
		sumsA[k] += h * rates.For(e.ListID)
	}

	for _, k := range order {
		r.Buckets = append(r.Buckets, Bucket{
			Label:  k,
			Hours:  round2(sumsH[k]),
			Amount: round2(sumsA[k]),
		})
	}

	// Ordinamento: per giorno cronologico (label asc); altrimenti ore desc, tie label asc.
	sort.SliceStable(r.Buckets, func(i, j int) bool {
		a, b := r.Buckets[i], r.Buckets[j]
		if groupBy == GroupByDay {
			return a.Label < b.Label
		}
		if a.Hours != b.Hours {
			return a.Hours > b.Hours
		}
		return a.Label < b.Label
	})

	var th, ta float64
	for _, b := range r.Buckets {
		th += b.Hours
		ta += b.Amount
	}
	r.TotalHours = round2(th)
	r.TotalAmount = round2(ta)
	return r
}
```

- [ ] **Step 5: Esegui i test di report (GREEN)**

Run: `go test ./internal/report/ -v`
Expected: PASS (inclusi i nuovi `TestRatesFor`, `TestBuildPerListRates`, `TestBuildMixedRatePerTask`).

- [ ] **Step 6: Aggiorna i call site in `tui` + helper `ratesFromConfig`**

Il package `tui` non compila più (chiama `Build` con firma vecchia). In `internal/tui/app.go` aggiungi il helper (vicino a `loadEntriesCmd`):

```go
// ratesFromConfig costruisce le tariffe per il report dalla config (default + override).
func ratesFromConfig(cfg config.Config) report.Rates {
	return report.Rates{Default: cfg.Rate, ByList: cfg.Rates}
}
```

Poi aggiorna i **due** call site di `Build`:

In `internal/tui/app.go` (case `entriesMsg`):

```go
		m.report = report.Build(msg.entries, groupBy, ratesFromConfig(m.cfg), m.cfg.Currency, m.year, m.month)
```

In `internal/tui/report.go` (case `"g"`):

```go
		m.report = report.Build(m.entries, g, ratesFromConfig(m.cfg), m.cfg.Currency, m.year, m.month)
```

- [ ] **Step 7: Esegui l'intera suite + build (GREEN)**

Run: `go test ./... && go build ./...`
Expected: PASS su tutti i package + build ok. (I test TUI esistenti come `TestReportCycleGroupBy` continuano a passare: verificano il `GroupBy`, non gli importi.)

- [ ] **Step 8: Commit**

```bash
git add internal/report internal/tui/app.go internal/tui/report.go
git commit -m "feat: per-list rate resolution in report.Build (amount computed per entry)"
```

---

## Task 3: `clickup` — `ListName` con cache

**Files:**
- Modify: `internal/clickup/client.go`
- Create: `internal/clickup/list.go`
- Modify: `internal/clickup/clickup_test.go`

**Interfaces:**
- Produces: `func (c *Client) ListName(ctx context.Context, listID string) (string, error)` — `GET /list/{id}`, cache in-memory sul Client (una chiamata per id unico), fallback all'id in caso d'errore.

- [ ] **Step 1: Scrivi i test (RED)**

In `internal/clickup/clickup_test.go`, aggiungi:

```go
func TestListNameResolvesAndCaches(t *testing.T) {
	var calls atomic.Int32
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Write([]byte(`{"id":"l1","name":"Cliente A"}`))
	})
	defer srv.Close()

	n, err := c.ListName(context.Background(), "l1")
	if err != nil || n != "Cliente A" {
		t.Fatalf("got %q err %v", n, err)
	}
	// seconda chiamata: servita dalla cache, nessuna HTTP aggiuntiva
	n2, _ := c.ListName(context.Background(), "l1")
	if n2 != "Cliente A" {
		t.Fatalf("cached name wrong: %q", n2)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 HTTP call (cached), got %d", calls.Load())
	}
}

func TestListNameFallbackOnError(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"err":"boom","ECODE":"X"}`))
	})
	defer srv.Close()
	n, err := c.ListName(context.Background(), "l9")
	if err == nil {
		t.Fatal("expected error on 500")
	}
	if n != "l9" {
		t.Fatalf("fallback should be the id, got %q", n)
	}
}
```

- [ ] **Step 2: Esegui (RED)**

Run: `go test ./internal/clickup/ -run TestListName -v`
Expected: FAIL (`ListName` non definito).

- [ ] **Step 3: Aggiungi cache al `Client` (`client.go`)**

In `internal/clickup/client.go`, aggiungi `sync` all'import e i campi cache allo struct, e inizializza in `New`:

```go
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)
```

```go
// Client interroga la ClickUp API v2.
type Client struct {
	token   string
	BaseURL string
	http    *http.Client

	mu        sync.Mutex        // protegge listNames (usata da comandi in goroutine)
	listNames map[string]string // cache list_id -> nome
}

// New crea un client con il token personale.
func New(token string) *Client {
	return &Client{
		token:     token,
		BaseURL:   "https://api.clickup.com/api/v2",
		http:      &http.Client{Timeout: 30 * time.Second},
		listNames: make(map[string]string),
	}
}
```

- [ ] **Step 4: Implementa `list.go`**

Create `internal/clickup/list.go`:

```go
package clickup

import "context"

// ListName risolve il nome di una lista (GET /list/{id}) con cache in-memory.
// In caso d'errore ritorna il list_id come fallback (insieme all'errore).
func (c *Client) ListName(ctx context.Context, listID string) (string, error) {
	c.mu.Lock()
	name, ok := c.listNames[listID]
	c.mu.Unlock()
	if ok {
		return name, nil
	}

	var resp struct {
		Name string `json:"name"`
	}
	if err := c.get(ctx, "/list/"+listID, nil, &resp); err != nil {
		return listID, err
	}

	c.mu.Lock()
	c.listNames[listID] = resp.Name
	c.mu.Unlock()
	return resp.Name, nil
}
```

- [ ] **Step 5: Esegui (GREEN)**

Run: `go test ./internal/clickup/ -v`
Expected: PASS (inclusi i due nuovi test).

- [ ] **Step 6: Commit**

```bash
git add internal/clickup
git commit -m "feat: resolve ClickUp list names via GET /list/{id} with cache"
```

---

## Task 4: `tui` — risoluzione nomi lista in `loadEntriesCmd`

**Files:**
- Modify: `internal/tui/app.go`, `internal/tui/app_test.go`

**Interfaces:**
- Consumes: `clickup.Client.ListName`.
- Produces: dopo `loadEntriesCmd`, ogni `report.TimeEntry.ListName` è il nome leggibile (fallback: il `list_id`).

- [ ] **Step 1: Scrivi il test (RED)**

In `internal/tui/app_test.go`, assicura l'import di `strings` e aggiungi:

```go
func TestLoadEntriesResolvesListNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/time_entries"):
			w.Write([]byte(`{"data":[{"id":"e1","task":{"id":"t","name":"T"},"task_location":{"list_id":"55"},"user":{"id":1,"username":"x"},"start":"1751360400000","duration":"3600000"}]}`))
		case strings.Contains(r.URL.Path, "/list/"):
			w.Write([]byte(`{"id":"55","name":"Cliente Z"}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	c := clickup.New("t")
	c.BaseURL = srv.URL

	msg := loadEntriesCmd(c, "900", 2026, time.July, "me")()
	em, ok := msg.(entriesMsg)
	if !ok {
		t.Fatalf("expected entriesMsg, got %T", msg)
	}
	if len(em.entries) != 1 || em.entries[0].ListName != "Cliente Z" {
		t.Fatalf("list name not resolved: %+v", em.entries)
	}
}
```

- [ ] **Step 2: Esegui (RED)**

Run: `go test ./internal/tui/ -run TestLoadEntriesResolvesListNames -v`
Expected: FAIL (`ListName` è ancora il `list_id` "55", non "Cliente Z").

- [ ] **Step 3: Risolvi i nomi in `loadEntriesCmd`**

In `internal/tui/app.go`, nel corpo di `loadEntriesCmd`, sostituisci il blocco finale:

```go
		start, end := report.MonthRange(year, month)
		entries, err := c.TimeEntries(ctx, teamID, start, end, assignees)
		if err != nil {
			return errMsg{err: err}
		}
		return entriesMsg{entries: entries}
```

con:

```go
		start, end := report.MonthRange(year, month)
		entries, err := c.TimeEntries(ctx, teamID, start, end, assignees)
		if err != nil {
			return errMsg{err: err}
		}
		// Risolvi i nomi leggibili delle liste (cache sul client deduplica le chiamate).
		for i := range entries {
			if entries[i].ListID != "" {
				if name, err := c.ListName(ctx, entries[i].ListID); err == nil {
					entries[i].ListName = name
				}
			}
		}
		return entriesMsg{entries: entries}
```

(In caso di errore su `ListName`, si mantiene il valore già presente — che `TimeEntries` ha impostato al `list_id` — quindi nessun blocco del report.)

- [ ] **Step 4: Esegui (GREEN) + build**

Run: `go test ./internal/tui/ -v && go build ./...`
Expected: PASS + build ok.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat: resolve readable list names when loading time entries"
```

---

## Task 5: `tui` — schermata tariffe (tasto `p`)

**Files:**
- Create: `internal/tui/rates.go`
- Modify: `internal/tui/app.go` (enum `screenRates`, campo `ratesScreen`, routing, view), `internal/tui/report.go` (tasto `p`, help line)
- Modify: `internal/tui/app_test.go`

**Interfaces:**
- Consumes: `report.TimeEntry`, `config.Config`, `report.Build`, `ratesFromConfig`.
- Produces:
  - `type ratesModel struct { ... }`, `func newRates(entries []report.TimeEntry, cfg config.Config) ratesModel`, `func (rt ratesModel) view() string`.
  - `func (m Model) updateRates(msg tea.KeyMsg) (tea.Model, tea.Cmd)`.
  - `Model.ratesScreen ratesModel`; `screen` costante `screenRates`.

- [ ] **Step 1: Scrivi i test (RED)**

In `internal/tui/app_test.go` aggiungi:

```go
func TestRatesScreenOpensFromReport(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1", Rate: 30, Currency: "EUR"})
	m.year, m.month = 2026, 7
	entries := []report.TimeEntry{
		{ListID: "55", ListName: "Cliente Z", Start: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), Duration: time.Hour},
	}
	u, _ := m.Update(entriesMsg{entries: entries})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = u.(Model)
	if m.screen != screenRates {
		t.Fatalf("p dalla vista report deve aprire screenRates, got %v", m.screen)
	}
	if len(m.ratesScreen.rows) != 1 || m.ratesScreen.rows[0].name != "Cliente Z" {
		t.Fatalf("righe tariffe errate: %+v", m.ratesScreen.rows)
	}
}

func TestRatesScreenEditSaveRecomputes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("CLICKUP_TOKEN", "")

	m := New(config.Config{Token: "t", WorkspaceID: "1", Rate: 30, Currency: "EUR"})
	m.year, m.month = 2026, 7
	entries := []report.TimeEntry{
		{ListID: "55", ListName: "Z", Start: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), Duration: 2 * time.Hour},
	}
	u, _ := m.Update(entriesMsg{entries: entries})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = u.(Model)
	// Enter -> editing riga 0
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if !m.ratesScreen.editing {
		t.Fatal("dovrebbe essere in editing")
	}
	// digita "50"
	for _, r := range "50" {
		u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = u.(Model)
	}
	// Enter conferma il valore
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	// 's' salva e ricalcola
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = u.(Model)
	if m.screen != screenReport {
		t.Fatalf("salvataggio deve tornare al report, got %v", m.screen)
	}
	if m.cfg.Rates["55"] != 50 {
		t.Fatalf("override non salvato: %+v", m.cfg.Rates)
	}
	if m.report.TotalAmount != 100 { // 2h * 50
		t.Fatalf("report non ricalcolato: TotalAmount %v, want 100", m.report.TotalAmount)
	}
}
```

- [ ] **Step 2: Esegui (RED)**

Run: `go test ./internal/tui/ -run TestRatesScreen -v`
Expected: FAIL (`screenRates`, `ratesScreen`, `newRates` non definiti).

- [ ] **Step 3: Aggiungi `screenRates`, il campo e il routing in `app.go`**

In `internal/tui/app.go`:

(a) nell'enum `screen`, aggiungi `screenRates` prima di `screenError`:

```go
const (
	screenSetup screen = iota
	screenHome
	screenLoading
	screenReport
	screenExport
	screenRates
	screenError
)
```

(b) aggiungi il campo al `Model` (vicino agli altri sotto-modelli `setup/home/rep/export`):

```go
	ratesScreen ratesModel
```

(c) in `routeKey`, aggiungi il case:

```go
	case screenRates:
		return m.updateRates(msg)
```

(d) in `View`, aggiungi il case:

```go
	case screenRates:
		return m.ratesScreen.view()
```

- [ ] **Step 4: Implementa `rates.go`**

Create `internal/tui/rates.go`:

```go
package tui

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// rateRow è una lista mostrata nella schermata tariffe.
type rateRow struct {
	listID string
	name   string
}

type ratesModel struct {
	rows    []rateRow
	idx     int
	editing bool
	input   textinput.Model
	rates   map[string]float64 // override correnti (list_id -> tariffa)
	def     float64            // tariffa di default
	cur     string             // valuta
}

// newRates costruisce la schermata dalle liste del report corrente unite a quelle
// già presenti in config (cfg.Rates).
func newRates(entries []report.TimeEntry, cfg config.Config) ratesModel {
	names := map[string]string{}
	var order []string
	remember := func(id, name string) {
		if id == "" {
			return
		}
		if _, ok := names[id]; !ok {
			order = append(order, id)
			names[id] = id // etichetta di default = id
		}
		if name != "" {
			names[id] = name
		}
	}
	for _, e := range entries {
		remember(e.ListID, e.ListName)
	}
	for id := range cfg.Rates {
		remember(id, "")
	}

	rows := make([]rateRow, len(order))
	for i, id := range order {
		rows[i] = rateRow{listID: id, name: names[id]}
	}
	rates := map[string]float64{}
	for k, v := range cfg.Rates {
		rates[k] = v
	}
	return ratesModel{rows: rows, rates: rates, def: cfg.Rate, cur: cfg.Currency}
}

func (m Model) updateRates(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rt := m.ratesScreen

	if rt.editing {
		switch msg.Type {
		case tea.KeyEnter:
			v := rt.input.Value()
			if v == "" {
				delete(rt.rates, rt.rows[rt.idx].listID) // vuoto = torna al default
				rt.editing = false
			} else if f, err := strconv.ParseFloat(v, 64); err == nil {
				rt.rates[rt.rows[rt.idx].listID] = f
				rt.editing = false
			}
			// se non valido, resta in editing (l'utente corregge)
			m.ratesScreen = rt
			return m, nil
		case tea.KeyEsc:
			rt.editing = false
			m.ratesScreen = rt
			return m, nil
		}
		var cmd tea.Cmd
		rt.input, cmd = rt.input.Update(msg)
		m.ratesScreen = rt
		return m, cmd
	}

	switch msg.String() {
	case "up", "k":
		if rt.idx > 0 {
			rt.idx--
		}
	case "down", "j":
		if rt.idx < len(rt.rows)-1 {
			rt.idx++
		}
	case "enter":
		if len(rt.rows) > 0 {
			ti := newNumberInput("tariffa (vuoto = usa default)")
			rt.editing = true
			rt.input = ti
		}
	case "s", "esc":
		m.cfg.Rates = rt.rates
		_ = config.Save(m.cfg)
		g := m.report.GroupBy
		if g == "" {
			g = report.GroupByTotal
		}
		m.report = report.Build(m.entries, g, ratesFromConfig(m.cfg), m.cfg.Currency, m.year, m.month)
		m.report.Scope = m.scope
		m.rep = newReport(m.report)
		m.screen = screenReport
		m.ratesScreen = rt
		return m, nil
	}
	m.ratesScreen = rt
	return m, nil
}

func (rt ratesModel) view() string {
	b := styleTitle.Render("Tariffe per lista") + "\n\n"
	if len(rt.rows) == 0 {
		b += styleHelp.Render("Nessuna lista nel report corrente.") + "\n"
	}
	for i, row := range rt.rows {
		rate := rt.def
		tag := "(default)"
		if v, ok := rt.rates[row.listID]; ok {
			rate = v
			tag = "(override)"
		}
		cursor := "  "
		line := fmt.Sprintf("%-28s %8.2f %s %s", truncate(row.name, 28), rate, rt.cur, tag)
		if i == rt.idx {
			cursor = "▸ "
			line = styleAccent.Render(line)
		}
		b += cursor + line + "\n"
	}
	if rt.editing {
		b += "\n" + rt.input.View()
	}
	b += "\n\n" + styleHelp.Render("↑/↓ scegli · Enter: modifica tariffa · s/Esc: salva e torna · (vuoto = usa default)")
	return b
}
```

- [ ] **Step 5: Aggiungi il tasto `p` e la help line in `report.go`**

In `internal/tui/report.go`, nel `switch msg.String()` di `updateReport`, aggiungi il case:

```go
	case "p":
		m.ratesScreen = newRates(m.entries, m.cfg)
		m.screen = screenRates
```

e aggiorna la help line della `view()` per **documentare** il tasto `p`:

```go
	help := styleHelp.Render("g: raggruppamento · e: esporta · p: tariffe · m/s: cambia mese/scope · r: ricarica · q: esci")
```

- [ ] **Step 6: Esegui i test + build (GREEN)**

Run: `go test ./... && go build ./...`
Expected: PASS su tutti i package + build ok.

- [ ] **Step 7: Commit**

```bash
git add internal/tui
git commit -m "feat: TUI per-list rates screen (key 'p'), saves config and recomputes report"
```

---

## Task 6: README + verifica finale

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Aggiorna il README**

In `README.md`:

(a) nella sezione "Comandi nella TUI", aggiungi la riga per il tasto `p`, ad es.:

```markdown
| `p` | (nella vista report) apre la schermata **Tariffe per lista** |
```

(b) nella schermata tariffe, documenta i tasti `↑/↓`, `Enter`, `s`/`Esc`.

(c) nella sezione "Configurazione", documenta la mappa `rates`:

```markdown
`rates` è opzionale: una mappa `list_id: tariffa` con le tariffe orarie specifiche
per lista. Le liste non elencate usano la tariffa di default `rate`. La si compila
comodamente dalla TUI premendo `p` nella vista report.

Esempio:

```yaml
token: pk_xxx
workspace_id: "900"
currency: EUR
rate: 45
rates:
  "111": 60
  "222": 30
```
```

- [ ] **Step 2: Formattazione e vet**

Run: `gofmt -l . && go vet ./...`
Expected: nessun file da `gofmt`, nessun warning. Se `gofmt -l` elenca file: `gofmt -w .`.

- [ ] **Step 3: Suite completa con race**

Run: `go test ./... -race`
Expected: PASS su tutti i package.

- [ ] **Step 4: Verifica build binario**

Run: `go build ./cmd/clickup && echo ok`
Expected: `ok`. (L'e2e interattivo con token reale resta a carico dell'utente: `CLICKUP_TOKEN=pk_xxx go run ./cmd/clickup`, poi `p` per le tariffe.)

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "docs: document per-list rates and the 'p' key in the TUI"
```

---

## Self-Review (svolto in fase di scrittura del piano)

**1. Copertura spec:**
- Tariffe per lista + fallback → Task 1 (config) + Task 2 (`Rates.For`, `Build`).
- Importo corretto sui raggruppamenti misti → Task 2 (`TestBuildMixedRatePerTask`).
- Nomi lista leggibili via API con cache → Task 3 (`ListName`) + Task 4 (risoluzione in `loadEntriesCmd`).
- Schermata TUI tariffe col tasto `p` → Task 5.
- Tasto `p` documentato in help + README → Task 5 (help line) + Task 6 (README).
- Valuta unica, retrocompatibilità → nativo (config senza `rates` ⇒ `ByList` nil ⇒ `For` usa Default).
- `Report.Rate` = default per export → Task 2 (verificato da `TestBuildPerListRates`).

**2. Placeholder scan:** nessun TBD/TODO.

**3. Type consistency:** `report.Rates`/`For`, `Build(rates Rates, ...)`, `ratesFromConfig(cfg) report.Rates`, `clickup.Client.ListName`, `newRates(entries, cfg) ratesModel`, `Model.ratesScreen`, `screenRates` usati coerentemente. I due call site di `Build` aggiornati in Task 2; un terzo (salvataggio tariffe) introdotto in Task 5 usa lo stesso helper.

**4. Ambiguità risolte:** `Report.Rate` **non** rimosso ma ridefinito come default (l'export lo legge); cambio di arrotondamento (importo per-entry) esplicitato e il test relativo aggiornato (9.9 → 10.0); `Config` con map non è più comparabile con `==` → i test che lo facevano vengono corretti in Task 1.

**Rischi noti:**
- Durante l'editing di una tariffa, il tasto `q` (gestito globalmente) esce dall'app invece di essere digitato. Le tariffe sono numeriche, quindi `q` non è un carattere valido: impatto trascurabile. Se dovesse dar fastidio, escludere `screenRates` dallo short-circuit di `q` in `Update`. (Non bloccante; da valutare in review.)
- La risoluzione dei nomi aggiunge N chiamate `GET /list/{id}` (una per lista unica, cache-ata) al primo caricamento del mese: latenza extra accettabile e documentata.
