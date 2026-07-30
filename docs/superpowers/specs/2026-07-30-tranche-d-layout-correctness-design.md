# Tranche D — layout & correttezza (design)

> v1.9, tranche D. Chiude **6 issue**: #137, #136, #135, #141, #138 (punto 1) e
> le caselle aperte della #28. Le tranche A/B/C hanno già portato theme registry
> con renderer iniettato, footer/help, keymap + nav stack, overlay compositor,
> command palette, tabella report `lipgloss/table`, gauge e sparkline.

## 1. Obiettivo

Le schermate misurano i layout in **colonne di display**; tre helper diversi
tagliano e riempiono in **rune**. Le due cose coincidono solo in ASCII, e i nomi
di lista e task di ClickUp sono esattamente il posto dove vivono emoji e CJK.
Questa tranche unifica il taglio su una primitiva sola, chiude tre difetti di
layout misurati e smarca il debito residuo della #28.

Non è una tranche di feature: nessuna schermata nuova, nessun tasto nuovo,
nessuna chiamata API nuova. Il valore è che invarianti già dichiarate nei
commenti diventino vere.

## 2. Evidenza misurata

Ogni numero qui sotto è stato prodotto eseguendo codice, e **ogni numero porta
la fixture che lo genera** — un numero senza fixture non è rigenerabile, quindi
non è una misura. Tre affermazioni delle issue sono risultate sbagliate, una
casella è risultata già chiusa, e una correzione che avevo scritto in bozza era
a sua volta sbagliata.

### 2.1 Larghezze e rune

| Affermazione | Verdetto |
|---|---|
| `truncate` (`report.go:270`) taglia per rune | **Vero.** |
| `fmt` riempie `%-Ns` in rune, non in colonne | **Vero.** `"日本語のリスト"` (7 rune, 14 colonne) con `%-10s` riceve 3 spazi e esce a 17 colonne. Sbagliano **entrambe** le metà della coppia `truncate(s,N)` + `%-Ns`: sistemare solo il taglio lascia la colonna disallineata. |

### 2.2 `ansi.Truncate` (x/ansi v0.11.6)

Tutto misurato, niente dedotto:

- larghezza in colonne di display; `n=0` → `""`; `n=1` → `"…"`; non panica.
- mai più larga di `n`.
- su un cluster largo a cavallo del taglio **scarta l'intero cluster** e può
  tornare **più stretta** di `n`: `Truncate("日本語のリスト", 2, "…")` = `"…"`
  (1 colonna, non 2). È per questo che serve un pad misurato dopo il taglio.
- **È ANSI-aware**: `Truncate("\x1b[1mabcdef\x1b[0m", 3, "…")` =
  `"\x1b[1mab…\x1b[0m"`, larghezza 3, sequenze preservate.
- **Ma non chiude uno stile aperto**: `Truncate("\x1b[31mabcdef", 3, "…")` =
  `"\x1b[31mab…"` — nessun reset in coda, lo stile cola oltre il taglio. Conta
  per §4.3: l'input di `clampWidth` viene da `help.View`, che chiude i propri
  stili, ma l'invariante va inchiodata da un test, non assunta.
- `Truncate("ab\ncd", 3, "…")` = `"ab\n…"`: il newline passa. Gli helper di
  §3 dichiarano input a riga singola.

### 2.3 I tre difetti di layout

