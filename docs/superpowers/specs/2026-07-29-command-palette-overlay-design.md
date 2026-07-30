# Command palette + overlay compositor — design

**Data:** 2026-07-29
**Milestone:** v1.9 — TUI design system, tranche B3
**Issue:** #71 (command palette), #59 ultima casella (overlay ortogonale a `m.screen` + compositor di righe)

---

## 1. Obiettivo

Due cose che sono una cosa sola.

`View()` oggi ritorna **una** schermata: `screenBody()` più il footer. Non esiste modo di
disegnare qualcosa *sopra* quello che c'è già. #59 chiama questa mancanza per nome —
«uno stato di overlay ortogonale a `m.screen` + un compositor di righe» — e la lascia
aperta da due tranche perché un compositor senza clienti è un'astrazione modellata sul
nulla.

Il cliente è #71: una **command palette** aperta da `ctrl+p`, che filtra in fuzzy le
azioni disponibili e ne esegue una con Enter. È il primo pezzo di UI di questo programma
che ha senso solo se galleggia: una palette che sostituisce la schermata sottostante ha
perso metà del suo scopo, che è sceglierla *guardando* quello su cui stai lavorando.

Questa tranche chiude entrambe.

---

## 2. Decisioni prese

Quattro decise con l'utente durante il brainstorming, due prese qui e messe agli atti.

### 2.1 Contenuto della palette — azioni di schermata + navigazione globale

Due sorgenti (§6). Le azioni della schermata corrente sono **derivate** dal keymap, non
riscritte; la navigazione globale è un elenco esplicito di sole voci `"Go to X"`.

Lo **switcher mese/lista/membro** che #71 nomina di sfuggita («doubling as a
month/list/member switcher») **esce da questa tranche** e diventa una issue nuova nella
milestone v1.12 — *Navigation/views/preset*, che esiste esattamente per questo. Motivo:
sono righe che non sono azioni ma selezioni di stato, cioè una seconda modalità della
palette, con un suo modello di dati, una sua resa e una sua superficie di test.

### 2.2 Il compositor taglia per colonna, ANSI-aware

`composite(body, box, x, y)` innesta il box dentro le righe del corpo, che resta visibile
tutto intorno. L'alternativa scartata (il box occupa righe intere) è più semplice ma
chiude la casella di #59 solo in senso debole: quella casella dice «gli overlay
richiedono la stratificazione», e sostituire righe intere non è stratificare.

### 2.3 `?` resta il footer espanso

#59 nomina «l'overlay `?`» tra i clienti previsti dello stato di overlay, e #69 chiuse
con una nota che diceva che promuoverlo dopo sarebbe stato «una modifica a una funzione».
**Non lo promuoviamo, e non perché costi.** Un pannello d'aiuto che copre proprio la
schermata su cui stai chiedendo aiuto è peggio di un footer che non copre niente. #69 è
chiusa con questa scelta da due release e nessuno l'ha contestata.

Questa decisione è definitiva, non rimandata: **non apriamo una issue** per riconsiderarla.
Se un giorno servirà davvero, il compositor c'è.

### 2.4 Il matching fuzzy è scritto in casa

Nuovo pacchetto puro `internal/fuzzy` (§7). Nessuna dipendenza nuova nella supply chain
di un binario firmato cosign e distribuito su brew/scoop/AUR. Gli indici agganciati non
servono solo a ordinare: decidono **quali lettere si illuminano** nella riga, quindi il
matching deve essere ottimo e non greedy.

### 2.5 (presa qui) Il box è costruito a mano, non con `th.Box`

`th.Box` è `Border(RoundedBorder()).Padding(0, 1)`. Per un overlay la larghezza resa non è
un dettaglio estetico: è il numero che il compositor usa per decidere dove riprendere la
riga del corpo. La tranche C ha già pagato una volta il prezzo di fidarsi dell'aritmetica
di bordo di una libreria (`Table.Width()` che contava solo i separatori di colonna e
amputava il bordo destro).

Quindi il box si costruisce riga per riga: bordo superiore con il titolo innestato, righe
`"│ " + contenuto + " │"`, bordo inferiore. La larghezza è `boxWidth` **per costruzione**,
e un test lo verifica su ogni riga con `lipgloss.Width`.

### 2.6 (presa qui) La query è una `string`, non un `textinput.Model`

`textinput.New()` deriva i suoi stili dal renderer di default di lipgloss — la stessa
ragione per cui `footer.go` rifiuta `help.New()`. Aggiungerebbe inoltre tre campi di stato
per affordance di editing (cursore mobile, `ctrl+u`, cancellazione per parola) che un
filtro da venti voci non usa.

La query è una `string` con append di rune e backspace. Dieci righe, nessun renderer.

---

## 3. Architettura per file

```
NUOVO  internal/fuzzy/fuzzy.go            puro: Match(query, target) (score int, idx []int, ok bool)
NUOVO  internal/fuzzy/fuzzy_test.go
NUOVO  internal/tui/overlay.go            composite(body, box string, x, y int) string
NUOVO  internal/tui/overlay_test.go
NUOVO  internal/tui/actions.go            type action; screenActions(m); globalActions(m); paletteActions(m)
NUOVO  internal/tui/actions_test.go
NUOVO  internal/tui/palette.go            paletteModel, newPalette, updatePalette, view, layout
NUOVO  internal/tui/palette_test.go

MOD    internal/tui/app.go                overlayKind + campi sul Model; ordine dei tasti in Update;
                                          View() compone; estrazione delle openX()
MOD    internal/tui/nav.go                goTo/replace/pop/resetTo dissolvono l'overlay (§5.2c)
MOD    internal/tui/keys.go               keyDefaults.Palette/PaletteUp/PaletteDown; keyMap.Palette;
                                          paletteKeys(); keyMap.paletteBindings(); keysFor si
                                          sdoppia in keysFor (overlay-aware) + screenKeys (§5.2b)
MOD    internal/tui/home.go               chiama le openX() estratte
MOD    internal/tui/report.go             chiama le openX() estratte
MOD    internal/tui/entries.go            openEntries() cambia firma (§6.3)
MOD    keys_test.go, report_test.go, export_test.go, members_test.go, filters_test.go,
       range_test.go, budget_test.go, log_test.go, rates_test.go, entries_test.go,
       listbrowser_test.go                "ctrl+p" nelle liste want dei test di etichette (§5.3)
MOD    internal/tui/testdata/*.golden     footer *_full rigenerati; golden nuovi della palette
MOD    go.mod                             github.com/charmbracelet/x/ansi: indirect -> direct
MOD    README.md, README.it.md, CHANGELOG.md, docs/demo.tape, docs/demo.gif
```

