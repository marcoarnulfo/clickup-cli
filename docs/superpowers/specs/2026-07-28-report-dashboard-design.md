# Report dashboard: griglia, sparkline e gauge (v1.9, tranche C) — Design

> Spec della quarta tranche della milestone **v1.9 TUI design system**.
> Copre #117, #66 e #80. Ramo: `feat/report-dashboard`.

## 1. Obiettivo

Tre cose, in quest'ordine:

1. **#117** — collassare i quattro indici di selezione paralleli dell'editor di
   fatturazione in un solo array indicizzato per sezione. Refactor puro,
   nessun cambio di comportamento.
2. **#66** — sostituire il report costruito a mano con `fmt.Sprintf` con una
   `lipgloss/table` vera: header, righe zebra, colonne numeriche allineate a
   destra, TOTAL colorato, **larghezza adattiva al terminale**.
3. **#80** — trasformare il report in un cruscotto: una sparkline delle ore per
   giorno sul report, e un gauge a blocchi al posto della barra ASCII nella
   schermata Budget.

L'esito visibile: il report smette di essere una colonna fissa a 32 caratteri
che va a capo su un terminale stretto e spreca spazio su uno largo, e guadagna
un colpo d'occhio sull'andamento del periodo.

## 2. Perché #117 va per primo

Lo dice la sua stessa issue: entrambe le review della #116 hanno riconosciuto la
duplicazione e l'hanno rimandata alla v1.9 «perché è un refactor meccanico di
uno schermo che il design system ristrutturerà comunque», e il reviewer
whole-branch ha aggiunto che gli switch a 4 vie di `move`/`selCount` sono «già
al limite della manutenibilità», quindi va affrontato **per primo** quando la
v1.9 tocca quello schermo.

La v1.9 quello schermo l'ha già toccato tre volte (tranche A gli ha dato il
tema, B1 la keymap, B2 il footer e la nota sulla tariffa zero) senza mai fare il
refactor. Questa è l'ultima tranche che lo tocca: o adesso o l'issue resta
aperta a milestone chiusa.

## 3. Stato attuale (misurato, non supposto)

### 3.1 `internal/tui/rates.go`

- `ratesModel` porta quattro campi di selezione: `idx` (Liste), `memIdx`
  (Membri), `ovIdx` (Override), `ruleIdx` (Regole).
- `move` fa **due switch**, uno per leggere l'indice della sezione attiva (a 3
  vie: `secLists` è gestito dall'inizializzatore `cur := rt.idx`) e uno a 4 vie
  per riscriverlo. È l'unica funzione la cui duplicazione sia interamente
  eliminabile.
- `selCount`, `startEdit` e `clearSelected` fanno anch'esse uno switch sulla
  sezione, ma **il loro switch non è duplicazione**: i conteggi sono
  genuinamente diversi (`len(rows)`, `len(members)`, `len(overrides)+1`,
  `ruleCount`) e i comportamenti di modifica/cancellazione divergono per
  sezione. In quelle funzioni cambia solo la *lettura* dell'indice.
- `commit` **non** fa uno switch sulla sezione: il suo switch è a 8 vie su
  `rt.edit` (`editKind`). Legge i quattro indici, e basta. La issue #117 dice
  altro; la issue si sbaglia.
- `rates_view.go` legge i quattro campi in **sette occorrenze** distribuite
  sulle quattro sezioni (righe 54 e 56 per le Liste, 77 e 79 per i Membri, 95 e
  97 per gli Override, 134 per le Regole).
- **C'è uno scrittore esterno al file**: `listbrowser.go:152` fa `rt.idx = found`
  quando il browser delle liste restituisce una lista alla sezione Liste.
- **Nessun test tocca i quattro campi direttamente.** I 31 `TestRates*` di
  `rates_test.go` guidano tutti da messaggi di tastiera. La suite è quindi una
  rete di sicurezza vera, non una che verifica l'implementazione: è per questo
  che il refactor si può fare senza riscriverla.

### 3.2 `internal/tui/report.go`

- `reportModel.view` costruisce l'header e ogni riga con `fmt.Sprintf` a
  larghezze fisse: `%-32s %8s %8s %s`. Le etichette si troncano a 32 rune.
- La colonna `Amount` è l'ultima e non è allineata: `%s` la lascia crescere a
  destra, quindi `625.00 EUR` e `0.00 EUR` non si incolonnano (visibile nel
  golden attuale).