| # | Cosa dice la issue | Cosa dice la misura |
|---|---|---|
| #136.1 | la riga è larga 94 colonne | **Vero per il golden**: `testdata/budget.golden` con `625.00 / 1000.00 EUR (remaining 375.00)` misura 94 colonne (90 di contenuto + 4 di `th.Box`). Non è un numero fisso: con `337.50 / 450.00` fa 93, con `12.50 / 20.00` fa 89. **Cresce con gli importi, nessun clamp.** Il minimo teorico è ~86: sfora 80 sempre. |
| #136.2 | fra 99,5% e 100% label e barra litigano | **Vero al centesimo**: 99,4 → `99%` + 19 blocchi (accordo); 99,5 / 99,6 / 99,9 → `100%` + 19 blocchi (disaccordo); 100 → `100%` + 20. E 97,6 → `98%` + 19 blocchi: è la misura che decide §4.5. |
| #137 | `Total` duplica `TOTAL` | **Vero.** Fixture: `demoEntries` su luglio 2026, `Rates{Default:50}`, `Currencies{mobile:USD}`, `DefaultCurrency:EUR`, `GroupByTotal` → riga bucket `["Total" "18.00" "17.00" "337.50 EUR + 512.50 USD"]` e riga totali `["TOTAL" "18.00" "17.00" ""]`. (La issue cita `308.75 + 90.00`: altra fixture, stesso difetto.) |
| #138.1 | 84 colonne in un terminale da 80 | **Vero, ma serve la congiunzione**: label lunga **e** Hours/Billed più larghi di 8. Fixture `{label 46 col, 1234567.50, 1234567.50, "625.00 EUR"}` → 64 col a width 60, 84 a width 80. Con la stessa label e `12.50` non sfora mai (55 col a width 60). Overflow esatto = 2 × (larghezza numerica − 8). |
| #138.1 | «5 colonne inutilizzate a width 60» | **La issue aveva ragione per il suo caso, la mia bozza aveva torto.** Con label lunghe e cifre normali (il caso dei golden `report`/`report_narrow`) l'inutilizzato è 5. Con label corte è 21+, perché Item è tappato dalla label più lunga e la tabella non si allarga mai. Dipende dalla fixture, e va detto così. |
| #138.1 | (corollario) | «con label corte non sfora mai» è **falso**, ma il numero va misurato **rendendo**, non calcolando: label corta + Hours/Billed a 10 caratteri rende **44** colonne a width 40 (a mano ne avevo dedotte 47, dimenticando che `reportAmountWidth` riprende spazio ad Amount come ultima risorsa; 47 è la larghezza *naturale*, che si ottiene a ogni width ≥ 48). |
| #138.1 | (limite del fix) | Dopo il fix la tabella **non può** stare in 40 colonne con cifre a 10: il minimo è `chrome(10) + floor Item + 10 + 10 + header Amount(6)` = **48** con label lunghe e **43** con label corte. Misurato rendendo. Un test che pretende ≤ 40 fallisce contro il codice corretto. |

### 2.4 Il resto della #28

- **Demo, preset non mensili: casella già chiusa.** `demoEntries` usa giorni
  offset modulo lo span dalla #4. Misurato: `this_month` 8 entry / 7 giorni
  distinti, `last_7d` 8/5, `last_30d` 8/7, `this_week` 8/5. Nessuna è vuota.
- **`task_tags` con una stringa nuda non degrada, esplode.** Verificato con
  `httptest`: `json: cannot unmarshal string into Go struct field
  rawEntry.data.task_tags`. Fallisce l'intera `TimeEntries`, non solo i tag.
- **`lipgloss.NewStyle()` in produzione sotto `internal/`: una sola
  occorrenza**, `footer.go:62`. L'altro hit del grep è un commento in
  `report_table.go:163`. La seconda casella della #141 è quindi già
  soddisfatta: va registrata, non lavorata.
- **`countTrue` è scandito due volte** (`Filter` chiama `Empty()` e poi
  ricalcola, `filter.go:40-43`) e i tre blocchi duplicati di rebuild esistono
  (`report.go:83`, `app.go:719`, `rates.go:855`).

### 2.5 Fatti che decidono l'implementazione

- **L'header determina la larghezza di colonna** in `lipgloss/table`: a parità
  di celle `"12.50"`, header `"H"` → 7 colonne, header `"Billed"` → 8. Quindi
  l'aritmetica di budget di §4.6 **deve** misurare Hours e Billed includendo
  gli header, come `reportItemWidth` già fa per Item e Amount
  (`report_table.go:102-103`). Misurando solo i dati il budget assume 5+5=10
  mentre il renderer usa 5+6=11, e la tabella sfora di 1.
- **`th.Box` costa esattamente 4 colonne** (RoundedBorder + `Padding(0,1)`,
  `theme.go:54`). Misurato: `Box.Render("12345")` → 9 colonne.