`internal/report` e `internal/duration` non vengono toccati. `internal/fuzzy` nasce con la
stessa regola: nessun I/O, nessun import di `config`/`clickup`/`tui`.

---

## 4. Il compositor

### 4.1 Contratto

```go
// composite draws box over body with its top-left cell at (x, y) and returns
// the combined text. Cells of body outside the box's rectangle are preserved
// verbatim, including their ANSI styling.
func composite(body, box string, x, y int) string
```

`x` e `y` sono **celle**, non byte né rune. Per ogni riga `i` del corpo con
`y <= i < y+altezzaBox`:

```
sinistra := ansi.Cut(riga, 0, x)                          // taglio ANSI-aware
sinistra += spazi fino a x                                 // se la riga è più corta di x
destra   := ansi.Cut(riga, x+larghezzaBox, ansi.StringWidth(riga))
risultato := sinistra + rigaBox[i-y] + destra
```

`ansi.Cut(s, left, right)` è di `github.com/charmbracelet/x/ansi` v0.11.6, già nel grafo
dei moduli via lipgloss v1.1.0: promuoverlo a dipendenza diretta non scarica niente di
nuovo. È cell-based, consapevole dei caratteri wide e non spezza le sequenze di escape.

Comportamento **misurato**, non dedotto dalla documentazione (sonda eseguita su
`"\x1b[31mHELLO\x1b[0m world"`):

| Chiamata | Risultato |
|---|---|
| `Cut(s, 0, 3)` | `"\x1b[31mHEL\x1b[0m"` — **chiude** lo stile aperto |
| `Cut(s, 3, 11)` | `"\x1b[31mLO\x1b[0m world"` — **riemette** lo stile attivo al punto di taglio |
| `Cut(s, 8, 3)` | `""` — `left > right` non è un panic |
| `Cut(s, 20, 30)` | `"\x1b[31m\x1b[0m"` — **non vuoto** oltre la fine: coppia di escape a larghezza zero |
| `Cut("\x1b[31mHELLO world", 0, 3)` | `"\x1b[31mHEL"` — su input **non terminato** NON chiude |

Da cui tre regole per l'implementazione:

- Se `x + larghezzaBox >= larghezza(riga)`, la parte destra è `""` per costruzione, senza
  chiamare `Cut`. Con `right = larghezza(riga)` la chiamata ritornerebbe comunque `""`
  (`truncate.go:31-33`: `right <= left → ""`), quindi la regola non ripara un guasto: fa
  vedere nel codice che quel caso è previsto, invece di lasciarlo dipendere da un dettaglio
  interno della libreria.
- Dopo la parte sinistra si aggiunge `ansi.ResetStyle` (`"\x1b[m"`) **quando contiene un
  `\x1b`**. Sull'input ben formato è ridondante; sull'input con uno stile non terminato è
  l'unica cosa che impedisce al colore di colare dentro il box. Sotto `termenv.Ascii` non
  ci sono escape, quindi i golden non cambiano.
- **Un glifo wide a cavallo di un bordo va scartato, non tenuto intero.** `Cut` tratta i
  due lati in modo asimmetrico: a sinistra **scarta** il cluster che scavalca il limite
  (la parte sinistra può rendere `x-1` colonne), a destra lo **tiene intero**
  (`truncate.go:208-216`), quindi la parte destra può partire una colonna troppo presto e
  la riga composta esce larga una colonna in più. A sinistra il padding a `x` già lo
  copre, purché sia calcolato sulla larghezza **misurata** del risultato di `Cut` e non
  solo «se la riga è più corta di `x`». A destra bisogna ritagliare una colonna più in là
  e restituire la colonna persa come spazio: mezzo glifo non è disegnabile, e allineare
  vale più che conservare un carattere sul bordo di un box che lo copre a metà.

  Questo è il caso che i test ASCII non vedono — la stessa cecità che #135 documenta. La
  §9.2 vuole una fixture con un carattere wide su entrambi i bordi.

### 4.2 I quattro casi limite

Ognuno ha il suo test, e ognuno di quei test va **verificato fallente** contro
l'implementazione che lo ignora prima di essere considerato valido. Questa non è
diligenza generica: nella tranche C quattro test sono stati colti a passare proprio sul
bug che esistevano per prendere.

1. **Riga del corpo più corta di `x`** → padding a spazi fino a `x`, altrimenti il box si
   sposta a sinistra su quella riga sola.
2. **Corpo più corto di `y + altezzaBox`** → righe vuote aggiunte, altrimenti il box viene
   troncato in basso.
3. **Stato SGR che sopravvive al taglio.** Se il colore aperto nella metà sinistra della
   riga cola dentro il box, il bug è **invisibile ai golden**: `TestMain` fissa
   `termenv.Ascii` per il renderer di default, quindi i golden non contengono escape.
   Il test costruisce un renderer suo con un profilo a colori (come i test di stile della
   tranche C) e asserisce sugli escape veri.
4. **`x + larghezzaBox` oltre la fine della riga** → `destra` è vuota, nessun panic.

### 4.3 Dove si compone

`View()` compone sul **corpo**, non sull'output completo:

```go
func (m Model) View() string {
    body := m.screenBody()
    if m.screen == screenError { return body }
    if m.overlay == overlayPalette {
        box, x, y := m.palette.layout(m.theme, m.width, m.height)
        body = composite(body, box, x, y)
    }
    return strings.TrimRight(body, "\n") + "\n\n" + footerView(m.theme, m.width, m.helpAll, keysFor(m))
}
```

