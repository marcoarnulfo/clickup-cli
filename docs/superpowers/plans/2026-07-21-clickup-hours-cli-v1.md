# ClickUp Hours CLI — Implementation Plan (v1.0)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Costruire una TUI Go, TUI-first pura, che mostra ed esporta il report ore mensile di ClickUp (self + team) con calcolo dell'importo da fatturare.

**Architecture:** Cuore logico puro e senza I/O (`internal/report`) testato in TDD; un client HTTP verso ClickUp API v2 (`internal/clickup`); serializzatori di export (`internal/export`); una config su file (`internal/config`); una TUI bubbletea (`internal/tui`) come guscio sottile che orchestra il tutto.

**Tech Stack:** Go 1.26 · Charm (`bubbletea`, `bubbles`, `lipgloss`) · `net/http` stdlib · `gopkg.in/yaml.v3` per la config.

## Global Constraints

Ogni task eredita implicitamente questi vincoli:

- **Go 1.26+** (toolchain locale: go1.26.5).
- **Module path:** `github.com/marcoarnulfo/clickup-cli` (da adeguare al vero org GitHub prima della pubblicazione; usare questo valore in tutti gli import fino ad allora).
- **Binario:** `clickup` (da `cmd/clickup`).
- **Licenza:** MIT.
- **Dipendenze esterne consentite:** solo `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/bubbles`, `github.com/charmbracelet/lipgloss`, `gopkg.in/yaml.v3`. Tutto il resto è stdlib. **Nessun SDK ClickUp esterno** — l'API si usa via `net/http`.
- **ClickUp API base URL:** `https://api.clickup.com/api/v2`.
- **Auth:** header `Authorization: <personal_token>` (token nudo, **senza** prefisso `Bearer`).
- **Durate API:** ClickUp restituisce le durate in **millisecondi come stringa**; vanno parse-ate a `time.Duration`.
- **TDD obbligatorio** su `internal/config`, `internal/report`, `internal/export`, `internal/clickup`. La TUI si testa via `Update()` dei modelli.
- **Commit frequenti**, uno per task (o per step logico). Messaggi in inglese, prefissi Conventional Commits (`feat:`, `test:`, `chore:`, `docs:`). **Mai** aggiungere `Co-Authored-By`.

---

## File Structure

```
clickup-cli/
  cmd/clickup/main.go            # entrypoint: carica config, avvia la TUI
  internal/
    config/config.go            # Config, Path/Load/Save/Valid, env override
    config/config_test.go
    report/model.go             # tipi dominio: TimeEntry, Bucket, Report
    report/aggregate.go         # Build, MonthRange, round2, costanti GroupBy
    report/aggregate_test.go
    export/export.go            # CSV/JSON/Markdown + ToFile
    export/export_test.go
    clickup/client.go           # Client, New, do(), parsing errori/429
    clickup/user.go             # CurrentUser
    clickup/teams.go            # Teams
    clickup/timeentries.go      # TimeEntries -> []report.TimeEntry
    clickup/clickup_test.go     # httptest per tutti gli endpoint
    tui/styles.go               # stili lipgloss condivisi
    tui/app.go                  # root model, routing tra screen, comandi async
    tui/setup.go                # wizard primo avvio
    tui/home.go                 # picker mese + scope
    tui/report.go               # vista report + keybinding + grouping
    tui/export.go               # menu esporta
    tui/app_test.go             # test Update() delle transizioni
  go.mod
  go.sum
  README.md
  LICENSE
  .gitignore                    # già presente
```

**Confini:** `report` non importa nulla (dominio puro). `clickup` importa `report` (mappa il JSON nel dominio). `export` importa `report`. `tui` importa tutti. `config` è indipendente.

---

## Task 1: Scaffolding del progetto

**Files:**
- Create: `go.mod`, `LICENSE`, `README.md`, `cmd/clickup/main.go`

**Interfaces:**
- Produces: modulo compilabile `github.com/marcoarnulfo/clickup-cli` con binario `clickup` che stampa la versione.

- [ ] **Step 1: Inizializza il modulo**

Run:
```bash
go mod init github.com/marcoarnulfo/clickup-cli
```

- [ ] **Step 2: Crea la licenza MIT**