- **Su Home il cambio di mese, di settimana e di preset non ricarica niente.**
  `home.go:33-53` muta `month`/`periodMode`, `range.go:139-141` scrive
  `m.preset` e fa `pop`; il reload parte solo da `k.Generate`
  (`home.go:75-79`). Decide §4.7.

## 3. Architettura

Una primitiva sotto tutto: `ansi.Truncate`, già dipendenza diretta dalla #59.
File nuovo `internal/tui/width.go`:

```go
// truncateWidth cuts s to at most cols DISPLAY columns, ellipsis included.
// cols <= 0 returns "". s must be a single line: the behavior with "\n" is
// undefined (ansi.Truncate lets the newline through).
func truncateWidth(s string, cols int) string

// cell renders s into exactly cols display columns: truncated when too wide,
// space-padded when too narrow. It replaces the truncate(s,N) + "%-Ns" pair,
// where BOTH halves measured runes instead of columns. cols <= 0 returns "".
// Same single-line contract as truncateWidth.
func cell(s string, cols int) string
```

Il pad non è cosmetico: siccome `ansi.Truncate` può tornare più stretta di
`cols` su un cluster largo (§2.2), senza pad misurato la colonna si disallinea
comunque, solo nell'altra direzione.

Conseguenze: la `truncate` rune-based **si elimina**, e con lei la
`shaveToWidth` di `report_table.go`, che è una shave-loop che fa a mano ciò che
`ansi.Truncate` fa in un passo.

`shaveToWidth` ha **5 chiamanti**, non 2: `report_table.go:209,210` e — questo
è il punto che si perde facilmente — `palette.go:214,219,253`. Eliminarla senza
migrare anche il palette non compila. Anche `report.go:269`,
`report_table.go:142` e `report_table.go:195` la citano nei commenti e vanno
aggiornati, altrimenti restano a nominare funzioni che non esistono più.

**Divergenza voluta da `truncate`, a `cols == 1`:** `truncate("x", 1)` = `"…"`,
perché la guardia `n <= 1` scatta prima del controllo "ci sta"
(`report.go:274`). `cell("x", 1)` = `"x"`. Il comportamento nuovo è quello
giusto e nessun call site usa `cols < 20`, ma la frase «per ASCII l'output è
identico» vale **per `cols >= 2`**.

## 4. Gli interventi

### 4.1 `width.go` e i due helper (#135, prima metà)

Nuovo file, due funzioni, test con fixture a rune larghe **obbligatorie** e i
casi di bordo di §2.2 (`cols<=0`, `cols==1`, cluster a cavallo, input con
escape terminati e non terminati).

### 4.2 Migrazione dei call site (#135, seconda metà)

**15 siti da migrare**, su 17 righe elencate — le altre 2
(`report_table.go:254,256`) stanno dentro `shaveToWidth`, che sparisce con la
funzione. (La bozza diceva 19: era un grep che contava anche le due menzioni di
`truncate(...)` nel commento a `report_table.go:244-245`.)

- `budget.go:75`
- `rates_view.go:53, 57, 76, 82, 92, 93, 143, 147, 149`
- `entries.go:583, 593, 643, 648`
- `log.go:540`

Regola: coppia `truncate(s,N)` + verbo `%-Ns` → `cell(s, N)` con `%s`; sito
senza pad → `truncateWidth(s, N)`.

**Proprietà attesa:** per contenuto ASCII e `cols >= 2` l'output è identico, e i
golden esistenti restano verdi. Va **verificata rigenerando**, non dichiarata:
la spec non ha implementato la migrazione.

### 4.3 `clampWidth` senza renderer di default (#141)

```go
return ansi.Truncate(s, width-1, "") + th.Help.Render("…")
```

Più un test che inchioda che l'input non porta stili aperti oltre il taglio
(§2.2: `ansi.Truncate` non chiude una SGR non terminata).

Il commento deve dire che i test **non possono** accorgersi della differenza,
perché `TestMain` fissa `termenv.Ascii` e uno stile costruito sul renderer di
default rende identico a uno costruito su quello iniettato — così nessuno lo
reintroduce credendo che la suite lo prenda.

La seconda casella della #141 («grep di ogni `lipgloss.NewStyle()`») è **già
soddisfatta**: §2.4 la registra, non c'è lavoro.

