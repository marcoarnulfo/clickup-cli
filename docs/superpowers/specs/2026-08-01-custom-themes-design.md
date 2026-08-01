# Temi personalizzati e registro dei temi (design)

> v1.9. Chiude la **#82** limitatamente alle caselle 1 e 3 (tema da YAML, due
> temi built-in) e il **punto 2 della #138** (il colore del bordo di `th.Box`).
> La casella 2 della #82 — override delle keybinding — è una tranche a sé,
> perché tocca 51 binding e non condivide niente con i colori se non il file di
> configurazione.

## 1. Obiettivo

Far scegliere all'utente i colori della TUI: due tavolozze pronte, e la
possibilità di scriversene una nel proprio `config.yml`. Più il residuo della
#138: le due cornici della schermata report devono pesare uguale.

Nessuna chiamata API nuova, nessuna schermata nuova, nessun tasto nuovo.

## 2. Evidenza misurata

Ogni numero e ogni sequenza di escape qui sotto viene dall'aver **eseguito** il
codice. È la regola che le tranche precedenti hanno dovuto imparare a caro
prezzo, e nella tranche D quattro numeri scritti a mano erano falsi.

### 2.1 lipgloss non protesta mai

È la misura che decide la forma della validazione. Reso su un renderer a profilo
`TrueColor`:

| valore | risultato |
|---|---|
| `#FF00AA` | `\x1b[38;2;255;0;170m` |
| `#fff` | `\x1b[38;2;255;255;255m` (hex corto accettato) |
| `236` | `\x1b[38;5;236m` |
| `bogus` | **`x`** — nessun escape, nessun errore, nessun panico |
| `""` | **`x`** — idem |
| `#GGGGGG` | **`x`** — idem |
| `999` | **`\x1b[38;5;999m`** — accettato ed emesso |

Due conseguenze, entrambe vincolanti:

- **Un refuso in un tema produce una TUI scolorita e nessun messaggio.** La
  validazione dobbiamo scriverla noi: lipgloss non ci dirà mai che un colore era
  sbagliato.
- **Un indice fuori intervallo è peggio di un colore assente**: `999` non viene
  respinto, viene *emesso*, e `\x1b[38;5;999m` è una sequenza che nessun
  terminale interpreta. L'intervallo 0-255 va imposto da noi.

### 2.2 Il raggio d'azione della #138.2 è due golden, non diciassette

La #138 dice che dare un colore al bordo di `th.Box` «sposta i golden di ogni
schermata che lo usa — da qui la sua esclusione dalla #66». Misurato,
confrontando `th.Box.Render("Box")` con e senza `BorderForeground(p.Muted)`:

| profilo | esito |
|---|---|
| **Ascii** | **byte-identico** su entrambi i fondi |
| ANSI | `\x1b[90m` sul bordo |
| TrueColor | `\x1b[38;5;240m` sul bordo |

L'affermazione era vera quando è stata scritta e **non lo è più**: `TestMain`
(`internal/tui/golden_test.go:29`) fissa `termenv.Ascii` per tutto il package, ed
è arrivato nella tranche A della v1.9, cioè *dopo* la #66. Dei 17 golden che
contengono un bordo tondo, 15 girano sotto Ascii e non si muovono; si muovono
solo `palette_dark` e `palette_light`, che `paletteTheme` rende a `TrueColor`
(`theme_test.go:58-63`) e che includono `th.Box` fra i token campionati.

### 2.3 Lo slot a 16 colori si può derivare

La tranche precedente ha dato a `Subtle` un `CompleteAdaptiveColor` con il valore
a 16 colori scritto a mano (`8` su fondo scuro, `7` su chiaro), perché la
conversione automatica lo faceva sparire. Se un tema utente scrivesse un solo
valore per token, quel fix si disferebbe in silenzio su ogni tema
personalizzato.