- Il blocco totali, i sottototali per valuta e la nota
  `billable … · non-billable …` finiscono **dentro** `th.Box` insieme alle
  righe, quindi la nota oggi sta dentro il bordo.
- Larghezza risultante: **63 colonne** di contenuto più il bordo di `th.Box`.
- `reportModel` porta solo `report.Report` e una nota. **Non ha le entry**, il
  che vincola da dove può arrivare la serie giornaliera (§6.3).
- `newReport` ha **4 siti di produzione** (`app.go:689`, `rates.go:869`,
  `report.go:73`, `report.go:195`) e 6 siti nei test.

### 3.3 `internal/tui/budget.go`

- `renderBudgetBar(percentUsed float64) string` produce `[####----] 60%` con
  glifi ASCII, riempimento clampato a `[0, 100]` e **percentuale non clampata**
  — e il commento dice che è deliberato, perché sforare il budget è
  esattamente ciò che quella vista deve far vedere.
- La funzione non riceve il tema: la barra è monocroma.
- La riga di budget renderizzata è larga **94 colonne** (golden
  `budget.golden`), quindi già oggi va a capo su un terminale da 80. Fuori
  ambito, §9.

### 3.4 Vincoli dell'ambiente di test

- `TestMain` fissa `lipgloss.SetColorProfile(termenv.Ascii)`, quindi **i golden
  non contengono ANSI**. Ne discende che i golden **non possono vedere**:
  - le righe zebra (uno sfondo);
  - il colore rosso di uno sforamento di budget;
  - quale stile porta la riga TOTAL.

  Questa non è una limitazione da aggirare: i golden restano la rete per il
  *layout*, e ciò che è invisibile ad Ascii richiede test diretti sugli stili
  (§7). Lo stesso ragionamento della tranche A, dove i golden non potevano
  vedere due ruoli di stile scambiati.

### 3.5 Cosa fanno davvero le librerie

Verificato leggendo il sorgente delle versioni bloccate in `go.mod`, non la
documentazione:

- **`lipgloss/table` v1.1.0** — `table.New()` lascia `borderStyle` al suo valore
  zero, e uno `lipgloss.Style` zero renderizza attraverso **il renderer di
  default**: va iniettato con `BorderStyle`, altrimenti è la stessa falla che la
  tranche B2 ha evitato non usando `help.New()`. I default sono
  `borderColumn: true` (separatori fra
  colonne) e **`wrap: true`** — quest'ultimo manderebbe a capo un'etichetta
  lunga invece di troncarla, spezzando l'invariante "una riga per bucket".
  Entrambi vanno spenti.
- **`bubbles/progress` v1.0.0** — `percentageView` fa
  `math.Max(0, math.Min(1, percent))` **prima** di formattare: un budget
  sforato al 130% si legge `100%`. Inoltre `New()` prende
  `termenv.ColorProfile()`, cioè il terminale vero e non il renderer iniettato,
  quindi userebbe richiedere l'esposizione del profilo colore sul `theme`. La
  libreria **non viene adottata**: §6.4.

## 4. Decisioni prese

| Domanda | Decisione |
|---|---|
| Dove va il gauge, dato che sommare budget in valute diverse è vietato dalla visione di progetto (multi-valuta a subtotali, no FX)? | Sparkline sul report, gauge nella schermata Budget: ogni `BudgetLine` porta già la sua valuta, quindi il problema FX non si pone e non c'è nessuno stato condizionale da specificare. |
| Quanto la tabella si adatta al terminale? | Adattiva: le colonne numeriche restano fisse, tutto lo spazio in più o in meno va alla colonna `Item`. |
| Come rendiamo il gauge? | Con la nostra funzione pura, potenziata — non con `bubbles/progress`, che clampa il numero che la vista esiste per mostrare. |
| Come facciamo le righe zebra? | Sfondo alternato da un nuovo token `Subtle` della palette. |

## 5. Fuori ambito

- **`bubbles/progress`**: motivato in §6.4. La checkbox della #80 va chiusa con
  un commento che spiega perché.
- **La larghezza della schermata Budget.** La sua riga è larga 94 colonne e va
  a capo a 80. È un difetto **preesistente**, non introdotto qui: questa
  tranche cambia i glifi e i colori di quella riga, non la sua struttura. Va
  aperta una issue dedicata prima di chiudere la tranche — non basta scriverlo
  qui, perché una spec non è un tracker.