### 4.4 La riga `Total` duplicata (#137)

In `reportRows`: se `r.GroupBy == report.GroupByTotal && len(r.Buckets) == 1`,
non emettere la riga bucket. `firstTotal` diventa 0, e con `firstTotal == 0` lo
`StyleFunc` fa già la cosa giusta (riga 0 = `TOTAL`, in grassetto). Il caso
`len(Buckets) == 0` non arriva a `reportRows`: `report.go:208` intercetta con
«No hours to show.».

Per `GroupByTotal` il bucket unico raccoglie tutte le entry, quindi i suoi
Amounts sono **esattamente** i subtotali: sopprimerlo non perde nulla, nemmeno
il caveat sul drift di arrotondamento che vale alle granularità fini.

**Nessun golden esistente si muove**: tutti i golden del report girano su
`goldenReport()`, che è `GroupByList`, e in `testdata/` non esiste una riga
bucket `Total`. Vanno quindi **creati** due golden nuovi sotto `GroupByTotal`,
uno a valuta singola e uno multi-valuta. Un piano che si aspetta un golden
rosso qui sta descrivendo qualcosa che non c'è.

### 4.5 Il budget in 80 colonne (#136)

`budgetModel.view` riceve la larghezza come già fa `reportModel.view`.

```
usable  = width - 4                 // th.Box: misurato, non assunto
fixed   = 1 + budgetBarWidth + 1 + pctW + 2
nameW   = usable - fixed - figuresW
```

`pctW` e `figuresW` sono i massimi **misurati sulle righe vere** (le cifre non
hanno larghezza fissa: è per questo che una riserva costante si rompe). Nome
reso con `cell(name, nameW)`.

**Ordine di valutazione, una passata sola** — la circolarità va rotta, non
lasciata all'implementatore:

1. calcola `figuresW` **con** `(remaining …)` e da lì `nameW`;
2. se `nameW < budgetMinNameWidth` (**costante = 12**, la stessa soglia sotto
   cui `reportMinItemWidth` dice che «le label smettono di essere label»),
   ricalcola `figuresW` **senza** `(remaining …)` e da lì `nameW`, e non lo
   reintroduce;
3. se `nameW` resta sotto la soglia anche così, `nameW = budgetMinNameWidth` e
   **la riga sfora**. Accettato ed esplicito, esattamente come la tabella
   report accetta di sforare sotto la sua soglia. Un `nameW` negativo non deve
   mai raggiungere `cell` (che comunque ritorna `""`).

La soglia sotto cui si sfora è
`4 + 1 + budgetBarWidth + 1 + pctW + 2 + figuresW(senza remaining) + 12`, cioè
**63** con le cifre del golden (`625.00 / 1000.00 EUR`, pct 3) e **65** con
importi a quattro cifre e una percentuale sopra 100 (pct 4). Misurato
simulando la funzione. Non è «~48»: quel numero vale per la tabella report
(`reportAmountWidth`) e applicarlo qui farebbe credere che 60 colonne stiano
sopra la soglia di degrado, quando invece è la prima larghezza che sfora.

`width <= 0` (primo render, prima del `WindowSizeMsg`) → layout naturale
attuale, `nameW = 24`, stesso fallback che `reportItemWidth` ha già
(`report_table.go:108-110`).

A width 80 col golden attuale: con `(remaining)` `nameW` sarebbe 10 → sotto 12 →
si ricalcola senza, `nameW = 29`.

**La barra:** si arrotonda **per difetto la label**, non per eccesso il
riempimento. Arrotondare il riempimento porterebbe la barra piena a 97,6% sotto
una label `98%` (§2.3), e una barra piena significa «budget esaurito». Meglio
`99%` con 19 blocchi che `100%` con 20 a budget non finito. La label resta non
clampata sopra 100: è il motivo per cui questa schermata non usa
`bubbles/progress`.

### 4.6 Le colonne numeriche misurate (#138, punto 1)

`reportItemWidth` e `reportAmountWidth` misurano Hours e Billed dal contenuto
**inclusi gli header** (§2.5) invece di sottrarre `2*reportNumWidth`; la
costante sparisce.