`termenv.ConvertToRGB` funziona sia sugli indici sia sugli hex, il che rende la
derivazione possibile. La regola scelta **non ha una soglia arbitraria**: fra le
due sole sfumature disponibili a 16 colori — `8` (#808080) e `7` (#C0C0C0) — si
prende quella percettivamente più vicina, con `DistanceLab` di go-colorful.
Misurato:

| colore | d(8) | d(7) | scelta |
|---|---|---|---|
| `236` (#303030) | 0.3372 | 0.5784 | **8** |
| `254` (#E4E4E4) | 0.3700 | 0.1288 | **7** |
| `#44475A` (dracula) | 0.2602 | 0.4867 | **8** |
| `#3B4252` (nord) | 0.2776 | 0.5089 | **8** |
| `#808080` | 0.0000 | 0.2412 | 8 |
| `#A0A0A0` | 0.1228 | 0.1184 | 7 (il punto di scambio è qui intorno) |

Le prime due righe sono la proprietà che conta: **la derivazione riproduce
esattamente i due valori scelti a mano ieri**. Il tema di default costruito così
rende byte per byte come quello di oggi, a ogni profilo, e un test lo inchioda —
se un giorno qualcuno cambia la regola, quel test cade prima dei golden.

### 2.4 dracula e nord su fondo chiaro

Rapporti di contrasto WCAG dei token proposti in §6, calcolati dalla luminanza
relativa lineare:

| tema | token | vs nero | vs bianco |
|---|---|---|---|
| dracula | Primary `#BD93F9` | 8.71:1 | **2.41:1** |
| dracula | Accent `#FF79C6` | 8.80:1 | **2.39:1** |
| dracula | Muted `#6272A4` | **4.46:1** | 4.71:1 |
| dracula | Danger `#FF5555` | 6.68:1 | **3.14:1** |
| dracula | Success `#50FA7B` | 15.30:1 | **1.37:1** |
| nord | Primary `#88C0D0` | 10.50:1 | **2.00:1** |
| nord | Accent `#81A1C1` | 7.80:1 | **2.69:1** |
| nord | Muted `#4C566A` | **2.85:1** | 7.38:1 |
| nord | Danger `#BF616A` | 5.13:1 | **4.09:1** |
| nord | Success `#A3BE8C` | 10.30:1 | **2.04:1** |

E il caso peggiore, che non è un testo ma uno sfondo: `Subtle` è la zebratura,
dipinta **sotto il colore di default del terminale**.

| striscia | testo sopra, su terminale chiaro |
|---|---|
| dracula `#44475A` | **2.29:1** |
| nord `#3B4252` | **2.09:1** |

Righe scure sotto testo scuro, sotto il minimo di 3:1 previsto persino per il
testo grande. Su un terminale chiaro questi due temi non sono soltanto infedeli:
parti della TUI **non si leggono**.

Due numeri vanno detti anche per il fondo scuro, dove i temi sono a casa loro:
`Muted` sta a 4.46:1 (dracula) e 2.85:1 (nord). Sono i colori che quelle
tavolozze destinano ai commenti, cioè volutamente arretrati, e li spediamo così
per fedeltà — ma il README deve dire che il testo d'aiuto in `nord` è tenue, non
lasciarlo scoprire.

**Questa è una conseguenza accettata, non un difetto da correggere.** La scelta è
la fedeltà: un tema chiamato `dracula` deve essere Dracula. La difesa è il
README, che porterà questi numeri e non un generico «potrebbero non stare bene».

## 3. Architettura: un package nuovo, e perché

> **Questa sezione è stata riscritta dopo una review.** La prima versione diceva
> che un package nuovo era *forzato*, perché mettere il tipo in
> `internal/config` avrebbe creato un ciclo. **È falso**, e la review l'ha
> dimostrato con il grafo delle dipendenze: `internal/config` non importa
> **nessun** package interno, e `internal/tui` importa `config`. Un tipo dentro
> `config` non chiude alcun ciclo. Il ciclo esisterebbe solo mettendolo in `tui`,
> perché allora `config` dovrebbe importare `tui` per deserializzare. L'errore
> vale la pena raccontarlo: stava per finire, in inglese, dentro il commento di
> un package.

Il tipo che descrive un tema serve a `internal/config`, che lo deserializza
dallo YAML, e a `internal/tui`, che lo trasforma in stili lipgloss. Sta in un
terzo package **per scelta**, e le ragioni sono tre:

- **`internal/config` resterebbe altrimenti legato a lipgloss e termenv.** Oggi
  dipende da `yaml.v3` e dalla stdlib; farne il posto dove vivono i tipi colore
  significherebbe farci entrare una libreria di rendering per un package il cui
  lavoro è leggere un file.
- **La logica dei colori si testa meglio fuori dalla TUI.**
  `internal/tui/golden_test.go:29` fissa `termenv.Ascii` per **tutto il
  package**: qualunque test sui colori scritto lì dentro deve costruirsi un
  renderer proprio per vedere un solo byte di escape. Un package a sé non ha
  quel vincolo.
- **`internal/tui` è già il package più grande del repo.** La validazione e la
  risoluzione dei temi sono un'unità che si tiene in testa da sola; metterla lì
  la nasconderebbe.

L'alternativa scartata — `Value`/`Spec` in `config` e `Palette`/`Resolve` in
`tui`, con `cli` che chiama `tui.ResolvePalette` — non ha cicli e non aggiunge
package. Costa però i tre punti qui sopra, e in più esporterebbe da `tui` due
identificatori che nessuno dentro `tui` usa.

### 3.1 `internal/themes`

Foglia; dipende da `lipgloss`, `termenv` e `yaml.v3` — quest'ultimo perché
`Value` porta il proprio `UnmarshalYAML` (§4), e il tipo deve saper leggere le
sue due forme da sé. Non fa I/O, non conosce la TUI, non conosce la
configurazione su disco.

```go
package themes

// Palette is the six semantic colors the whole TUI is built from.
type Palette struct {
	Primary lipgloss.AdaptiveColor
	Accent  lipgloss.AdaptiveColor
	Muted   lipgloss.AdaptiveColor
	Danger  lipgloss.AdaptiveColor
	Success lipgloss.AdaptiveColor
	Subtle  lipgloss.CompleteAdaptiveColor
}

// Value is one token as written in YAML: a bare string, or a light/dark pair.
type Value struct{ Light, Dark string }

// Spec is one user-written theme: token name -> value.
type Spec map[string]Value

func Default() Palette
func Names() []string                                    // built-in names, sorted
func Resolve(name string, custom map[string]Spec) (Palette, error)
```

**Il nome del package è `themes` al plurale per una ragione meccanica**, e va
scritto o sembrerà arbitrario fra sei mesi: `internal/tui` dichiara già un tipo
chiamato `theme` (`theme.go:30`), usato in **18 file** come `func …view(th theme)`.
Il nome di un package importato vive nel *file block* e quello del tipo nel
*package block*, e il Go spec vieta che lo stesso identificatore stia in
entrambi — verificato con un package di prova: `import ".../theme"` dentro un
package che dichiara `type theme` non compila, *«theme already declared through
import of package theme»*, anche quando import e tipo stanno in file diversi.

Un import con alias compilerebbe, quindi il plurale non è l'unica via: è quella
che evita di dover scrivere un alias a ogni import. L'altra alternativa,
rinominare il tipo, costerebbe 18 file per una ragione che non è di questa
tranche.

### 3.2 Chi chiama cosa

- `internal/config` guadagna due campi grezzi: `Theme string` e
  `Themes map[string]themes.Spec`. Non valida niente: tiene lo YAML com'è.
- `internal/cli` guadagna `resolveTheme(cfg) (themes.Palette, error)`, estratta
  da `runTUI` per la stessa ragione per cui `programOptions` lo era già
  (`cli.go:17-26`): `runTUI` blocca su un terminale e non è chiamabile da un
  test, mentre la decisione sì. `runTUI` la chiama **prima** di costruire il
  programma e ritorna l'errore — è l'ultimo punto in cui un errore di
  configurazione può ancora finire su stderr invece che dentro una TUI già
  avviata, ed è dove `Execute` lo stampa già come `error: …`.
- `internal/tui` cambia firma: `New(cfg config.Config, pal themes.Palette) Model`.

**I call site di `New` sono 44, su 8 file** — misurati, non stimati: `app_test.go`
34, `demo_test.go` 3, `log_test.go` 2, `report_test.go` 2, `golden_test.go` 1,
`home_test.go` 1, `palette_demo_test.go` 1, `internal/cli/cli.go:57` 1, più la
definizione in `app.go`.

> **Una nota di metodo, perché il numero sbagliato è stato scritto qui prima.**
> La prima versione di questa sezione diceva «nove call site», perché avevo
> cercato la stringa letterale `New(cfg)`. Quella cerca non trova
> `New(config.Config{…})` (34 volte nel solo `app_test.go`), né
> `New(demoConfig())`, né `New(realCfg)`. Un piano costruito su quel numero
> avrebbe lasciato `internal/tui` non compilabile a metà esecuzione, con quattro
> file da toccare fuori dallo scope dichiarato del task.

L'alternativa — lasciare `New(cfg)` e farle risolvere il tema da sola — vorrebbe
un ramo «non può succedere» per l'errore, dato che `cli` avrebbe già rifiutato di
partire. Un ramo irraggiungibile che nessun test può esercitare è peggio di una
firma con un argomento in più.

**Demo mode**: `cli` risolve dal config vero e passa la tavolozza; `New` sostituisce
solo `cfg` con `demoConfig()`. Quindi `CLICKUP_DEMO=1` **onora il tema
dell'utente**, che è giusto — il tema è una preferenza di interfaccia, non un
dato. In CI, dove un config non esiste, resta il default e il GIF non cambia.

## 4. Il formato YAML

```yaml
theme: dracula

themes:
  mine:
    accent: {light: "127", dark: "205"}   # coppia: adattivo
    muted: "240"                          # valore singolo: uguale su entrambi i fondi
```

- **`theme`** è il nome da usare. Assente o vuoto significa `default`.
- **`themes`** sono i temi dell'utente, per nome.
- **Un valore può essere una stringa o una coppia.** Una stringa sola vale per
  entrambi i fondi. `Value` ha un `UnmarshalYAML` tollerante che accetta le due
  forme, come `tagName` in `internal/clickup` fa già per il JSON di ClickUp.
- **`Value` ha anche un `MarshalYAML`, e non è un vezzo.** `config.Save`
  serializza l'intera `Config`, quindi senza di esso un `muted: "240"` scritto
  dall'utente tornerebbe su disco come `muted: {light: "240", dark: "240"}` alla
  prima cosa che salva il config. `MarshalYAML` ricollassa a stringa quando i due
  lati coincidono, così un salvataggio non riscrive il file dell'utente in una
  forma che non ha scelto. **E per una coppia vera deve ritornare una struct con
  i tag, non una mappa**: misurato, una `map[string]string` viene serializzata
  con le chiavi in ordine alfabetico, cioè `dark:` prima di `light:` — un
  riordino silenzioso di quello che l'utente aveva scritto.
- **Un tema dichiara solo ciò che cambia.** I token non nominati arrivano da
  `Default()`. Il caso comune — «voglio solo un accento diverso» — sono due
  righe, non sei.
- **I nomi dei token** sono `primary`, `accent`, `muted`, `danger`, `success`,
  `subtle`: gli stessi sei campi di `Palette`, in minuscolo.
- **Un colore** è un hex (`#RGB` o `#RRGGBB`) oppure un indice decimale 0-255.

## 5. Validazione

Tutti errori **bloccanti**, con il precedente già stabilito da
`billing.rounding.increment` (`internal/service/pricing.go:71`): una
configurazione che non si riesce a interpretare ferma l'avvio invece di
degradare in silenzio. Qui la ragione è più forte del solito, perché §2.1 dice
che il degrado sarebbe *invisibile*: testo senza colore, o una sequenza che il
terminale ignora.

| caso | messaggio |
|---|---|
| nome di tema sconosciuto | elenca i nomi disponibili, built-in e utente, ordinati |
| un tema utente chiamato come un built-in | «`dracula` is a built-in theme; choose another name» |
| token sconosciuto dentro un tema | elenca i sei token validi |
| colore non hex e non intero | nomina tema, token e valore |
| indice fuori da 0-255 | idem, con l'intervallo |
| valore vuoto (o metà coppia vuota) | idem |

Il messaggio nomina sempre **tema, token e valore**: un errore che dice solo
«invalid color» costringe a cercarlo a mano in un config con sei temi.

**Si validano tutti i temi dell'utente, non solo quello selezionato.** La prima
versione della spec non lo diceva, e il piano che ne era uscito controllava le
collisioni di nome su tutti e i colori solo su quello scelto — un refuso in un
tema non selezionato sarebbe rimasto invisibile fino al giorno in cui l'utente
lo avesse selezionato, che è il momento peggiore per scoprirlo. La regola è
uniforme: un `config.yml` che contiene un tema invalido è un config invalido.

**Un hex non quotato è un commento YAML**, e va detto dentro il messaggio
d'errore. Misurato: `muted: #fff` non è un colore, è la chiave `muted` con
valore vuoto seguita da un commento, quindi arriva alla validazione come
stringa vuota. Un errore che dicesse soltanto «empty color» manderebbe l'utente
a cercare una chiave che ha scritto benissimo. Il messaggio per il valore vuoto
deve suggerire le virgolette. (Gli indici numerici non quotati invece
funzionano: `muted: 240` si decodifica correttamente.)

## 6. I due temi built-in

Valori dalle tavolozze ufficiali, mappati sui sei token:

**dracula** (draculatheme.com)

| token | sorgente Dracula | hex |
|---|---|---|
| Primary | Purple | `#BD93F9` |
| Accent | Pink | `#FF79C6` |
| Muted | Comment | `#6272A4` |
| Danger | Red | `#FF5555` |
| Success | Green | `#50FA7B` |
| Subtle | Current Line | `#44475A` |

**nord** (nordtheme.com)

| token | sorgente Nord | hex |
|---|---|---|
| Primary | nord8 (Frost) | `#88C0D0` |
| Accent | nord9 (Frost) | `#81A1C1` |
| Muted | nord3 (Polar Night) | `#4C566A` |
| Danger | nord11 (Aurora) | `#BF616A` |
| Success | nord14 (Aurora) | `#A3BE8C` |
| Subtle | nord1 (Polar Night) | `#3B4252` |

Entrambi dichiarano **un valore solo per token**, quindi rendono identici su
fondo chiaro e scuro: è la fedeltà scelta in §2.4. `Primary` e `Accent` restano
vicini fra loro in entrambi i temi, esattamente come nel default, dove i due
token portano lo stesso valore.

Il `Subtle` di entrambi deriva sull'`8` (§2.3), quindi la zebratura sopravvive a
16 colori anche con questi temi — sui terminali scuri per cui sono fatti.

## 7. Il bordo di `th.Box` (#138 punto 2)

```go
Box: r.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(p.Muted).Padding(0, 1),
```

Lo stesso colore di `th.Border`, che è ciò che rende la cornice della tabella del
report: dopo questa riga le due cornici della stessa schermata pesano uguale, che
è il difetto che la #138 descrive. Due golden da rigenerare (§2.2), **guardati**
dopo.

## 8. Test

- **Il test che porta il peso ha due metà.** La prima confronta i **valori** di
  `Default()` con quelli che `internal/tui` porta oggi, letterali scritti a mano
  nel test. La seconda **rende** ogni token a tutti e quattro i profili su
  entrambi i fondi e confronta con la tavolozza di oggi ricostruita nel test,
  byte per byte. Le due insieme dicono che spostare il codice non ha cambiato un
  colore, e inchiodano la derivazione di §2.3: se la regola di vicinanza cambia,
  `Subtle` smette di dare `8`/`7` e cadono entrambe.
- **La derivazione** ha una tabella sua, con i valori misurati in §2.3, compresi
  i due dei temi built-in e il punto di scambio.
- **La validazione** ha una tabella di input cattivi, uno per riga di §5, e ogni
  caso asserisce che l'errore **nomina il token e il valore** — non solo che un
  errore c'è. Un test che si accontenta di `err != nil` passa anche con un
  messaggio inutile.
- **Nessun token di nessun tema built-in può rendere vuoto.** È la trappola di
  §2.1: un refuso in un hex scritto da noi non darebbe errore, darebbe testo
  scolorito. Il test rende ogni token di ogni tema built-in a `TrueColor` e
  pretende una sequenza di escape.
- **Il round-trip** di `theme:` e `themes:` su disco, con la forma stringa e la
  forma coppia — e in particolare che **un valore scritto come stringa torni su
  disco come stringa** dopo un `Save`. Senza `MarshalYAML` quel test fallisce, ed
  è l'unico modo per accorgersi che stiamo riscrivendo il file dell'utente.
- **Ogni difetto ha un test visto fallire** contro il difetto, col transcript. In
  particolare: la derivazione va vista fallire invertendo la scelta fra `8` e
  `7`, e il bordo di `th.Box` va visto fallire togliendo `BorderForeground`.
- **Golden**: si muovono esattamente `palette_dark` e `palette_light`. Ogni altro
  golden che si muove è un difetto dell'implementazione, non un golden stantio:
  non va rigenerato, va indagato.
- Gate: `gofmt -l .`, `go vet ./...`,
  `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`,
  `go test ./... -race`.

## 9. Fuori scope (dichiarato)

- **Override delle keybinding** (casella 2 della #82): tranche a sé. Tocca 51
  binding e non condivide con i colori nulla se non il file di configurazione.
- **Una directory di temi** (`~/.config/clup/themes/*.yml`): tutto sta in
  `config.yml`. Una directory aggiunge scoperta di file, precedenze e nuovi
  percorsi d'errore per un beneficio che nessuno ha chiesto.
- **Un selettore di tema dentro la TUI**: si sceglie dal config. Una schermata
  per cambiare colori è una feature, non questa issue.
- **Adattare dracula e nord al fondo chiaro**: deciso in §2.4, con i numeri.
- **Il punto 1 della #138** (`reportNumWidth`): già chiuso dalla tranche D, che
  ha sostituito la costante con `reportNumWidths` misurato dal contenuto.
- **Rinominare il tipo `theme` di `internal/tui`**: 18 file per una ragione che
  non è di questa tranche (§3.1).

## 10. Vincoli globali

- `internal/report` e `internal/duration` restano **puri**: solo stdlib, nessun
  I/O, nessun import di `internal/config`, `internal/clickup`, `internal/tui`.
- `internal/themes` è foglia: **non** importa `config`, `tui`, `clickup`,
  `service`; non fa I/O.
- Nessuna funzione di stile in produzione chiama `lipgloss.NewStyle()`.
- Mai chiamare l'API ClickUp vera: il comportamento di rete si esercita solo con
  `httptest`.
- Tutto ciò che vive nel repo è in **inglese** — codice, commenti, stringhe UI,
  nomi e messaggi dei test, messaggi di commit. Eccezioni: `README.it.md`,
  `CONTRIBUTING.it.md` e i doc di design sotto `docs/superpowers/`.
- **Ogni numero e ogni sequenza di escape scritta in un commento va misurata
  eseguendo.**
- I golden si rigenerano solo con `go test ./internal/tui -update`, mai a mano, e
  si **guardano** dopo.
- Conventional Commits. **MAI** `Co-Authored-By`.
