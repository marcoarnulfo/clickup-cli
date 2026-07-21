# Log ore (v1.2) — Design

- **Data:** 2026-07-21
- **Stato:** Approvato (roadmap; implementazione successiva ai residui di backlog)
- **Rilascio target:** v1.2
- **Dipende da:** v1.1 (risoluzione nomi lista, cache client)

## 1. Sommario e obiettivo

Aggiungere alla CLI `clickup` la capacità di **creare** time entry su ClickUp
direttamente dalla TUI (finora l'app solo legge). Una schermata "Log ore" offre
tre modalità di inserimento per coprire esigenze diverse:

1. **Guidato** — scegli una lista, poi un task nella lista, poi durata/data/nota.
2. **Task ID/URL** — incolli l'ID (o un URL ClickUp) del task e vai diretto al form.
3. **Timer** — start/stop di un cronometro che, allo stop, registra la time entry.

Si loggano **le proprie** ore (utente autenticato). Dopo il log si offre di
ricaricare il report così le nuove ore compaiono subito.

## 2. Requisiti

### Funzionali
- **Accesso:** dalla vista report e dalla home, tasto **`n`** apre la schermata Log ore.
- **Scelta modalità:** `1) Guidato · 2) Task ID/URL · 3) Timer`.
- **Guidato:** picker lista (dalle liste note: report corrente ∪ config) → picker task
  della lista (`GET /list/{id}/task`) → form finale.
- **Task ID/URL:** input di un ID task o di un URL ClickUp (da cui si estrae l'ID) → form.
- **Timer:** selezione del task (via guidato o ID) → `POST …/time_entries/start`;
  visualizzazione del cronometro; `POST …/time_entries/stop` registra. All'apertura,
  se esiste un timer in corso (`GET …/time_entries/current`) lo si mostra e si consente lo stop.
- **Form finale:** durata (parser flessibile), data (default oggi, editabile), nota opzionale;
  `POST /team/{id}/time_entries` con `{start, duration, tid, description}`.
- **Parser durata:** accetta `2h30`, `2h30m`, `1.5h`, `1,5h`, `90m`, `45` (numero nudo = ore).
- **Post-log:** conferma dell'esito; opzione per ricaricare il report corrente.

### Non funzionali
- Validazione degli input (durata > 0, data valida) con messaggi a schermo, coerente
  con le altre schermate (setup/tariffe).
- Nessun blocco su errori API: messaggio chiaro, si resta sulla schermata.

### Fuori scope (v1.2)
- Log per altri utenti (assignee diverso da sé).
- Picker su **tutte** le liste del workspace (richiederebbe di percorrere Space/Folder);
  in v1.2 il picker copre le liste note, il resto passa per il percorso ID.
- Flag **billable** (i nostri importi si calcolano con le tariffe; il flag ClickUp è
  ridondante qui) → eventuale backlog.
- Modifica/cancellazione di time entry esistenti.

## 3. Architettura

### Parser durata (puro)
Una funzione pura, testabile in TDD, isolata (in `internal/report` o un nuovo
`internal/duration`):

```go
func ParseDuration(s string) (time.Duration, error) // "2h30"|"1.5h"|"90m"|"45" -> Duration
```

Regole: `Nh`/`NhMm`/`NhM` (ore+minuti), `N.Nh` o `N,Nh` (ore decimali), `Nm` (minuti),
numero nudo = ore. Errore su input non riconosciuto o durata ≤ 0.

### Client `clickup` (nuove chiamate)
- `CreateTimeEntry(ctx, teamID, tid string, start time.Time, dur time.Duration, description string) error`
  → `POST /team/{team_id}/time_entries`, body `{start, duration, tid, description}` (ms).
- `ListTasks(ctx, listID string) ([]Task, error)` → `GET /list/{list_id}/task`
  (`Task{ID, Name string}`).
- **Timer:**
  - `StartTimer(ctx, teamID, tid, description string) error` → `POST /team/{id}/time_entries/start`.
  - `StopTimer(ctx, teamID string) (report.TimeEntry, error)` → `POST /team/{id}/time_entries/stop`.
  - `CurrentTimer(ctx, teamID string) (*RunningTimer, error)` → `GET /team/{id}/time_entries/current`
    (nil se nessun timer in corso).
- Utility per estrarre l'ID task da un URL ClickUp (`.../t/<id>` o `.../<id>`).

### TUI
Nuovo file `internal/tui/log.go` con `logModel` a sotto-stati:
`modeSelect → (listPick | idInput) → taskPick → form → (timerRunning)`.
Nuovo `screenLog`; il tasto `n` in `report.go`/`home.go` apre `newLog(...)`.
La schermata riusa i pattern esistenti (list navigabili, `textinput`, help line, `styleErr`).
I comandi API girano come `tea.Cmd` (goroutine) e restituiscono msg tipizzati
(`taskListMsg`, `logDoneMsg`, `timerMsg`, `errMsg`).

## 4. Flusso (schermate)

1. `n` → **Log ore / scelta modalità**.
2. **Guidato:** lista (picker) → task (picker, caricato via `ListTasks`) → **Form**.
   **ID:** input ID/URL → **Form**. **Timer:** seleziona task (guidato/ID) → schermata timer.
3. **Form:** durata + data + nota → invio → `CreateTimeEntry` → **conferma**.
4. **Timer:** start → cronometro a schermo → stop → conferma (entry creata da ClickUp).
5. **Conferma:** esito + `r` per ricaricare il report corrente / `Esc` per tornare.

## 5. Testing
- **`ParseDuration`** (TDD): tutti i formati + errori (input non valido, durata 0/negativa).
- **`clickup`** (httptest): `CreateTimeEntry` (body corretto: ms, tid, description),
  `ListTasks` (parsing), timer start/stop/current, estrazione ID da URL.
- **`tui`**: `logModel` — transizioni di stato (scelta modalità, selezioni, submit),
  gestione errori, avvio/stop timer via `Update()` + msg simulati.

## 6. Roadmap impact
- Chiude la voce **v1.2 "Log ore rapido"** della roadmap.
- Il picker "tutte le liste del workspace" e il flag billable diventano voci di backlog.

## 7. Decisioni chiave prese
1. **Tre modalità** (guidato + ID + timer) in v1.2 (scelta esplicita dell'utente).
2. **Solo le proprie ore.**
3. **Tasto `n`** da report e home.
4. **Parser durata flessibile** (`2h30`/`1.5h`/`90m`/`45`).
5. **Data default oggi**, editabile.
6. **Billable omesso**, picker liste limitato alle liste note (fallback su ID).
7. **Post-log**: offerta di ricaricare il report.
