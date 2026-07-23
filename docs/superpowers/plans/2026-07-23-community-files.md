# File di community (#108) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Aggiungere i due file di community che mancano davvero — `CHANGELOG.md` e `SECURITY.md` — e abilitare il canale privato di segnalazione vulnerabilità su GitHub.

**Architecture:** Nessun codice Go. Due file markdown alla radice più un'impostazione del repo. Il contenuto si ricostruisce da fonti reali (note di release pubblicate, `git log`, percorsi effettivi nel codice), non da modelli generici.

**Tech Stack:** Markdown, `gh` CLI per l'impostazione del repo.

**Spec:** `docs/superpowers/specs/2026-07-23-community-files-design.md` (leggerla; questo piano la implementa).

## Global Constraints

- **Tre voci dell'issue esistono già** — `CONTRIBUTING.md` + `CONTRIBUTING.it.md`, `CODE_OF_CONDUCT.md`, i template issue/PR — e **non vanno toccate**. Lo scope reale è `CHANGELOG.md`, `SECURITY.md` e l'impostazione del repo.
- **CHANGELOG in inglese soltanto.** `CLAUDE.md` stabilisce che il testo in-repo è in inglese tranne `README.it.md` e i doc di design. Il bilinguismo per l'utente vive nelle note di release e nei README.
- **Copertura del CHANGELOG: da v1.2.0 a v1.6.0, più `Unreleased`.** `v1.7.0` **non è ancora taggata**: l'ultima release pubblicata è `v1.6.0`, e il lavoro v1.7 sta su `main` senza tag. Inventare una voce `v1.7.0` datata sarebbe falso.
- **Il limite inferiore va dichiarato nel file.** Esistono quattro release più vecchie (`v0.1.0`, `v0.1.1`, `v1.1.0`, `v1.1.1`) con note **solo in italiano**: ricostruirle in inglese sarebbe tradurre e riassumere a posteriori. Il CHANGELOG chiude con una riga che lo dice e rimanda alla pagina delle release.
- **Niente indirizzi email** in `SECURITY.md`: il canale è il private vulnerability reporting di GitHub.
- **Verificare i fatti prima di scriverli.** Percorsi, permessi dei file e nomi delle variabili d'ambiente vanno letti dal codice, non ricordati.
- **Processo:** Conventional Commits, **MAI** `Co-Authored-By`. Nessun codice cambia, ma il gate resta: `gofmt -l .` (vuoto), `go vet ./...`, `go build ./...`, `go test ./... -race` (verde).

## File Structure

- `CHANGELOG.md` (nuovo) — storia delle release in formato Keep a Changelog.
- `SECURITY.md` (nuovo) — versioni supportate, canale di segnalazione, modello di minaccia.
- Impostazione del repo GitHub (nessun file): private vulnerability reporting.

---

### Task 1: `CHANGELOG.md`

**Files:**
- Create: `CHANGELOG.md`

- [ ] **Step 1: raccogliere le fonti.** Eseguire e leggere:

```bash
gh release list --limit 20
for t in v1.2.0 v1.3.0 v1.4.0 v1.5.0 v1.6.0; do
  echo "═══ $t"; gh release view "$t" --json publishedAt,body --jq '.publishedAt, .body'
done
git log --oneline v1.6.0..HEAD
```

Le note delle release sono bilingui: estrarre **solo la metà inglese**. `git log` serve per il lavoro dopo `v1.6.0` (che va in `Unreleased`) e per ciò che le note non menzionano.

- [ ] **Step 2: scrivere il file.** Struttura, in inglese:

```markdown
# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- …

## [1.6.0] - 2026-07-22

### Changed
- …

## [1.5.0] - 2026-07-21
…
```

Regole di contenuto:
- una sezione per ogni release pubblicata, da `1.6.0` a `1.2.0`, in ordine decrescente, con la **data reale** di `publishedAt`;
- voci raggruppate in `Added` / `Changed` / `Fixed` / `Deprecated`, omettendo i gruppi vuoti;
- voci scritte per un lettore, non copiate dai subject dei commit;
- `Unreleased` contiene il lavoro v1.7 (profondità di fatturazione) e diventerà `1.7.0` al momento del tag;
- **in fondo al file**, una riga che dichiara il limite inferiore, del tipo: `Releases before 1.2.0 predate this changelog; see the [GitHub releases](https://github.com/marcoarnulfo/clickup-cli/releases) page.`

**Due voci da non perdere,** perché sono quelle che un lettore cerca:
- **1.6.0** — rebrand del binario `clickup` → `clup`, con `clickup` mantenuto come shim deprecato e il fallback di lettura della vecchia config; è un cambio che tocca chiunque abbia installato prima;
- **Unreleased (v1.7)** — config schema v2, **additiva**: un file v1 continua a funzionare e viene marcato alla prima scrittura.

- [ ] **Step 3: verificare i fatti** — per ogni voce, confermare che descriva qualcosa che esiste davvero (`git log`, il codice, o le note di release). Nessuna voce inventata o "migliorata".

- [ ] **Step 4: commit**

```bash
git add CHANGELOG.md
git commit -m "docs: add CHANGELOG backfilled from v1.2.0 to v1.6.0 (#108)"
```

---

### Task 2: `SECURITY.md` e canale di segnalazione

