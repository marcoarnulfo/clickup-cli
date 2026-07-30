# Follow-up di layout: allineamento budget e clamp dell'intestazione (design)

> v1.9. Chiude **#144** e **#143**, i due follow-up emersi dalla review finale
> della tranche D. Spec breve: sono tre interventi piccoli, tutti già misurati
> prima di scriverla.

## 1. Obiettivo

Tre residui di larghezza che la tranche D ha scoperto senza chiudere. Nessuna
feature nuova, nessuna chiamata API nuova, nessun tasto nuovo.

## 2. Evidenza misurata

Ogni numero qui sotto viene dall'aver **reso** la schermata, non dall'aver
sommato colonne a mano. È la regola che la tranche D ha dovuto imparare: due
numeri del suo piano erano sbagliati perché calcolati.

### 2.1 L'intestazione del report (#143, seconda metà)

Righe della `reportModel.view` che sforano, misurate con `goldenReport()` più la
nota membri `" (2/5 members)"`:

| terminale | riga 0 (titolo) | riga 1 (margine del titolo) | altre |
|---|---|---|---|
| 40 | **68** ✗ | **68** ✗ | tutte ≤ 39 |
| 60 | **68** ✗ | **68** ✗ | tutte ≤ 39 |
| 80 | 68 ✓ | 68 ✓ | tutte ≤ 39 |

Tre cose che la issue non diceva:

- **Il titolo sfora anche a 60 colonne**, non solo a 40. La issue era stata
  scritta guardando `palette_narrow.golden`, dove il titolo è 54 colonne perché
  quel modello non ha la nota membri; con la nota diventa 68.
- **La riga 1 è la riga di margine del titolo** (`th.Title` ha `MarginBottom(1)`,
  che emette una seconda riga di spazi larga quanto il contenuto). Sfora insieme
  al titolo, e si sistema insieme a lui: troncando il testo **prima** di
  `th.Title.Render` la riga di margine si accorcia da sola. Troncare dopo il
  `Render` la lascerebbe larga.
- **Nessun'altra riga sfora** in questa fixture: `summary` è 37 colonne e la nota
  billable 38, entrambe sotto 40. Con contenuti più lunghi — molte valute nel
  `SummaryLine` — potrebbero sforare anche loro, ma è fuori dallo scope di
  questa issue, che nomina l'intestazione. Registrato qui perché la prossima
  persona non lo riscopra da zero.

### 2.2 L'esposizione del fondo accanto al box (#143, prima metà)

Misurato rendendo dal percorso vero di `View()` (verificato: la variante «oggi»
riproduce `View()` byte per byte), per ogni larghezza:

| terminale | box | esposto a destra |
|---|---|---|
| 40 | 36 @ x=2 | **16 colonne** |
| 60 | 52 @ x=4 | 0 |
| 80 | 52 @ x=14 | 0 |
| 120 | 52 @ x=34 | 0 |
| 160 | 52 @ x=54 | 0 |

Il box è **centrato**, quindi da 60 colonne in su non resta esposto niente. E le
16 colonne a larghezza 40 sono dell'**intestazione**, non della tabella: la riga
esposta più larga è il titolo. Quindi §2.1 è la causa della maggior parte del
sintomo di §2.2, ed è per questo che va fatto prima.

**L'attenuazione del fondo è invisibile a questa suite.** Misurato: ristilare il
segmento esposto con `th.Help` produce output **byte-identico** a oggi sotto
`termenv.Ascii`, che `TestMain` fissa per tutto il package — sia avvolgendo il
segmento sia strippandolo prima. Nessun golden potrebbe distinguerla. È la
trappola della #141, dove la prova ha dovuto essere un grep.

### 2.3 L'allineamento del budget (#144)

`budgetLayout` misura `pctW` come massimo delle larghezze delle etichette
percentuale sulle righe vere, lo usa nell'aritmetica che dimensiona la colonna
nome, e poi il rendering non ci allinea niente. Con righe a `62%` e `104%` le
cifre della prima partono una colonna più a sinistra. Mai un overflow —
l'aritmetica riserva per la più larga, quindi sbagliare per difetto è sicuro.