Il footer resta sotto e visibile, e mostra **i tasti della palette**, perché `keysFor`
ritorna `paletteKeys` quando l'overlay è aperto (§5). Se il box è più alto del corpo, il
footer scende: accettato e voluto — il box deve essere intero.

`screenError` non compone: quella schermata ritorna prima, come già fa oggi.

---

## 5. Lo stato di overlay

### 5.1 I campi

```go
type overlayKind int

const (
    overlayNone overlayKind = iota
    overlayPalette
)
```

Sul `Model`: `overlay overlayKind` e `palette paletteModel`.

Ortogonale a `m.screen` nel senso letterale: aprire la palette **non** tocca `m.nav`,
chiuderla **non** fa `pop()`. Un overlay non è un posto dove sei andato.

### 5.2 L'ordine dei tasti in `Update` è load-bearing

```go
case tea.KeyMsg:
    if key.Matches(msg, defaultKeys().ForceQuit) { return m, tea.Quit }   // 1
    if m.overlay != overlayNone { return m.updateOverlay(msg) }           // 2
    if key.Matches(msg, keysFor(m).Quit) { return m, tea.Quit }           // 3
    if key.Matches(msg, keysFor(m).Help) { m.helpAll = !m.helpAll; return m, nil }
    if key.Matches(msg, keysFor(m).Palette) { return m.openPalette(), nil }
    return m.routeKey(msg)
```

`ForceQuit` sale in cima (oggi è dopo `Quit`). I due tasti non si sovrappongono (`ctrl+c`
contro `q`), quindi lo scambio non cambia niente sul comportamento attuale, ma rende
`ctrl+c` l'unica via d'uscita incondizionata anche con un overlay aperto.

**Seconda garanzia, indipendente:** `paletteKeys` lascia `Quit` e `Help` non assegnati
(binding zero → `keys == nil` → `Enabled() == false` → `key.Matches` non scatta mai;
verificato in `bubbles/key@v1.0.0/key.go:106`). Se un giorno qualcuno riordina i controlli,
la palette non diventa comunque un modo per uscire per sbaglio.

**Che cosa rende load-bearing l'ordine, esattamente.** La prima stesura di questa sezione
diceva «altrimenti scrivere `q` chiude il programma» e ordinava di verificare
`TestPaletteQueryAcceptsQ` fallente contro l'ordine invertito. **Era falso**, e falso nel
modo peggiore: con il passo 2 sotto il passo 3, il passo 3 legge `keysFor(m)`, che con
l'overlay aperto è `paletteKeys`, dove `Quit` non è assegnato — quindi `q` finisce nella
query comunque. La seconda garanzia neutralizzava proprio il RED che la sezione
prescriveva. È la trappola n.1 della §11 applicata alla spec stessa.

Il tasto su cui l'ordine è davvero load-bearing è `ctrl+p`: `paletteKeys` **assegna**
`Palette` (serve per chiudere), quindi con il passo 2 sotto il passo 5 un `ctrl+p` a
palette aperta chiamerebbe `openPalette()` una seconda volta, azzerando la query invece di
chiudere. Il test che discrimina è `TestPaletteCtrlPClosesRatherThanReopening`.

`TestPaletteQueryAcceptsQ` resta, ma come test di regressione, non come RED: la spec dice
qui che non fallisce contro il solo riordino, così nessuno lo usa per «dimostrare»
l'ordine.

### 5.2c L'overlay si dissolve a ogni navigazione

Un overlay appartiene alla schermata su cui è stato alzato. Se quella schermata viene
sostituita **sotto** di lui, l'overlay non ha più un posto a cui appartenere:

- `errMsg` con 401 fa `resetTo(screenSetup)` (`app.go:634-641`). Senza dissoluzione, la
  palette resterebbe disegnata e operativa **sopra il wizard di setup**, che la §5.3
  esclude esplicitamente.
- `retryableErrMsg` con origine non-Home fa `replace(screenError)`. `View()` ritorna prima
  per quella schermata, quindi la palette diventa **invisibile** — ma il passo 2 continua a
  mandarle ogni tasto. La schermata dice «press a key to return home» e mente, finché
  l'utente non indovina `esc`.

Entrambi sono raggiungibili: la palette si apre su `screenFilters` o `screenMembers` mentre
un fetch è in volo, e la risposta arriva dopo.

La regola sta in **un posto solo**, `nav.go`: `goTo`, `replace`, `pop` e `resetTo` azzerano
`m.overlay` e `m.palette`. Non serve ricordarsene in undici handler, e le azioni della
palette non sono un'eccezione da gestire — eseguono già dopo `closePalette()`, quindi
trovano l'overlay chiuso e l'azzeramento è un no-op.

### 5.2b `keysFor` si sdoppia — altrimenti la palette si svuota appena scrivi

`keysFor(m)` deve ritornare `paletteKeys` quando l'overlay è aperto, perché è ciò che il
footer legge. Ma `screenActions(m)` (§6.1) deriva le sue voci **dal keymap della schermata
sottostante**: se chiamasse `keysFor(m)` con l'overlay aperto si ritroverebbe i tasti della
palette e le azioni di schermata sparirebbero.

Il guasto non si vede all'apertura — `openPalette()` gira quando `m.overlay` vale ancora
`overlayNone` — ma **alla prima rune digitata**, quando la lista si ricalcola e metà delle
voci svanisce.

Quindi:

```go
// screenKeys is the binding set for m.screen, ignoring any overlay.
func screenKeys(m Model) keyMap { switch m.screen { /* il corpo di oggi */ } }

// keysFor is what the footer and Update read: the overlay owns the keyboard.
func keysFor(m Model) keyMap {
    if m.overlay == overlayPalette { return paletteKeys(defaultKeys()) }
    return screenKeys(m)
}
```

`screenActions` chiama `screenKeys`. `TestPaletteKeepsScreenActionsWhileTyping` va scritto
contro la versione che chiama `keysFor` e verificato fallente.

### 5.3 Dove `ctrl+p` è attivo

Ovunque **tranne**:

| Schermata / modo | Perché no |
|---|---|
| `screenSetup` | il wizard va finito: senza token configurato quasi nessuna azione è eseguibile |
| `screenLoading` | una navigazione mentre l'`entriesMsg` è in volo verrebbe annullata quando atterra (il suo handler fa `resetTo(Home).goTo(Report)` incondizionatamente) |
| `screenError` | qualunque tasto torna a Home: `ctrl+p` non può reclamarne uno |
| `entriesConfirmDelete` | il footer di quel modo promette «any key cancel» (`anyKeyHelp`, `keys.go:481-484`). Un `ctrl+p` che apre la palette lascerebbe la conferma di cancellazione pendente sotto e smentirebbe il footer nella stessa riga. `Help` è già non assegnato lì per la stessa ragione: `Palette` la segue |

Su tutte le altre `Palette` è assegnato nel `keyMap` e compare **solo in `full`**, mai in
`short`: i golden `footer_*_short` restano invariati, i `footer_*_full` si rigenerano.

**Ricaduta sui test di parità, da non scoprire a metà implementazione.** `keyMap` guadagna
un campo `key.Binding`, quindi `TestAllBindingsCoversEveryField` (`keys_test.go:152`, che
conta i campi per riflessione) obbliga ad aggiungere `k.Palette` ad `allBindings()`. Da lì
`enabledLabels` (`keys_test.go:16-26`) inizia a raccogliere anche `"ctrl+p"`, e siccome
ordina alfabeticamente e confronta con `slices.Equal`, **ogni** lista `want` dei test di
etichette per schermata va aggiornata inserendo `"ctrl+p"` subito dopo `"ctrl+c"`. Sono
una ventina di casi in undici file: `keys_test.go`, `report_test.go`, `export_test.go`,
`members_test.go`, `filters_test.go`, `range_test.go`, `budget_test.go`, `log_test.go`,
`rates_test.go`, `entries_test.go`, `listbrowser_test.go`.

Il costo va guardato in faccia perché c'è una scorciatoia che lo evita e che sarebbe
sbagliata: controllare `defaultKeys().Palette` globalmente in `Update`, come fa
`ForceQuit`. Costa zero test, e in cambio `ctrl+p` sparisce da ogni footer — una palette
che nessuno può scoprire — **e** le quattro esclusioni qui sopra smettono di essere
espresse dall'enablement e devono essere riscritte a mano da qualche altra parte.

`ctrl+p` non entra in conflitto con le schermate che hanno un `textinput` focalizzato: non
è un carattere stampabile, e `bubbles/textinput` lo lega a `PrevSuggestion`, che è inerte
finché non si popolano i suggerimenti (cosa che questo programma non fa da nessuna parte).

---

## 6. Il registro delle azioni

```go
type action struct {
    label string                              // "Export report", "Go to rates"
    hint  string                              // "e" per le azioni di schermata, "" per le globali
    run   func(Model) (tea.Model, tea.Cmd)
}
```

`run` ritorna `tea.Model` e non `Model` di proposito: è la firma di `routeKey`, così
un'azione di schermata è un rinvio diretto senza type assertion.

### 6.1 Sorgente 1 — le azioni della schermata, derivate dal keymap

Non un elenco nuovo. `keysFor(m)` filtrato da un `paletteBindings()` curato, gemello
dell'`allBindings()` che già esiste in `keys.go`: tiene i binding che sono **comandi** e
scarta la meccanica del cursore.

Dentro (`Generate`, `GroupBy`, `ChangeRange`, `Reload`, `Export`, `Rates`, `Filters`,
`Budget`, `OpenEntries`, `LogHours`, `Timer`, `Members`, `Range`, `ToggleScope`,
`ToggleWeek`, `PrevMonth`, `NextMonth`, `ListCurrency`, `ListBudget`, `NewOverride`,
`ClearValue`, `BrowseList`, `Save`, `Delete`, `Edit`, `History`, `Tags`, `NewTag`,
`StopTimer`).

Fuori (`Up`, `Down`, `Confirm`, `ToggleItem`, `SelectAll`, `NextField`, `PrevField`,
`NextSection`, `PrevSection`, `Back`, `Help`, `Quit`, `ForceQuit`, `Palette`,
`ConfirmDelete`, `Yes`, `No`, `PickGuided`, `PickByID`, `PickTimer`).

`Generate` (Enter su Home, «generate report») sta **dentro** anche se il suo tasto non è
una rune: è l'azione più usata del programma e una palette che non sa generare un report
sarebbe una palette a metà. `Confirm` resta fuori pur avendo lo stesso tasto — l'esclusione
è per campo, non per tasto.

Tre proprietà cadono gratis da questa derivazione:

- **Il gating è già fatto.** Un binding disabilitato non compare. La stessa `Enabled()`
  che decide cosa il footer annuncia decide cosa la palette offre — non possono divergere.
- **Le parole sono le stesse.** L'etichetta è `b.Help().Desc` con l'iniziale maiuscola
  (`unicode.ToUpper` sulla prima rune, non `strings.ToUpper` sul primo byte). Quello che
  trovi nella palette è quello che il footer ti ha insegnato.
- **L'esecuzione non duplica niente.** `run` ripropone il tasto attraverso `routeKey`.

L'ultima passa da un convertitore minuscolo e deliberatamente parziale:

```go
// keyMsgFor rebuilds the tea.KeyMsg a binding's first key would produce.
// ok is false for any key shape the palette does not replay, so an action
// that cannot be executed faithfully is dropped rather than mis-fired.
func keyMsgFor(s string) (tea.KeyMsg, bool) {
    switch s {
    case "enter": return tea.KeyMsg{Type: tea.KeyEnter}, true
    case "left":  return tea.KeyMsg{Type: tea.KeyLeft}, true
    case "right": return tea.KeyMsg{Type: tea.KeyRight}, true
    }
    if r := []rune(s); len(r) == 1 { return tea.KeyMsg{Type: tea.KeyRunes, Runes: r}, true }
    return tea.KeyMsg{}, false
}
```

`key.Matches` confronta `msg.String()` con le stringhe di `b.Keys()`, e per queste quattro
forme il round-trip è esatto: `{KeyRunes, ['g']}.String() == "g"`,
`{KeyEnter}.String() == "enter"`, e `keyNames[KeyLeft] == "left"` /
`keyNames[KeyRight] == "right"` (`bubbletea@v1.3.10/key.go:301,303`).

