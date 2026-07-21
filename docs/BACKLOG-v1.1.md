# Backlog — post v1.0

Raccolta dei miglioramenti emersi dalla review finale di v1.0 (review indipendente
con modello Opus) e dalla roadmap del piano. Nessuno di questi è bloccante per v1.0;
sono rimandati ai rilasci successivi.

## Hardening (dalla review finale v1.0)

Rilievi minori/importanti-non-bloccanti individuati dopo il merge di v1.0. L'unico
bloccante (token mostrato in chiaro nel wizard) è già stato risolto in v1.0
(`fix: mask API token input in setup wizard`).

- [ ] **Nome leggibile della lista** — `internal/clickup/timeentries.go`: l'endpoint
  `time_entries` restituisce solo `list_id`. Oggi usiamo l'ID come `ListName` (il
  raggruppamento "per progetto" funziona ma mostra l'ID). Risolvere il nome via
  `GET /list/{id}` con una cache `list_id -> name`.
- [ ] **`flexString` più robusto** — `internal/clickup/timeentries.go:16`: usa
  `strings.Trim(b, '"')`. Passare a `strconv.Unquote` per gestire caratteri
  escaped, e aggiungere una guardia per `list_id` JSON `null` (oggi diventerebbe
  la stringa letterale `"null"` come etichetta di un bucket).
- [ ] **Parsing durata/inizio con errore esplicito** — `internal/clickup/timeentries.go:64`:
  gli errori di `strconv.ParseInt` su `duration`/`start` sono scartati; un valore
  malformato dall'API produce silenziosamente una entry a durata zero datata
  all'epoch, distorcendo i totali. Propagare un errore invece di ingoiarlo.
- [ ] **Scope "team": errore se workspace non trovato** — `internal/tui/app.go`
  (`loadEntriesCmd`): se il workspace configurato non è tra quelli di `Teams()`,
  `assignees` resta vuoto e il report mostra silenziosamente solo le ore "me" sotto
  l'intestazione "team". Segnalare l'errore invece del fallback silenzioso.
- [ ] **Validazione input nel wizard** — `internal/tui/setup.go`: (a) `s.loading`
  non blocca l'input durante la validazione del token (l'utente può ridigitare/
  ri-inviare, lanciando una seconda validazione); (b) l'errore di `strconv.ParseFloat`
  sulla tariffa è scartato (input non valido → `0` silenzioso, senza feedback).
- [ ] **Stato duplicato in `homeModel`** — i campi `homeModel.scope/year/month`
  duplicano lo stato del root `Model`, sincronizzati a mano. Rimuoverli o far
  accettare a `newHome` lo scope, per evitare drift futuri.
- [ ] **Allineamento colonne tabella report** (cosmetico) — `internal/tui/report.go`:
  l'header usa `%12s` mentre le righe usano `%10.2f %s`; le colonne non sono
  allineate. Uniformare i formati.
- [ ] **Asserzione test più stretta** — `internal/report/aggregate_test.go`:
  `TestBuildByTaskSortedByHoursDesc` non asserisce `Buckets[1]`; verificare anche
  il secondo bucket per coprire completamente il tie-break dell'ordinamento.

## Roadmap funzionale (dal piano v1.0)

- **v1.1** — Tariffe **per progetto/cliente** (mappatura in config), oltre alla
  tariffa unica attuale.
- **v1.2** — **Log ore rapido** da TUI (creazione di time entry via API).
- **v1.3** — Filtri (progetto/tag/status) + **range date custom** (non solo mese);
  **selezione puntuale dei membri** del team (multiselezione), oggi lo scope "team"
  aggrega tutti i membri del workspace.
- **v2.0** — Funzioni adiacenti: riepiloghi settimanali, export fattura, multi-valuta.

## Note operative

- **Test e2e interattivo**: da eseguire con un token reale in un terminale vero
  (`CLICKUP_TOKEN=pk_xxx go run ./cmd/clickup`); non eseguibile in ambienti headless.