`renderBudgetBar` produce barra e percentuale come una stringa sola e **non
conosce `pctW`**: è il motivo per cui l'allineamento non è mai avvenuto.

## 3. Gli interventi

### 3.1 `renderBudgetBar` riceve la larghezza della percentuale (#144)

```go
func renderBudgetBar(th theme, percentUsed float64, pctW int) string
```

La percentuale viene allineata a destra dentro `pctW` colonne. `pctW <= 0`
significa larghezza naturale, che è il comportamento di oggi: i due call site di
test che non stanno misurando l'allineamento passano `0` e restano validi.

Il chiamante di produzione (`budget.go:149`) passa il `pctW` che `budgetLayout`
ha già misurato — la stessa quantità, non una ricalcolata, così le due non
possono divergere.

**L'etichetta continua ad arrotondare per difetto** (#136): il riempimento
tronca, quindi un'etichetta che arrotondasse per eccesso rivendicherebbe una
soglia che la barra non ha raggiunto. Non toccare quella parte.

Le cifre restano non paddate: una volta fissata la percentuale sono sgranate a
destra e non a sinistra, e il box riempie comunque il bordo destro. Misurare
prima di cambiare idea.

### 3.2 L'intestazione del report viene troncata (#143, seconda metà)

In `reportModel.view`, il testo del titolo viene troncato con `truncateWidth`
**prima** di `th.Title.Render`, così anche la riga di margine si accorcia.

`width <= 0` (primo render, prima del `WindowSizeMsg`) lascia il titolo intatto:
`truncateWidth(s, 0)` ritorna `""` e cancellerebbe l'intestazione. È lo stesso
fallback che `reportItemWidth` e `budgetLayout` hanno già.

### 3.3 La colonna residua accanto al box (#143, prima metà)

**Da decidere dopo §3.2, con i numeri nuovi.** Fatto il clamp
dell'intestazione, la riga più larga del body diventa la tabella e l'esposizione
attesa scende da 16 colonne a una. Il task rimisura e riporta; se resta una
colonna sola, il ritaglio è una riga di codice con un golden che lo prova, e va
fatto. Se resta di più, il task si ferma e riporta invece di decidere.

Il ritaglio è sicuro in principio: l'unica cosa mai esposta è contenuto che già
sfonda il terminale. Ma va deciso contro una misura, non contro un'aspettativa.

## 4. Fuori scope (dichiarato)

- Il padding delle **cifre** del budget (§3.1): sgranate a destra, dove il box
  riempie comunque.
- L'overflow di `summary` e della nota billable con contenuti molto lunghi
  (§2.1): reale ma non in queste issue.
- L'**attenuazione** del fondo (§2.2): non implementabile in modo verificabile
  da questa suite.

## 5. Test

- Ogni difetto ha un test **visto fallire** contro il difetto, col transcript.
- I test di larghezza usano le larghezze **misurate** in §2, non larghezze
  scelte per prudenza: la tranche D ha prodotto tre test che asserivano sotto il
  minimo raggiungibile dal codice corretto e quindi non potevano passare.
- Golden rigenerati con `go test ./internal/tui -update`, mai a mano, e
  **guardati** dopo.
- Gate: `gofmt -l .`, `go vet ./...`,
  `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`,
  `go test ./... -race`.

## 6. Vincoli globali

- `internal/report` e `internal/duration` restano **puri**.
- Nessuna funzione di stile in produzione chiama `lipgloss.NewStyle()`.
- Tutto ciò che vive nel repo è in **inglese** — codice, commenti, stringhe UI,
  nomi e messaggi dei test, messaggi di commit. Eccezione: i doc di design qui
  sotto `docs/superpowers/`.
- **Ogni numero scritto in un commento va misurato rendendo.** La tranche D ha
  corretto quattordici commenti che affermavano cose false.
- Conventional Commits. **MAI** `Co-Authored-By`.