`"left"` e `"right"` sono lì perché **servono**: `PrevMonth` e `NextMonth` hanno
`key.WithKeys("left", "h")` e `key.WithKeys("right", "l")` (`keys.go:124-125`), quindi il
loro primo tasto non è una rune. Sono anche gli unici due binding della lista «Dentro» in
questo caso — controllati uno per uno. Senza questo ramo,
`TestEveryPaletteBindingIsReplayable` fallirebbe su Home, dove entrambi sono abilitati, e
le due voci sparirebbero in silenzio dalla palette.

Una precisazione perché la motivazione non venga letta più forte di quello che è:
`{KeyRunes, Runes: []rune("tab")}.String()` vale `"tab"`, quindi il round-trip via
`key.Matches` funzionerebbe anche lì. Quello che si romperebbe è ogni handler che guarda
`msg.Type` invece di `key.Matches`. L'elenco resta chiuso per questo, non perché il
confronto fallisca.

`TestEveryPaletteBindingIsReplayable` cammina la tabella schermata×modo che
`footer_golden_test.go` già possiede e verifica che `keyMsgFor` ritorni `ok` per **ogni**
binding abilitato di `paletteBindings()`. Il `false` è una rete, non una scorciatoia: il
test dice che oggi non si sfrutta mai.

### 6.2 Sorgente 2 — la navigazione globale

Solo `"Go to X"`, più Quit. La riga è netta e vale come regola:

> **Le azioni globali sono solo navigazione. Tutto ciò che cambia stato resta un'azione di
> schermata**, dove il footer la insegna e la palette la ripete.

| Etichetta | Bersaglio | Abilitata quando |
|---|---|---|
| `Go to home` | `screenHome` | sempre |
| `Go to report` | `screenReport` | `len(m.entries) > 0` |
| `Go to rates` | `screenRates` | sempre |
| `Go to budgets` | `screenBudget` | `len(m.entries) > 0` |
| `Go to filters` | `screenFilters` | `len(m.entries) > 0` |
| `Go to entries` | `screenEntries` | `len(m.entries) > 0` |
| `Go to export` | `screenExport` | `len(m.entries) > 0` |
| `Go to range` | `screenRange` | sempre |
| `Go to members` | `screenMembers` | `m.scope == "team"` |
| `Go to log hours` | `screenLog` | sempre |
| `Quit` | — | sempre |

`Go to log hours`, non `Log hours`: su Home e su Report la sorgente 1 produce già
un'azione con etichetta `Log hours`, e due righe **identiche** nella stessa lista non sono
la ridondanza leggibile che la §6.4 accetta — sono un bug che sembra un bug. Il prefisso
`Go to` è quello che tutte le altre righe di navigazione portano già.

`len(m.entries) > 0` è il predicato giusto per report/budgets/filters/entries/export
perché le entries si caricano solo attraverso `entriesMsg`, il cui handler costruisce
sempre `m.report` e `m.rep` prima di atterrare: entries non vuote implica un report
costruito. `Go to members` ricalca il gate di `homeKeys` (`m.scope == "team"`).

Un'azione globale **sparisce quando il suo bersaglio è già `m.screen`**.

### 6.3 L'estrazione delle `openX()` — perché la sorgente 2 non duplica gli handler

Un'azione globale non può fare `m.goTo(screenExport)` e basta: deve anche costruire il
sotto-modello, che oggi è responsabilità del `case` dentro `updateReport`/`updateHome`.
Copiare quel corpo sarebbe la seconda fonte di verità che tutto il resto del design evita.

Quindi si estraggono, come metodi di `Model`, esattamente nella forma che
**`openListBrowser()` ha già in `app.go:533`** e che `openEntries()` / `openBudgetView()`
hanno già:

| Metodo | Corpo estratto da | Firma |
|---|---|---|
| `openExport()` | `report.go` case `Export` | `func (m Model) openExport() Model` |
| `openRates()` | `report.go` case `Rates` | `func (m Model) openRates() Model` |
| `openLog()` | `report.go` case `LogHours` + `home.go` case `LogHours` | `func (m Model) openLog() Model` |
| `openRange()` | `home.go` case `Range` | `func (m Model) openRange() Model` |
| `openFilters()` | `report.go` case `Filters` | `func (m Model) openFilters() (Model, tea.Cmd)` |
| `openMembers(origin screen)` | `home.go` case `Members` | `func (m Model) openMembers(origin screen) (Model, tea.Cmd)` |

Handler e azione globale chiamano lo stesso metodo. `openBudgetView()` esiste già e non si
tocca. Due firme meritano una riga di motivazione:

- **`openMembers` prende `origin`** invece di passare `screenHome` fisso come fa oggi
  `loadMembersCmd`. Quel parametro decide dove atterra un `retryableErrMsg`: con
  `screenHome` fisso, un errore di caricamento membri lanciato dalla palette mentre sei su
  Rates ti teletrasporterebbe a Home con un errore inline, attribuendo il guasto a una
  schermata su cui non eri. Home passa `screenHome`, la palette passa `m.screen`.
- **`openEntries()` diventa `(Model, tea.Cmd)`**, assorbendo il recupero pigro di
  `currentUserCmd()` che oggi vive nel `case OpenEntries` di `report.go:119-125`. Senza
  questo l'azione globale dovrebbe ricopiarlo, che è esattamente la duplicazione che
  l'estrazione elimina.

Questa estrazione è un miglioramento del codice esistente nell'area che stiamo comunque
modificando, non un refactoring opportunistico: senza di essa la sorgente 2 non è
scrivibile senza duplicare.

### 6.4 La ridondanza accettata

Sul Report compaiono sia `Export  e` (sorgente 1) sia `Go to export` (sorgente 2). Sono la
stessa cosa e funzionano entrambe.

Sopprimerne una richiederebbe una mappa binding→schermata scritta a mano, cioè esattamente
la seconda fonte di verità che la derivazione della §6.1 esiste per evitare. Le due righe
si leggono diverse — «fai qui» contro «portami là» — e il costo è una riga in più in una
lista filtrata.