Il guadagno per Item dove è vincolato dal budget è **+5** (16 riservate → 11
misurate: `"Hours"`=5, `"Billed"`=6), non +6. Misurando solo i dati il budget
assumerebbe 10 e il renderer userebbe 11: la tabella sforerebbe di 1, cioè
proprio la classe di bug che questa tranche chiude.

Il test vuole la **congiunzione**: label lunga **e** Hours/Billed più larghi di
8. Con label lunga e cifre normali la tabella non sfora (55 col a width 60), e
con label corta nemmeno a width 60 — un test che manca uno dei due fattori
passa contro il bug.

E deve fermarsi alle larghezze raggiungibili: il minimo post-fix è 48 con label
lunga e 43 con label corta, quindi asserire `≤ 40` fa fallire il test **contro
il codice corretto**. Sweep misurato, larghezza resa dal codice attuale:

| width | label lunga + cifre larghe | label corta + cifre larghe |
|---|---|---|
| 40 | 48 ✗ | 44 ✗ |
| 44 | 48 ✗ | 47 ✗ |
| 48 | 52 ✗ | 47 ✓ |
| 60 | 64 ✗ | 47 ✓ |
| 80 | 84 ✗ | 47 ✓ |
| 100 | 86 ✗ | 47 ✓ |

Quindi: per la label lunga usa **60, 80, 100** (oggi 64, 84, 86 — rosso su
tutte; post-fix il minimo 48 sta in tutte). Per la label corta usa **44**, la
larghezza in cui oggi rende 47 e sfora di 3 mentre il minimo post-fix è 43:
è l'unica finestra in cui quella direzione dell'overflow è sia rossa prima sia
verde dopo.

Effetto sui golden: cambiano solo dove Item è vincolato dal budget — `report` e
`report_narrow`; `report_wide`, multi-valuta e `partial_month` restano intatti.
Da verificare rigenerando.

### 4.7 Range pinnato e scroll dei Filters (#28)

**Il pin ha bisogno di un trasporto, non di un ricalcolo.** Ripinnare
richiamando `currentRange()` nell'handler del messaggio ricalcola il range
*dopo* il load: un load alle 23:59:59 su R1 il cui messaggio arriva alle
00:00:01 pinnerebbe R2 ≠ R1, e il pin nascerebbe disallineato dalle entry che
descrive — cioè proprio il drift che esiste per eliminare. Quindi:

- `entriesMsg` e `entriesReloadedMsg` **portano** `start`/`end`, calcolati dal
  cmd che ha fatto il load;
- l'handler pinna **quelli** su `m.loadedStart` / `m.loadedEnd`;
- `m.activeRange()` torna la coppia pinnata se presente, altrimenti ricalcola
  con `currentRange()`. Il fallback è raggiungibile davvero (schermata rates
  aperta da Home prima di ogni load), non è codice morto.

**Classificazione dei 10 call site** (sono 10, non 11: l'undicesima riga del
grep è la definizione a `app.go:266`):

| Sito | Cosa fa | Verdetto |
|---|---|---|
| `home.go:99` `rangeLabel` | descrive il range che il **prossimo** `enter` caricherà | **fresco.** Pinnarlo congela la label: su Home mese, settimana e preset cambiano senza reload (§2.5), quindi la navigazione mese sembrerebbe rotta. |
| `app.go:287` `reloadEntriesCmd` | **è** il load | fresco, e passa la coppia al messaggio |
| `entries.go:564` `reloadForBrowser` | **è** un load | fresco, e passa la coppia a `entriesReloadedMsg`, così un reload del browser aggiorna il pin invece di lasciarlo stantio |
| `app.go:714` handler `entriesMsg` | costruisce il report all'arrivo | usa la coppia **del messaggio** e la pinna |
| `entries.go:436` `updateEntryCmd` | decide se l'entry modificata è uscita dal range **visibile** | **pinnato.** «moved outside the current range» deve riferirsi al range delle entry a schermo, altrimenti a mezzanotte il messaggio mente |
| `report.go:67` `dailySeries` | serie giornaliera sulle entry visibili | pinnato |
| `report.go:81` cambio GroupBy | rebuild sulle entry già caricate | pinnato |
| `report.go:127` schermata budget | rebuild | pinnato |
| `report.go:184` `applyReport` | rebuild | pinnato |
| `rates.go:854` cambio tariffe | rebuild | pinnato |

