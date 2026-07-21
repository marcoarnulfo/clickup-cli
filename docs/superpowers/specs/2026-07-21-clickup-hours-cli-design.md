# ClickUp Hours CLI — Design (v1)

- **Data:** 2026-07-21
- **Stato:** Approvato (in attesa di piano d'implementazione)
- **Autore:** marcoarnulfo + Claude

## 1. Sommario e obiettivo

Una **TUI (applicazione da terminale) in Go**, moderna e colorata, con un unico
scopo: essere **il miglior strumento per il report ore mensile di ClickUp**.

Il tool interroga la ClickUp API v2, aggrega le ore tracciate nel mese scelto
(per l'utente autenticato o per il team), e mostra un report leggibile con il
calcolo dell'**importo da fatturare** in base a una tariffa oraria. I report
sono esportabili in CSV, JSON e Markdown.

Progetto **open source** (licenza MIT), distribuito come **binario singolo**
scaricabile ed eseguibile senza dipendenze runtime.

### Perché esiste
Esistono già CLI generaliste per ClickUp (krodak/clickup-cli,
triptechtravel/clickup-cli, willmarple/clickup) che fanno "tutto" (task, sprint,
commenti, docs...). **Nessuna** è focalizzata sul reporting ore come caso d'uso
principale. Questo progetto occupa quella nicchia scoperta: *fare una cosa
benissimo* invece di essere l'ennesima CLI generalista.

## 2. Requisiti (v1)

### Funzionali
- **Scope self + team:** di default le ore dell'utente autenticato; opzione per
  vedere le ore del team/membri selezionati.
- **Report mensile:** selezione del mese/anno. Il default a schermo è la
  **sintesi del mese** (ore totali + importo).
- **Raggruppamenti** (ciclabili nella vista): per **task**, per
  **progetto/lista**, per **giorno**, oppure **solo totale**.
- **Tariffa oraria unica** (salvabile in config, valuta configurabile) →
  calcolo dell'importo da fatturare.
- **Export** del report corrente in **CSV / JSON / Markdown** (azione interna
  alla TUI, tasto `e`).

### Non funzionali
- **TUI-first pura:** l'interazione avviene sempre dentro l'interfaccia
  full-screen, colorata, navigabile da tastiera. Nessuna modalità headless in
  v1 (export = azione da tasto, non da flag).
- **Semplice di default, potente su richiesta:** all'apertura mostra la sintesi;
  il dettaglio si ottiene con i keybinding.
- Gestione pulita di errori, stati vuoti e rate limit API.

### Fuori scope (v1)
- Tariffe diverse per progetto/cliente (→ v1.1).
- Creazione/log di time entry (→ v1.2).
- Filtri avanzati e range date custom oltre al mese (→ v1.3).
- Uso headless/scriptabile via flag (`clickup export ...`): esplicitamente
  escluso, l'utente ha scelto TUI-first puro.

## 3. Architettura

Strategia scelta: **TUI-first pura**. Un unico binario `clickup` che apre
un'applicazione da terminale. L'export è un'azione interna (menu su tasto `e`).

### Principio di isolamento
Il cuore logico è **puro e senza I/O**, la TUI è un guscio sottile sopra.
Questo rende la logica facile da testare e da fidarsi.

```
clickup-cli/
  cmd/clickup/main.go        # entrypoint: carica config, avvia la TUI
  internal/
    clickup/                 # client HTTP verso ClickUp API v2
      client.go              #   costruzione richieste, auth header, rate limit
      user.go                #   GET /user  (utente autenticato)
      teams.go               #   GET /team  (workspace + membri)
      timeentries.go         #   GET /team/{id}/time_entries
    report/                  # LOGICA PURA (nessun I/O) — TDD
      model.go               #   TimeEntry, aggregati
      aggregate.go           #   byTask / byList / byDay / total
      billing.go             #   ore -> importo (tariffa * ore)
    export/                  # serializzazione del report
      csv.go
      json.go
      markdown.go
    config/                  # load/save config, gestione token
      config.go
    tui/                     # modelli bubbletea (Elm architecture)
      app.go                 #   root model, routing tra schermate
      setup.go               #   wizard primo avvio
      home.go                #   configuratore report (mese + scope)
      report.go              #   vista report + keybinding
      export.go              #   menu esporta
      styles.go              #   lipgloss styles condivisi
  go.mod
  README.md
  LICENSE                    # MIT
```

### Componenti e responsabilità

| Unità | Cosa fa | Dipende da |
|-------|---------|-----------|
| `clickup/` | Parla con la ClickUp API v2, restituisce dati grezzi tipizzati | `net/http`, config (token) |
| `report/` | Aggrega le time entry in viste; calcola l'importo. **Puro.** | nulla (solo tipi) |
| `export/` | Serializza un report aggregato in CSV/JSON/MD | `report/` |
| `config/` | Legge/scrive config, risolve il token (file o env) | filesystem |
| `tui/` | Presenta tutto all'utente, gestisce input da tastiera | tutte le altre |

## 4. Stack tecnico

- **Go** (binario singolo, cross-compile per macOS/Linux/Windows).
- **Charm ecosystem:**
  - `bubbletea` — architettura TUI (Model/Update/View).
  - `bubbles` — componenti: `list`, `table`, `textinput`, `spinner`,
    `filepicker`, `help`.
  - `lipgloss` — stile, colori, bordi, tabelle.
- **`net/http`** standard per l'API (nessun SDK esterno necessario).
- **Config:** file `config.yml` sotto `os.UserConfigDir()` →
  `~/.config/clickup-cli/config.yml`. Token sovrascrivibile con env
  `CLICKUP_TOKEN`.
- **Licenza:** MIT. **Binario:** `clickup`.

## 5. Flusso TUI (schermate)

1. **Setup wizard** (primo avvio o config mancante/invalida)
   - Input token API personale → validazione live (`GET /user`).
   - Selezione workspace di default (lista da `GET /team`).
   - Tariffa oraria + valuta (opzionali).
   - Salvataggio config.
2. **Home / configuratore report**
   - Picker **mese/anno**.
   - **Scope:** `Me` oppure `Team` → selezione membri.
   - Avvio caricamento (spinner) → chiamata `time_entries`.
3. **Vista report**
   - Tabella `lipgloss`. **Default = sintesi mese** (ore totali + importo).
   - Keybinding:
     - `g` → cicla raggruppamento: task → progetto → giorno → totale
     - `e` → menu **Esporta** (CSV / JSON / Markdown) → scrive file → conferma path
     - `m` → cambia mese · `s` → cambia scope · `r` → ricarica
     - `?` → aiuto · `q` → esci
4. **Stati** vuoti/errore gestiti a schermo (nessuna ora, token invalido, errore rete).

## 6. Integrazione ClickUp API v2

| Endpoint | Uso |
|----------|-----|
| `GET /user` | Utente autenticato (id per lo scope "Me", validazione token) |
| `GET /team` | Lista workspace + membri (setup, scope team) |
| `GET /team/{team_id}/time_entries` | Voci di tempo del mese |

- Parametri chiave di `time_entries`: `start_date`, `end_date` (epoch **ms**),
  `assignee` (uno o più user id per lo scope team).
- Ogni voce restituisce: task, lista/progetto, utente, durata (ms), data inizio.
- **Scope team:** richiede permessi workspace adeguati per leggere le ore altrui;
  se mancano, mostrare messaggio chiaro.
- **Rate limit ClickUp** (~100 req/min sul free): backoff/retry sul client.
- **Auth:** header `Authorization: <personal_token>`.

## 7. Modello dati (report/)

```go
type TimeEntry struct {
    ID        string
    TaskID    string
    TaskName  string
    ListID    string
    ListName  string   // "progetto"
    UserID    int
    UserName  string
    Start     time.Time
    Duration  time.Duration
}

type Bucket struct {   // riga aggregata generica
    Label    string    // nome task / lista / giorno
    Hours    float64
    Amount   float64   // Hours * rate (se rate > 0)
}

type Report struct {
    Month     time.Month
    Year      int
    Scope     string       // "me" | "team"
    GroupBy   string       // "task" | "list" | "day" | "total"
    Buckets   []Bucket
    TotalHours  float64
    TotalAmount float64
    Currency  string
}
```

Le funzioni di aggregazione sono **pure**: `[]TimeEntry + rate + groupBy -> Report`.

## 8. Gestione errori

- **Token mancante/invalido** → rilancia il setup wizard con messaggio chiaro.
- **Permessi insufficienti** (scope team) → messaggio esplicito, fallback su "Me".
- **Nessuna ora nel mese** → stato vuoto amichevole ("Nessuna ora tracciata a <mese>").
- **Errore rete / API** → messaggio + possibilità di ricaricare (`r`).
- **Rate limit (429)** → backoff automatico con indicazione a schermo.

## 9. Testing

- **TDD** su `report/` (funzioni pure): aggregazioni per task/lista/giorno/totale,
  calcolo importo, arrotondamenti ore, mesi vuoti, entry a cavallo di giorni.
- **`clickup/`** testato con `httptest`: risposte mockate, parsing, gestione 429/errori.
- **`export/`** testato: CSV/JSON/MD generati corretti per report noti.
- **`tui/`** testato via `Update()` dei modelli bubbletea (transizioni di stato,
  keybinding) — i modelli sono unità testabili senza terminale reale.

## 10. Roadmap dei rilasci

- **v1.0** — TUI report ore self+team · grouping task/progetto/giorno/totale ·
  tariffa unica→importo · export CSV/JSON/MD · setup wizard.
- **v1.1** — Tariffe **per progetto/cliente**.
- **v1.2** — **Log ore rapido** da TUI (crea time entry via API).
- **v1.3** — Filtri (progetto/tag/status) + **range date custom** (non solo mese).
- **v2.0** — Funzioni adiacenti: riepiloghi settimanali, export fattura, multi-valuta.

## 11. Decisioni chiave prese

1. **Linguaggio: Go** — binario singolo, distribuzione pulita per open source.
2. **Scope: self + team** — default self, opzione team.
3. **Output: 4 formati** — terminale (nativo TUI) + CSV + JSON + Markdown.
4. **Raggruppamenti: tutti** — default sintesi mese, resto opzionale via keybinding.
5. **Tariffa: unica in v1** — per-progetto rimandata a v1.1.
6. **Architettura: TUI-first pura** — nessun uso headless in v1 (scelta esplicita).
7. **Focus mono-scopo** — niente clone generalista; reporting ore fatto benissimo.