### 6.5 Ordinamento

`paletteActions(m)` concatena sorgente 1 poi sorgente 2. Con una query, ordinamento per
punteggio decrescente; a parità di punteggio vince l'ordine di costruzione (sorgente 1
prima, poi ordine di registro). L'ordinamento è **stabile** (`sort.SliceStable`).

Con query vuota tutti i punteggi valgono 0, quindi l'ordine è quello di costruzione: le
azioni della schermata corrente in cima. «Cosa posso fare qui» prima di «portami altrove».

---

## 7. `internal/fuzzy`

```go
// Match reports whether query matches target as a case-insensitive subsequence.
// When ok, score ranks the match and idx holds the rune indices of target that
// were matched, in ascending order.
//
// An empty query always matches with score 0 and a nil idx.
func Match(query, target string) (score int, idx []int, ok bool)
```

### 7.1 Perché programmazione dinamica e non greedy

Greedy prende sempre la prima occorrenza utile. Con query `"rt"` e target `"report table"`
prende `r@0` e `t@5` — cioè evidenzia `[r]epor[t] table`. Il match giusto è `r@0`, `t@7`:
`[r]eport [t]able`. Gli indici non ordinano soltanto: **decidono quali lettere si
illuminano**, quindi un match subottimo è un bug visibile.

`best[j][i]` = miglior punteggio che aggancia le prime `j+1` rune della query con `q[j]`
su `t[i]`, con puntatori al predecessore per ricostruire `idx`. Costo `O(|q|·|t|²)`, che
su etichette da quaranta rune non è niente.

### 7.2 Punteggio

Confronto case-insensitive (`unicode.ToLower` su entrambe le rune).

| Contributo | Valore | Quando |
|---|---|---|
| Consecutivo | `+10` | la rune agganciata segue immediatamente la precedente |
| Inizio parola | `+8` | indice 0, o la rune precedente è uno spazio, `-`, `_`, `/` o `.` |
| Gap iniziale | `-min(i₀, 10)` | `i₀` è l'indice della prima rune agganciata |

Il gap iniziale è limitato a 10 di proposito: senza limite un'etichetta lunga verrebbe
penalizzata al punto da perdere contro un match peggiore ma più a sinistra.

Ogni candidato consuma **tutta** la query, quindi non serve un contributo per rune: sarebbe
costante fra i candidati e non ordinerebbe niente.

I test bloccano gli **ordinamenti osservabili** (`"exp"` mette `Export` sopra `Go to
export`; `"rt"` su `"report table"` restituisce `[0, 7]`), più un caso minuscolo con
punteggio esatto. Le costanti si possono ritoccare senza riscrivere la suite.

`Export`, non `Export report`: la sorgente 1 produce `capitalize(b.Help().Desc)`, e il desc
di `Export` è `"export"` (`keys.go:148`). Una prima stesura di questa sezione citava
un'etichetta che nessuna delle due sorgenti genera. La conclusione regge con i valori veri
(`Export` 28 punti contro `Go to export` 22), ma il test va scritto sull'etichetta reale.

---

## 8. La palette

### 8.1 Il modello

```go
type paletteItem struct {
    a     action
    idx   []int // indici di rune agganciati, da fuzzy.Match — servono a evidenziare
    score int
}

type paletteModel struct {
    query string        // append di rune + backspace
    items []paletteItem // filtrato e ordinato
    idx   int           // riga selezionata dentro items
    top   int           // prima riga visibile (finestra di scorrimento)
}
```

Non c'è un campo con l'elenco completo: a ogni cambio di query `updateOverlay` (che è un
metodo di `Model`) ricalcola da zero con `paletteActions(m)` e rifiltra. Venti voci per un
match fuzzy non sono niente, e non tenere una copia significa non poterla far invecchiare.

`idx` e `top` tornano a 0 a ogni cambio di query.

Il ricalcolo è legato ai tasti, non ai messaggi: se un `runningTimerMsg` azzera il timer
mentre la palette è aperta, la riga `Stop timer` resta visibile fino al tasto successivo.
**Accettato.** L'esecuzione è comunque sicura — il replay ripassa da `key.Matches` con
l'enablement aggiornato, quindi una riga stantia è un no-op, non un'azione sbagliata — e
ricalcolare a ogni msg asincrono significherebbe far muovere la lista sotto le dita di chi
sta scegliendo.

Lo scorrimento è quello classico e tiene il cursore sempre visibile:
`if idx < top { top = idx }`, `if idx >= top+righe { top = idx - righe + 1 }`. È
letteralmente la cosa che manca alla schermata Filters ed è una casella aperta di #28: qui
si fa giusta dall'inizio.

### 8.2 Geometria — valori esatti

```go
const (
    paletteWidth    = 52 // larghezza preferita del box, bordi inclusi
    paletteMinWidth = 24 // sotto questa non si scende
    paletteMaxRows  = 8  // righe di azione visibili
    paletteChrome   = 4  // bordo alto + riga query + separatore + bordo basso
    paletteTopY     = 2  // riga del corpo su cui poggia il bordo superiore
)
```

- `boxW = paletteWidth`; se `m.width > 0`, `boxW = max(paletteMinWidth, min(paletteWidth, m.width-4))`.
- `x = max(0, (m.width - boxW) / 2)`; con `m.width == 0` (nessun `WindowSizeMsg` ancora
  arrivato) vale `x = 0`.
- `y = paletteTopY`, così la riga di titolo della schermata sottostante e la riga vuota
  sotto di essa restano leggibili — **ma corretto verso il basso quando il corpo è più
  alto del terminale**. Il renderer standard di bubbletea, quando la view supera l'altezza,
  tiene le **ultime** `m.height` righe (`bubbletea@v1.3.10/standard_renderer.go:186-188`).
  `y` è una coordinata del corpo, non del terminale: su un report raggruppato per task con
  cinquanta bucket, un box a `y = 2` finisce **sopra il bordo visibile** e l'utente scrive
  in una palette che non vede — proprio sulla schermata dove la palette serve di più.

  Quindi `layout` riceve anche il numero di righe del corpo e sposta il box esattamente di
  quanto il corpo verrà scorso via:

  ```go
  y := paletteTopY
  if height > 0 {
      if overflow := bodyLines + 2 - height; overflow > 0 { // +2 = riga vuota + footer
          y += overflow
      }
  }
  ```
