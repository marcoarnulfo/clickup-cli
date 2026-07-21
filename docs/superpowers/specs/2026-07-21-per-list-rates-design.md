# Tariffe per lista — Design (v1.1)

- **Data:** 2026-07-21
- **Stato:** Approvato (in attesa di piano d'implementazione)
- **Rilascio target:** v1.1
- **Dipende da:** v1.0 (spec `2026-07-21-clickup-hours-cli-design.md`)

## 1. Sommario e obiettivo

Estendere la CLI `clickup` per supportare **tariffe orarie diverse per lista/progetto**
ClickUp, invece dell'unica tariffa globale di v1.0. Il calcolo dell'importo da
fatturare usa la tariffa specifica di ciascuna lista, con fallback sulla tariffa
di default per le liste senza override.

La feature include anche la **risoluzione del nome leggibile delle liste** (finora
si mostrava il `list_id` numerico), necessaria perché la configurazione delle
tariffe avviene in una **schermata TUI interattiva** che deve mostrare nomi
comprensibili. Questo chiude anche l'item "Nome leggibile della lista" del backlog.

## 2. Requisiti

### Funzionali
- **Tariffe per lista:** una mappa `list_id → tariffa` in config, con fallback sulla
  tariffa di default (`rate`) per le liste non mappate.
- **Importo corretto su tutti i raggruppamenti:** l'importo si calcola **per singola
  entry** (`ore_entry × tariffa_della_lista_della_entry`) e si somma nei bucket. Così
  anche raggruppando per **task** o **giorno** — dove un bucket può contenere entry di
  liste diverse — l'importo totale del bucket è la somma corretta (tariffa "mista"
  pesata), non un'unica tariffa applicata al monte ore.
- **Nomi leggibili delle liste:** risolti via API ClickUp (`GET /list/{id}`) con cache,
  e usati sia nella schermata tariffe sia nel raggruppamento "per progetto" del report.
- **Schermata TUI tariffe:** raggiungibile dalla vista report premendo **`p`**. Mostra
  le liste presenti nel report corrente (unione con quelle già in config), con nome e
  tariffa attuale (override o default). Si naviga su/giù, `Enter` edita la tariffa di
  una lista (input numerico), `Esc` salva in config e torna al report **ricalcolandolo**
  con le nuove tariffe.
- **Discoverability del tasto `p`:** il tasto **`p`** che apre le tariffe deve essere
  **esplicitamente documentato** nella riga di help in fondo alla vista report, nella
  schermata tariffe stessa, e nella sezione "Comandi nella TUI" del README.

### Non funzionali
- **Valuta unica** (invariata da v1.0). La multi-valuta resta rimandata a v2.0.
- Retrocompatibile con la config v1.0: una config senza `rates` si comporta come oggi
  (tutto alla tariffa `rate`).
- La risoluzione dei nomi non deve rendere il caricamento fragile: se `GET /list/{id}`
  fallisce per una lista, si usa il `list_id` come nome (fallback), senza bloccare il
  report.

### Fuori scope (v1.1)
- Tariffe per Space/Folder o per "cliente" (gruppo di liste) — l'utente ha scelto la
  granularità **List**.
- Multi-valuta / valuta per lista (→ v2.0).
- Assegnazione tariffe a liste non ancora apparse in alcun report e non in config.

## 3. Architettura

### Modello dati (config)

`config.Config` guadagna un campo:

```go
type Config struct {
    Token       string             `yaml:"token"`
    WorkspaceID string             `yaml:"workspace_id"`
    Currency    string             `yaml:"currency"`
    Rate        float64            `yaml:"rate"`             // tariffa di default
    Rates       map[string]float64 `yaml:"rates,omitempty"`  // list_id -> tariffa override
}
```

`Rates` è opzionale: assente ⇒ comportamento v1.0.

### Risoluzione tariffa (report)

Nuovo tipo puro nel package `report`:

```go
type Rates struct {
    Default float64
    ByList  map[string]float64
}

func (r Rates) For(listID string) float64 {
    if v, ok := r.ByList[listID]; ok {
        return v
    }
    return r.Default
}
```

`report.Build` cambia firma: il parametro `rate float64` diventa `rates Rates`.
L'aggregazione resta invariata per le **ore**; l'**importo** di ogni bucket diventa
la somma, sulle entry del bucket, di `entry.Duration.Hours() × rates.For(entry.ListID)`
(arrotondata a 2 decimali come oggi). Il campo `Report.Rate float64` viene **rimosso**:
non serve più un rate scalare sul `Report` perché l'importo calcolato vive già in
`Bucket.Amount` (e in `Report.TotalAmount`); `Report.Currency` resta. Nessun nuovo
campo `Rates` sul `Report`.

