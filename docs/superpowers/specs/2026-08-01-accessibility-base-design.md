# Accessibility base: rotella del mouse e degradazione a 16 colori (design)

> v1.9. Chiude la **#74** — le due caselle rimaste aperte: supporto mouse e
> downconvert truecolor→256→16. Le altre due (`NO_COLOR`, `SetColorProfile`)
> erano già state chiuse dalla tranche A.

## 1. Obiettivo

«Rispettare la cittadinanza del terminale», nelle parole della issue. Due
interventi indipendenti fra loro, tenuti in una spec sola perché sono entrambi
piccoli e riguardano lo stesso tema: comportarsi bene su terminali che non sono
quello dello sviluppatore.

Nessuna chiamata API nuova, nessuna schermata nuova.

## 2. Evidenza misurata

Ogni numero e ogni sequenza di escape qui sotto viene dall'aver **eseguito** il
codice, non dall'averlo letto. È la regola che le tranche precedenti hanno dovuto
imparare a caro prezzo: in tranche D quattro numeri scritti a mano erano falsi, e
uno era finito in un commento pubblico su una issue.

### 2.1 Il downconvert esiste già

termenv converte da solo, e lo fa bene. Misurato rendendo ogni tinta di
`defaultPalette()` su un renderer con profilo fissato:

| tinta | sorgente | TrueColor | ANSI256 | ANSI (16) |
|---|---|---|---|---|
| Primary dark | `205` | `38;5;205` | `38;5;205` | `95` (magenta brillante) |
| Primary light | `127` | `38;5;127` | `38;5;127` | `35` (magenta) |
| Muted | `240` | `38;5;240` | `38;5;240` | `90` (nero brillante) |
| Danger dark | `196` | `38;5;196` | `38;5;196` | `91` (rosso brillante) |
| Danger light | `124` | `38;5;124` | `38;5;124` | `31` (rosso) |
| Success dark | `42` | `38;5;42` | `38;5;42` | `92` (verde brillante) |
| Success light | `28` | `38;5;28` | `38;5;28` | `32` (verde) |
| **Subtle dark** | `236` | `48;5;236` | `48;5;236` | **`40` (sfondo nero)** |
| **Subtle light** | `254` | `48;5;254` | `48;5;254` | **`107` (sfondo bianco brillante)** |

E lo stesso vale partendo da un hex, che è quello che i temi YAML della #82
permetteranno: `#FF5FAF` → `38;2;255;95;175` a TrueColor → `38;5;205` a 256 →
`95` a 16. **Non c'è una pipeline di conversione da costruire.**