- **La riorganizzazione di `rates.go`** oltre la #117. Il file è a 914 righe,
  ma la #117 chiede un refactor puro e nient'altro.
- **`report.Report.Daily`**: la serie giornaliera *non* entra nel tipo di
  dominio né nell'output JSON del CLI (§6.3).

## 6. Disegno

### 6.1 #117 — `sel [secCount]int`

`ratesModel` perde `idx`, `memIdx`, `ovIdx`, `ruleIdx` e guadagna:

```go
// sel is the selected row of each section, indexed by ratesSection. One array
// instead of four fields means move() needs no switch at all, and a fifth
// section is a non-event.
sel [secCount]int
```

`move` diventa:

```go
func (rt ratesModel) move(delta int) ratesModel {
	next := rt.sel[rt.sec] + delta
	if next < 0 || next > rt.selCount()-1 {
		return rt
	}
	rt.sel[rt.sec] = next
	return rt
}
```

Ogni altra lettura diventa esplicita nella sezione in cui si trova —
`rt.rows[rt.sel[secLists]]`, `rt.members[rt.sel[secMembers]]`,
`rt.sel[secOverrides]`, `rt.sel[secRules]` — **non** `rt.sel[rt.sec]`: dentro un
`case secLists:` la costante nomina l'invariante, la variabile la nasconde.

`listbrowser.go:152` diventa `rt.sel[secLists] = found`.

**Il valore zero dell'array è identico a quattro `int` a zero**, quindi ogni
golden esistente resta byte per byte lo stesso e i 31 test di `rates_test.go`
non si toccano.

**Nota per chi implementa: qui non esiste un RED.** È un refactor puro: il test
nuovo (§7.1) deve passare *prima* del refactor, perché fissa la proprietà per
cui i quattro indici esistevano. Un implementatore a cui si dice «TDD RED→GREEN»
su un refactor si inventa un test rotto per poterlo far diventare verde.

### 6.2 #66 — la tabella

Nuovo file `internal/tui/report_table.go`, così `report.go` resta sulle sue
~270 righe invece di gonfiarsi:

```go
// reportTable renders the report's buckets and totals as one table, sized to
// width. width <= 0 means "natural width" (the first render, before the
// terminal has sent its WindowSizeMsg).
func reportTable(th theme, r report.Report, width int) string
```

**Aritmetica delle colonne** — deve essere esatta, o due implementatori
scrivono due tabelle diverse:

- `Hours` e `Billed`: larghezza fissa **8**.
- `Amount`: la più lunga delle stringhe effettivamente renderizzate, **senza
  tetto**. Troncare un importo nasconde soldi; se un bucket multi-valuta è
  largo, a cedere è l'etichetta.
- Overhead: **2** bordi verticali (solo esterni: i separatori fra colonne sono
  spenti, vedi sotto) + 4 colonne × 2 di padding = **10**. La costante è una
  previsione sul comportamento di `lipgloss/table`, non un dato: l'invariante
  vincolante è la *larghezza renderizzata*, ed è il test §7.3 a stabilirla. Se
  il conteggio reale risulta diverso, si corregge la costante, non il test.
- `itemW = width - 10 - 8 - 8 - amountW`, dove `maxLabel` è la più lunga fra le
  etichette dei bucket **e** quelle dei totali (`TOTAL`, `  subtotal EUR`), poi:
  - non si allarga sopra `maxLabel`: niente spazio vuoto stirato;
  - non si stringe sotto 12: a quel punto si accetta lo sforamento, perché a 30
    colonne non c'è niente da salvare.
  - **Quando `maxLabel` è minore di 12 vince il tetto, non il pavimento.** Non è
    un caso di scuola: `GroupByTotal` è il raggruppamento del primo caricamento
    e le sue etichette sono `Total` e `TOTAL`, cioè `maxLabel = 5`. Un
    `clamp(x, 12, 5)` è un intervallo invertito, e prendere il pavimento per
    primo stirerebbe la colonna a 12 contraddicendo la riga sopra. La formula è
    quindi `max(min(12, maxLabel), min(maxLabel, disponibile))`.
- `width <= 0` → `itemW = min(maxLabel, 32)`. È la stessa **regola** di prima
  della #66 (un tetto a 32 colonne), non lo stesso rendering: oggi la colonna è
  larga 32 fisse anche quando le etichette sono corte, mentre questa si adatta
  al contenuto. Il primo render cambierà aspetto, ed è voluto.