**Scroll dei Filters.** `scrollPalette` diventa `scrollWindow(idx, top, rows)
int` condiviso; il commento a `palette.go:139` chiedeva già esattamente questo.

Il cursore dei Filters è `(sec, row)` su quattro sezioni con header e righe
`(none)` interposte, quindi `idx` non esiste finché non si definisce la
proiezione. **La finestra è sulle righe visuali dell'intera vista** (header
inclusi), non sulle opzioni della sola sezione attiva: l'overflow di cui si
lamenta la #28 è quello della vista intera, e una finestra per-sezione
lascerebbe fuori schermo gli header delle sezioni successive. L'indice del
cursore è la riga visuale su cui sta `(sec, row)`.

### 4.8 Il resto della #28

- **`task_tags`**: `UnmarshalJSON` sull'elemento tag che accetta sia la stringa
  nuda sia l'oggetto `{name}`. Motivo: lo schema auto-generato della pagina
  range di ClickUp tipizza `task_tags` come `["string"]` mentre l'esempio
  concreto dei docs è un array di oggetti `{name, tag_fg, tag_bg, creator}`, e
  oggi una stringa nuda fa esplodere l'intera `TimeEntries` (§2.4). Col decoder
  tollerante l'assunzione muore invece di essere annotata. La verifica contro un
  workspace vero resta alla **#129**, che esiste già per lo stesso tipo di check
  sul PUT dei tag di entry.
- **Micro-nit**: `applyReport` fa DRY dei tre blocchi di rebuild; `Empty()` +
  `Filter()` non scandiscono `countTrue` due volte.
- **Demo**: casella chiusa come già risolta, con un test che inchioda la densità
  per `last_7d` / `this_week` / `last_30d` così non regredisca.

## 5. Fuori scope (dichiarato)

- Scroll del **browser entries**: non è in nessuna delle 6 issue. Dopo
  l'estrazione di `scrollWindow` costa poco, ma è un'altra issue.
- **#138 punto 2** (`th.Box` col colore bordo): muove i golden di ogni
  schermata che usa `th.Box`; va nella tranche dei temi, come dice la issue
  stessa.
- **Rotella del mouse** sui Filters: è #74.

## 6. Test

- Ogni difetto ha un test **visto fallire** contro il difetto, col transcript
  allegato. Non è una raccomandazione: su questo progetto quattro test in una
  sola tranche sono stati colti a passare contro il bug che dovevano prendere.
- **Unica eccezione, dichiarata: la #141.** È behaviorally inert — `MaxWidth` e
  `ansi.Truncate` tagliano in modo indipendente dal profilo colore, e
  `TestMain` fissa `termenv.Ascii`, quindi nessun test comportamentale può
  fallire contro quel difetto. La prova è il grep di §2.4 più il test
  anti-bleed di §4.3, non un red-green.
- Ogni test di larghezza nasce con una fixture a **rune larghe** o con la
  **congiunzione** che il difetto richiede (§4.6). Il test di larghezza della
  tabella report è passato contro il bug per una tranche intera perché tutte le
  label erano ASCII.
- Golden rigenerati con `go test ./internal/tui -update`, mai a mano.
- Gate pre-commit: `gofmt -l .`, `go vet ./...`,
  `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`,
  `go test ./... -race`.

## 7. Vincoli globali

- `internal/report` e `internal/duration` restano **puri**: nessun I/O, nessun
  import di `config`/`clickup`/`tui`.
- Nessuna funzione di stile in produzione chiama `lipgloss.NewStyle()`: gli
  stili vengono dal renderer iniettato del `theme`.
- Mai chiamare l'API ClickUp reale; solo `httptest` con `client.BaseURL`.
- Tutto ciò che vive nel repo è in **inglese** — codice, identificatori,
  commenti, stringhe UI, nomi e messaggi dei test, messaggi di commit.
  Eccezione: i doc di design in `docs/superpowers/`.
- Conventional Commits. **Mai** `Co-Authored-By`.