Create `LICENSE` (sostituisci `<ANNO>` con l'anno corrente e `<AUTORE>` col tuo nome):

```
MIT License

Copyright (c) 2026 marcoarnulfo

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- [ ] **Step 3: README minimale**

Create `README.md`:

```markdown
# clickup — ClickUp Hours CLI

TUI da terminale per il report ore mensile di ClickUp (self + team), con
calcolo dell'importo da fatturare ed export CSV/JSON/Markdown.

## Installazione

```bash
go install github.com/marcoarnulfo/clickup-cli/cmd/clickup@latest
```

## Uso

Lancia `clickup`. Al primo avvio un wizard ti chiede il token API personale
(lo trovi in ClickUp → Settings → Apps → API Token).

## Licenza

MIT
```

- [ ] **Step 4: Entrypoint minimale**

Create `cmd/clickup/main.go`:

```go
package main

import "fmt"

var version = "0.1.0-dev"

func main() {
	fmt.Printf("clickup %s\n", version)
}
```

- [ ] **Step 5: Verifica build**

Run:
```bash
go build ./... && go run ./cmd/clickup
```
Expected: stampa `clickup 0.1.0-dev`, nessun errore di build.

- [ ] **Step 6: Commit**

```bash
git add go.mod LICENSE README.md cmd/clickup/main.go
git commit -m "chore: scaffold module, MIT license, entrypoint"
```

---

## Task 2: Package `config`

**Files:**
- Create: `internal/config/config.go`, `internal/config/config_test.go`

**Interfaces:**
- Produces:
  - `type Config struct { Token, WorkspaceID, Currency string; Rate float64 }`
  - `func Path() (string, error)` → `<UserConfigDir>/clickup-cli/config.yml`
  - `func Load() (Config, error)` → legge il file; se manca ritorna `Config{}` senza errore; l'env `CLICKUP_TOKEN`, se presente, **sovrascrive** `Token`.
  - `func Save(c Config) error` → crea la dir e scrive YAML (perm 0600).
  - `func (c Config) Valid() bool` → `Token != "" && WorkspaceID != ""`.

- [ ] **Step 1: Aggiungi la dipendenza YAML**

Run:
```bash
go get gopkg.in/yaml.v3
```

- [ ] **Step 2: Scrivi i test (falliscono)**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveThenLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir) // os.UserConfigDir usa XDG_CONFIG_HOME su Linux; su macOS no
	t.Setenv("CLICKUP_TOKEN", "")    // evita override durante il test

	want := Config{Token: "tok_123", WorkspaceID: "900", Currency: "EUR", Rate: 45}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, want)
	}
}

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CLICKUP_TOKEN", "")
	got, err := Load()
	if err != nil {
		t.Fatalf("Load on missing file should not error, got %v", err)
	}
	if got != (Config{}) {
		t.Fatalf("expected zero Config, got %+v", got)
	}
}

func TestEnvOverridesToken(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CLICKUP_TOKEN", "")
	if err := Save(Config{Token: "file_tok", WorkspaceID: "1"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	t.Setenv("CLICKUP_TOKEN", "env_tok")
	got, _ := Load()
	if got.Token != "env_tok" {
		t.Fatalf("env should override token, got %q", got.Token)
	}
}

func TestValid(t *testing.T) {
	if (Config{Token: "x"}).Valid() {
		t.Fatal("missing workspace should be invalid")
	}
	if !(Config{Token: "x", WorkspaceID: "1"}).Valid() {
		t.Fatal("token+workspace should be valid")
	}
}

func TestPathUnderConfigDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if filepath.Base(p) != "config.yml" {
		t.Fatalf("expected config.yml, got %s", p)
	}
	if _, err := os.Stat(filepath.Dir(filepath.Dir(p))); err != nil {
		t.Fatalf("config dir root should exist: %v", err)
	}
}
```

- [ ] **Step 3: Esegui i test (devono fallire in compilazione)**

Run: `go test ./internal/config/ -v`
Expected: FAIL (`undefined: Config`, `Save`, `Load`, ...).

- [ ] **Step 4: Implementa `config.go`**

Create `internal/config/config.go`:

```go
// Package config gestisce la lettura/scrittura della configurazione utente.
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config è la configurazione persistita della CLI.
type Config struct {
	Token       string  `yaml:"token"`
	WorkspaceID string  `yaml:"workspace_id"`
	Currency    string  `yaml:"currency"`
	Rate        float64 `yaml:"rate"`
}

// Valid indica se la config è utilizzabile per interrogare l'API.
func (c Config) Valid() bool {
	return c.Token != "" && c.WorkspaceID != ""
}

// Path ritorna il percorso del file di config, es. ~/.config/clickup-cli/config.yml.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "clickup-cli", "config.yml"), nil
}

// Load legge la config dal disco. File mancante -> Config{} senza errore.
// L'env CLICKUP_TOKEN, se valorizzato, sovrascrive il token del file.
func Load() (Config, error) {
	var c Config
	p, err := Path()
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// nessun file: config vuota
	case err != nil:
		return c, err
	default:
		if err := yaml.Unmarshal(data, &c); err != nil {
			return c, err
		}
	}
	if env := os.Getenv("CLICKUP_TOKEN"); env != "" {
		c.Token = env
	}
	return c, nil
}

// Save scrive la config su disco creando le directory necessarie.
func Save(c Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}
```

- [ ] **Step 5: Esegui i test (devono passare)**

Run: `go test ./internal/config/ -v`
Expected: PASS (tutti i test).

Nota: su macOS `os.UserConfigDir()` ritorna `~/Library/Application Support` e **ignora** `XDG_CONFIG_HOME`. I test usano `t.TempDir()` + `XDG_CONFIG_HOME`; se giri su macOS e un test fallisse per questo, non è un bug del codice — in CI (Linux) passano. Per robustezza locale puoi anche settare `t.Setenv("HOME", t.TempDir())`.

- [ ] **Step 6: Commit**

```bash
git add internal/config go.mod go.sum
git commit -m "feat: config load/save with env token override"
```

---

## Task 3: Package `report` (cuore puro)

**Files:**
- Create: `internal/report/model.go`, `internal/report/aggregate.go`, `internal/report/aggregate_test.go`

**Interfaces:**
- Produces:
  - `type TimeEntry struct { ID, TaskID, TaskName, ListID, ListName, UserName string; UserID int; Start time.Time; Duration time.Duration }`
  - `type Bucket struct { Label string; Hours, Amount float64 }`
  - `type Report struct { Year int; Month time.Month; Scope, GroupBy, Currency string; Rate float64; Buckets []Bucket; TotalHours, TotalAmount float64 }`
  - costanti: `GroupByTask="task"`, `GroupByList="list"`, `GroupByDay="day"`, `GroupByTotal="total"`
  - `func Build(entries []TimeEntry, groupBy string, rate float64, currency string, year int, month time.Month) Report`
  - `func MonthRange(year int, month time.Month) (start, end time.Time)` → primo istante del mese e primo istante del mese successivo (half-open `[start, end)`), in UTC.

- [ ] **Step 1: Scrivi i test (falliscono)**

Create `internal/report/aggregate_test.go`:

```go
package report

import (
	"testing"
	"time"
)

func d(h float64) time.Duration { return time.Duration(h * float64(time.Hour)) }

func sampleEntries() []TimeEntry {
	base := time.Date(2026, time.July, 1, 9, 0, 0, 0, time.UTC)
	return []TimeEntry{
		{TaskName: "Bug fix", ListName: "Cliente A", UserName: "me", Start: base, Duration: d(2)},
		{TaskName: "Bug fix", ListName: "Cliente A", UserName: "me", Start: base.AddDate(0, 0, 1), Duration: d(1)},
		{TaskName: "Feature X", ListName: "Cliente B", UserName: "me", Start: base, Duration: d(3)},
	}
}

func TestBuildTotal(t *testing.T) {
	r := Build(sampleEntries(), GroupByTotal, 50, "EUR", 2026, time.July)
	if r.TotalHours != 6 {
		t.Fatalf("total hours = %v, want 6", r.TotalHours)
	}
	if r.TotalAmount != 300 {
		t.Fatalf("total amount = %v, want 300", r.TotalAmount)
	}
	if len(r.Buckets) != 1 || r.Buckets[0].Label != "Totale" {
		t.Fatalf("total should have one bucket labelled Totale, got %+v", r.Buckets)
	}
}

func TestBuildByTaskSortedByHoursDesc(t *testing.T) {
	r := Build(sampleEntries(), GroupByTask, 0, "EUR", 2026, time.July)
	if len(r.Buckets) != 2 {
		t.Fatalf("want 2 task buckets, got %d", len(r.Buckets))
	}
	// "Bug fix" = 3h, "Feature X" = 3h -> pari merito, ordine per label asc
	if r.Buckets[0].Label != "Bug fix" || r.Buckets[0].Hours != 3 {
		t.Fatalf("bucket[0] = %+v", r.Buckets[0])
	}
}

func TestBuildByList(t *testing.T) {
	r := Build(sampleEntries(), GroupByList, 0, "EUR", 2026, time.July)
	if len(r.Buckets) != 2 {
		t.Fatalf("want 2 list buckets, got %d", len(r.Buckets))
	}
	m := map[string]float64{}
	for _, b := range r.Buckets {
		m[b.Label] = b.Hours
	}
	if m["Cliente A"] != 3 || m["Cliente B"] != 3 {
		t.Fatalf("list hours wrong: %+v", m)
	}
}

func TestBuildByDayChronological(t *testing.T) {
	r := Build(sampleEntries(), GroupByDay, 0, "EUR", 2026, time.July)
	if len(r.Buckets) != 2 {
		t.Fatalf("want 2 day buckets, got %d", len(r.Buckets))
	}
	if r.Buckets[0].Label != "2026-07-01" || r.Buckets[1].Label != "2026-07-02" {
		t.Fatalf("days not chronological: %+v", r.Buckets)
	}
	if r.Buckets[0].Hours != 5 || r.Buckets[1].Hours != 1 {
		t.Fatalf("day hours wrong: %+v", r.Buckets)
	}
}

func TestBuildEmpty(t *testing.T) {
	r := Build(nil, GroupByTask, 50, "EUR", 2026, time.July)
	if r.TotalHours != 0 || len(r.Buckets) != 0 {
		t.Fatalf("empty report should be zero, got %+v", r)
	}
}

func TestRoundingTwoDecimals(t *testing.T) {
	e := []TimeEntry{{TaskName: "x", Start: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), Duration: d(1.0 / 3.0)}}
	r := Build(e, GroupByTask, 30, "EUR", 2026, time.July)
	if r.Buckets[0].Hours != 0.33 {
		t.Fatalf("hours should round to 0.33, got %v", r.Buckets[0].Hours)
	}
	if r.TotalAmount != 9.9 { // 0.33 * 30
		t.Fatalf("amount should be 9.9, got %v", r.TotalAmount)
	}
}

func TestMonthRange(t *testing.T) {
	start, end := MonthRange(2026, time.July)
	if !start.Equal(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("start = %v", start)
	}
	if !end.Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("end = %v", end)
	}
}
```

- [ ] **Step 2: Esegui i test (devono fallire)**

Run: `go test ./internal/report/ -v`
Expected: FAIL (`undefined: Build`, `MonthRange`, ...).

- [ ] **Step 3: Implementa `model.go`**

Create `internal/report/model.go`:

```go
// Package report contiene il modello dominio e la logica di aggregazione
// delle ore. È puro: nessun I/O, nessuna dipendenza esterna.
package report

import "time"

// TimeEntry è una singola voce di tempo normalizzata dal dominio ClickUp.
type TimeEntry struct {
	ID       string
	TaskID   string
	TaskName string
	ListID   string
	ListName string // il "progetto"
	UserID   int
	UserName string
	Start    time.Time
	Duration time.Duration
}

// Bucket è una riga aggregata del report.
type Bucket struct {
	Label  string
	Hours  float64
	Amount float64
}

// Report è il risultato aggregato pronto per la presentazione/export.
type Report struct {
	Year        int
	Month       time.Month
	Scope       string // "me" | "team"
	GroupBy     string
	Currency    string
	Rate        float64
	Buckets     []Bucket
	TotalHours  float64
	TotalAmount float64
}
```

- [ ] **Step 4: Implementa `aggregate.go`**

Create `internal/report/aggregate.go`:

```go
package report

import (
	"math"
	"sort"
	"time"
)

// Modalità di raggruppamento supportate.
const (
	GroupByTask  = "task"
	GroupByList  = "list"
	GroupByDay   = "day"
	GroupByTotal = "total"
)

// round2 arrotonda a 2 decimali.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// MonthRange ritorna l'intervallo half-open [start, end) del mese in UTC.
func MonthRange(year int, month time.Month) (start, end time.Time) {
	start = time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end = start.AddDate(0, 1, 0)
	return start, end
}

// keyFor determina l'etichetta del bucket per una entry, dato il groupBy.
func keyFor(e TimeEntry, groupBy string) string {
	switch groupBy {
	case GroupByTask:
		return e.TaskName
	case GroupByList:
		return e.ListName
	case GroupByDay:
		return e.Start.Format("2006-01-02")
	default:
		return "Totale"
	}
}

// Build aggrega le entry in un Report secondo groupBy, applicando la tariffa.
func Build(entries []TimeEntry, groupBy string, rate float64, currency string, year int, month time.Month) Report {
	r := Report{
		Year:     year,
		Month:    month,
		GroupBy:  groupBy,
		Currency: currency,
		Rate:     rate,
	}

	sums := map[string]float64{}
	var order []string
	for _, e := range entries {
		k := keyFor(e, groupBy)
		if _, seen := sums[k]; !seen {
			order = append(order, k)
		}
		sums[k] += e.Duration.Hours()
	}

	for _, k := range order {
		h := round2(sums[k])
		r.Buckets = append(r.Buckets, Bucket{Label: k, Hours: h, Amount: round2(h * rate)})
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

	var total float64
	for _, b := range r.Buckets {
		total += b.Hours
	}
	r.TotalHours = round2(total)
	r.TotalAmount = round2(r.TotalHours * rate)
	return r
}
```

- [ ] **Step 5: Esegui i test (devono passare)**

Run: `go test ./internal/report/ -v`
Expected: PASS (tutti).

- [ ] **Step 6: Commit**

```bash
git add internal/report
git commit -m "feat: pure report aggregation (task/list/day/total) with billing"
```

---

## Task 4: Package `export`

**Files:**
- Create: `internal/export/export.go`, `internal/export/export_test.go`

**Interfaces:**
- Consumes: `report.Report`, `report.Bucket`.
- Produces:
  - `func CSV(w io.Writer, r report.Report) error`
  - `func JSON(w io.Writer, r report.Report) error`
  - `func Markdown(w io.Writer, r report.Report) error`
  - `func ToFile(format string, r report.Report, path string) error` — `format` ∈ {`"csv"`,`"json"`,`"markdown"`}; errore su formato ignoto.

- [ ] **Step 1: Scrivi i test (falliscono)**

Create `internal/export/export_test.go`:

```go
package export

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func sample() report.Report {
	return report.Report{
		Year: 2026, Month: time.July, Scope: "me", GroupBy: "list", Currency: "EUR", Rate: 50,
		Buckets: []report.Bucket{
			{Label: "Cliente A", Hours: 3, Amount: 150},
			{Label: "Cliente B", Hours: 3, Amount: 150},
		},
		TotalHours: 6, TotalAmount: 300,
	}
}

func TestCSVHasHeaderAndRows(t *testing.T) {
	var b bytes.Buffer
	if err := CSV(&b, sample()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.HasPrefix(out, "label,hours,amount,currency\n") {
		t.Fatalf("bad header: %q", out)
	}
	if !strings.Contains(out, "Cliente A,3,150,EUR") {
		t.Fatalf("missing row: %q", out)
	}
	if !strings.Contains(out, "TOTAL,6,300,EUR") {
		t.Fatalf("missing total row: %q", out)
	}
}

func TestJSONRoundTrips(t *testing.T) {
	var b bytes.Buffer
	if err := JSON(&b, sample()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `"total_hours": 6`) {
		t.Fatalf("json missing total_hours: %s", b.String())
	}
	if !strings.Contains(b.String(), `"label": "Cliente A"`) {
		t.Fatalf("json missing bucket: %s", b.String())
	}
}

func TestMarkdownTable(t *testing.T) {
	var b bytes.Buffer
	if err := Markdown(&b, sample()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "| Label | Ore | Importo |") {
		t.Fatalf("missing md header: %q", out)
	}
	if !strings.Contains(out, "| Cliente A | 3.00 | 150.00 EUR |") {
		t.Fatalf("missing md row: %q", out)
	}
	if !strings.Contains(out, "**Totale** | **6.00** | **300.00 EUR**") {
		t.Fatalf("missing md total: %q", out)
	}
}

func TestToFileUnknownFormat(t *testing.T) {
	if err := ToFile("pdf", sample(), t.TempDir()+"/x"); err == nil {
		t.Fatal("expected error on unknown format")
	}
}

func TestToFileWritesCSV(t *testing.T) {
	p := t.TempDir() + "/out.csv"
	if err := ToFile("csv", sample(), p); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Esegui i test (devono fallire)**

Run: `go test ./internal/export/ -v`
Expected: FAIL (`undefined: CSV`, ...).

- [ ] **Step 3: Implementa `export.go`**

Create `internal/export/export.go`:

```go
// Package export serializza un report.Report in CSV, JSON o Markdown.
package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// CSV scrive il report come CSV con riga di intestazione e riga totale.
func CSV(w io.Writer, r report.Report) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"label", "hours", "amount", "currency"}); err != nil {
		return err
	}
	num := func(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
	for _, b := range r.Buckets {
		if err := cw.Write([]string{b.Label, num(b.Hours), num(b.Amount), r.Currency}); err != nil {
			return err
		}
	}
	if err := cw.Write([]string{"TOTAL", num(r.TotalHours), num(r.TotalAmount), r.Currency}); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

// jsonReport è la forma serializzata (snake_case) del report.
type jsonReport struct {
	Year        int             `json:"year"`
	Month       int             `json:"month"`
	Scope       string          `json:"scope"`
	GroupBy     string          `json:"group_by"`
	Currency    string          `json:"currency"`
	Rate        float64         `json:"rate"`
	Buckets     []report.Bucket `json:"buckets"`
	TotalHours  float64         `json:"total_hours"`
	TotalAmount float64         `json:"total_amount"`
}

// JSON scrive il report come JSON indentato.
func JSON(w io.Writer, r report.Report) error {
	jr := jsonReport{
		Year: r.Year, Month: int(r.Month), Scope: r.Scope, GroupBy: r.GroupBy,
		Currency: r.Currency, Rate: r.Rate, Buckets: r.Buckets,
		TotalHours: r.TotalHours, TotalAmount: r.TotalAmount,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jr)
}

// Markdown scrive il report come tabella Markdown.
func Markdown(w io.Writer, r report.Report) error {
	fmt.Fprintf(w, "# Report ore %04d-%02d\n\n", r.Year, int(r.Month))
	fmt.Fprintln(w, "| Label | Ore | Importo |")
	fmt.Fprintln(w, "|---|---:|---:|")
	for _, b := range r.Buckets {
		fmt.Fprintf(w, "| %s | %.2f | %.2f %s |\n", b.Label, b.Hours, b.Amount, r.Currency)
	}
	fmt.Fprintf(w, "| **Totale** | **%.2f** | **%.2f %s** |\n", r.TotalHours, r.TotalAmount, r.Currency)
	return nil
}

// ToFile scrive il report nel formato dato sul path indicato.
func ToFile(format string, r report.Report, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	switch format {
	case "csv":
		return CSV(f, r)
	case "json":
		return JSON(f, r)
	case "markdown":
		return Markdown(f, r)
	default:
		return fmt.Errorf("formato non supportato: %q", format)
	}
}
```

- [ ] **Step 4: Esegui i test (devono passare)**

Run: `go test ./internal/export/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/export
git commit -m "feat: export report to CSV/JSON/Markdown"
```

---

## Task 5: Package `clickup` (client API v2)

**Files:**
- Create: `internal/clickup/client.go`, `internal/clickup/user.go`, `internal/clickup/teams.go`, `internal/clickup/timeentries.go`, `internal/clickup/clickup_test.go`

**Interfaces:**
- Consumes: `report.TimeEntry`.
- Produces:
  - `type Client struct { ... }` con `func New(token string) *Client` e campo esportabile `BaseURL string` (per i test httptest).
  - `type User struct { ID int; Username string }` — `func (c *Client) CurrentUser(ctx context.Context) (User, error)` (GET `/user`).
  - `type Member struct { ID int; Username string }`, `type Team struct { ID, Name string; Members []Member }` — `func (c *Client) Teams(ctx context.Context) ([]Team, error)` (GET `/team`).
  - `func (c *Client) TimeEntries(ctx context.Context, teamID string, start, end time.Time, assignees []int) ([]report.TimeEntry, error)` (GET `/team/{id}/time_entries`).

- [ ] **Step 1: Scrivi i test (falliscono)**

Create `internal/clickup/clickup_test.go`:

```go
package clickup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient(h http.HandlerFunc) (*Client, *httptest.Server) {
	srv := httptest.NewServer(h)
	c := New("tok_test")
	c.BaseURL = srv.URL
	return c, srv
}

func TestCurrentUser(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "tok_test" {
			t.Errorf("missing/incorrect auth header: %q", r.Header.Get("Authorization"))
		}
		w.Write([]byte(`{"user":{"id":42,"username":"Marco"}}`))
	})
	defer srv.Close()
	u, err := c.CurrentUser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != 42 || u.Username != "Marco" {
		t.Fatalf("got %+v", u)
	}
}