Le etichette si troncano a `itemW` con la `truncate` esistente, che taglia per
rune e non per byte.

**Il dimensionamento è implicito, e `Table.Width()` non va chiamata.** Le
colonne si dimensionano da sole sul contenuto: celle numeriche già formattate,
etichette già troncate a `itemW`, `Padding(0, 1)` applicato a ogni cella dalla
`StyleFunc`. `Table.Width(w)` è una trappola con questa configurazione: in
`resizing.go`, `totalHorizontalBorder()` conta **solo i separatori fra colonne**,
quindi con `BorderColumn(false)` restituisce 0 e ignora i due bordi esterni,
mentre `String()` taglia l'output con `MaxWidth(t.width)`. Il risultato è una
tabella renderizzata a `w+2` e poi amputata a `w`: **il bordo destro sparisce**.
Il suo resizer inoltre restringe le colonne più larghe senza riguardo per le
larghezze fisse (che onora solo via `style.GetWidth()`, dove `Style.Width`
include il padding, quindi «8» darebbe 6 colonne di contenuto).

**Configurazione della tabella:**

- `Border(lipgloss.RoundedBorder())`, `BorderStyle(th.Border)` — iniettato,
  §3.5.
- `BorderColumn(false)`: nessun separatore verticale fra colonne, come il
  layout attuale.
- `BorderHeader(true)`: una riga di separazione sotto l'header. È l'unico
  separatore orizzontale che si ottiene, perché `BorderRow` è tutto-o-niente.
- `Wrap(false)`: il troncamento lo facciamo noi, §3.5.
- Allineamento: `Item` a sinistra, `Hours`/`Billed`/`Amount` a destra.

**Righe:** i bucket, poi il blocco totali, **nella stessa tabella** — è tutto il
punto dell'allineamento. Il blocco totali è una riga sola quando il report è a
valuta singola, altrimenti una riga `TOTAL` con la cella `Amount` vuota seguita
da un sottototale per valuta. Il TOTAL si distingue per grassetto e colore, non
per una linea, perché `lipgloss/table` non sa disegnare un separatore su una
riga sola.

**La nota `billable … · non-billable …` esce dal bordo** e va sotto la tabella.
Oggi sta dentro `th.Box` insieme ai dati; non è una riga di dati, e dentro una
tabella si prenderebbe la zebratura e la divisione in colonne. Questo cambia il
golden ed è voluto.

`th.Box` esce dal report — il bordo lo disegna la tabella — e resta per il caso
vuoto (`No hours to show.`) e per tutte le altre schermate.

**Firma:** `reportModel.view(th theme, width int)`. È la prima view del progetto
a ricevere la larghezza; `screenBody` le passa `m.width`, lo stesso valore che
il footer già usa.

**Il tema** guadagna due stili e la palette un token:

```go
// palette
Subtle lipgloss.AdaptiveColor // zebra row background

// theme
Border lipgloss.Style // the report table's frame: Muted foreground
Zebra  lipgloss.Style // alternate row background
```

`Subtle` va commentato come gli altri cinque token, ma il criterio è diverso:
236 su scuro e 254 su chiaro sono scelti perché il **foreground di default
continui a passare il contrasto sopra di essi**, non contro il fondo del
terminale.

### 6.3 #80 — la sparkline

La serie giornaliera è dominio, quindi sta in `internal/report/daily.go`, puro:

```go
// DailyHours returns one element per day in [start, end), each the total hours
// of the entries that started that day.
func DailyHours(entries []TimeEntry, start, end time.Time, loc *time.Location) []float64
```

Regole, tutte vincolanti:

- Le ore di un'entry sono accreditate **al giorno in cui l'entry inizia**,
  letto in `loc` — la stessa regola di `groupKeys` per `GroupByDay`, così le
  due viste non possono divergere.
- **Si contano tutte le ore, fatturabili e non**, coerentemente con
  `Bucket.Hours` e con l'etichetta `hours/day`. Non è ovvio e va detto: la
  sparkline risponde a «quando ho lavorato», non a «quando ho fatturato», e la
  quota fatturabile ha già la sua riga sotto la tabella.