- `righe = paletteMaxRows`; se `m.height > 0`,
  `righe = max(3, min(paletteMaxRows, m.height - paletteChrome - paletteTopY - 2))`.
  I due termini sottratti sono la riga vuota e il footer che `View()` aggiunge sempre; il
  pavimento a 3 dice che sotto una certa altezza la palette si stringe ma non sparisce.

Se `m.width` è così stretto che `boxW` finisce a `paletteMinWidth` e sfora comunque, il box
resta a 24 colonne e `x` vale 0: preferiamo un box che sborda a destra a un box illeggibile.
**È il comportamento voluto**, e ha un test che lo dichiara.

### 8.3 La resa

Costruzione manuale, riga per riga, con `th.Border` per la cornice (lo stesso token della
tabella del report) — vedi §2.5:

```
╭─ Command palette ──────────────────────────────────╮
│ > exp                                              │
├────────────────────────────────────────────────────┤
│ ▸ Export report                                 e  │
│   Go to export                                     │
╰────────────────────────────────────────────────────╯
```

- Il cursore è `▸ ` / `  `, la convenzione di `members.go`, `export.go` e `filters.go`.
- Le rune agganciate si illuminano in `th.Accent`. Sotto `termenv.Ascii` i golden non lo
  vedono, quindi serve un **test diretto sullo stile** — la lezione della tranche C.
- La riga selezionata: prefisso `▸` in `th.Accent`, testo in `th.Cell`. Lo sfondo `th.Zebra`
  **non** si usa qui: si scontrerebbe con l'evidenziazione delle rune agganciate.
- Il suggerimento tasto è allineato a destra dentro la larghezza interna, in `th.Help`.
- L'etichetta si tronca **per larghezza di visualizzazione**, riusando `shaveToWidth`
  (`report_table.go:250`), non per rune. Il troncamento per rune contro larghezza di
  visualizzazione è già costato un bug in tranche C: la palette non diventa un sesto sito
  rotto di **#135**.
- **Prima si tronca, poi si evidenzia.** `fuzzy.Match` restituisce indici di rune
  dell'etichetta *intera*; applicarli dopo il taglio richiede di scartare quelli che cadono
  oltre. La regola esatta: dopo `shaveToWidth`, sopravvivono gli indici `< len(rune
  dell'etichetta troncata) - 1` quando il troncamento ha aggiunto `…` (l'ellissi non si
  illumina), altrimenti `< len(rune dell'etichetta troncata)`. L'ordine inverso —
  evidenziare e poi tagliare — spezzerebbe le sequenze di escape a metà.
  `TestPaletteHighlightSurvivesTruncation` copre un'etichetta più lunga del box.
- Nessun risultato → una riga sola, `no matching action`, in `th.Help`. Enter non fa niente.

### 8.4 I tasti della palette

| Tasto | Effetto |
|---|---|
| rune stampabile | appesa alla query; `items` ricalcolato; `idx`, `top` a 0 |
| `backspace` | ultima rune tolta; stesso ricalcolo |
| `↑` / `↓` | `idx` si muove, con clamp agli estremi (nessun wrap) |
| `enter` | chiude l'overlay **e poi** esegue l'azione selezionata |
| `esc` / `ctrl+p` | chiude l'overlay; nessun altro stato cambia |
| `ctrl+c` | esce dal programma (intercettato prima, §5.2) |

La condizione esatta per «rune stampabile» è
`(msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace) && !msg.Alt`, e le rune si leggono
sempre da `msg.Runes`. Lo spazio ha bisogno del suo ramo: bubbletea v1 riclassifica una
rune sola pari a `' '` come `KeySpace` pur lasciandola in `Runes`
(`bubbletea@v1.3.10/key.go:698-702`). Senza quel ramo la query non potrebbe contenere
spazi, e `"log h"` non troverebbe niente.

L'ordine su `enter` conta: l'overlay va chiuso **prima** di eseguire, altrimenti l'azione
cambia schermata e la palette resta disegnata sopra la nuova.

`openPalette` azzera anche `m.helpAll`. Il footer full occupa più righe di quello breve, e
la formula d'altezza della §8.2 sottrae **due** righe (riga vuota + footer) presumendone
una sola: aprire la palette con l'aiuto esteso già aperto sballerebbe il conto. Chiudere
l'aiuto esteso è anche la cosa giusta a prescindere — sono due modi di rispondere alla
stessa domanda, e non serve mostrarli insieme.

`paletteKeys(d)` assegna `Back` (esc), `Confirm` (enter), `Up`, `Down`, `ForceQuit` e
`Palette` — quest'ultimo perché `ctrl+p` chiude come esc — e lascia `Quit`, `Help` e ogni
altro binding non assegnati. In `short` va solo `↑/↓ move · enter run · esc close ·
ctrl+c force quit`: `ctrl+p` chiude ma non si pubblicizza, perché `esc` è la via che tutte
le altre schermate insegnano già.

> **Emendamento (scritto durante il piano).** I campi `Up`/`Down` della `keyMap` della
> palette **non** possono ricevere `d.Up`/`d.Down`: quei default valgono `up`/`k` e
> `down`/`j`, quindi scrivere `j` o `k` nella query muoverebbe il cursore invece di
> digitare una lettera. Servono due default nuovi in `keyDefaults`, solo frecce:
>
> ```go
> PaletteUp:   key.NewBinding(key.WithKeys("up"),   key.WithHelp("↑", "move up")),
> PaletteDown: key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "move down")),
> ```
>
> assegnati ai campi `Up`/`Down` della keyMap della palette (nessun campo nuovo in
> `keyMap`, quindi `TestAllBindingsCoversEveryField` non cambia; `keyDefaults` non ha un
> test di parità per riflessione). `TestPaletteTypesJAndK` va scritto contro la versione
> che usa `d.Up`/`d.Down` e verificato fallente.
>
> Il backspace **non** diventa un binding: è editing di testo, che nessuna delle dieci
> schermate con un `textinput` focalizzato pubblicizza. `updateOverlay` controlla
> `msg.Type == tea.KeyBackspace` direttamente.