func TestTeams(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"teams":[{"id":"900","name":"Workspace","members":[{"user":{"id":1,"username":"a"}}]}]}`))
	})
	defer srv.Close()
	teams, err := c.Teams(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(teams) != 1 || teams[0].ID != "900" || len(teams[0].Members) != 1 {
		t.Fatalf("got %+v", teams)
	}
}

func TestTimeEntriesParsesDurationAndTask(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("start_date"); got == "" {
			t.Errorf("missing start_date")
		}
		w.Write([]byte(`{"data":[{
			"id":"e1",
			"task":{"id":"t1","name":"Bug fix"},
			"task_location":{"list_id":"l1"},
			"user":{"id":7,"username":"Marco"},
			"start":"1751360400000",
			"duration":"7200000"
		}]}`))
	})
	defer srv.Close()
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	entries, err := c.TimeEntries(context.Background(), "900", start, end, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.TaskName != "Bug fix" || e.UserID != 7 || e.Duration != 2*time.Hour {
		t.Fatalf("bad entry: %+v", e)
	}
}

func TestAPIErrorStatus(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"err":"Token invalid","ECODE":"OAUTH_017"}`))
	})
	defer srv.Close()
	_, err := c.CurrentUser(context.Background())
	if err == nil {
		t.Fatal("expected error on 401")
	}
}
```

- [ ] **Step 2: Esegui i test (devono fallire)**

Run: `go test ./internal/clickup/ -v`
Expected: FAIL (undefined symbols).

- [ ] **Step 3: Implementa `client.go`**

Create `internal/clickup/client.go`:

```go
// Package clickup è un client minimale per la ClickUp API v2 (time tracking).
package clickup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client interroga la ClickUp API v2.
type Client struct {
	token   string
	BaseURL string
	http    *http.Client
}