- I giorni si iterano per `AddDate(0, 0, 1)` sulla mezzanotte in `loc` e si
  indicizzano per chiave formattata, **mai** per aritmetica sui secondi: con
  l'ora legale un giorno non dura 24 ore.
- Le entry il cui giorno cade fuori da `[start, end)` si ignorano.
- `end <= start` → `nil`.

Il riempimento a zero è il motivo per cui non si riusano i bucket:
`GroupByDay` **non crea un bucket per i giorni vuoti**, quindi una sparkline
costruita sui bucket comprimerebbe le pause e mostrerebbe un mese pieno per tre
giorni di lavoro.

Il rendering è presentazione e sta separato, in `internal/tui/sparkline.go`:

```go
// sparkline renders values as block glyphs, resampled to at most maxCells.
func sparkline(values []float64, maxCells int) string
```

- Zero → **spazio**. Una pausa deve leggersi come una pausa; `▁` direbbe «un
  po' di lavoro». La sparkline è sempre seguita da un'etichetta, quindi nessuno
  spazio finisce a fine riga.
- Non-zero → `▁▂▃▄▅▆▇█`, livello `clamp(ceil(v/max*8), 1, 8)`.
- Oltre `maxCells`: bucket contigui `values[i*n/k : (i+1)*n/k]` con `k = maxCells`,
  media aritmetica di ciascuno. Aritmetica intera, resto distribuito in modo
  deterministico. Serve davvero: un range custom di un anno sono 365 celle.
- `values` vuoto → stringa vuota.

**Da dove arrivano i valori.** `reportModel` non ha le entry (§3.2), quindi la
serie le viene passata: `newReport(r report.Report, note string, daily []float64)`.
I 4 siti di produzione la calcolano da un helper sul Model, che ha già range,
entry visibili e `loc`:

```go
func (m Model) dailySeries() []float64
```

**Non** si aggiunge un campo `Daily` a `report.Report`: caricherebbe il tipo di
dominio di un campo che esiste solo per una vista. (L'output JSON del CLI non
c'entra: `export.JSON` serializza una sua `jsonReport`, non `report.Report`,
quindi un campo in più lì non lo toccherebbe comunque.)

La sparkline compare sotto la riga di summary, **solo** se il range copre ≥ 2
giorni e il report ha almeno un bucket, seguita dall'etichetta attenuata
` hours/day`. Le celle disponibili sono `maxCells = max(1, width - 12)` — dodici
è la lunghezza dell'etichetta più un margine — e **31 quando `width <= 0`**, che
è il mese più lungo.

La demo la esercita senza modifiche a `demo.go`: le entry fixture cadono nei
giorni **2, 3, 5, 6, 7, 9 e 10** del range, quindi i buchi si vedono. Il giorno
2 è la sessione di sprint planning non fatturabile, e comparirà come barra
proprio perché la serie conta tutte le ore.

### 6.4 #80 — il gauge

`renderBudgetBar` cambia firma e glifi:

```go
func renderBudgetBar(th theme, percentUsed float64) string
```

- Glifi `█` (pieno) e `░` (vuoto) al posto di `#` e `-`.
- Riempimento clampato a `[0, 100]`, **percentuale reale e non clampata** —
  comportamento invariato rispetto a oggi, ed è il motivo per cui
  `bubbles/progress` non va bene: clampa il numero a 100 e nasconde lo
  sforamento.
- Pieno colorato `th.OK` fino al 100%, `th.Err` oltre. Vuoto `th.Help`.

Il resto della schermata Budget non si tocca.

## 7. Come si verifica

Il criterio guida: **un golden verde non dimostra niente su ciò che `termenv.Ascii`
cancella** (§3.4). Ogni scelta invisibile ai golden ha un test diretto.

### 7.1 #117

- I 31 `TestRates*` esistenti, **invariati**: sono la rete di sicurezza.
- Un test nuovo, `TestRatesSelectionIsPerSection`: scendi di una riga nelle
  Liste, `tab` ai Membri, verifica che i Membri partano da 0, torna alle Liste e
  verifica che la riga selezionata sia ancora quella. **Passa prima e dopo il
  refactor**: caratterizza la proprietà per cui i quattro indici esistevano.
- I golden restano invariati (valore zero identico).

### 7.2 Puri

- `report.DailyHours`, table-driven: riempimento a zero, entry fuori range, un
  giorno con più entry, attraversamento di un cambio d'ora legale, `end <= start`,
  slice vuota.