---

## 9. Test

### 9.1 `internal/fuzzy`

Table-driven: query vuota; nessun match; match esatto; sottosequenza; case-insensitivity in
entrambe le direzioni; bonus di inizio parola; bonus consecutivo; **ottimalità della DP**
(`"rt"` su `"report table"` → `[0, 7]`, il caso che smaschera greedy); target non-ASCII con
rune multibyte (gli indici sono indici di **rune**, non di byte); query più lunga del target.

### 9.2 `overlay.go`

I quattro casi limite della §4.2, ognuno **verificato fallente** contro l'implementazione
che lo ignora, con la trascrizione allegata al report dell'implementer. Più: splice a
`x == 0`; box più largo del corpo; larghezza di ogni riga composta pari a
`max(larghezzaCorpo, x+larghezzaBox)`.

Il test di larghezza vuole **due** fixture: una ASCII e una con un glifo wide (CJK) messo
a cavallo di entrambi i bordi del box. Con la sola fixture ASCII quel test passa contro il
bug del taglio straddle della §4.1 — che è precisamente il modo in cui, nella tranche C, un
test di larghezza è passato sopra un overflow da 78 colonne in un terminale da 60.

### 9.3 `actions.go`

Le azioni di schermata coincidono con i binding abilitati di `paletteBindings()` su ogni
schermata; `TestEveryPaletteBindingIsReplayable` (§6.1); l'azione globale col bersaglio
uguale a `m.screen` sparisce; i gate (`Go to members` solo team, i cinque
`len(m.entries) > 0`); l'etichetta ha l'iniziale maiuscola anche con una prima rune
multibyte.

### 9.4 `palette.go` e integrazione

`ctrl+p` apre e non tocca `m.nav`; `esc` chiude e non fa `pop()`; scrivere `q` non chiude
il programma (test di regressione, **non** un RED — vedi §5.2);
`TestPaletteCtrlPClosesRatherThanReopening` (§5.2, questo sì scritto contro l'ordine
sbagliato e verificato fallente); `TestPaletteKeepsScreenActionsWhileTyping` (§5.2b,
scritto contro la versione che chiama `keysFor` e verificato fallente); `?` diventa un
carattere della query e non ribalta `m.helpAll`; **lo spazio entra nella query**
(`tea.KeySpace`, §8.4); `j` e `k` sono lettere, non movimenti (emendamento §8.4);
backspace su query vuota è un no-op; `↑`/`↓` clampano; la finestra di scorrimento tiene il
cursore visibile con più azioni che righe; `TestPaletteHighlightSurvivesTruncation` (§8.3,
con una query che aggancia rune **oltre** il taglio, altrimenti il ramo non viene
esercitato); l'evidenziazione usa davvero `th.Accent` (renderer con profilo a colori, non
`testTheme`, che fissa Ascii); `enter` su lista vuota è un no-op; `enter` su un'azione di
schermata produce lo stesso `Model` della pressione diretta del tasto; `enter` su
un'azione globale apre la schermata giusta; `ctrl+p` non apre su Setup, Loading, Error né
su `entriesConfirmDelete`; **un 401 che rilancia il wizard dissolve la palette** e
**un `retryableErrMsg` che atterra su `screenError` la dissolve** (§5.2c); il box scende
quando il corpo è più alto del terminale (§8.2).

Golden nuovi: `palette_report` (palette sopra il Report a larghezza fissa),
`palette_filtered` (con query), `palette_no_match`, `palette_narrow`.
Golden rigenerati: `footer_*_full` delle schermate che ora annunciano `ctrl+p`.

I golden si rigenerano **solo** con `go test ./internal/tui -update`, mai a mano.

### 9.5 Parità con la demo

`CLICKUP_DEMO=1` deve dare la stessa palette: nessuna azione fa I/O all'apertura, e le
azioni globali che ne fanno (`Go to filters` → arricchimento stati, `Go to members`)
passano già dai rami demo dei rispettivi `openX()`. Un test in demo mode lo verifica.

---

## 10. Fuori scope, dichiarato

- **`?` come pannello flottante** — decisione definitiva, §2.3, nessuna issue aperta.
- **Switcher mese/lista/membro di #71** — issue nuova nella milestone v1.12.
- **I siti di troncamento preesistenti di #135** — quindici, su `budget.go`,
  `rates_view.go`, `entries.go` e `log.go` — restano a #135. Questa tranche ne introduce
  uno nuovo e lo fa giusto; non ripara i vecchi.
- **Un secondo tipo di overlay.** `overlayKind` è un iota con due valori perché ne esiste
  un cliente. Il terzo valore lo aggiunge chi porta il terzo cliente.
- **Ricerca nel testo del report, azioni componibili, cronologia dei comandi.** Nessuna
  delle tre è in #71.

---

## 11. Trappole ereditate da onorare

Dalle tranche precedenti, e valide anche qui:

1. **Un test scritto contro un bug specifico va verificato fallente contro quel bug**, con
   la trascrizione allegata al report. Nella tranche C quattro test sono stati colti a
   passare sul bug che dovevano prendere.
2. **I golden sono ciechi al colore.** `TestMain` fissa `termenv.Ascii`, che toglie sfondi
   *e* grassetto. Ogni decisione di colore vuole un test diretto sullo `lipgloss.Style`
   restituito.
3. **Larghezza di visualizzazione ≠ conteggio di rune.** Coincidono solo in ASCII.
4. **Nessuna funzione di stile chiama `lipgloss.NewStyle()`**: si passa dal `theme`, che
   porta il renderer iniettato. `th.Cell` esiste per questo.
5. **`Co-Authored-By` non compare in nessun messaggio di commit.**
6. Tutto ciò che vive nel repo è in **inglese** — codice, commenti, stringhe UI, messaggi
   dei test, messaggi di commit. Questa spec è l'eccezione storica di `docs/superpowers/`.