Nota implementativa: poiché l'importo non è più `bucket.Hours × rate_unico`, il calcolo
va fatto accumulando per entry durante l'aggregazione, non a posteriori sui bucket.

### Risoluzione nomi lista (clickup)

Il `Client` guadagna una cache e un metodo:

```go
// cache: map[list_id]name, popolata lazily
func (c *Client) ListName(ctx context.Context, listID string) (string, error) // GET /list/{id}
```

In fase di caricamento delle entry (nel comando TUI `loadEntriesCmd`), per ogni
`list_id` unico si risolve il nome una sola volta (cache) e si valorizza
`TimeEntry.ListName`. Fallback: se la chiamata fallisce, `ListName = list_id`.

La cache vive sul `Client` per l'intera sessione. L'accesso alla cache deve essere
sicuro rispetto all'uso da un comando bubbletea (che gira in goroutine): proteggere la
mappa con un mutex sul Client.

### Schermata TUI tariffe

Nuovo screen `screenRates` e sotto-modello `ratesModel`:
- Costruito dalla vista report quando si preme `p`, ricevendo le liste del report
  corrente (id + nome) unite a quelle già presenti in `cfg.Rates`.
- Righe: `<nome lista>  <tariffa attuale> <valuta>` (indicando se è override o default).
- Keybinding: `↑/↓` naviga · `Enter` entra in editing della riga (input numerico) →
  `Enter` conferma il valore, `Esc` annulla l'editing · `s` o `Esc` (fuori editing)
  salva in `config` e torna al report **ricalcolato** · una riga di help documenta i tasti.
- Alla conferma, gli override vengono scritti in `cfg.Rates` (una tariffa uguale al
  default può essere omessa o salvata; scelta implementativa nel piano) e la config è
  persistita via `config.Save`.

### Integrazione flusso

- `Model` conserva già `entries` (da v1.0). Alla pressione di `p` nella vista report,
  si passa a `screenRates` con le liste correnti.
- Al salvataggio delle tariffe, si ricostruisce `report.Rates` da `cfg` e si richiama
  `report.Build(entries, groupBy, rates, currency, year, month)` mantenendo il
  `groupBy` corrente, poi si torna a `screenReport`.
- La riga di help della vista report diventa:
  `g: raggruppamento · e: esporta · p: tariffe · m/s: cambia mese/scope · r: ricarica · q: esci`.

## 4. Impatti su codice esistente

- `internal/config/config.go`: aggiungere `Rates`.
- `internal/report/model.go` + `aggregate.go`: `Rates` type, `Report` con `Rate float64`
  rimosso, `Build` con `rates Rates`, calcolo importo per-entry.
- `internal/clickup`: `ListName` + cache con mutex; il Client diventa non più
  banalmente value-copyable — usare sempre il puntatore (già così).
- `internal/tui`: nuovo `rates.go` (screen + model), modifiche a `report.go` (tasto `p`,
  help line), `app.go` (screen enum `screenRates`, routing, ricalcolo tariffe;
  risoluzione nomi in `loadEntriesCmd`; costruzione di `report.Rates` da `cfg`),
  aggiornamento delle chiamate a `report.Build`.
- `README.md`: sezione "Comandi nella TUI" con il tasto `p`; sezione "Configurazione"
  con la mappa `rates`.

## 5. Testing

- **`report`** (TDD): `Rates.For` (override/fallback); `Build` con tariffe per lista;
  **importo misto**: un bucket "per task" con entry di due liste a tariffe diverse →
  importo = somma pesata; retrocompatibilità (Rates con solo Default = comportamento v1.0).
- **`clickup`** (httptest): `ListName` risolve e **cache** (id ripetuto ⇒ una sola
  chiamata HTTP); fallback su errore.
- **`tui`**: `ratesModel` — navigazione, editing di una riga, salvataggio che scrive
  `cfg.Rates` e ricalcola il report; `p` dalla vista report apre `screenRates`.

## 6. Roadmap impact

- Chiude l'item **"Nome leggibile della lista"** del backlog v0.1.1.
- Sostituisce la voce **v1.1 "Tariffe per progetto/cliente"** della roadmap (con
  granularità List, come deciso).

## 7. Decisioni chiave prese

1. **Granularità: per List** (non Space/Folder né "cliente"-gruppo).
2. **Config tariffe: schermata TUI interattiva** (non solo file), tasto **`p`**.
3. **Nomi liste risolti via API** con cache (chiude l'item di backlog).
4. **Importo calcolato per-entry** e sommato nei bucket (corretto sui raggruppamenti misti).
5. **Valuta unica** (multi-valuta → v2.0).
6. **Scope schermata tariffe:** liste del report corrente ∪ liste già in config.
7. **Tasto `p` documentato** esplicitamente in help TUI + README (requisito dell'utente).