// New crea un client con il token personale.
func New(token string) *Client {
	return &Client{
		token:   token,
		BaseURL: "https://api.clickup.com/api/v2",
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// apiError rappresenta il corpo d'errore standard di ClickUp.
type apiError struct {
	Err   string `json:"err"`
	ECODE string `json:"ECODE"`
}

// get esegue una GET autenticata e decodifica il JSON in out.
// Gestisce il 429 con un retry dopo backoff.
func (c *Client) get(ctx context.Context, path string, query map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.token)
	if query != nil {
		q := req.URL.Query()
		for k, v := range query {
			if v != "" {
				q.Set(k, v)
			}
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusTooManyRequests {
		// backoff semplice e singolo retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
		return c.get(ctx, path, query, out)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var ae apiError
		_ = json.Unmarshal(body, &ae)
		if ae.Err != "" {
			return fmt.Errorf("clickup API %d: %s (%s)", resp.StatusCode, ae.Err, ae.ECODE)
		}
		return fmt.Errorf("clickup API %d: %s", resp.StatusCode, string(body))
	}
	return json.Unmarshal(body, out)
}
```

- [ ] **Step 4: Implementa `user.go` e `teams.go`**

Create `internal/clickup/user.go`:

```go
package clickup

import "context"

// User è l'utente autenticato.
type User struct {
	ID       int
	Username string
}

// CurrentUser ritorna l'utente proprietario del token.
func (c *Client) CurrentUser(ctx context.Context) (User, error) {
	var resp struct {
		User struct {
			ID       int    `json:"id"`
			Username string `json:"username"`
		} `json:"user"`
	}
	if err := c.get(ctx, "/user", nil, &resp); err != nil {
		return User{}, err
	}
	return User{ID: resp.User.ID, Username: resp.User.Username}, nil
}
```

Create `internal/clickup/teams.go`:

```go
package clickup

import "context"

// Member è un membro di un workspace.
type Member struct {
	ID       int
	Username string
}

// Team è un workspace ClickUp (nell'API "team").
type Team struct {
	ID      string
	Name    string
	Members []Member
}

// Teams ritorna i workspace accessibili col token, con i loro membri.
func (c *Client) Teams(ctx context.Context) ([]Team, error) {
	var resp struct {
		Teams []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Members []struct {
				User struct {
					ID       int    `json:"id"`
					Username string `json:"username"`
				} `json:"user"`
			} `json:"members"`
		} `json:"teams"`
	}
	if err := c.get(ctx, "/team", nil, &resp); err != nil {
		return nil, err
	}
	teams := make([]Team, 0, len(resp.Teams))
	for _, t := range resp.Teams {
		team := Team{ID: t.ID, Name: t.Name}
		for _, m := range t.Members {
			team.Members = append(team.Members, Member{ID: m.User.ID, Username: m.User.Username})
		}
		teams = append(teams, team)
	}
	return teams, nil
}
```

- [ ] **Step 5: Implementa `timeentries.go`**

Create `internal/clickup/timeentries.go`:

```go
package clickup

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// rawEntry rispecchia una voce dell'array "data" di /team/{id}/time_entries.
type rawEntry struct {
	ID   string `json:"id"`
	Task struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"task"`
	TaskLocation struct {
		ListID string `json:"list_id"`
	} `json:"task_location"`
	User struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
	} `json:"user"`
	Start    string `json:"start"`    // epoch ms come stringa
	Duration string `json:"duration"` // ms come stringa
}

// TimeEntries ritorna le voci di tempo del workspace nel range [start, end).
// Se assignees è non vuoto, filtra su quegli utenti (scope team).
func (c *Client) TimeEntries(ctx context.Context, teamID string, start, end time.Time, assignees []int) ([]report.TimeEntry, error) {
	q := map[string]string{
		"start_date": strconv.FormatInt(start.UnixMilli(), 10),
		"end_date":   strconv.FormatInt(end.UnixMilli(), 10),
	}
	if len(assignees) > 0 {
		ids := make([]string, len(assignees))
		for i, a := range assignees {
			ids[i] = strconv.Itoa(a)
		}
		q["assignee"] = strings.Join(ids, ",")
	}

	var resp struct {
		Data []rawEntry `json:"data"`
	}
	if err := c.get(ctx, "/team/"+teamID+"/time_entries", q, &resp); err != nil {
		return nil, err
	}

	out := make([]report.TimeEntry, 0, len(resp.Data))
	for _, r := range resp.Data {
		ms, _ := strconv.ParseInt(r.Duration, 10, 64)
		startMs, _ := strconv.ParseInt(r.Start, 10, 64)
		out = append(out, report.TimeEntry{
			ID:       r.ID,
			TaskID:   r.Task.ID,
			TaskName: r.Task.Name,
			ListID:   r.TaskLocation.ListID,
			ListName: r.TaskLocation.ListID, // nome lista risolto in v1.1; per ora l'ID
			UserID:   r.User.ID,
			UserName: r.User.Username,
			Start:    time.UnixMilli(startMs).UTC(),
			Duration: time.Duration(ms) * time.Millisecond,
		})
	}
	return out, nil
}
```

Nota: l'endpoint `time_entries` non restituisce il **nome** della lista, solo `list_id`. In v1.0 usiamo l'ID come `ListName` (il raggruppamento "per progetto" funziona comunque). Il nome leggibile della lista arriverà in v1.1 con una cache `list_id -> name` via `GET /list/{id}`.

- [ ] **Step 6: Esegui i test (devono passare)**

Run: `go test ./internal/clickup/ -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/clickup
git commit -m "feat: ClickUp API v2 client (user, teams, time_entries)"
```

---

## Task 6: TUI — stili, app root e comandi async

**Files:**
- Create: `internal/tui/styles.go`, `internal/tui/app.go`, `internal/tui/app_test.go`
- Modify: `cmd/clickup/main.go`

**Interfaces:**
- Consumes: `config`, `clickup`, `report`.
- Produces:
  - `type screen int` con `screenSetup, screenHome, screenLoading, screenReport, screenExport, screenError`.
  - `type Model struct { ... }` che implementa `tea.Model` (`Init`, `Update`, `View`).
  - `func New(cfg config.Config) Model` — sceglie lo screen iniziale (`screenSetup` se `!cfg.Valid()`, altrimenti `screenHome`).
  - messaggi async: `entriesMsg []report.TimeEntry`, `teamsMsg []clickup.Team`, `errMsg error`.
  - comandi: `func loadEntriesCmd(c *clickup.Client, teamID string, year int, month time.Month, assignees []int) tea.Cmd`.
  - stili esportati usati dagli altri screen: `styleTitle`, `styleHelp`, `styleErr`, `styleAccent` (package-level in `styles.go`).

- [ ] **Step 1: Aggiungi le dipendenze Charm**

Run:
```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/lipgloss@latest
```

- [ ] **Step 2: Implementa `styles.go`**

Create `internal/tui/styles.go`:

```go
package tui

import "github.com/charmbracelet/lipgloss"

var (
	colAccent = lipgloss.Color("205") // magenta ClickUp-ish
	colDim    = lipgloss.Color("240")
	colErr    = lipgloss.Color("196")
	colOK     = lipgloss.Color("42")

	styleTitle  = lipgloss.NewStyle().Bold(true).Foreground(colAccent).MarginBottom(1)
	styleHelp   = lipgloss.NewStyle().Foreground(colDim)
	styleErr    = lipgloss.NewStyle().Foreground(colErr).Bold(true)
	styleAccent = lipgloss.NewStyle().Foreground(colAccent)
	styleOK     = lipgloss.NewStyle().Foreground(colOK)
	styleBox    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)
```

- [ ] **Step 3: Scrivi i test dell'app (falliscono)**

Create `internal/tui/app_test.go`:

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func TestNewStartsInSetupWhenInvalid(t *testing.T) {
	m := New(config.Config{})
	if m.screen != screenSetup {
		t.Fatalf("invalid config should start in setup, got %v", m.screen)
	}
}

func TestNewStartsInHomeWhenValid(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1"})
	if m.screen != screenHome {
		t.Fatalf("valid config should start in home, got %v", m.screen)
	}
}

func TestErrMsgSwitchesToErrorScreen(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1"})
	updated, _ := m.Update(errMsg{err: errTest})
	mm := updated.(Model)
	if mm.screen != screenError {
		t.Fatalf("errMsg should switch to error screen, got %v", mm.screen)
	}
}

func TestEntriesMsgBuildsReportAndShowsReportScreen(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1", Rate: 10, Currency: "EUR"})
	m.year, m.month = 2026, 7
	updated, _ := m.Update(entriesMsg{entries: []report.TimeEntry{}})
	mm := updated.(Model)
	if mm.screen != screenReport {
		t.Fatalf("entriesMsg should switch to report screen, got %v", mm.screen)
	}
}

func TestQuitKey(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1"})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("q should return a quit command")
	}
}

var errTest = &testErr{}

type testErr struct{}

func (*testErr) Error() string { return "boom" }
```

- [ ] **Step 4: Esegui i test (devono fallire)**

Run: `go test ./internal/tui/ -v`
Expected: FAIL (undefined `New`, `Model`, `screenSetup`, ...).

- [ ] **Step 5: Implementa `app.go`**

Create `internal/tui/app.go`:

```go
package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

type screen int

const (
	screenSetup screen = iota
	screenHome
	screenLoading
	screenReport
	screenExport
	screenError
)

// Messaggi async.
type (
	entriesMsg struct{ entries []report.TimeEntry }
	teamsMsg   struct{ teams []clickup.Team }
	errMsg     struct{ err error }
)

// Model è il modello radice della TUI.
type Model struct {
	cfg    config.Config
	client *clickup.Client
	screen screen
	err    error

	width, height int

	// selezione corrente
	year      int
	month     time.Month
	scope     string // "me" | "team"
	assignees []int

	// dati
	report report.Report

	// sotto-modelli
	setup  setupModel
	home   homeModel
	rep    reportModel
	export exportModel
}

// New costruisce il modello radice a partire dalla config.
func New(cfg config.Config) Model {
	now := time.Now()
	m := Model{
		cfg:    cfg,
		year:   now.Year(),
		month:  now.Month(),
		scope:  "me",
		client: clickup.New(cfg.Token),
	}
	if cfg.Valid() {
		m.screen = screenHome
		m.home = newHome(m.year, m.month)
	} else {
		m.screen = screenSetup
		m.setup = newSetup()
	}
	return m
}

func (m Model) Init() tea.Cmd { return nil }

// loadEntriesCmd chiama l'API in background e ritorna entriesMsg o errMsg.
func loadEntriesCmd(c *clickup.Client, teamID string, year int, month time.Month, assignees []int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		start, end := report.MonthRange(year, month)
		entries, err := c.TimeEntries(ctx, teamID, start, end, assignees)
		if err != nil {
			return errMsg{err: err}
		}
		return entriesMsg{entries: entries}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "q" && m.screen != screenSetup {
			return m, tea.Quit
		}
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		return m.routeKey(msg)

	case errMsg:
		m.err = msg.err
		m.screen = screenError
		return m, nil

	case entriesMsg:
		m.report = report.Build(msg.entries, report.GroupByTotal, m.cfg.Rate, m.cfg.Currency, m.year, m.month)
		m.report.Scope = m.scope
		m.rep = newReport(m.report)
		m.screen = screenReport
		return m, nil

	case teamsMsg:
		// consegnato al setup per la scelta workspace
		var cmd tea.Cmd
		m.setup, cmd = m.setup.withTeams(msg.teams)
		return m, cmd
	}
	return m, nil
}

// routeKey inoltra i tasti allo screen attivo.
func (m Model) routeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenSetup:
		return m.updateSetup(msg)
	case screenHome:
		return m.updateHome(msg)
	case screenReport:
		return m.updateReport(msg)
	case screenExport:
		return m.updateExport(msg)
	case screenError:
		// qualsiasi tasto torna alla home
		m.screen = screenHome
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	switch m.screen {
	case screenSetup:
		return m.setup.view()
	case screenHome:
		return m.home.view()
	case screenLoading:
		return styleTitle.Render("Caricamento ore…")
	case screenReport:
		return m.rep.view()
	case screenExport:
		return m.export.view()
	case screenError:
		return styleErr.Render("Errore: ") + m.err.Error() + "\n\n" + styleHelp.Render("premi un tasto per tornare alla home")
	}
	return ""
}
```

Nota: i metodi `updateSetup/updateHome/updateReport/updateExport`, `newSetup`, `newHome`, `newReport`, `newExport`, `setup.withTeams`, e i tipi `setupModel/homeModel/reportModel/exportModel` sono definiti nei task 7–10. Fino ad allora il package **non compila**: crea in questo task degli stub minimi in un file `internal/tui/stubs.go` per far girare i test dell'app, poi rimuovili man mano.

- [ ] **Step 6: Crea gli stub temporanei per compilare**

Create `internal/tui/stubs.go` (verrà svuotato nei task successivi):

```go
package tui