**Files:**
- Create: `SECURITY.md`
- Repo setting: private vulnerability reporting

**Interfaces:**
- Consumes: i percorsi e i permessi reali letti da `internal/config/config.go`.

- [ ] **Step 1: verificare i fatti nel codice.** Leggere `internal/config/config.go` e confermare, senza fidarsi della memoria: la funzione usata per la directory di config (`os.UserConfigDir`), il nome del file, il **percorso legacy** ancora letto, i **permessi** con cui il file viene creato, e il nome della variabile d'ambiente alternativa per il token. Questi valori finiscono nel documento e devono essere esatti.

- [ ] **Step 2: abilitare il canale privato**

```bash
gh api repos/marcoarnulfo/clickup-cli/private-vulnerability-reporting   # prima: {"enabled":false}
gh api -X PUT repos/marcoarnulfo/clickup-cli/private-vulnerability-reporting
gh api repos/marcoarnulfo/clickup-cli/private-vulnerability-reporting   # dopo: {"enabled":true}
```

La conferma va letta **da questo endpoint**, non da `.security_and_analysis` del repo: quel
campo riporta secret scanning e Dependabot, non il private vulnerability reporting, quindi
guardarlo darebbe una verifica che non verifica nulla. Se l'endpoint fallisce per permessi, riportarlo invece di proseguire in silenzio: senza il canale abilitato, il file rimanderebbe a un pulsante inesistente.

- [ ] **Step 3: scrivere `SECURITY.md`**, in inglese, con queste sezioni:

**Supported versions** — solo l'ultima minor riceve correzioni di sicurezza. È l'unica promessa mantenibile da un progetto con un solo manutentore; scriverla è più onesto che promettere backport.

**Reporting a vulnerability** — usare il pulsante "Report a vulnerability" nella tab Security del repo, che apre un advisory privato. Nessun indirizzo email. Indicare un'aspettativa di risposta realistica (best effort, progetto mantenuto nel tempo libero).

**Where your ClickUp token lives** — la sezione che conta:
- il token personale sta **in chiaro** nel file di config, creato con i permessi verificati allo Step 1;
- il percorso dipende dal sistema operativo (indicare la forma Linux e quella macOS, oltre al percorso legacy che può contenere ancora un token);
- **conseguenza dichiarata:** chi ha accesso al filesystem dell'utente ha accesso al token;
- esiste l'alternativa via variabile d'ambiente, che il salvataggio non persiste mai su disco;
- il keychain di sistema è pianificato (milestone v1.10).

**What clup talks to** — ClickUp API v2 con il token dell'utente, e nient'altro. Quando #104 sarà a bordo, aggiungere: una richiesta anonima a `api.github.com` per verificare l'esistenza di una release più recente, senza token e senza scaricare né eseguire codice — non esiste self-update — disattivabile. È la prima domanda di chi legge il SECURITY.md di uno strumento che parla con la rete.

- [ ] **Step 4: verificare** — rileggere il file confrontando ogni percorso e ogni permesso con quanto letto allo Step 1.

- [ ] **Step 5: commit**

```bash
git add SECURITY.md
git commit -m "docs: add SECURITY policy with the token threat model (#108)"
```

---

### Task 3: aggiornare la checklist dell'issue

**Files:** nessuno (solo GitHub).

- [ ] **Step 1: correggere la forma della checklist, poi spuntarla.** Attenzione: il corpo di [#108](https://github.com/marcoarnulfo/clickup-cli/issues/108) **non** ha cinque caselle, ne ha **una sola per lingua** che elenca tutti i file su una riga — quindi "spuntare le voci" non è eseguibile così com'è. Prima si spezza quella riga in una casella per file, in **entrambe** le sezioni linguistiche, poi si spuntano quelle vere.

- [ ] **Step 2: commentare** — aggiungere un commento bilingue che chiarisce che `CONTRIBUTING`, `CODE_OF_CONDUCT` e i template esistevano già **prima** di questo lavoro: senza, sembrano voci lasciate cadere invece che già fatte.

- [ ] **Step 3: verificare** — `gh issue view 108` e controllare che il corpo aggiornato sia coerente in entrambe le lingue.

---

## Self-Review (autore)

- **Copertura della spec:** §1 (che cosa manca davvero, checklist da correggere) → T1/T2/T3; §2 (CHANGELOG: formato, lingua, copertura, fonti, voci da non perdere) → T1; §3 (SECURITY: versioni supportate, canale, modello di minaccia, riga su #104) → T2; §4 (niente README, nessun codice) → rispettato: nessun task tocca il README; §5 (fuori scope) → nessun task lo attraversa. ✓
- **Segnaposto:** nessuno. I contenuti che dipendono da fatti del repo (date di release, percorsi, permessi) non sono scritti nel piano come valori fissi ma come **step di verifica**, perché scriverli qui a memoria significherebbe pubblicare percorsi sbagliati in un documento di sicurezza.
- **YAGNI:** nessun tocco ai file di community già presenti, nessun sito docs, nessun template nuovo.
- **Dipendenza dichiarata:** la riga di `SECURITY.md` sull'avviso di aggiornamento si scrive quando #104 è a bordo; se questo piano viene eseguito prima, la si omette e la si aggiunge nel task di documentazione di #104.