C'è un difetto solo, ed è nelle due righe in grassetto: la conversione sceglie il
colore *più vicino*, e a 16 colori il grigio medio non esiste. `236` (#303030)
finisce sul nero e `254` (#E4E4E4) sul bianco brillante — cioè, su un terminale
normale, **esattamente il colore dello sfondo**. La zebratura della tabella del
report sparisce su entrambi i fondi.

### 2.2 `CompleteAdaptiveColor` risolve, e con quale costo

`lipgloss.CompleteAdaptiveColor` dichiara un valore per profilo invece di
lasciar convertire. Misurato mettendo l'**indice 256 anche nello slot TrueColor**,
confrontato con l'`AdaptiveColor{Light: "254", Dark: "236"}` di oggi:

| fondo | profilo | oggi | con Complete | identico? |
|---|---|---|---|---|
| scuro | TrueColor | `48;5;236` | `48;5;236` | **sì** |
| scuro | ANSI256 | `48;5;236` | `48;5;236` | **sì** |
| scuro | ANSI | `40` | `100` | no — è il punto |
| scuro | Ascii | *(niente)* | *(niente)* | **sì** |
| chiaro | TrueColor | `48;5;254` | `48;5;254` | **sì** |
| chiaro | ANSI256 | `48;5;254` | `48;5;254` | **sì** |
| chiaro | ANSI | `107` | `47` | no — è il punto |
| chiaro | Ascii | *(niente)* | *(niente)* | **sì** |

Tre profili su quattro non si muovono di un byte. Questo è il motivo per cui
lo slot TrueColor porta un indice e non un hex: un hex avrebbe cambiato anche il
profilo truecolor senza guadagnare niente, e con un trabocchetto in più —
misurato, lipgloss rende `#E4E4E4` come `48;2;227;227;227`, non `228`, per un
arrotondamento nel percorso hex→float→uint8 di go-colorful. Invisibile
all'occhio, letale per un test che asserisse la tripletta esatta.

**La trappola vera di `CompleteColor`, e vale soprattutto per la #82.** Non
converte niente: è una tabella di lookup. Misurato, un
`CompleteColor{TrueColor: "#303030"}` con gli altri due slot vuoti rende il testo
**senza alcun colore** sia a 256 sia a 16 colori. Se i temi YAML accetteranno un
valore solo per tinta, quel valore non va salvato come `CompleteColor`, o il
colore sparisce su ogni terminale non truecolor.

### 2.3 Perché 8 e 7, e cosa non possiamo sapere

A 16 colori le uniche scelte che non sono una *tinta* (rosso, verde, blu…) sono
le quattro sfumature: 0 nero, 8 nero brillante, 7 bianco, 15 bianco brillante.

Su fondo scuro cadono 0 (è lo sfondo) e 15 (è il testo), e restano **due**
candidati, 7 e 8 — non uno solo. Il criterio che sceglie fra i due è che la
striscia deve restare **vicina alla sfumatura che sostituisce**: il 236 di oggi è
#303030, un grigio scuro, e l'8 è la sua controparte a 16 colori, mentre il 7 è
un grigio chiaro che su molti temi coincide con il colore di default del testo —
una striscia che si confonde col testo è peggio di una che si confonde con lo
sfondo. Specularmente su fondo chiaro cadono 15 e 0, restano 7 e 8, e il 254
(#E4E4E4, grigio chiarissimo) porta al **7**.

Va detto onestamente: gli indici 0-15 sono il tema del terminale dell'utente, non
un colore che conosciamo. Il contrasto esatto della striscia **non è
conoscibile** da qui — su xterm l'8 è #808080, ma un tema qualsiasi può metterlo
altrove. La scelta è quindi la migliore possibile per costruzione (l'unica
sfumatura che non coincide né con il fondo né con il testo), non una scelta
verificata contro una soglia WCAG come quelle di `defaultPalette()`.

### 2.4 Il mouse oggi non esiste

Nessun `tea.MouseMsg` in tutto il repo, e `internal/cli/cli.go:40` costruisce il
programma con `tea.WithAltScreen()` e basta.

Ma l'idioma per farlo funzionare c'è già: `internal/tui/actions.go:61` fa
eseguire alla command palette le azioni di schermata costruendo un `tea.KeyMsg`
sintetico e **rigiocandolo dentro `routeKey`**. Una rotella tradotta in su/giù e
mandata per la stessa strada eredita il cursore di ogni schermata, e i test che
lo coprono, senza toccarne nessuna.

### 2.5 Due contesti in cui una tacca non deve arrivare

Rigiocare la rotella come su/giù la fa arrivare anche dove **qualunque tasto è
una risposta**, non un movimento di cursore. Misurato mandando un `KeyDown`
dentro `routeKey` su ogni schermata, due casi fanno danno:

| contesto | cosa succede con un `KeyDown` | perché è un problema |
|---|---|---|
| `screenError` | `resetTo(screenHome)` (`app.go:953-960`) | una tacca accidentale **chiude il messaggio d'errore** prima che sia stato letto |
| `entriesConfirmDelete` | `mode` torna a `entriesList` (`entries.go:262-266`) | una tacca **annulla la conferma di cancellazione** |

Il secondo è dal lato sicuro — la cancellazione richiede `y`, e una rotella non
la conferma mai — ma resta sorprendente. Il primo no: con il mouse tracking
attivo e l'inerzia di un trackpad, perdere un errore per una tacca di troppo
succederà davvero.

Sono esattamente i due contesti in cui il codebase ha già dovuto disattivare a
mano il tasto `?` (`app.go:675-678` li elenca fra i posti dove `keysFor(m).Help`
resta non assegnata). La proprietà che li accomuna non è la schermata: è che
**ogni tasto è una risposta a una domanda**, e una tacca di rotella non è una
risposta.

### 2.6 La forma degli eventi rotella

**Misurato dal sorgente di bubbletea v1.3.10**, e non assunto: gli eventi rotella
portano sempre `Action == MouseActionPress`. Il parser SGR lo dice esplicitamente
(`mouse.go:186`, *«Wheel buttons don't have release events»*, e la condizione a
188 esclude `IsWheel()` dal ramo di rilascio); il parser X10 assegna il rilascio
solo al ramo dei pulsanti normali (`mouse.go:249-253`) e non applica il bit di
motion agli eventi rotella (`mouse.go:257`). **Una tacca produce esattamente un
messaggio**: non serve filtrare sull'`Action`, e non c'è doppio scatto.

## 3. Gli interventi

### 3.1 La rotella diventa su/giù (#74, casella «mouse»)

Un file nuovo, `internal/tui/mouse.go`, con una funzione pura:

```go
func wheelKey(msg tea.MouseMsg) (tea.KeyMsg, bool)
```

`MouseButtonWheelUp` → `tea.KeyMsg{Type: tea.KeyUp}`, `MouseButtonWheelDown` →
`tea.KeyMsg{Type: tea.KeyDown}`, **tutto il resto** — click, motion, rotella
orizzontale, pulsanti laterali — → `false`.

Non passa da `keyMsgFor`: quella funzione ha un insieme chiuso di proposito
(`actions.go:140-148`) perché serve alla palette, e allargarlo per un chiamante
diverso ne tradirebbe il commento. `wheelKey` costruisce i suoi due messaggi e si
porta dietro la propria motivazione.

In `app.go`, dentro il type-switch di `Update`, un caso che ricalca
l'instradamento dei tasti **meno le scorciatoie globali**:

```go
case tea.MouseMsg:
	k, ok := wheelKey(msg)
	if !ok {
		return m, nil
	}
	if m.overlay != overlayNone {
		return m.updateOverlay(k)
	}
	if m.anyKeyIsAnAnswer() {
		return m, nil
	}
	return m.routeKey(k)
```

ForceQuit, Quit, `?` e `ctrl+p` non sono controllati perché una rotella non può
corrispondervi: sono tutti tasti. Il controllo sull'overlay invece resta, ed è
nello stesso ordine del ramo `tea.KeyMsg` (`app.go:662-668`), perché con la
palette aperta la rotella deve muovere la selezione della palette e non la
schermata sotto. E viene **prima** del cancello di §2.5: un overlay possiede la
rotella come possiede la tastiera, qualunque cosa ci sia sotto.

Il cancello è un predicato con un criterio di ammissione dichiarato, non un
elenco di schermate:

```go
func (m Model) anyKeyIsAnAnswer() bool
```

Vero per `screenError` e per `screenEntries` in `entriesConfirmDelete`, cioè per
i due contesti misurati in §2.5. Il nome dice la proprietà e non il meccanismo,
di proposito: è quello che dà alla lista un criterio per crescere — un contesto
nuovo ci entra se e solo se ogni tasto vi è una risposta — invece di lasciarla
marcire come elenco di casi speciali.

**Una tacca muove una riga.** Nessuna accelerazione, nessun moltiplicatore: la
misura in §2.4 dice che il terminale manda già un messaggio per tacca, quindi
qualunque fattore sarebbe una preferenza travestita da costante.

La conseguenza voluta: la rotella funziona **da subito su ogni schermata che già
gestisce su/giù** — report, entries, filtri, rates, list browser, members, range,
e la palette in overlay — senza un modello di geometria e senza modificare una
sola `updateX`.

### 3.2 La chiave `mouse` nel config (#74, cittadinanza)

```go
Mouse *bool `yaml:"mouse,omitempty"`

func (c Config) MouseEnabled() bool { return c.Mouse == nil || *c.Mouse }
```

Puntatore per la ragione già documentata su `UpdateCheck` (`config.go:78-81`):
con un `bool` semplice una chiave assente decodificherebbe come `false`, e ogni
config scritto prima che il campo esistesse spegnerebbe di soppiatto una feature
accesa di default.

In `internal/cli/cli.go`:

```go
opts := []tea.ProgramOption{tea.WithAltScreen()}
if cfg.MouseEnabled() {
	opts = append(opts, tea.WithMouseCellMotion())
}
p := tea.NewProgram(tui.New(cfg), opts...)
```

`WithMouseCellMotion` (modo 1002) e non `WithMouseAllMotion` (1003): il secondo
riporta ogni movimento del puntatore anche senza pulsanti premuti, un fiume di
messaggi che butteremmo via subito. Nessuno dei due è «solo rotella»: quel modo
DEC non esiste. È per questo che il costo esiste ed è dichiarato — vedi §5.

### 3.3 La zebratura sopravvive a 16 colori (#74, casella «downconvert»)

In `theme.go`, il campo `Subtle` della `palette` cambia tipo:

```go
Subtle lipgloss.CompleteAdaptiveColor // zebra row background
```

```go
Subtle: lipgloss.CompleteAdaptiveColor{
	Light: lipgloss.CompleteColor{TrueColor: "254", ANSI256: "254", ANSI: "7"},
	Dark:  lipgloss.CompleteColor{TrueColor: "236", ANSI256: "236", ANSI: "8"},
},
```

Le altre cinque tinte **restano `AdaptiveColor` e non si toccano**: la tabella in
§2.1 dice che la loro conversione automatica arriva già dove deve. Rendere
esplicito anche il resto sarebbe cinque volte più superficie per zero cambiamenti
misurabili.

Il tipo della `palette` diventa quindi eterogeneo — cinque `AdaptiveColor` e una
`CompleteAdaptiveColor`. È voluto, e il commento accanto al campo deve dire
perché, così la #82 non lo uniforma per estetica: `Subtle` è l'unica tinta che a
16 colori ha bisogno di una sfumatura che la conversione automatica non sa
trovare.

## 4. Test

- **Il test che porta il peso** è sul tema, e ha due metà. Con un renderer a
  profilo `termenv.ANSI`, `th.Zebra.Render(...)` deve contenere `\x1b[100m` su
  fondo scuro e `\x1b[47m` su fondo chiaro. E con `termenv.TrueColor` e
  `termenv.ANSI256` deve restare **byte per byte identico** a quello che
  produrrebbe `lipgloss.AdaptiveColor{Light: "254", Dark: "236"}`. La seconda
  metà è la guardia che tiene chirurgico il cambiamento: senza, nessuno si
  accorge se qualcuno «migliora» gli slot mettendoci un hex.
- **`wheelKey`**: tabella su su/giù/sinistra/destra, click sinistro, motion.
- **Instradamento**: la rotella muove il cursore su una schermata a lista; con la
  palette aperta muove la selezione della palette e non la schermata sotto; non
  chiude la schermata d'errore e non annulla la conferma di cancellazione (§2.5);
  un click non cambia niente.
- **Il test sul click parte da un cursore diverso da zero.** Asserire «riga 0
  prima, riga 0 dopo» lo farebbe passare anche se il click venisse tradotto in
  *su*: a riga 0 il cursore è già in cima e non si muove comunque. Va portato
  prima a riga 1 con una tacca, e lì il click deve lasciarlo a 1 — così il test
  vede sia click→giù sia click→su.
- **Config**: `MouseEnabled()` per `nil`/`true`/`false`, e il round-trip su disco
  di `mouse: false`.
- **Ogni difetto ha un test visto fallire** contro il difetto, col transcript. In
  particolare il test del tema va visto fallire rimettendo `AdaptiveColor`, e il
  test dell'instradamento con overlay va visto fallire togliendo il controllo
  su `m.overlay`.
- **Golden**: attesi immutati, perché `TestMain` fissa `termenv.Ascii` e la
  tabella in §2.2 dice che sotto Ascii lo sfondo non esce comunque. **Da
  verificare eseguendo la suite, non da assumere**; se qualcuno si muove, si
  rigenera con `go test ./internal/tui -update` e si **guarda**.
- Gate: `gofmt -l .`, `go vet ./...`,
  `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`,
  `go test ./... -race`.

### 4.1 Un limite dichiarato invece che nascosto

`tea.ProgramOption` è una funzione opaca: un test su `cli.go` può provare che
**viene aggiunta un'opzione in più** quando il mouse è acceso, non *quale*
opzione. La copertura vera sta su `MouseEnabled()`, che è puro e testabile a
fondo. Il test su `cli.go` esiste per intercettare l'`append` dimenticato, e il
suo commento deve dire esattamente questo — un test che promettesse di più
sarebbe un test che passa per la ragione sbagliata.

## 5. Il costo, dichiarato

Abilitare il mouse tracking fa smettere al terminale di gestire la **selezione
nativa del testo**: gli eventi vanno all'applicazione. Su un'app da cui si
copiano importi non è un dettaglio.

Due vie d'uscita, ed è per questo che la §3.2 esiste:

- **momentanea**: Shift+trascina, che quasi tutti i terminali riservano alla
  selezione locale proprio per questo caso;
- **permanente**: `mouse: false` nel config.

Entrambe vanno nei README, in inglese e in italiano, insieme alla chiave nuova.

## 6. Fuori scope (dichiarato)

- **Click e hit-testing.** Muovere il cursore sulla riga puntata richiede che
  ogni schermata dichiari dove comincia il contenuto e quanto è alta una riga:
  oggi le view producono stringhe, non geometria. È un modello nuovo, e va
  progettato insieme al design system (#82/#138), non incastrato qui.
- **Toggle del mouse a runtime.** Shift+trascina copre il caso momentaneo e la
  chiave di config quello permanente; un terzo meccanismo non aggiunge un caso
  d'uso.
- **Rotella orizzontale** e pulsanti laterali: `wheelKey` li scarta
  esplicitamente.
- **Valori per-profilo sulle altre cinque tinte**: misurato che non servono
  (§2.1).
- **`FORCE_COLOR`**: già escluso dalla issue stessa, con la sua motivazione.

## 7. Vincoli globali

- `internal/report` e `internal/duration` restano **puri**.
- Nessuna funzione di stile in produzione chiama `lipgloss.NewStyle()`.
- Mai chiamare l'API ClickUp vera: il comportamento di rete si esercita solo con
  `httptest`.
- Tutto ciò che vive nel repo è in **inglese** — codice, commenti, stringhe UI,
  nomi e messaggi dei test, messaggi di commit. Eccezione: i doc di design qui
  sotto `docs/superpowers/`.
- **Ogni numero e ogni sequenza di escape scritta in un commento va misurata
  eseguendo.**
- Conventional Commits. **MAI** `Co-Authored-By`.
