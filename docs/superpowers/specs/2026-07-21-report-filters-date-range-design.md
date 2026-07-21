# Design — Filtri report + range date custom (#2, milestone v1.4)

> Spec di progetto. Documento di design in italiano (convenzione `docs/superpowers/`).
> Tutto ciò che finisce nel codice (identificatori, commenti, stringhe UI, test) va in **inglese**.

## Obiettivo

Restringere il report per **intervallo di date arbitrario** (non solo il mese) e
per **lista/progetto, tag e status**. I filtri sono lato client sul set di entry
già caricate; l'intervallo di date cambia il caricamento a monte.

Issue: [#2](https://github.com/marcoarnulfo/clickup-cli/issues/2) — milestone `v1.4`.

## Contesto (stato attuale, post-#3)

- La Home tiene `year int`, `month time.Month`, `scope` sul root `Model`. `◂/▸`
  scorrono i mesi; `Enter` → `screenLoading` → `reloadEntriesCmd`.
- `reloadEntriesCmd` sceglie demo vs reale e calcola gli assignees (solo team).
  Il caricamento reale usa `report.MonthRange(year, month)` per `[start, end)` e
  chiama `clickup.TimeEntries(ctx, teamID, start, end, assignees)`.
- `report.Build(entries, groupBy, rates, currency, year, month)` (puro): `year`
  e `month` finiscono **solo** in `Report.Year`/`Report.Month` (display); il
  raggruppamento per giorno usa `e.Start`, non year/month.
- `reportModel.view` mostra `Report %04d-%02d — scope … — grouped by …`.
- Export: `clickup-report-%04d-%02d.<ext>` da `Report.Year/Month`.
- `clickup.TimeEntries` legge da `rawEntry` (task {id,name}, task_location.list_id,
  user, start, duration). **Nessun tag né status** nella risposta attuale.
- Cache nomi lista già presente (`Client.ListName`, una fetch per list_id unico).
- Selezione membri (#3): stato di sessione `selectedMembers`, filtro server-side
  via `assignee`. I nuovi filtri (client-side) si **compongono** con questa.

## Decisioni di design (dal brainstorming)

1. **Range date**: preset (This month, Last month, Last 7 days, Last 30 days,
   This week) + **Custom** (`From`/`To` in `YYYY-MM-DD`, `To` inclusivo).
2. **Filtri**: una **schermata Filters** (dal Report) a sezioni
   **Lists / Tags / Statuses**, checklist per sezione.
3. **Enrichment**: **tag** col load (`include_task_tags=true`); **status** lazy
   all'apertura di Filters (fetch per-task unico, cache di sessione).
4. **Semantica filtri**: dentro una dimensione = **OR**; tra dimensioni = **AND**;
   dimensione vuota = nessun vincolo. Default: niente selezionato = nessun filtro.
5. **Stato di sessione** (nessuna persistenza su config), coerente con la #3.

## Architettura

```
Home
  Range: [This month] ◂▸   Scope: me            ← preset picker + ◂/▸ (mese)
  'd' -> preset list / custom From-To inputs
  Enter -> screenLoading -> reloadEntriesCmd (range [start,end), +include_task_tags)
       -> entriesMsg (entries con Tags)  -> Report

Report
  'f' -> screenFilters
           (first open) -> statusEnrichCmd "Loading statuses…" -> statusesMsg
           Lists / Tags / Statuses checklists (Tab tra sezioni, Space toggle)
           Enter apply -> rebuild report from report.Filter(entries, criteria)
           Esc cancel
  title: "<period> — scope … — grouped by …  · filtered"  (· filtered se attivo)
```

## Componenti e modifiche per package

### `internal/report` (puro)

- `TimeEntry` acquisisce `Tags []string` e `Status string`.
- `Report` sostituisce `Year int`/`Month time.Month` con `Start, End time.Time`
  (il periodo `[Start, End)`). Il raggruppamento per giorno resta su `e.Start`.
- `Build` cambia firma:
  `Build(entries []TimeEntry, groupBy string, rates Rates, currency string, start, end time.Time) Report`.
  Imposta `r.Start=start`, `r.End=end`. Nessun'altra logica cambia.
- Nuova `FilterCriteria struct { Lists, Tags, Statuses map[string]bool }` e
  funzione **pura** `Filter(entries []TimeEntry, c FilterCriteria) []TimeEntry`:
  un'entry passa se, per **ogni** dimensione con almeno una voce selezionata,
  l'entry matcha (lista in `Lists`; **almeno un** tag in `Tags`; status in
  `Statuses`). Dimensione vuota = nessun vincolo. `c.Empty()` → nessun filtro.
- Helper **puri** per il periodo (prendono `now` esplicito, restano testabili):
  - `RangeForPreset(preset string, year int, month time.Month, now time.Time) (start, end time.Time)`
    per i preset non-custom (`this_month` usa year/month; gli altri usano `now`).
  - `PeriodLabel(start, end time.Time) string`: `"January 2006"` se `[start,end)`
    è esattamente un mese di calendario, altrimenti
    `"2006-01-02 → 2006-01-02"` (secondo giorno = `end-1g`, inclusivo).
  - `PeriodFileSlug(start, end time.Time) string`: `"2006-01"` per un mese,
    `"2006-01-02_2006-01-02"` altrimenti (per il nome file export).
  Costanti preset esportate: `PresetThisMonth`, `PresetLastMonth`, `PresetLast7d`,
  `PresetLast30d`, `PresetThisWeek` (+ `PresetCustom` gestito dalla TUI).

Definizioni preset (tutte in UTC, half-open `[start,end)`):
- `this_month`: `MonthRange(year, month)`.
- `last_month`: mese di calendario precedente a `now`.
- `last_7d`: `[midnight(now)-6g, midnight(now)+1g)` (7 giorni incl. oggi).
- `last_30d`: `[midnight(now)-29g, midnight(now)+1g)`.
- `this_week`: `[lunedì di questa settimana 00:00, +7g)` (settimana ISO piena).

### `internal/clickup`

- `TimeEntries` aggiunge il query param `include_task_tags=true`; `rawEntry`
  acquisisce `task.tags[].name` → `TimeEntry.Tags`.
- Nuovo `TaskStatus(ctx, taskID string) (string, error)`: `GET /task/{id}`,
  legge `status.status` (stringa). Errore/inaccessibile gestito dal chiamante.

### `internal/tui`

**Range date (Home + app.go):**
- Root Model: `preset string` (default `report.PresetThisMonth`),
  `customStart, customEnd time.Time` (per il preset custom). `year/month`
  restano per il preset mensile e il `◂/▸`.
- `m.currentRange() (start, end time.Time)`: se `preset == custom` usa
  `customStart..customEnd+1g`; altrimenti `report.RangeForPreset(preset, year,
  month, time.Now())`. Demo: `reloadEntriesCmd` passa comunque il range calcolato
  ai dati demo (le entry demo sono già dentro il mese scelto — il filtro per range
  in demo si applica sul campo `Start`).
- Home: `d` apre un piccolo selettore preset (lista) e, per `custom`, due input
  `From`/`To` (validati `YYYY-MM-DD`); la view mostra `Range: <preset/period>`.
  `◂/▸` restano attivi solo per il preset mensile.
- `reloadEntriesCmd`/`loadEntriesCmd` usano `m.currentRange()` invece di
  `MonthRange(year, month)`.

**Schermata Filters (nuovo `internal/tui/filters.go`, `screenFilters`):**
- `filtersModel`: le tre sezioni (Lists/Tags/Statuses) come liste di voci con
  checkbox, l'indice di sezione attiva e di riga, `loadingStatuses bool`, e i tre
  set `selected` (copie difensive di quelli sul root, così `Esc` scarta).
- Le **opzioni** derivano dalle entry: Lists = union `ListName` (fallback id);
  Tags = union `Tags`; Statuses = union `Status` (dopo l'enrichment).
- Navigazione: `Tab`/`shift+Tab` cambia sezione, `↑/↓` la riga, `Space` toggle,
  `a` all/none nella sezione, `Enter` applica (scrive i set sul root + ricostruisce
  il report), `Esc` annulla.
- Apertura da Report (`f`): se gli status non sono ancora in cache →
  `loadingStatuses=true` e `statusEnrichCmd` (demo: `demoStatusEnrichCmd`); a
  `statusesMsg` si popola la cache e la sezione Statuses.
- Messaggi tipizzati: `statusesMsg struct{ byTask map[string]string }` (status per
  task id) o `errMsg`. La cache status vive sul root (`taskStatus map[string]string`).

**Applicazione del filtro:**
- Il report si costruisce sempre da `report.Build(report.Filter(m.entries,
  m.filterCriteria()), groupBy, rates, currency, start, end)`. `filterCriteria()`
  compone i tre set di sessione. Vale per il load (`entriesMsg`), il reload, il
  cambio raggruppamento (`g`), il salvataggio tariffe (`p`→`s`) e l'export.
- Enrichment status: quando arrivano gli status via `statusesMsg`, si assegna
  `e.Status` alle entry (per task id) prima di ricostruire.

**Report/Export:**
- `reportModel.view`: titolo con `report.PeriodLabel(Start,End)` + `· filtered`
  quando `!filterCriteria().Empty()`; la riga help aggiunge `f: filters`.
- Export: nome file `clickup-report-<PeriodFileSlug>.<ext>`.
- `newReport(r, note)` invariato; la nota membri e il marcatore `· filtered`
  coesistono nel titolo.

**Demo mode:** le entry demo acquisiscono `Tags` e `Status` plausibili;
`demoStatusEnrichCmd` restituisce gli status dai dati fittizi (nessuna I/O).

## Composizione e invarianti

- **Filtri × membri**: la selezione membri (#3, server-side) e i filtri
  (client-side) si compongono; il report è `Build(Filter(entries_filtrati_membri))`.
- **Importi/tariffe e raggruppamento invariati**: `Build` ricalcola bucket,
  importi e totali sul set filtrato; le tariffe per-lista continuano ad applicarsi.
- **Purezza** di `internal/report` mantenuta: `Filter`, `RangeForPreset`,
  `PeriodLabel`, `PeriodFileSlug` sono pure (nessun `time.Now()` interno).
- **Empty = nessun filtro**: `FilterCriteria.Empty()` → il report è identico a oggi.

## Gestione errori

- `statusEnrichCmd` fallisce → `errMsg` (auth → wizard; altro → screenError,
  come sempre). Status di un task non recuperabile → `""` (non matcha filtri status).
- Input date custom non valido → messaggio sulla schermata Home, nessun load.

## Testing (TDD, table-driven)

- `report`: `Filter` (OR intra-dimensione, AND inter-dimensione, vuoto=tutti, tag
  multipli, status/lista mancanti); `RangeForPreset` per ogni preset con `now`
  fisso; `PeriodLabel`/`PeriodFileSlug` (mese vs custom, `To` inclusivo);
  `Build` con la nuova firma start/end.
- `clickup`: parse `Tags` da `include_task_tags`; `TaskStatus` (httptest, campo
  `status.status`); assenza tag → `Tags` vuoto.
- `tui`: Home preset switch + input custom (validazione, `currentRange`);
  `filtersModel` (Tab sezioni, Space toggle, all/none, apply scrive i set + Esc
  scarta); apertura Filters lazy status (`statusesMsg` popola cache/sezione);
  `filterCriteria`/`· filtered`/`PeriodLabel` nel titolo; export slug periodo;
  demo status enrichment.

## Fuori scope

- Persistenza di range/filtri su `config.yml`.
- Status "al momento dell'entry" (usiamo lo status **corrente** del task).
- Filtri su assegnatari singoli via questa schermata (resta la selezione membri #3).
- Ricerca testuale libera per task/nota.

## Note di realizzazione

- Feature ampia (~9–12 task): un unico ciclo, ma i task ordinati così che il
  **range date** atterri e sia testabile prima dei **filtri**, e l'**enrichment
  status** dopo la schermata. Il cambio di firma di `Build` (year/month → start/end)
  è un task cross-cutting a sé, con aggiornamento di tutti i call site e test.
- La #3 è già su `main`; la #2 parte da lì (branch `v1.4-report-filters`).