- `sparkline`, table-driven: vuota, tutti zero, un solo valore, valori uguali,
  ricampionamento quando `n > maxCells`, mappatura dei livelli agli estremi. Su
  quest'ultimo punto la formula va letta prima di scrivere il caso: con
  `clamp(ceil(v/max*8), 1, 8)` il minimo non-zero dà `▁` **solo se è ≤ max/8** —
  per `{7, 8}` dà `▇`. Il caso di test va costruito rispettando il vincolo.

### 7.3 Larghezza

`TestReportTableWidth`, table-driven su 0/40/60/80/120: nessuna riga supera la
larghezza richiesta (quando questa sta sopra il pavimento), l'etichetta è la
prima a cedere, le colonne numeriche non si restringono mai.

### 7.4 Ciò che i golden non vedono

- `TestReportTableStyleFunc`: asserisce sullo `Style` **restituito** dalla
  `StyleFunc`, non sull'output renderizzato — parità delle righe zebra, righe
  totali con lo stile dei totali e non zebrate, header con lo stile dell'header.
- `TestBudgetBarOverBudget`: verifica che oltre il 100% lo stile del pieno sia
  `th.Err` e non `th.OK`. Il *numero* (130%) lo vede già il golden; il colore no.

### 7.4-bis Test esistenti che il cambio di firma rompe

Due test di `budget_test.go` vanno riscritti, non solo ricompilati:
`TestRenderBudgetBarClampsFillNotPercent` (riga 38) chiama
`renderBudgetBar(150)` con un argomento solo e asserisce
`strings.Repeat("#", budgetBarWidth)`, e `TestBudgetViewRendersProgressBar`
(riga 24) asserisce che la barra sia fatta di `#` e `-`. Entrambi vanno portati
alla nuova firma e ai nuovi glifi.

I due golden della palette (`palette_dark`, `palette_light`) cambiano perché
`paletteSample` in `theme_test.go` va esteso con i token nuovi — altrimenti
`Border` e `Zebra` non comparirebbero nell'unico posto della suite che cattura
le sequenze di colore reali. Va aggiornato anche il commento in cima a
`theme.go`, che oggi dice che un tema utente sovrascriverà «questi cinque
valori».

### 7.5 Golden

- `report` rigenerato, più `report_narrow` (60) e `report_wide` (120).
- Un golden multi-valuta, per fissare la riga `TOTAL` con la cella `Amount`
  vuota e i sottototali.
- `budget` rigenerato con i glifi nuovi.

## 8. Ordine di esecuzione

1. **#117** — il refactor degli indici. Primo, come chiede la sua issue.
2. **Token del tema** — `Subtle` nella palette, `Border` e `Zebra` negli stili.
3. **La tabella** — `report_table.go`, cablata in `report.go` con la larghezza.
4. **`report.DailyHours`** — puro, con i suoi test.
5. **La sparkline** — `sparkline.go`, cablata nella view del report.
6. **Il gauge** — `renderBudgetBar` con tema e glifi a blocchi.
7. **Chiusura** — CHANGELOG, rigenerazione di `docs/demo.gif` (il report cambia
   aspetto, quindi la GIF del README nasce stale), issue sulla larghezza della
   schermata Budget, commenti di chiusura su #117/#66/#80.

## 9. Vincoli globali

- `internal/report` e `internal/duration` restano **puri**: nessun I/O, nessuna
  dipendenza da `config` o `clickup`. `DailyHours` rispetta il vincolo.
- Tutto ciò che vive nel repo è in **inglese** — codice, commenti, messaggi di
  test, stringhe UI, messaggi di commit. Questa spec è in italiano per
  l'eccezione storica di `docs/superpowers/`.
- **Conventional Commits**, e **mai** un trailer `Co-Authored-By`.
- Prima di ogni commit: `gofmt -l .`, `go vet ./...`,
  `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`,
  `go test ./... -race`, tutti puliti.
- **Parità in demo mode** per ogni funzionalità: `CLICKUP_DEMO=1 clup` deve
  mostrare tabella, sparkline e gauge senza toccare la rete.
- Nessuna chiamata reale all'API ClickUp durante lo sviluppo.
- Nessuna nuova dipendenza in `go.mod`: `lipgloss/table` è un sottopacchetto di
  una dipendenza già presente, e `bubbles/progress` non viene adottato.
