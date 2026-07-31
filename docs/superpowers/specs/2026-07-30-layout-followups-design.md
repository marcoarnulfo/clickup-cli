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

**Questa sezione è stata riscritta dopo una review: la prima versione misurava
la cosa sbagliata, e il numero che ne era uscito è finito anche in un commento
sulla issue.** Vale la pena raccontare l'errore, perché è istruttivo.

La formula era `rigaPiùLargaDelBody − (x + boxW)`. Ma la riga più larga del body
è la **0**, il titolo, e il box comincia a `paletteTopY = 2`: il titolo sta
**sopra** il box, non accanto. La formula misurava quindi lo sforamento del
titolo — cioè il difetto di §2.1 — e lo attribuiva al compositore.

Misurato di nuovo, distinguendo le due quantità, iterando solo sulle righe che
il box copre davvero (`y .. y+len(boxLines)-1`, cioè 2..13):

| terminale | box | riga più larga del body | la vecchia formula diceva | esposto **accanto al box** |
|---|---|---|---|---|
| 40 | 36 @ x=2 | riga 0, 54 col | 16 | **1** |
| 50 | 46 @ x=2 | riga 0, 54 col | 6 | **0** |
| 60 | 52 @ x=4 | riga 0, 54 col | 0 | **0** |
| 80 | 52 @ x=14 | riga 0, 54 col | 0 | **0** |
| 120 | 52 @ x=34 | riga 0, 54 col | 0 | **0** |

La riga coperta più larga è sempre la **7**, la tabella a 39 colonne, a ogni
larghezza. Da cui tre conseguenze:

- **L'esposizione accanto al box è una colonna sola, e solo a larghezza 40.** È
  il bordo destro della tabella, contenuto legittimo dentro il terminale.
- **Il clamp dell'intestazione non la cambia**, perché la riga coperta più larga
  è la tabella e non il titolo. La dipendenza §3.3 → §3.2 che la prima versione
  dichiarava era un artefatto della formula sbagliata.
- **La vecchia formula sbagliava anche a 50 colonne**, dove diceva 6: quella
  larghezza non era nello sweep originale, che saltava da 40 a 60.

**Sull'attenuazione, una seconda correzione.** La prima versione diceva che
attenuare il fondo è «invisibile a questa suite» perché sotto `termenv.Ascii`
l'output è byte-identico. La premessa è vera per i **golden**, ma la conclusione
no: `TestCompositeDoesNotLeakStyleIntoTheBox`
(`internal/tui/overlay_test.go:95`) costruisce un renderer proprio con un
profilo colore reale e asserisce sui byte — il suo stesso commento dice che è
«the one failure mode a golden can never see». Con quella tecnica
l'attenuazione **sarebbe** verificabile. Resta tagliata, ma per la ragione
giusta: vedi §3.3.

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

### 3.3 Il compositore non si tocca (#143, prima metà)

**Chiuso senza modifiche al codice, e questa volta contro una misura.** Tre
ragioni, in ordine di peso.

**Non c'è un difetto da correggere.** L'esposizione accanto al box è **una
colonna** a larghezza 40 e **zero** a ogni altra larghezza (§2.2). Quella colonna
è il bordo destro della tabella del report, contenuto legittimo che sta dentro il
terminale. Le sedici colonne che sembravano un problema del compositore erano lo
sforamento del titolo, cioè §3.2.

**Il ritaglio violerebbe un contratto documentato.** `overlay.go:11-13` dice che
«cells of body outside the box's rectangle survive verbatim — that layering is
the whole point of #59», e tre test lo inchiodano:
`TestCompositeSplicesTheBoxIntoTheBody` pretende il segmento destro,
`TestCompositeKeepsEveryLineWidth` e
`TestCompositeHandlesWideGlyphsOnBothEdges` pretendono righe piene col box in
mezzo. Un ritaglio incondizionato al bordo destro del box li fa fallire tutti e
tre: non è una riga di codice, è un cambio di contratto.

**E la giustificazione di sicurezza che avevo scritto era falsa.** Dicevo che
«l'unica cosa mai esposta è contenuto che già sfonda il terminale». Dopo §3.2
niente sfonda più il terminale, e su un terminale largo con un report largo il
body sta legittimamente fra il bordo del box e il bordo dello schermo: il
ritaglio cancellerebbe proprio l'effetto di profondità che il contratto difende.

**L'attenuazione** resta tagliata perché non risolve niente — non c'è un difetto
da attenuare — non perché non sia verificabile: come dice §2.2, lo sarebbe con
la tecnica di `TestCompositeDoesNotLeakStyleIntoTheBox`.

Resta un lavoro, e non è codice: aggiungere un test che **inchioda** l'unica
colonna esposta a larghezza 40 come comportamento voluto, così che chi la vedrà
nel golden trovi scritto perché c'è. E chiudere la prima metà della #143 con la
misura.

## 4. Fuori scope (dichiarato)

- Il padding delle **cifre** del budget (§3.1): sgranate a destra, dove il box
  riempie comunque. **Conseguenza per i test:** un test che cerchi la colonna
  delle cifre dovrà ancorarsi a qualcosa che non dipende dalla larghezza del
  campo Billed, o usare una fixture in cui i Billed hanno la stessa larghezza —
  altrimenti asserisce un allineamento che questa spec ha deciso di non dare.
- L'overflow di `summary` e della nota billable con contenuti molto lunghi
  (§2.1): reale ma non in queste issue. Misurato che nelle fixture usate dai
  test non sforano (37 e 38 colonne a larghezza 40), quindi il fuori-scope non
  rompe un test dentro lo scope.
- Il **ritaglio** e l'**attenuazione** del fondo esposto (§3.3): non c'è un
  difetto da correggere, e il ritaglio violerebbe il contratto di layering
  della #59.

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
