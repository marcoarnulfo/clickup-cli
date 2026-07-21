# Backlog — post v1.0

Raccolta dei miglioramenti emersi dalla review finale di v1.0 (review indipendente
con modello Opus) e dalla roadmap del piano. Nessuno di questi è bloccante per v1.0;
sono rimandati ai rilasci successivi.

## Hardening (dalla review finale v1.0)

Rilievi minori/importanti-non-bloccanti individuati dopo il merge di v1.0. L'unico
bloccante (token mostrato in chiaro nel wizard) è già stato risolto in v1.0
(`fix: mask API token input in setup wizard`).

### ✅ Chiusi in v0.1.1

- [x] **`flexString` più robusto** — ora usa `encoding/json` (string/number/null);
  `null` → stringa vuota, stringhe de-escaped correttamente.
- [x] **Parsing durata/inizio con errore esplicito** — `TimeEntries` ora ritorna
  errore su `duration`/`start` malformati (i timer in corso restano skippati).
- [x] **Scope "team": errore se workspace non trovato** — `loadEntriesCmd` segnala
  l'errore invece del fallback silenzioso su "me".
- [x] **Validazione input nel wizard** — input bloccato durante la validazione del
  token; tariffa non numerica rifiutata con messaggio d'errore.
- [x] **Allineamento colonne tabella report** — header allineato alle righe dati.
- [x] **Asserzione test più stretta** — `TestBuildByTaskSortedByHoursDesc` verifica
  anche `Buckets[1]`.

### Ancora aperti

- [ ] **Nome leggibile della lista** — `internal/clickup/timeentries.go`: l'endpoint
  `time_entries` restituisce solo `list_id`. Oggi usiamo l'ID come `ListName` (il
  raggruppamento "per progetto" funziona ma mostra l'ID). Risolvere il nome via
  `GET /list/{id}` con una cache `list_id -> name`. (Da valutare con la feature v1.1.)
- [ ] **Stato duplicato in `homeModel`** — i campi `homeModel.scope/year/month`
  duplicano lo stato del root `Model`, sincronizzati a mano. Rimuoverli o far
  accettare a `newHome` lo scope, per evitare drift futuri.
- [ ] **Test path "happy" scope team** — manca un test che copra `loadEntriesCmd`
  con `found == true` (membri del team → `entriesMsg`); oggi è coperto solo il caso
  workspace-non-trovato. Gap preesistente, non regressione.

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
