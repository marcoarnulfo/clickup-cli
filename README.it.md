[English](README.md) · **Italiano**

# clickup — ClickUp Hours CLI

[![CI](https://github.com/marcoarnulfo/clickup-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/marcoarnulfo/clickup-cli/actions/workflows/ci.yml)
[![Ultima release](https://img.shields.io/github/v/release/marcoarnulfo/clickup-cli)](https://github.com/marcoarnulfo/clickup-cli/releases)
[![Versione Go](https://img.shields.io/github/go-mod/go-version/marcoarnulfo/clickup-cli)](go.mod)
[![Licenza: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![PR benvenute](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.it.md)

> TUI da terminale, veloce e colorata, per tirare giù le tue **ore mensili di ClickUp** — self o team — calcolare l'**importo da fatturare** e registrare tempo su ClickUp. Libera e open-source (MIT).

## Funzionalità

- 📊 **Report ore mensile** (self o intero team), raggruppabile per totale / task / lista / giorno.
- 💶 **Importo da fatturare** da una tariffa oraria di default, con **tariffe per-lista**.
- ⏱️ **Log ore** su ClickUp dalla TUI: guidato (lista → task), da ID/URL del task, o con timer start/stop.
- 📤 **Export** in CSV / JSON / Markdown.
- ⌨️ TUI interattiva, guidata da tastiera (basata su [Charm](https://charm.sh) bubbletea).
- 🔒 Il token resta in locale (file di config o variabile `CLICKUP_TOKEN`).

## Demo

![clickup demo](docs/demo.gif)

Provala senza account ClickUp: **`CLICKUP_DEMO=1 clickup`** avvia una modalità demo con dati
fittizi. Il GIF è registrato con [vhs](https://github.com/charmbracelet/vhs) da
[`docs/demo.tape`](docs/demo.tape) (lancia `vhs docs/demo.tape` per rigenerarlo).

## Requisiti

- **[Go](https://go.dev/dl/) 1.26 o superiore** — serve solo per installare/compilare da sorgente.
  - macOS: `brew install go` · Linux: [install ufficiale](https://go.dev/doc/install) · verifica con `go version`.
- Un **token API personale ClickUp** (ClickUp → Settings → Apps → API Token).

## Installazione

```bash
go install github.com/marcoarnulfo/clickup-cli/cmd/clickup@latest
```

Installa il binario `clickup` in `$(go env GOPATH)/bin` (assicurati che sia nel `PATH`).

<details>
<summary>Compilare da sorgente</summary>

```bash
git clone https://github.com/marcoarnulfo/clickup-cli.git
cd clickup-cli
go build -o clickup ./cmd/clickup
./clickup
```
</details>

## Avvio rapido

1. **Installa** (vedi sopra) e lancia `clickup`.
2. Al primo avvio, il **wizard di setup** chiede token API, workspace, tariffa oraria opzionale e valuta — salvati in `~/.config/clickup-cli/config.yml`.
3. Scegli un **periodo** (`d`) e lo **scope** (`me`/`team`) nella home, premi `Enter` → il report. Premi `n` per loggare ore, `e` per esportare, `p` per le tariffe per lista.

## Uso

Lancia `clickup`. Al primo avvio parte un wizard di setup che chiede, in
sequenza: il token API personale (lo trovi in ClickUp → Settings → Apps →
API Token), il workspace da usare (scelto tra quelli visibili al token),
una tariffa oraria opzionale e la valuta (default `EUR`). Il risultato viene
salvato in `~/.config/clickup-cli/config.yml` e riusato ai lanci successivi.

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
| `t` | Home | Alterna scope `me` / `team` |
| `f` | Home | Apre la **selezione membri** (scope team): multiselezione dei membri inclusi nel report |
| `Enter` | Home | Genera il report per il periodo/scope selezionati |
| `g` | Report | Cicla il raggruppamento: totale → task → lista → giorno → membro (team) → totale |
| `e` | Report | Apre il menu di export (CSV/JSON/Markdown) |
| `m` / `s` | Report | Torna alla home per cambiare range/scope |
| `r` | Report | Ricarica le voci ore dall'API per lo stesso periodo/scope |
| `p` | Report | Apre la schermata **Tariffe per lista** |
| `f` | Report | Apre la schermata **Filtri** (lista/tag/status) |
| `n` | Home / Report | Apre la schermata **Log ore** (registra tempo su ClickUp) |
| `↑`/`↓` (anche `k`/`j`) | Export | Seleziona il formato |
| `Enter` | Export | Salva `clickup-report-<periodo>.<ext>` nella cwd (`<periodo>` è `YYYY-MM` per un mese di calendario, oppure `YYYY-MM-DD_YYYY-MM-DD` per un periodo personalizzato) |
| `Esc` | Export | Torna al report senza esportare |
| `q` | Ovunque tranne setup / rates / range | Esce dall'applicazione |
| `Ctrl+C` | Sempre | Esce dall'applicazione |

Nella schermata di setup non è previsto `q` per uscire, per evitare di
premerlo per errore durante l'inserimento del token: usa `Ctrl+C`.

#### Schermata Tariffe per lista

Dalla schermata del report, premendo `p` si apre la schermata **Tariffe per lista**,
dove è possibile configurare una tariffa specifica per ogni lista (diverse dal default).
I comandi disponibili sono:

- `↑` / `↓` (anche `k` / `j`): naviga tra le liste
- `Enter`: modifica la tariffa della lista selezionata (solo cifre e separatore decimale)
- `b`: apre il **browser workspace liste** per aggiungere una lista non ancora tracciata
- `d`: ripristina la lista alla tariffa di default
- `s`: salva le modifiche e torna al report
- `Esc`: annulla (scarta le modifiche non salvate) e torna al report

Dalla v1.1, ogni importo è calcolato dalle ore reali della lista moltiplicato per la sua
tariffa specifica (non dalle ore arrotondate), quindi il singolo importo può differire di
qualche centesimo dal prodotto `ore_mostrate × tariffa_lista`; tuttavia, il totale della
fatturazione resta sempre la somma esatta degli importi mostrati.

#### Schermata Filtri

Dalla schermata del report, premendo `f` si apre la schermata **Filtri**, con tre
sezioni: Liste, Tag e Status. Ogni sezione elenca i valori distinti presenti nelle
voci caricate; selezionando uno o più valori in una sezione si mantengono solo le
voci corrispondenti (OR all'interno della sezione, AND tra sezioni diverse);
lasciare una sezione vuota equivale a "nessun filtro" per quella dimensione. Gli
status dei task non fanno parte del caricamento iniziale dall'API, quindi la
prima volta che apri Filtri in una sessione l'app recupera lo status corrente di
ogni task caricato da ClickUp (mostrando "Loading statuses…"); da quel momento
resta in cache per il resto della sessione. I filtri si compongono con la
selezione membri e con il periodo attivo: restringono solo ciò che è già stato
caricato. Quando il periodo cambia, le selezioni dei filtri si adattano
automaticamente alle nuove voci: ogni valore selezionato che non compare più
viene scartato, così il report non resta mai bloccato vuoto per un filtro
ormai obsoleto. Comandi disponibili:

- `Tab` / `Shift+Tab`: cambia sezione
- `↑` / `↓` (anche `k` / `j`): naviga all'interno della sezione
- `Space`: attiva/disattiva il valore evidenziato
- `a`: seleziona/deseleziona tutti i valori della sezione
- `Enter`: applica il filtro e torna al report
- `Esc`: scarta le modifiche e torna al report

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
**Tariffe per lista**) mostra tutti gli spazi, le cartelle e le liste del tuo workspace
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

## Configurazione

La configurazione persiste in `~/.config/clickup-cli/config.yml` (segue
`os.UserConfigDir()`, quindi rispetta `XDG_CONFIG_HOME` su Linux):

```yaml
token: pk_xxx...
workspace_id: "123456"
currency: EUR
rate: 45
rates:
  "111": 60
  "222": 30
```

- `token`: token API personale ClickUp.
- `workspace_id`: id del workspace (team ClickUp) scelto in fase di setup.
- `currency`: valuta usata nel report e negli export.
- `rate`: tariffa oraria di default usata per calcolare l'importo da fatturare.
- `rates` (opzionale): mappa `list_id: tariffa` con tariffe orarie specifiche per
  singola lista. Le liste non elencate usano la tariffa di default `rate`. La mappa
  si compila comodamente dalla TUI premendo `p` nella schermata del report.

La variabile d'ambiente `CLICKUP_TOKEN`, se impostata, sovrascrive sempre il
`token` letto dal file di config (comodo per CI o per non salvare il token su
disco):

```bash
CLICKUP_TOKEN=pk_xxx clickup
```

## Contribuire

I contributi sono benvenuti — è un progetto libero e open-source. Vedi
**[CONTRIBUTING.it.md](CONTRIBUTING.it.md)** per come preparare l'ambiente di sviluppo,
lanciare i test e aprire una PR. Sei alle prime armi? Cerca la label
[`good first issue`](https://github.com/marcoarnulfo/clickup-cli/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22).
Leggi anche il [Codice di Condotta](CODE_OF_CONDUCT.md).

## Roadmap

Roadmap e backlog vivono nelle **[GitHub Issues](https://github.com/marcoarnulfo/clickup-cli/issues)**
(label `roadmap`/`enhancement`, milestone `v1.3` / `v2.0`). In evidenza: range date
custom (v1.3), riepiloghi settimanali, export fattura, multi-valuta (v2.0).

## Licenza

[MIT](LICENSE)
