[English](README.md) · **Italiano**

# clup — ClickUp Hours CLI

[![CI](https://github.com/marcoarnulfo/clickup-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/marcoarnulfo/clickup-cli/actions/workflows/ci.yml)
[![Ultima release](https://img.shields.io/github/v/release/marcoarnulfo/clickup-cli)](https://github.com/marcoarnulfo/clickup-cli/releases)
[![Versione Go](https://img.shields.io/github/go-mod/go-version/marcoarnulfo/clickup-cli)](go.mod)
[![Licenza: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![PR benvenute](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.it.md)

> TUI da terminale, veloce e colorata, per tirare giù le tue **ore mensili di ClickUp** — self o team — calcolare l'**importo da fatturare** e registrare tempo su ClickUp. Libera e open-source (MIT).

## Funzionalità

- 📊 **Report ore mensile** (self o intero team), raggruppabile per totale / task / lista / giorno / membro / tag.
- 💶 **Motore di fatturazione**: tariffe orarie di default, per-lista, per-membro e per-(lista,membro), split billable/non billable, arrotondamento configurabile e subtotali per valuta (multi-valuta, senza FX).
- 🎯 **Budget per lista** con vista burn-down, per vedere a colpo d'occhio quanto budget di ogni progetto è già fatturato.
- ⏱️ **Log ore** su ClickUp dalla TUI: guidato (lista → task), da ID/URL del task, o con timer start/stop.
- ⏲️ **Timer live e gestione delle voci**: un indicatore live nella home per il timer in corso, e un browser per modificare, cancellare, modificare i tag o consultare lo storico delle voci passate.
- 📤 **Export** in CSV / JSON / Markdown / HTML self-contained (stampabile in PDF) / fattura CSV riga per riga.
- ⌨️ TUI interattiva, guidata da tastiera (basata su [Charm](https://charm.sh) bubbletea).
- 🔒 Il token resta in locale (file di config o variabile `CLICKUP_TOKEN`).

## Demo

![clup demo](docs/demo.gif)

Provala senza account ClickUp: **`CLICKUP_DEMO=1 clup`** avvia una modalità demo con dati
fittizi — che include anche il modello di fatturazione: split billable/non billable, due
valute di fatturazione, voci taggate e un budget per lista. Il GIF è registrato con
[vhs](https://github.com/charmbracelet/vhs) da [`docs/demo.tape`](docs/demo.tape) (lancia
`vhs docs/demo.tape` per rigenerarlo).

## Requisiti

- **[Go](https://go.dev/dl/) 1.26 o superiore** — serve solo per installare/compilare da sorgente.
  - macOS: `brew install go` · Linux: [install ufficiale](https://go.dev/doc/install) · verifica con `go version`.
- Un **token API personale ClickUp** (ClickUp → Settings → Apps → API Token).

## Installazione

### Binari precompilati

Scarica l'archivio per il tuo OS/arch dalla
[latest release](https://github.com/marcoarnulfo/clickup-cli/releases/latest)
(darwin/linux/windows × amd64/arm64), estrailo e metti `clup` nel `PATH`.

Ogni release include `checksums.txt`, firmato con cosign keyless — verifica con:

```bash
cosign verify-blob --bundle checksums.txt.sigstore.json checksums.txt
```

### Homebrew (macOS/Linux)

```bash
brew install marcoarnulfo/tap/clup
```

### Scoop (Windows)

```powershell
scoop bucket add clup https://github.com/marcoarnulfo/scoop-bucket
scoop install clup
```

### AUR (Arch Linux)

```bash
yay -S clup-bin   # o il tuo AUR helper preferito
```

_in attesa della registrazione della chiave AUR da parte del maintainer, tracciato in
[#103](https://github.com/marcoarnulfo/clickup-cli/issues/103) — nel frattempo usa
un binario precompilato._

### go install (da sorgente)

Richiede Go 1.26+:

```bash
go install github.com/marcoarnulfo/clickup-cli/cmd/clup@latest
```

Installa il binario `clup` in `$(go env GOPATH)/bin` (assicurati che sia nel `PATH`).

<details>
<summary>Compilare da sorgente</summary>

```bash
git clone https://github.com/marcoarnulfo/clickup-cli.git
cd clickup-cli
go build -o clup ./cmd/clup
./clup
```
</details>

## Avvio rapido

1. **Installa** (vedi sopra) e lancia `clup`.
2. Al primo avvio, il **wizard di setup** chiede token API, workspace, tariffa oraria opzionale e valuta — salvati nel file di config (vedi [Configurazione](#configurazione) per il percorso esatto).
3. Scegli un **periodo** (`d`) e lo **scope** (`me`/`team`) nella home, premi `Enter` → il report. Premi `n` per loggare ore, `e` per esportare, `p` per le impostazioni di fatturazione (tariffe, valute, budget, arrotondamento), `b` per la vista burn-down dei budget.

## Uso

Lancia `clup`. Al primo avvio parte un wizard di setup che chiede, in
sequenza: il token API personale (lo trovi in ClickUp → Settings → Apps →
API Token), il workspace da usare (scelto tra quelli visibili al token),
una tariffa oraria opzionale e la valuta (default `EUR`). Il risultato viene
salvato nel file di config (vedi [Configurazione](#configurazione)) e
riusato ai lanci successivi.

Dalla home scegli un periodo e lo scope, poi `Enter` genera il report. Il report non
è più limitato a un mese di calendario: premi `d` nella home per aprire il
**selettore del periodo**, che offre preset (questo mese, mese scorso, ultimi 7
giorni, ultimi 30 giorni, questa settimana) più un periodo **personalizzato**
`From`/`To` (date in formato `YYYY-MM-DD`). Nel report puoi
cambiare raggruppamento, riesportare o tornare alla home. Se il token risulta
invalido o revocato durante l'uso, la TUI ripropone automaticamente il wizard
di setup.

### Comandi nella TUI

| Tasto | Schermata | Azione |
|---|---|---|
| `d` | Home | Apre il **selettore del periodo** (preset + personalizzato from/to) |
| `◂` / `▸` (frecce sin/dx, anche `h`/`l`) | Home | Cambia mese (solo mentre il periodo `this month` è attivo) |
| `w` | Home | Alterna la settimana ISO corrente |
| `t` | Home | Alterna scope `me` / `team` |
| `f` | Home | Apre la **selezione membri** (scope team): multiselezione dei membri inclusi nel report |
| `Enter` | Home | Genera il report per il periodo/scope selezionati |
| `g` | Report | Cicla il raggruppamento: totale → task → lista → giorno → tag → membro (team) → totale |
| `e` | Report | Apre il menu di export (CSV/JSON/Markdown/HTML/fattura CSV) |
| `m` / `s` | Report | Torna alla home per cambiare range/scope |
| `r` | Report | Ricarica le voci ore dall'API per lo stesso periodo/scope |
| `p` | Report | Apre la schermata **Impostazioni di fatturazione** (tariffe, valute, budget, arrotondamento, timezone) |
| `b` | Report | Apre la vista **Burn-down budget** |
| `f` | Report | Apre la schermata **Filtri** (lista/tag/status/billable) |
| `v` | Report | Apre il **browser delle voci ore** (edit/delete/tag/history) |
| `n` | Home / Report | Apre la schermata **Log ore** (registra tempo su ClickUp) |
| `c` | Home | Salta al timer in corso (visibile solo quando un timer è attivo) |
| `↑`/`↓` (anche `k`/`j`) | Export | Seleziona il formato |
| `Enter` | Export | Salva `clickup-report-<periodo>.<ext>` nella cwd (la fattura CSV viene salvata come `clickup-invoice-<periodo>.csv`; `<periodo>` è `YYYY-MM` per un mese di calendario, oppure `YYYY-MM-DD_YYYY-MM-DD` per un periodo personalizzato) |
| `Esc` | Export | Torna al report senza esportare |
| `q` | Ovunque tranne setup / rates / range / browser liste / log ore / voci ore | Esce dall'applicazione |
| `Ctrl+C` | Sempre | Esce dall'applicazione |

Nelle schermate setup, rates, range, browser liste, log ore e voci ore non è previsto
`q` per uscire, per evitare di premerlo per errore mentre si sta scrivendo (un token,
una tariffa, una nota, un ID task, ...): usa `Ctrl+C`.

#### Schermata Impostazioni di fatturazione

Dalla schermata del report, premendo `p` si apre la schermata **Impostazioni di
fatturazione**, con quattro tab (`Tab`/`Shift+Tab` per cambiare): **Lists** (tariffa,
valuta e budget per lista), **Members** (tariffa per membro), **Overrides** (tariffa
per coppia lista,membro — il livello più specifico della precedenza) e **Rules**
(valuta di default, arrotondamento increment/mode/scope, e timezone). Precedenza
delle tariffe, dalla più specifica: **(lista, membro) > membro > lista > default**.
Comandi disponibili:

- `Tab` / `Shift+Tab`: cambia tab
- `↑` / `↓` (anche `k` / `j`): naviga tra le righe
- `Enter`: modifica la tariffa della riga selezionata (in Rules: modifica il campo, o
  ne alterna il valore per mode/scope dell'arrotondamento)
- `c` (Lists): modifica la valuta della lista; `g` (Lists): modifica il budget della
  lista (invia un valore vuoto per cancellare entrambi)
- `n` (Overrides): crea un nuovo override (lista,membro) — scegli la lista, poi il
  membro, poi digita la tariffa
- `d`: cancella il valore selezionato, tornando al livello successivo della
  precedenza (una valuta o un budget di lista si cancellano invece riaprendo il
  proprio campo con `c`/`g` e inviando un valore vuoto)
- Digitare `0` per una tariffa è un'azione diversa dal cancellarla: `0` fattura la
  lista, il membro o la coppia a zero, mentre `d` cancella l'override e fa
  applicare la tariffa ereditata. Un budget di `0` non ha questo significato e
  resta rifiutato.
- `b` (Lists): apre il **browser workspace liste** per aggiungere una lista non
  ancora tracciata
- `s`: salva le modifiche e torna al report
- `Esc`: annulla (scarta le modifiche non salvate) e torna al report

Dalla v1.1, ogni importo è calcolato dalla durata esatta fatturata moltiplicata per la
tariffa effettiva, mai da un valore di ore già arrotondato — vedi
[Come vengono calcolati gli importi fatturati](#come-vengono-calcolati-gli-importi-fatturati)
per la regola completa.

#### Vista burn-down budget

Premendo `b` dalla schermata del report si apre la vista **Burn-down budget**: una
barra di progresso testuale per ogni lista con un budget configurato in
`billing.budgets`, ordinate dalla più consumata. Ogni barra mostra l'importo
fatturato rispetto al budget, nella valuta della lista (importo, non ore). Premi
`b` o `Esc` per tornare al report.

#### Schermata Filtri

Dalla schermata del report, premendo `f` si apre la schermata **Filtri**, con quattro
sezioni: Liste, Tag, Status e Billable. Le prime tre elencano i valori distinti
presenti nelle voci caricate; selezionando uno o più valori in una sezione si
mantengono solo le voci corrispondenti (OR all'interno della sezione, AND tra sezioni
diverse); lasciare una sezione vuota equivale a "nessun filtro" per quella dimensione.
Billable è diversa — un toggle a scelta singola (**All** / **Billable only** /
**Non-billable only**), un solo valore attivo alla volta. Gli status dei task non
fanno parte del caricamento iniziale dall'API, quindi la prima volta che apri Filtri
in una sessione l'app recupera lo status corrente di ogni task caricato da ClickUp
(mostrando "Loading statuses…"); da quel momento resta in cache per il resto della
sessione. I filtri si compongono con la selezione membri e con il periodo attivo:
restringono solo ciò che è già stato caricato. Quando il periodo cambia, le selezioni
dei filtri si adattano automaticamente alle nuove voci: ogni valore selezionato che
non compare più viene scartato, così il report non resta mai bloccato vuoto per un
filtro ormai obsoleto. Comandi disponibili:

- `Tab` / `Shift+Tab`: cambia sezione
- `↑` / `↓` (anche `k` / `j`): naviga all'interno della sezione
- `Space`: attiva/disattiva il valore evidenziato
- `a`: seleziona/deseleziona tutti i valori della sezione
- `Enter`: applica il filtro e torna al report
- `Esc`: scarta le modifiche e torna al report

#### Timer live e gestione delle voci

Quando un timer è in corso (avviato da **Log ore**, vedi sotto), la home mostra
un indicatore live che ticchetta — `⏱  running on <task> — HH:MM:SS  (X.XXh)` —
indipendentemente da quale schermata l'ha avviato, così non lo perdi mai di
vista. Premi `c` sulla home per saltarci direttamente e fermarlo.

Dalla schermata del report, premendo `v` si apre il **browser delle voci ore**:
le voci del periodo corrente, dalla più recente, navigabili con `↑`/`↓` (anche
`k`/`j`). Comandi disponibili:

- `e`: modifica la durata, data/ora, nota e flag billable della voce
  evidenziata — **solo sulle tue voci**
- `x`: cancella la voce evidenziata, con conferma `[y/N]` — **solo sulle tue
  voci**
- `t`: modifica i **tag** della voce evidenziata — **solo sulle tue voci**.
  Sono i tag propri della voce ore (mostrati come `#focus #client-A` nella
  riga), distinti dai tag del task. Apre un picker: `↑`/`↓` per muoversi,
  `space` per attivare/disattivare un tag, `n` per crearne uno nuovo, `Enter`
  per salvare, `Esc` per annullare
- `h`: consulta lo storico delle modifiche della voce (sola lettura) —
  disponibile su **qualsiasi** voce, non solo le tue
- `Esc`: torna al report

Modifica, cancellazione e tag sono vincolati alla proprietà: una voce
registrata da un collega compare nel browser (scope team) ma `e`/`x`/`t` non
fanno nulla su di essa — funziona solo `h`.

#### Schermata Log ore

Premendo `n` (dalla Home o dal Report) si apre **Log ore**, per registrare tempo
sui propri task ClickUp. Tre modalità:

1. **Guidato** — scegli una lista tra quelle note (report corrente ∪ config), poi
   un task della lista, poi compili il form. Il selettore di lista comprende una voce
   "**Esplora tutte le liste workspace…**" che apre il browser workspace liste,
   permettendoti di navigare tutti gli spazi, cartelle e liste nel tuo workspace
   (non solo quelli recenti o configurati).
2. **Task ID/URL** — incolli l'ID del task o un URL ClickUp (es. `.../t/86abc`) e
   vai diretto al form.
3. **Timer** — avvii un cronometro sul task scelto (guidato o ID); premendo `s` lo
   fermi e ClickUp registra la time entry. Se all'apertura c'è già un timer in
   corso, viene mostrato e puoi fermarlo subito.

Nel form la **durata** accetta formati flessibili: `2h30`, `2h30m`, `1.5h`, `1,5h`,
`90m`, `45` (numero nudo = ore). La **data** default è oggi (`YYYY-MM-DD`, editabile)
e la **nota** è opzionale. Infine scegli se la voce è **billable** (`Y`/`n`, default sì).
Dopo il salvataggio puoi premere `r` per ricaricare il report e vedere subito le nuove
ore. Si registrano sempre **le proprie** ore.

#### Browser workspace liste

Il browser workspace liste (aperto dalla modalità **Log ore** guidata o dalla schermata
**Impostazioni di fatturazione**) mostra tutti gli spazi, le cartelle e le liste del tuo workspace
come navigazione gerarchica drill-down: parti dalla radice del workspace → seleziona uno
spazio → naviga nelle cartelle di quello spazio → scegli una lista. I contenuti di ogni
spazio (cartelle e liste) sono caricati al primo accesso e messi in cache per la sessione;
l'apertura di una cartella non richiede altre richieste (le sue liste arrivano inline).
Comandi disponibili:

- `↑` / `↓` (anche `k` / `j`): naviga su/giù nel livello corrente
- `Enter`: entra/espandi lo spazio o cartella evidenziata; seleziona la lista evidenziata
- `Esc`: torna indietro di un livello (o ritorna alla schermata di origine al livello radice)

### Scope team

Per lo scope `team` il token deve avere permessi Owner/Admin sul workspace:
senza questi permessi la chiamata API fallisce e l'errore viene mostrato
nella schermata d'errore. Lo scope `team` aggrega le ore dei membri del
workspace; di default sono inclusi **tutti** i membri, ma puoi premere `f`
dalla Home per aprire la schermata di selezione membri e sceglierne alcuni
in particolare (una selezione parziale mostra una nota `(k/n members)` nel
titolo del report).

### Report headless

`clup report` stampa un report ore su stdout senza avviare la TUI — pensato per script,
cron job e agent. Riusa la stessa logica di periodo/scope/raggruppamento/fatturazione
del report interattivo, ma non tocca mai l'interfaccia a terminale.

```sh
clup report --month 2026-06 --scope me --format json
clup report --week 2026-W30 --billable --format csv-invoice > invoice.csv
```

Flag:

- `--month YYYY-MM` — report su un mese di calendario (default: mese corrente se non viene dato nessun altro flag di periodo).
- `--week YYYY-Www` — report su una settimana ISO-8601 (es. `2026-W30`); rifiuta un
  valore malformato o un numero di settimana fuori da 1–53.
- `--from YYYY-MM-DD --to YYYY-MM-DD` — periodo personalizzato, inclusivo (da usare insieme).
- `--preset this_month|last_month|last_7d|last_30d|this_week` — gli stessi preset del selettore periodo della TUI.
- Priorità del periodo quando ne viene passato più di uno: `--month` > `--week` >
  `--from`/`--to` > `--preset` > mese corrente (default).
- `--scope me|team` (default `me`).
- `--group total|task|list|day|member|tag` (default `total`).
- `--billable` — filtra solo le voci billable; passa `--billable=false` per tenere
  solo le voci non billable. Se il flag non viene passato non applica nessun filtro.
- `--tag TAG` — filtra le voci che portano questo tag; ripetibile (`--tag a --tag b`
  seleziona le voci che portano *uno qualsiasi* dei tag dati).
- `--tz IANA` — timezone per i confini del periodo e per il campo `timezone` del
  report (default: la `timezone` della config, altrimenti UTC — vedi
  [Configurazione](#configurazione)).
- `--format json|csv|md|html|csv-invoice` (default `json`).

Tutti i formati scrivono su stdout — usa la redirezione della shell per salvare
(es. `clup report --format csv > report.csv`).

Nota: `CLICKUP_DEMO=1` viene **ignorato** da `report` — carica sempre la config reale e
chiama la vera API; la demo mode è solo per la TUI.

L'output di `--format json` è uno **schema di scripting stabile** (chiavi snake_case,
timestamp RFC3339) — parsabile in sicurezza con `jq` e fissabile negli script. È
additivo e non-breaking: i campi pre-v1.7 `rate` e `currency` restano, ora
**deprecati**, insieme alle aggiunte v1.7 `schema_version`, `timezone`,
`currency_subtotals`, `billable_hours`, `non_billable_hours`, `billed_hours` e
`lines` (le righe fattura per singola unità di fatturazione). I nuovi script
dovrebbero leggere `currency_subtotals`/`lines` invece dei campi singolo-valore
deprecati `rate`/`currency`.

`--format html` scrive un report self-contained: CSS inline, nessun foglio di
stile, font, script o immagine esterna. Aprilo in un browser e stampalo in PDF
per un documento condivisibile.

`--format csv-invoice` scrive una riga per ogni unità di fatturazione (non per
bucket), con le colonne
`date, list_id, client, user, description, qty_hours, rate, amount, currency, billable`
— `client` contiene il nome della lista ClickUp (l'equivalente più vicino a un campo
cliente/progetto che uno strumento basato su liste possa avere). `qty_hours` è
espresso con 6 decimali di proposito, così che `qty_hours × rate` di ogni riga
riconcili con `amount` alla precisione del centesimo — un'unità di 20 minuti a 30/h
fattura esattamente 10.00, non 9.90.

## Configurazione

La configurazione persiste sotto `os.UserConfigDir()` (quindi rispetta
`XDG_CONFIG_HOME` su Linux): `~/Library/Application Support/clup/config.yml`
su macOS, `~/.config/clup/config.yml` su Linux. Se quel file non esiste ancora,
viene letto come fallback il percorso legacy pre-rebrand
(`~/.config/clickup-cli/config.yml` e il suo equivalente per-OS), così
l'aggiornamento da una vecchia installazione `clickup` non perde le
impostazioni.

```yaml
schema_version: 2
token: pk_xxx...
workspace_id: "123456"
currency: EUR
rate: 45
rates:
  "111": 60
  "222": 30
timezone: Europe/Rome
billing:
  default_currency: EUR
  rates_by_member:
    42: 60
  rate_overrides:
    - list: "111"
      member: 42
      rate: 70
  currencies:
    "111": EUR
    "222": USD
  budgets:
    "111": 2000
  rounding:
    increment: 15m
    mode: up
    scope: day
```

- `token`: token API personale ClickUp.
- `workspace_id`: id del workspace (team ClickUp) scelto in fase di setup.
- `currency`: valuta usata nel report e negli export.
- `rate`: tariffa oraria di default usata per calcolare l'importo da fatturare.
- `rates` (opzionale): mappa `list_id: tariffa` con tariffe orarie specifiche per
  singola lista. Le liste non elencate usano la tariffa di default `rate`. La mappa
  si compila comodamente dalla schermata **Impostazioni di fatturazione** della TUI
  (`p` nella schermata del report). Una tariffa di `0` (qui o in `rates_by_member`/
  `rate_overrides` più sotto) significa che la lista/membro/coppia fattura a zero —
  un valore deliberato, diverso dall'omettere la voce (che ricade sul livello
  successivo della precedenza).
- `schema_version`: scritto automaticamente al salvataggio — non va mai modificato a
  mano. Un file di config precedente alla v1.7 (schema v1) viene comunque letto
  così com'è, con i valori esistenti di `rate`/`rates`/`currency` intatti, e viene
  aggiornato a v2 al salvataggio successivo.
- `timezone` (opzionale): nome di zona IANA (es. `Europe/Rome`) che ancora i confini
  di giorno/settimana/mese del report. Due binari: la **TUI** la usa, ricadendo sulla
  zona locale della macchina se non impostata (e in quel caso mostra la propria zona
  come `Local`, non come nome IANA); il `clup report` headless usa sempre **UTC** di
  default, a meno che non venga sovrascritto da `--tz` o da questo campo. È
  consigliato impostarla esplicitamente; è modificabile anche dalla schermata
  **Impostazioni di fatturazione** della TUI.
- `billing` (opzionale, v1.7): additivo rispetto a `rate`/`rates`/`currency` sopra —
  nessuno di quei campi cambia significato.
  - `default_currency`: valuta ISO di fallback per le liste non presenti in
    `currencies` (ricade ulteriormente sulla `currency` di primo livello se non
    impostata).
  - `rates_by_member`: `user_id: tariffa` — una tariffa oraria per membro.
  - `rate_overrides`: una lista di `{list, member, rate}` — la tariffa più specifica,
    per un membro su una lista. Precedenza delle tariffe, dalla più specifica:
    **(lista, membro) > membro > lista > default**.
  - `currencies`: `list_id: codice ISO` — fattura ogni lista nella sua valuta. I
    subtotali sono sempre per valuta e non vengono mai sommati tra valute diverse
    (nessun FX); un totale complessivo unico viene mostrato solo quando una sola
    valuta porta importi (le altre possono comunque comparire con sole ore non
    fatturabili).
  - `budgets`: `list_id: importo` — un budget in valuta per lista, confrontato con
    gli **importi fatturati** (non le ore) e mostrato come barra burn-down nella TUI
    (`b` dalla schermata del report).
  - `rounding`: arrotonda le ore billable prima della fatturazione; le ore non
    billable non vengono mai arrotondate.
    - `increment`: una durata in formato umano (`15m`, `1h`, `2h30`); vuoto (default)
      significa arrotondamento disattivato. **Un valore non vuoto che non si riesce a
      interpretare è un errore bloccante**, non un "disattivato" silenzioso — un
      refuso qui non deve mai sotto-arrotondare in silenzio e sovra-fatturare.
    - `mode`: `up` arrotonda per eccesso; qualsiasi altro valore (incluso
      vuoto/omesso) arrotonda al valore più vicino.
    - `scope`: `day` arrotonda il totale per (giorno, lista, membro) invece che per
      singola voce; qualsiasi altro valore arrotonda ogni voce singolarmente.
- `update_check` (opzionale): impostare `false` per disattivare il controllo
  aggiornamenti descritto più sotto. Omettere la chiave (o impostare `true`) lo
  lascia attivo.

### Come vengono calcolati gli importi fatturati

L'importo di un'unità di fatturazione — una voce billable, oppure un gruppo
(giorno, lista, membro) quando `rounding.scope: day` — è arrotondato a 2 decimali a
partire dalla sua durata fatturata *esatta* moltiplicata per la tariffa, mai da un
valore di ore già arrotondato. Ogni totale (un bucket, un subtotale valuta, una riga
fattura) è poi la somma di importi di unità già arrotondati, così il CSV fattura, i
`currency_subtotals` del JSON e l'export HTML concordano sempre al centesimo. L'unico
caso in cui questo non vale è un report raggruppato più *fine* dell'unità di
fatturazione (es. per-task con arrotondamento per giorno): l'importo di un bucket in
quel caso è una ripartizione proporzionale **indicativa** delle sue unità e può
scostarsi di qualche centesimo — i subtotali valuta e le righe fattura
(`--format csv-invoice`, o il campo `lines` nell'output JSON) restano sempre gli
importi autoritativi.

La variabile d'ambiente `CLICKUP_TOKEN`, se impostata, sovrascrive sempre il
`token` letto dal file di config (comodo per CI o per non salvare il token su
disco):

```bash
CLICKUP_TOKEN=pk_xxx clup
```

### Controllo aggiornamenti

Una volta al giorno, `clup` chiede a GitHub se esiste una release più recente e, in
caso affermativo, mostra un breve avviso. È deliberatamente circoscritto in ciò che
fa:

- **Anonimo.** È una singola chiamata GET, con timeout di 2 secondi, all'endpoint
  pubblico `https://api.github.com/repos/marcoarnulfo/clickup-cli/releases/latest`,
  con i soli header `Accept` e `User-Agent`. Non c'è alcun header `Authorization` —
  il tuo token ClickUp non viaggia mai verso GitHub.
- **Nessun self-update.** `clup` non scarica né sostituisce mai il proprio binario;
  l'avviso si limita a segnalare che esiste una versione più recente e a indicare
  `go install github.com/marcoarnulfo/clickup-cli/cmd/clup@latest`.
- **In cache.** Il risultato è salvato in `os.UserCacheDir()/clup/update.json` e
  riusato per 24 ore, così la maggior parte delle esecuzioni non fa alcuna chiamata
  di rete.
- **La maggior parte delle build da sorgente sono esenti.** Se hai compilato
  `clup` tu stesso con un semplice `go build`, il binario riporta una
  pseudo-version anziché una release numerata e il controllo non parte mai —
  a meno che il checkout non sia pulito e posizionato esattamente su un tag di
  release, nel qual caso riporta esattamente quella versione e il controllo si
  comporta come per qualsiasi build di release. A tenerlo silenzioso sono i
  commit successivi al tag, oppure un albero sporco (`+dirty`).
- **Dove compare:** come riga aggiuntiva nella home della TUI e, per `clup report`,
  come riga su **stderr** stampata dopo il corpo del report — mai su stdout, così
  `clup report --format json` resta interpretabile dagli strumenti a valle.
- **Disattivarlo** con `CLUP_NO_UPDATE_CHECK=1` (qualsiasi valore non vuoto) o con
  `update_check: false` nel config; la variabile d'ambiente vince sempre sul
  config. Omettere la chiave lascia il controllo attivo. Anche la modalità demo
  (`CLICKUP_DEMO=1`) lo disattiva — ma **solo per la TUI**; `clup report` ignora
  `CLICKUP_DEMO` e controlla come in qualsiasi altra esecuzione.

## Contribuire

I contributi sono benvenuti — è un progetto libero e open-source. Vedi
**[CONTRIBUTING.it.md](CONTRIBUTING.it.md)** per come preparare l'ambiente di sviluppo,
lanciare i test e aprire una PR. Sei alle prime armi? Cerca la label
[`good first issue`](https://github.com/marcoarnulfo/clickup-cli/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22).
Leggi anche il [Codice di Condotta](CODE_OF_CONDUCT.md).

## Roadmap

La stella polare è far crescere il progetto da tool di report ore mensile a **client ClickUp
da terminale completo e bello** — mantenendo **time tracking e fatturazione come funzione di
punta** (nessun altro tool offre tariffe per-lista/membro, budget ed export del report in una TUI).

Il piano completo vive nelle **[GitHub Issues](https://github.com/marcoarnulfo/clickup-cli/issues)**,
tracciato dall'**[epic Roadmap 🗺️ #33](https://github.com/marcoarnulfo/clickup-cli/issues/33)**
e organizzato in milestone:

| Milestone | Focus |
|---|---|
| [v1.6 — Rebrand & fondamenta](https://github.com/marcoarnulfo/clickup-cli/milestone/4) | rebrand a `clup`, service layer, rate limiter, `report --json` |
| [v1.7 — Billing depth](https://github.com/marcoarnulfo/clickup-cli/milestone/5) | split billable, tariffe per-membro e per-coppia, arrotondamento, multi-valuta, budget & burn-down, export HTML/fattura CSV |
| [v1.8 — Live time tracking](https://github.com/marcoarnulfo/clickup-cli/milestone/6) | timer live, edit/delete entry |
| [v1.9 — TUI design system](https://github.com/marcoarnulfo/clickup-cli/milestone/7) | temi, tabelle, command palette, accessibilità |
| [v1.10 — Task context & account](https://github.com/marcoarnulfo/clickup-cli/milestone/8) | search, my-tasks, dettaglio task, keychain, profili |
| [v1.11 — Task management](https://github.com/marcoarnulfo/clickup-cli/milestone/9) | crea/aggiorna task, commenti, checklist |
| [v1.12 — Navigation, views & preset](https://github.com/marcoarnulfo/clickup-cli/milestone/10) | spaces/liste, view salvate, preset report |
| [v1.13 — Docs, Goals & Sprints](https://github.com/marcoarnulfo/clickup-cli/milestone/11) | ClickUp Docs, goals, sprint |
| [v2.0 — Git & AI](https://github.com/marcoarnulfo/clickup-cli/milestone/3) | git integration, `--jq`/`--template`, MCP, skill file |
| [Distribution & packaging](https://github.com/marcoarnulfo/clickup-cli/milestone/12) | goreleaser, Homebrew, completions, man page |
| [Docs & website](https://github.com/marcoarnulfo/clickup-cli/milestone/13) | landing page, docs site, screenshot |

**Fuori scope:** fatturazione fiscale (IVA, numerazione, PDF legale) — troppo dipendente dal
paese; il tool produce invece report pre-fattura condivisibili.

## Licenza

[MIT](LICENSE)