import tea "github.com/charmbracelet/bubbletea"

type setupModel struct{}
type homeModel struct{}
type reportModel struct{}
type exportModel struct{}

func newSetup() setupModel                 { return setupModel{} }
func newHome(int, interface{ String() string }) homeModel { return homeModel{} }

func (s setupModel) view() string { return "setup" }
func (h homeModel) view() string  { return "home" }
func (r reportModel) view() string { return "report" }
func (e exportModel) view() string { return "export" }

func (s setupModel) withTeams(any) (setupModel, tea.Cmd) { return s, nil }

func (m Model) updateSetup(tea.KeyMsg) (tea.Model, tea.Cmd)  { return m, nil }
func (m Model) updateHome(tea.KeyMsg) (tea.Model, tea.Cmd)   { return m, nil }
func (m Model) updateReport(tea.KeyMsg) (tea.Model, tea.Cmd) { return m, nil }
func (m Model) updateExport(tea.KeyMsg) (tea.Model, tea.Cmd) { return m, nil }
```

⚠️ Gli stub sopra hanno firme *provvisorie*. Nei task 7–10 sostituirai un file stub alla volta con l'implementazione reale, adeguando le firme a quelle indicate nei rispettivi task. Il `newHome` reale ha firma `newHome(year int, month time.Month) homeModel` — qui è volutamente sbagliato solo per compilare gli stub isolati; **quando implementi il Task 8, aggiorna sia `stubs.go` sia `app.go` alla firma reale.** Per semplicità, in questo task modifica `app.go` per chiamare `newHome(m.year, m.month)` e definisci lo stub con la stessa firma:

```go
func newHome(year int, month time.Month) homeModel { return homeModel{} }
```

(usa `time` nell'import di stubs.go). Mantieni una sola definizione di ciascun simbolo.

- [ ] **Step 7: Cabla `main.go`**

Replace `cmd/clickup/main.go`:

```go
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/tui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	p := tea.NewProgram(tui.New(cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "errore:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 8: Esegui i test (devono passare) e build**

Run: `go test ./internal/tui/ -v && go build ./...`
Expected: PASS + build ok.

- [ ] **Step 9: Commit**

```bash
git add internal/tui cmd/clickup/main.go go.mod go.sum
git commit -m "feat: TUI root model, screen routing, async load command"
```

---

## Task 7: TUI — Setup wizard

**Files:**
- Create: `internal/tui/setup.go`
- Modify: `internal/tui/stubs.go` (rimuovi gli stub di `setupModel`, `newSetup`, `setup.view`, `setup.withTeams`, `updateSetup`)

**Interfaces:**
- Consumes: `clickup.Client`, `clickup.Team`, `config`.
- Produces:
  - `type setupModel struct { ... }`, `func newSetup() setupModel`
  - `func (s setupModel) view() string`
  - `func (s setupModel) withTeams(teams []clickup.Team) (setupModel, tea.Cmd)`
  - `func (m Model) updateSetup(msg tea.KeyMsg) (tea.Model, tea.Cmd)` — gestisce gli step: (1) input token → Enter valida via `CurrentUser` + carica `Teams`; (2) scelta workspace da lista; (3) input tariffa; (4) input valuta → salva config e passa a `screenHome`.

- [ ] **Step 1: Scrivi il test (fallisce)**

Add to `internal/tui/app_test.go`:

```go
func TestSetupTokenStepAcceptsInput(t *testing.T) {
	m := New(config.Config{})
	// digita un carattere nel campo token
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	mm := updated.(Model)
	if mm.setup.token() == "" {
		t.Fatal("token input should capture typed characters")
	}
}
```

- [ ] **Step 2: Esegui (fallisce)**

Run: `go test ./internal/tui/ -run TestSetupToken -v`
Expected: FAIL (undefined `setup.token`, o stub).

- [ ] **Step 3: Implementa `setup.go`**

Rimuovi da `stubs.go` le righe relative a `setupModel`, `newSetup`, `setupModel.view`, `setupModel.withTeams`, `updateSetup`. Poi create `internal/tui/setup.go`:

```go
package tui

import (
	"context"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
)

type setupStep int

const (
	stepToken setupStep = iota
	stepWorkspace
	stepRate
	stepCurrency
)

type setupModel struct {
	step    setupStep
	input   textinput.Model
	teams   []clickup.Team
	teamIdx int
	tmpCfg  config.Config
	loading bool
	msg     string
}

func newSetup() setupModel {
	ti := textinput.New()
	ti.Placeholder = "pk_xxx… (ClickUp → Settings → Apps → API Token)"
	ti.Focus()
	ti.CharLimit = 128
	ti.Width = 50
	return setupModel{step: stepToken, input: ti}
}

func (s setupModel) token() string { return s.tmpCfg.Token }

func (s setupModel) withTeams(teams []clickup.Team) (setupModel, tea.Cmd) {
	s.teams = teams
	s.loading = false
	s.step = stepWorkspace
	return s, nil
}

func (m Model) updateSetup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.setup
	switch s.step {
	case stepToken:
		if msg.Type == tea.KeyEnter && s.input.Value() != "" {
			s.tmpCfg.Token = s.input.Value()
			s.loading = true
			s.msg = "Validazione token…"
			m.setup = s
			m.client = clickup.New(s.tmpCfg.Token)
			return m, validateAndLoadTeamsCmd(m.client)
		}
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		s.tmpCfg.Token = s.input.Value()
		m.setup = s
		return m, cmd

	case stepWorkspace:
		switch msg.String() {
		case "up", "k":
			if s.teamIdx > 0 {
				s.teamIdx--
			}
		case "down", "j":
			if s.teamIdx < len(s.teams)-1 {
				s.teamIdx++
			}
		case "enter":
			if len(s.teams) > 0 {
				s.tmpCfg.WorkspaceID = s.teams[s.teamIdx].ID
				s.step = stepRate
				s.input = newNumberInput("Tariffa oraria (es. 45) — vuoto per saltare")
			}
		}
		m.setup = s
		return m, nil

	case stepRate:
		if msg.Type == tea.KeyEnter {
			if v := s.input.Value(); v != "" {
				s.tmpCfg.Rate, _ = strconv.ParseFloat(v, 64)
			}
			s.step = stepCurrency
			s.input = newTextInput("Valuta (es. EUR) — vuoto per EUR")
			m.setup = s
			return m, nil
		}
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		m.setup = s
		return m, cmd

	case stepCurrency:
		if msg.Type == tea.KeyEnter {
			s.tmpCfg.Currency = s.input.Value()
			if s.tmpCfg.Currency == "" {
				s.tmpCfg.Currency = "EUR"
			}
			m.cfg = s.tmpCfg
			_ = config.Save(m.cfg)
			m.client = clickup.New(m.cfg.Token)
			m.home = newHome(m.year, m.month)
			m.screen = screenHome
			return m, nil
		}
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		m.setup = s
		return m, cmd
	}
	return m, nil
}

func (s setupModel) view() string {
	b := styleTitle.Render("Setup ClickUp Hours CLI") + "\n\n"
	switch s.step {
	case stepToken:
		b += "Incolla il tuo token API personale:\n\n" + s.input.View()
		if s.msg != "" {
			b += "\n\n" + styleHelp.Render(s.msg)
		}
	case stepWorkspace:
		b += "Scegli il workspace:\n\n"
		for i, t := range s.teams {
			cursor := "  "
			line := t.Name + " (" + t.ID + ")"
			if i == s.teamIdx {
				cursor = "▸ "
				line = styleAccent.Render(line)
			}
			b += cursor + line + "\n"
		}
	case stepRate:
		b += s.input.View()
	case stepCurrency:
		b += s.input.View()
	}
	b += "\n\n" + styleHelp.Render("Enter: conferma · Ctrl+C: esci")
	return b
}

// validateAndLoadTeamsCmd valida il token (CurrentUser) e carica i teams.
func validateAndLoadTeamsCmd(c *clickup.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if _, err := c.CurrentUser(ctx); err != nil {
			return errMsg{err: err}
		}
		teams, err := c.Teams(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return teamsMsg{teams: teams}
	}
}

func newTextInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.Width = 40
	return ti
}

func newNumberInput(placeholder string) textinput.Model {
	ti := newTextInput(placeholder)
	ti.CharLimit = 10
	return ti
}
```

Nota: l'`errMsg` durante il setup porta a `screenError`; da lì un tasto torna a `screenHome`. Se vuoi che torni al setup, in `routeKey` per `screenError` puoi controllare `!m.cfg.Valid()` e in tal caso rimettere `screenSetup`. Implementa questa riga:

In `app.go`, nel case `screenError` di `routeKey`, sostituisci con:

```go
	case screenError:
		if !m.cfg.Valid() {
			m.screen = screenSetup
			m.setup = newSetup()
		} else {
			m.screen = screenHome
		}
		return m, nil
```

- [ ] **Step 4: Esegui i test (devono passare)**

Run: `go test ./internal/tui/ -v && go build ./...`
Expected: PASS + build ok.

- [ ] **Step 5: Commit**

```bash
git add internal/tui
git commit -m "feat: TUI setup wizard (token, workspace, rate, currency)"
```

---

## Task 8: TUI — Home (picker mese + scope)

**Files:**
- Create: `internal/tui/home.go`
- Modify: `internal/tui/stubs.go` (rimuovi gli stub di `homeModel`, `newHome`, `home.view`, `updateHome`)

**Interfaces:**
- Produces:
  - `type homeModel struct { ... }`, `func newHome(year int, month time.Month) homeModel`
  - `func (h homeModel) view() string`
  - `func (m Model) updateHome(msg tea.KeyMsg) (tea.Model, tea.Cmd)` — frecce sx/dx cambiano mese; `t` alterna scope me/team; `Enter` avvia `loadEntriesCmd` e passa a `screenLoading`.

- [ ] **Step 1: Scrivi il test (fallisce)**

Add to `internal/tui/app_test.go`:

```go
func TestHomeChangesMonthAndScope(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1"})
	m.year, m.month = 2026, 7
	m.home = newHome(2026, 7)

	// freccia sinistra -> mese precedente
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	mm := updated.(Model)
	if mm.month != 6 {
		t.Fatalf("left should go to June, got %v", mm.month)
	}

	// 't' alterna scope
	updated2, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	mm2 := updated2.(Model)
	if mm2.scope != "team" {
		t.Fatalf("t should switch scope to team, got %q", mm2.scope)
	}
}

func TestHomeEnterStartsLoading(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1"})
	m.home = newHome(m.year, m.month)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(Model)
	if mm.screen != screenLoading {
		t.Fatalf("enter should switch to loading, got %v", mm.screen)
	}
	if cmd == nil {
		t.Fatal("enter should return a load command")
	}
}
```

- [ ] **Step 2: Esegui (fallisce)**

Run: `go test ./internal/tui/ -run TestHome -v`
Expected: FAIL.

- [ ] **Step 3: Implementa `home.go`**

Rimuovi da `stubs.go` gli stub `homeModel`, `newHome`, `homeModel.view`, `updateHome`. Poi create `internal/tui/home.go`:

```go
package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type homeModel struct {
	year  int
	month time.Month
	scope string
}

func newHome(year int, month time.Month) homeModel {
	return homeModel{year: year, month: month, scope: "me"}
}

func (m Model) updateHome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "h":
		m.month--
		if m.month < time.January {
			m.month = time.December
			m.year--
		}
	case "right", "l":
		m.month++
		if m.month > time.December {
			m.month = time.January
			m.year++
		}
	case "t":
		if m.scope == "me" {
			m.scope = "team"
		} else {
			m.scope = "me"
		}
	case "enter":
		m.home.year, m.home.month, m.home.scope = m.year, m.month, m.scope
		var assignees []int
		if m.scope == "team" {
			// tutti i membri del workspace selezionato
			for _, t := range teamMembersFor(m) {
				assignees = append(assignees, t)
			}
		}
		m.assignees = assignees
		m.screen = screenLoading
		return m, loadEntriesCmd(m.client, m.cfg.WorkspaceID, m.year, m.month, assignees)
	}
	m.home.year, m.home.month, m.home.scope = m.year, m.month, m.scope
	return m, nil
}

func (h homeModel) view() string {
	title := styleTitle.Render("ClickUp Hours — Report mensile")
	sel := styleBox.Render(fmt.Sprintf("Mese: %s  ◂ %04d-%02d ▸    Scope: %s",
		styleAccent.Render(h.month.String()), h.year, int(h.month), styleAccent.Render(h.scope)))
	help := styleHelp.Render("◂/▸ cambia mese · t: me/team · Enter: genera report · q: esci")
	return title + "\n\n" + sel + "\n\n" + help
}

// teamMembersFor ritorna gli id dei membri del workspace corrente (per lo scope team).
// In v1.0 i teams sono caricati nel setup; qui li ricarichiamo pigramente se assenti.
func teamMembersFor(m Model) []int {
	// La lista membri completa vive in setupModel dopo il setup; per robustezza,
	// in v1.0 se non disponibile ritorniamo nil (l'API senza assignee = solo utente).
	return nil
}
```

Nota v1.0: lo scope "team" richiede la lista dei membri. Per non complicare il flusso, in v1.0 `teamMembersFor` ritorna `nil` → l'API senza `assignee` restituisce comunque le voci visibili al token (che per un admin includono il team). Il filtro puntuale per-membro (con selezione multipla) è previsto come raffinamento in v1.3. Documenta questo comportamento nel README.

- [ ] **Step 4: Esegui i test (devono passare)**

Run: `go test ./internal/tui/ -v && go build ./...`
Expected: PASS + build ok.

- [ ] **Step 5: Commit**

```bash
git add internal/tui
git commit -m "feat: TUI home screen (month picker, scope toggle, load)"
```

---

## Task 9: TUI — Vista report (tabella + grouping + keybinding)

**Files:**
- Create: `internal/tui/report.go`
- Modify: `internal/tui/stubs.go` (rimuovi gli stub di `reportModel`, `newReport`, `report.view`, `updateReport`)
- Modify: `internal/tui/app.go` (il case `entriesMsg` già crea `m.rep = newReport(...)`)

**Interfaces:**
- Consumes: `report.Report`, `report`.
- Produces:
  - `type reportModel struct { r report.Report }`, `func newReport(r report.Report) reportModel`
  - `func (rm reportModel) view() string`
  - `func (m Model) updateReport(msg tea.KeyMsg) (tea.Model, tea.Cmd)` — `g` cicla groupBy (total→task→list→day→total) ricostruendo il report dai raw entries; `m`/`s` tornano alla home; `e` apre l'export; `r` ricarica.
  - il Model deve conservare i **raw entries** per poter ri-raggruppare: aggiungi campo `entries []report.TimeEntry` al `Model` e salvalo nel case `entriesMsg`.

- [ ] **Step 1: Aggiorna `app.go` per conservare i raw entries**

In `app.go`, aggiungi al `Model` il campo:

```go
	entries []report.TimeEntry
```

e nel case `entriesMsg`:

```go
	case entriesMsg:
		m.entries = msg.entries
		m.report = report.Build(msg.entries, report.GroupByTotal, m.cfg.Rate, m.cfg.Currency, m.year, m.month)
		m.report.Scope = m.scope
		m.rep = newReport(m.report)
		m.screen = screenReport
		return m, nil
```

- [ ] **Step 2: Scrivi il test (fallisce)**

Add to `internal/tui/app_test.go`:

```go
func TestReportCycleGroupBy(t *testing.T) {
	m := New(config.Config{Token: "t", WorkspaceID: "1", Rate: 10, Currency: "EUR"})
	m.year, m.month = 2026, 7
	updated, _ := m.Update(entriesMsg{entries: []report.TimeEntry{
		{TaskName: "A", ListName: "L", Start: time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), Duration: time.Hour},
	}})
	mm := updated.(Model)
	if mm.report.GroupBy != report.GroupByTotal {
		t.Fatalf("initial groupBy should be total, got %q", mm.report.GroupBy)
	}
	// 'g' -> task
	updated2, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	mm2 := updated2.(Model)
	if mm2.report.GroupBy != report.GroupByTask {
		t.Fatalf("after g groupBy should be task, got %q", mm2.report.GroupBy)
	}
}
```

(assicurati che `app_test.go` importi `time` e `report`.)

- [ ] **Step 3: Esegui (fallisce)**

Run: `go test ./internal/tui/ -run TestReportCycle -v`
Expected: FAIL.

- [ ] **Step 4: Implementa `report.go`**

Rimuovi da `stubs.go` gli stub `reportModel`, `newReport`, `reportModel.view`, `updateReport`. Poi create `internal/tui/report.go`:

```go
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

type reportModel struct {
	r report.Report
}

func newReport(r report.Report) reportModel { return reportModel{r: r} }

// nextGroupBy cicla total -> task -> list -> day -> total.
func nextGroupBy(g string) string {
	switch g {
	case report.GroupByTotal:
		return report.GroupByTask
	case report.GroupByTask:
		return report.GroupByList
	case report.GroupByList:
		return report.GroupByDay
	default:
		return report.GroupByTotal
	}
}

func (m Model) updateReport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "g":
		g := nextGroupBy(m.report.GroupBy)
		m.report = report.Build(m.entries, g, m.cfg.Rate, m.cfg.Currency, m.year, m.month)
		m.report.Scope = m.scope
		m.rep = newReport(m.report)
	case "m", "s":
		m.screen = screenHome
	case "r":
		m.screen = screenLoading
		return m, loadEntriesCmd(m.client, m.cfg.WorkspaceID, m.year, m.month, m.assignees)
	case "e":
		m.export = newExport(m.report)
		m.screen = screenExport
	}
	return m, nil
}

func (rm reportModel) view() string {
	r := rm.r
	title := styleTitle.Render(fmt.Sprintf("Report %04d-%02d — scope %s — raggruppo per %s",
		r.Year, int(r.Month), r.Scope, r.GroupBy))

	header := lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("%-32s %8s %12s", "Voce", "Ore", "Importo"))
	rows := header + "\n"
	for _, b := range r.Buckets {
		rows += fmt.Sprintf("%-32s %8.2f %10.2f %s\n",
			truncate(b.Label, 32), b.Hours, b.Amount, r.Currency)
	}

	total := styleOK.Render(fmt.Sprintf("%-32s %8.2f %10.2f %s",
		"TOTALE", r.TotalHours, r.TotalAmount, r.Currency))

	body := styleBox.Render(rows + total)
	help := styleHelp.Render("g: raggruppamento · e: esporta · m/s: cambia mese/scope · r: ricarica · q: esci")

	if len(r.Buckets) == 0 {
		body = styleBox.Render("Nessuna ora tracciata in questo mese.")
	}
	return title + "\n\n" + body + "\n\n" + help
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
```

- [ ] **Step 5: Esegui i test (devono passare)**

Run: `go test ./internal/tui/ -v && go build ./...`
Expected: PASS + build ok.

- [ ] **Step 6: Commit**

```bash
git add internal/tui
git commit -m "feat: TUI report view with grouping cycle and keybindings"
```

---

## Task 10: TUI — Menu Export

**Files:**
- Create: `internal/tui/export.go`
- Modify: `internal/tui/stubs.go` (rimuovi gli ultimi stub: `exportModel`, `export.view`, `updateExport`; a questo punto `stubs.go` deve essere vuoto → **eliminalo**)

**Interfaces:**
- Consumes: `report.Report`, `export` package.
- Produces:
  - `type exportModel struct { r report.Report; idx int; done string; err error }`, `func newExport(r report.Report) exportModel`
  - `func (e exportModel) view() string`
  - `func (m Model) updateExport(msg tea.KeyMsg) (tea.Model, tea.Cmd)` — su/giù scelgono formato (CSV/JSON/Markdown); Enter scrive il file in cwd (`clickup-report-YYYY-MM.<ext>`) e mostra conferma; `esc` torna al report.

- [ ] **Step 1: Scrivi il test (fallisce)**

Add to `internal/tui/app_test.go`:

```go
func TestExportWritesFile(t *testing.T) {
	dir := t.TempDir()
	oldwd, _ := os.Getwd()
	defer os.Chdir(oldwd)
	os.Chdir(dir)

	m := New(config.Config{Token: "t", WorkspaceID: "1", Currency: "EUR"})
	m.year, m.month = 2026, 7
	m.report = report.Report{Year: 2026, Month: 7, Currency: "EUR",
		Buckets: []report.Bucket{{Label: "A", Hours: 1, Amount: 0}}, TotalHours: 1}
	m.export = newExport(m.report)
	m.screen = screenExport

	// Enter sul primo formato (csv)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(Model)
	if mm.export.err != nil {
		t.Fatalf("export error: %v", mm.export.err)
	}
	if _, err := os.Stat("clickup-report-2026-07.csv"); err != nil {
		t.Fatalf("expected csv file: %v", err)
	}
}
```

(assicurati che `app_test.go` importi `os`.)

- [ ] **Step 2: Esegui (fallisce)**

Run: `go test ./internal/tui/ -run TestExportWrites -v`
Expected: FAIL.

- [ ] **Step 3: Implementa `export.go` ed elimina `stubs.go`**

Delete `internal/tui/stubs.go` (dopo aver rimosso tutti gli stub rimasti). Create `internal/tui/export.go`:

```go
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/export"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

type exportFormat struct {
	label  string
	key    string
	ext    string
}

var exportFormats = []exportFormat{
	{"CSV", "csv", "csv"},
	{"JSON", "json", "json"},
	{"Markdown", "markdown", "md"},
}

type exportModel struct {
	r    report.Report
	idx  int
	done string
	err  error
}

func newExport(r report.Report) exportModel { return exportModel{r: r} }

func (m Model) updateExport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	e := m.export
	switch msg.String() {
	case "up", "k":
		if e.idx > 0 {
			e.idx--
		}
	case "down", "j":
		if e.idx < len(exportFormats)-1 {
			e.idx++
		}
	case "esc":
		m.screen = screenReport
		m.export = e
		return m, nil
	case "enter":
		f := exportFormats[e.idx]
		path := fmt.Sprintf("clickup-report-%04d-%02d.%s", e.r.Year, int(e.r.Month), f.ext)
		if err := export.ToFile(f.key, e.r, path); err != nil {
			e.err = err
		} else {
			e.err = nil
			e.done = path
		}
	}
	m.export = e
	return m, nil
}

func (e exportModel) view() string {
	b := styleTitle.Render("Esporta report") + "\n\n"
	for i, f := range exportFormats {
		cursor := "  "
		line := f.label
		if i == e.idx {
			cursor = "▸ "
			line = styleAccent.Render(line)
		}
		b += cursor + line + "\n"
	}
	if e.done != "" {
		b += "\n" + styleOK.Render("Salvato: "+e.done)
	}
	if e.err != nil {
		b += "\n" + styleErr.Render("Errore: "+e.err.Error())
	}
	b += "\n\n" + styleHelp.Render("↑/↓ scegli · Enter: esporta · Esc: torna al report")
	return b
}
```

- [ ] **Step 4: Esegui TUTTI i test e build**

Run: `go test ./... && go build ./...`
Expected: PASS su tutti i package + build ok.

- [ ] **Step 5: Commit**

```bash
git add internal/tui
git commit -m "feat: TUI export menu (CSV/JSON/Markdown to file)"
```

---

## Task 11: Rifinitura, verifica end-to-end e tag

**Files:**
- Modify: `README.md`
- Create: `.github/workflows/ci.yml` (opzionale ma consigliato)

**Interfaces:** nessuna nuova; consolidamento.

- [ ] **Step 1: `go vet` e formattazione**

Run:
```bash
gofmt -l . && go vet ./...
```
Expected: nessun file listato da `gofmt`, nessun warning da `vet`. Se `gofmt -l` elenca file, esegui `gofmt -w .`.

- [ ] **Step 2: Esegui la suite completa con race detector**

Run:
```bash
go test ./... -race
```
Expected: PASS su tutti i package.

- [ ] **Step 3: Prova manuale end-to-end**

Con un token ClickUp reale in `CLICKUP_TOKEN` (o via setup):
```bash
CLICKUP_TOKEN=pk_xxx go run ./cmd/clickup
```
Verifica manualmente: setup (se serve) → home → scelta mese → report → `g` cicla i raggruppamenti → `e` esporta un CSV → il file `clickup-report-YYYY-MM.csv` compare nella cwd → `q` esce. Annota eventuali problemi e correggili prima di procedere.

- [ ] **Step 4: Espandi il README con keybinding e scope team**

Aggiorna `README.md` aggiungendo una sezione "Comandi nella TUI" (tabella dei tasti: `◂/▸`, `t`, `Enter`, `g`, `e`, `m/s`, `r`, `q`) e una nota sullo **scope team** (richiede permessi workspace; in v1.0 usa le voci visibili al token). Aggiungi anche la sezione "Configurazione" che documenta `~/.config/clickup-cli/config.yml` e l'env `CLICKUP_TOKEN`.

- [ ] **Step 5: (Opzionale) CI GitHub Actions**

Create `.github/workflows/ci.yml`:

```yaml
name: ci
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - run: go vet ./...
      - run: go test ./... -race
      - run: go build ./...
```

- [ ] **Step 6: Commit e tag v0.1.0**

```bash
git add README.md .github/workflows/ci.yml
git commit -m "docs: keybindings, config and team-scope notes; add CI"
git tag v0.1.0
```

---

## Self-Review (svolto in fase di scrittura del piano)

**1. Copertura spec:**
- Scope self+team → Task 8 (toggle) + Task 5 (assignee). *Nota:* selezione puntuale membri rimandata a v1.3 (documentato in Task 8).
- Report mensile, default sintesi mese → Task 6 (`GroupByTotal` iniziale) + Task 9.
- Raggruppamenti task/lista/giorno/totale → Task 3 + ciclo in Task 9.
- Tariffa unica → importo → Task 3 (billing) + config Task 2.
- Export CSV/JSON/MD → Task 4 + Task 10.
- Setup wizard → Task 7.
- API v2 (user/team/time_entries, rate limit) → Task 5.
- Testing (TDD core + httptest + Update()) → Task 2–10.
- Struttura file → rispecchia la sezione File Structure della spec.

**2. Placeholder scan:** nessun "TBD/TODO". Le note su v1.1/v1.3 (nome lista, selezione membri) sono scelte di scope esplicite e documentate, non lacune.

**3. Type consistency:** `report.TimeEntry`, `report.Report`, `report.Bucket`, `GroupBy*`, `Build`, `MonthRange` usati coerentemente tra `report`, `clickup`, `export`, `tui`. Firme `newHome(year int, month time.Month)`, `newReport(report.Report)`, `newExport(report.Report)`, `loadEntriesCmd(...)`, `config.Config` coerenti.

**Rischio noto:** l'ordine di implementazione TUI usa `stubs.go` per tenere il package compilabile tra Task 6 e Task 10. Ogni task 7–10 rimuove i propri stub; il Task 10 elimina il file. Segui l'ordine dei task per evitare simboli duplicati.
