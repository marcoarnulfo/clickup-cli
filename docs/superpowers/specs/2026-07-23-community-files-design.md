# File di community (#108) — design

> Spec per l'issue [#108](https://github.com/marcoarnulfo/clickup-cli/issues/108)
> (milestone *Docs & website*). Separata dalla spec dell'avviso di aggiornamento (#104):
> condividono il tema "release", non i file né le competenze di review.

## 1. Che cosa manca davvero

L'issue elenca cinque voci, ma **tre esistono già** nel repo:

| Voce | Stato |
|---|---|
| `CONTRIBUTING.md` + `CONTRIBUTING.it.md` | ✅ presenti |
| `CODE_OF_CONDUCT.md` (Contributor Covenant) | ✅ presente |
| `.github/ISSUE_TEMPLATE/` (bug, feature, config) + `pull_request_template.md` | ✅ presenti |
| `CHANGELOG.md` | ❌ **manca** |
| `SECURITY.md` | ❌ **manca** |

Lo scope reale è quindi **due file markdown più una impostazione del repo**. Va detto
esplicitamente, altrimenti sembra che tre voci della checklist siano state lasciate
cadere invece che essere già fatte; la checklist dell'issue va spuntata di conseguenza.

## 2. `CHANGELOG.md`

**Formato:** [Keep a Changelog](https://keepachangelog.com), versioning semantico.

**Lingua: inglese soltanto.** `CLAUDE.md` stabilisce che tutto ciò che vive nel repo è in
inglese tranne `README.it.md` e i doc di design; il CHANGELOG rientra nella regola. Il
bilinguismo per l'utente finale resta dove già c'è ed è curato: note di release e README.
Un `CHANGELOG.it.md` raddoppierebbe la manutenzione sull'unico file del repo destinato a
crescere per sempre.

**Copertura: da v1.2.0 a v1.6.0**, più una sezione `Unreleased`.

**Perché non si parte da v0.1.0.** Esistono quattro release precedenti — `v0.1.0`,
`v0.1.1`, `v1.1.0`, `v1.1.1` — le cui note sono però in italiano soltanto, scritte prima
che il progetto adottasse la doppia lingua. Ricostruirle in inglese significherebbe
tradurre e riassumere a posteriori il lavoro di quella fase, cioè scrivere storia
riscritta. Si parte da v1.2.0, che è la prima release con note bilingui da cui estrarre la
metà inglese senza interpretare nulla — e il CHANGELOG lo **dichiara in fondo** con una
riga che rimanda alla pagina delle release per ciò che precede. Un changelog che comincia
a metà senza spiegare perché è peggio di uno che comincia a metà dichiarandolo.

Il limite superiore non è una scelta stilistica: **v1.7.0 non è ancora taggata**. L'ultima
release pubblicata è `v1.6.0` (2026-07-22) e il lavoro v1.7 sta su `main` senza tag,
quindi non esistono note da cui ricostruire una voce v1.7.0. Il lavoro v1.7 va in
`Unreleased` e diventerà la voce `v1.7.0` quando il tag verrà creato.

**Fonti:** le note delle release GitHub v1.2.0–v1.6.0 (che sono bilingui: si estrae la
metà inglese) e `git log` per ciò che le note non coprono. Le voci si raggruppano in
`Added` / `Changed` / `Fixed` / `Deprecated` con la data di rilascio reale.

**Da non perdere nella ricostruzione**, perché sono i due punti che un lettore cerca:
il rebrand `clickup` → `clup` della v1.6 con lo shim deprecato, e la config schema v2
della v1.7 (additiva, un file v1 continua a funzionare).

## 3. `SECURITY.md`

**Versioni supportate:** solo l'ultima minor. È l'unica politica onesta per un progetto
mantenuto da una persona sola; promettere backport su versioni vecchie sarebbe una
promessa che nessuno può mantenere.

**Canale di segnalazione:** private vulnerability reporting di GitHub, da abilitare nelle
impostazioni del repo (`PUT /repos/{owner}/{repo}/private-vulnerability-reporting` via
`gh api`). Nessun indirizzo email pubblicato: il canale è quello che i ricercatori si
aspettano su GitHub e non espone una casella agli scraper.

**Sezione sul modello di minaccia** — la parte che non è boilerplate e che vale la pena
scrivere bene, perché riguarda specificamente questo tool:

- il token personale ClickUp vive **in chiaro** nel file di config, creato con permessi
  `0600`;
- il percorso dipende dal sistema — su Linux `~/.config/clup/config.yml`, su macOS
  `~/Library/Application Support/clup/config.yml` — e vale anche per il percorso legacy
  `clickup-cli`, che può contenere ancora un token;
- chi ha accesso al filesystem dell'utente ha accesso al token: è la conseguenza da
  dichiarare, non da nascondere;
- esiste l'alternativa `CLICKUP_TOKEN` come variabile d'ambiente, che `Save` non
  persiste mai su disco;
- il keychain di sistema è pianificato per la milestone *v1.10 — Task context (read) &
  accounts* (citare la milestone, non un numero di issue: la issue del keychain va
  verificata al momento della scrittura e, se non esiste ancora, si nomina solo la
  milestone).

**Una riga sull'avviso di aggiornamento**, quando #104 sarà a bordo: la chiamata è
anonima, non manda mai il token, e il tool non scarica né esegue codice — non c'è
self-update. È la prima domanda che si fa chi legge un SECURITY.md di uno strumento che
parla con la rete.

## 4. Note

Il README non va toccato in questo lavoro: la nota su come spegnere l'avviso di
aggiornamento appartiene a #104, che è la feature che la richiede.

Nessun codice Go cambia. Il gate resta comunque quello del repo (`gofmt`, `go vet`,
`go build`, `go test ./... -race`) come verifica di non aver rotto nulla per sbaglio.

## 5. Fuori scope

- Sito di documentazione e landing page (#106, #107).
- Screenshot e GIF per feature (#109).
- Abilitare Discussions e il funding link (#110).
- Qualunque modifica ai file di community già presenti: sono a posto così.
