# Follow-up di layout: piano di implementazione

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Chiudere #144 e #143, i due follow-up della tranche D.

**Spec:** `docs/superpowers/specs/2026-07-30-layout-followups-design.md`

> **Questa è la seconda versione.** La prima è stata bocciata da una review
> adversariale che ha trovato tre difetti critici: due test che non potevano
> passare contro il codice corretto, e una formula di misura che guardava le
> righe sbagliate e da cui dipendeva l'intero Task 3. Le correzioni sono
> incorporate qui e nella spec §2.2/§3.3.

## Global Constraints

- `internal/report` e `internal/duration` restano **puri**: nessun I/O, nessun
  import di `config`/`clickup`/`tui`.
- Nessuna funzione di stile in produzione chiama `lipgloss.NewStyle()`.
- Golden rigenerati con `go test ./internal/tui -update`, **mai** a mano, e
  guardati dopo.
- Gate prima di ogni commit, tutti e cinque puliti: `gofmt -l .`,
  `go vet ./...`, `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`,
  `go build ./...`, `go test ./... -race`.
- Tutto in **inglese**, ortografia americana: codice, identificatori, commenti,
  nomi e messaggi dei test, messaggi di commit.
- Conventional Commits. **MAI** `Co-Authored-By`.
- **Le larghezze si misurano rendendo, non calcolando**, e si misurano sulle
  righe **giuste**: la prima versione di questo piano ha misurato lo sforamento
  del titolo credendo di misurare l'esposizione accanto al box, perché il titolo
  è la riga più larga del body ma sta sopra il box, non accanto.
- **Un test scritto contro un bug non è attendibile finché non lo si è visto
  fallire contro quel bug**, e non deve asserire qualcosa che il codice corretto
  non può dare: due dei test della prima versione asserivano un allineamento che
  la spec dichiara fuori scope, e uno asseriva l'assenza di uno spazio che
  l'implementazione prescritta emette sempre.
- **Ogni numero scritto in un commento va misurato.**

## Helper esistenti (verificati contro il repo)

- `truncateWidth(s string, cols int) string`, `cell(s string, cols int) string`
  — `internal/tui/width.go`. `cols <= 0` ritorna `""` in entrambi.
- `testTheme(dark bool) theme` — `theme_test.go:15`.
- `golden(t, name, out)` — `golden_test.go:35`; `goldenReport()` — `:70`;
  `goldenPaletteModel()` — `:397`.
- `newReport(r report.Report, note string, daily []float64) reportModel`
  — `report.go:23`. Il terzo parametro si chiama **`daily`**.
- `reportModel.view(th theme, width int) string` — `report.go:198`, **già riceve
  la larghezza**; chiamata da `app.go:980`.
- `renderBudgetBar(th theme, percentUsed float64) string` — `budget.go:60`.
  Chiamanti: produzione `budget.go:149`, test `budget_test.go:43` e
  `budget_bar_test.go:57`.
- `budgetLayout(lines []report.BudgetLine, width int) (nameW int, showRemaining bool)`
  — `budget.go:116`. **Chiamanti di test: `budget_test.go:163`, `:171`, `:180`.**
  Cambiarne la firma non compila finché non si toccano tutti e tre.
- `composite(body, box string, x, y int) string` — `internal/tui/overlay.go:30`.
  Il suo contratto è documentato alle righe 11-13 e inchiodato da
  `TestCompositeSplicesTheBoxIntoTheBody`,
  `TestCompositeKeepsEveryLineWidth`,
  `TestCompositeHandlesWideGlyphsOnBothEdges` e
  `TestCompositeDoesNotLeakStyleIntoTheBox` (`overlay_test.go`).
- `paletteTopY = 2` — `palette.go:19`: il box comincia alla riga 2 del body.

## Ordine dei task

`1`, `2` e `3` sono **indipendenti**. La prima versione dichiarava `3` dipendente
da `2`; era un artefatto della formula sbagliata — la riga coperta più larga è la
tabella, non il titolo, quindi il clamp del titolo non cambia niente per il
Task 3 (spec §2.2).

---

### Task 1: allineare la percentuale del budget (#144)

**Files:**
- Modify: `internal/tui/budget.go` (`renderBudgetBar`, la misura di `pctW`, `view`)
- Modify: `internal/tui/budget_test.go` — il terzo argomento a riga 43, e i tre
  chiamanti di `budgetLayout` a righe **163, 171, 180** se ne cambi la firma
- Modify: `internal/tui/budget_bar_test.go:57` — il terzo argomento
- Test: `internal/tui/budget_test.go`
- Test: `internal/tui/testdata/budget.golden`

**Interfaces:**
- Produces: `renderBudgetBar(th theme, percentUsed float64, pctW int) string`.

- [ ] **Step 1: Il test che si vede fallire, con la fixture giusta**

La fixture è scelta, non casuale, e il perché va nel commento: i due `Billed`
hanno la **stessa** larghezza (`625.00` e `940.00`, sei caratteri) mentre le due
percentuali no (`62%` e `104%`). Così la posizione dello `/` isola esattamente il
padding della percentuale. Con `Billed` di larghezze diverse lo `/` divergerebbe
comunque, e il test asserirebbe l'allineamento delle **cifre**, che la spec §4
dichiara fuori scope — cioè non potrebbe passare mai.

```go
// budgetLayout measures the widest percentage label across the real rows and
// uses it to size the name column, but the rendering never padded to it: a row
// at 62% started its figures one column left of a row at 104% (#144).
//
// The fixture is picked, not arbitrary. Both Billed values are six characters
// wide while the two percentage labels are three and four, so the column of the
// "/" isolates the percentage padding and nothing else. With Billed values of
// different widths the "/" would diverge anyway, and this test would be
// asserting that the FIGURES line up — which the spec deliberately does not do.
//
// Measured before the fix: the "/" sits at column 65 on the 62% row and 66 on
// the 104% row.
func TestBudgetViewAlignsFiguresAcrossRows(t *testing.T) {
	t.Parallel()
	th := testTheme(true)
	bm := newBudget([]report.BudgetLine{
		{ListName: "Website", Billed: 625, Budget: 1000, Currency: "EUR", Remaining: 375, PercentUsed: 62.5},
		{ListName: "Mobile app", Billed: 940, Budget: 900, Currency: "EUR", Remaining: -40, PercentUsed: 940.0 / 900 * 100},
	})
	var cols []int
	for _, line := range strings.Split(bm.view(th, 100), "\n") {
		if i := strings.Index(line, "/"); i >= 0 {
			cols = append(cols, lipgloss.Width(line[:i]))
		}
	}
	if len(cols) != 2 {
		t.Fatalf("expected two budget rows, found %d figure columns", len(cols))
	}
	if cols[0] != cols[1] {
		t.Errorf("figures start at column %d on one row and %d on the other", cols[0], cols[1])
	}
}

// A percentage narrower than the measured maximum is right-aligned into it, so
// every column after it holds. pctW <= 0 keeps the natural width, which is what
// the callers that are not testing alignment pass.
//
// The natural case is checked by WIDTH, not by suffix: renderBudgetBar always
// emits one space between the bar and the label, so "ends in \" 62%\"" is true
// of every conforming implementation and would assert nothing.
func TestRenderBudgetBarRightAlignsThePercentage(t *testing.T) {
	t.Parallel()
	th := testTheme(true)
	wide := renderBudgetBar(th, 104, 4)
	narrow := renderBudgetBar(th, 62.5, 4)
	if lipgloss.Width(wide) != lipgloss.Width(narrow) {
		t.Errorf("bars are %d and %d columns wide, want equal", lipgloss.Width(wide), lipgloss.Width(narrow))
	}
	if !strings.HasSuffix(narrow, "  62%") {
		t.Errorf("narrow = %q, want the label right-aligned into 4 columns", narrow)
	}
	natural := renderBudgetBar(th, 62.5, 0)
	if got, want := lipgloss.Width(natural), budgetBarWidth+1+3; got != want {
		t.Errorf("natural width = %d, want %d (bar + separator + \"62%%\", no padding)", got, want)
	}
}
```

- [ ] **Step 2: Vedere il RED**

`TestRenderBudgetBarRightAlignsThePercentage` nomina `renderBudgetBar` con tre
argomenti, quindi il package non compila: `FAIL [build failed]` non è un RED.
Aggiungi quindi **solo** `TestBudgetViewAlignsFiguresAcrossRows`, che usa la
firma attuale di `view`, ed eseguilo.

Run: `go test ./internal/tui -run TestBudgetViewAlignsFiguresAcrossRows -v`
Expected: FAIL con `figures start at column 65 on one row and 66 on the other`.
**Se i numeri non sono 65 e 66, fermati**: la fixture non è quella misurata.

- [ ] **Step 3: Implementare**

```go
// pctW is the width the caller measured for the widest percentage label across
// all rows; the label is right-aligned into it so every row's figures start at
// the same column (#144). pctW <= 0 means natural width — the callers that are
// not testing alignment pass 0, and so does the pre-WindowSizeMsg path below.
func renderBudgetBar(th theme, percentUsed float64, pctW int) string {
	...
	label := fmt.Sprintf("%.0f%%", math.Floor(percentUsed))
	if pad := pctW - lipgloss.Width(label); pad > 0 {
		label = strings.Repeat(" ", pad) + label
	}
	return fmt.Sprintf("%s %s", bar, label)
}
```

Non toccare `math.Floor`: il riempimento tronca, quindi la label deve
arrotondare per difetto o rivendicherebbe una soglia che la barra non ha
raggiunto (#136).

`view` deve passare **la stessa** `pctW` che l'aritmetica di `budgetLayout` ha
usato, non una ricalcolata: se divergono, la riga sfora di quanto divergono. Due
strade, scegli tu:

- allargare la firma di `budgetLayout` a `(nameW, pctW int, showRemaining bool)`
  — ricorda i tre chiamanti di test a `budget_test.go:163, 171, 180`, o non
  compila;
- estrarre la misura in un helper che **entrambe** chiamano — verifica che non
  resti una copia della formula.

**Vincolo che vale per entrambe:** a `width <= 0` (primo render, prima del
`WindowSizeMsg`) `budgetLayout` ritorna il fallback senza misurare niente, quindi
la label resta **naturale**, `pctW = 0`. Non misurare `pctW` in quel ramo: le due
strade divergerebbero là, e nessun test lo coprirebbe.

- [ ] **Step 4: Eseguire e rigenerare il golden**

```bash
go test ./internal/tui -run 'TestBudget|TestRenderBudgetBar' -v
go test ./internal/tui -race
go test ./internal/tui -update
git diff internal/tui/testdata/
```
Expected: `budget.golden` **non si muove** — ha una riga sola, al 62%, quindi
`pctW` è 3 e il padding è un no-op. Verificalo; se si muove, guarda cosa è
cambiato e riportalo.

- [ ] **Step 5: Gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui
git commit -m "fix(tui): right-align the budget percentage so the figures line up (#144)"
```

---

### Task 2: troncare l'intestazione del report (#143, seconda metà)

**Files:**
- Modify: `internal/tui/report.go` (`reportModel.view`, il titolo a riga ~204)
- Test: `internal/tui/report_test.go`
- Test: `internal/tui/testdata/palette_narrow.golden`

**Interfaces:**
- Consumes: `truncateWidth`.

- [ ] **Step 1: Il test che si vede fallire**

```go
// The report table became width-aware in #66 and the budget screen in #136, but
// the header line above them never did: measured, it renders 68 columns on both
// a 40- and a 60-column terminal (the member note is what pushes it past the 54
// it measures without one). Line 1 is the title's own MarginBottom padding and
// overflows with it, which is why the text is truncated BEFORE th.Title.Render
// rather than after — truncating the rendered string would leave that line long.
//
// Measured: every other line of this view already fits at width 40 (summary 37,
// billable note 38, table 39, sparkline 15), so this test can assert on the
// whole view without depending on anything the spec left out of scope.
func TestReportViewHeaderFitsTheTerminal(t *testing.T) {
	t.Parallel()
	th := testTheme(true)
	rm := newReport(goldenReport(), " (2/5 members)", []float64{1, 0, 3, 2, 8})
	for _, width := range []int{40, 60, 80} {
		for i, line := range strings.Split(rm.view(th, width), "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Errorf("width %d: line %d rendered %d columns: %q", width, i, w, line)
			}
		}
	}
}

// Before the first WindowSizeMsg the width is 0 and nothing is sized against
// it: the header keeps its full text, because truncateWidth(s, 0) returns ""
// and would blank it.
func TestReportViewHeaderSurvivesZeroWidth(t *testing.T) {
	t.Parallel()
	th := testTheme(true)
	rm := newReport(goldenReport(), "", nil)
	if !strings.Contains(rm.view(th, 0), "Report ") {
		t.Error("the header vanished at width 0")
	}
}
```

- [ ] **Step 2: Vedere il RED**

Run: `go test ./internal/tui -run TestReportViewHeader -v`
Expected: il primo FAIL **quattro volte** — righe 0 e 1 a width 40, righe 0 e 1 a
width 60, ciascuna con **68** colonne. Il secondo PASS. Se i numeri non sono
questi, fermati e riporta.

- [ ] **Step 3: Implementare**

```go
	// Clamped before Render, not after: th.Title carries MarginBottom(1), which
	// emits a second line as wide as the content, so truncating the rendered
	// string would shorten the text and leave that padding line overlong.
	// width <= 0 is the first render, before the terminal has sent its size —
	// truncateWidth(s, 0) returns "" and would blank the header.
	head := fmt.Sprintf("Report %s — scope %s%s — grouped by %s — tz %s",
		report.PeriodLabel(r.Start, r.End), r.Scope, rm.note, r.GroupBy, r.Timezone)
	if width > 0 {
		head = truncateWidth(head, width)
	}
	title := th.Title.Render(head)
```

- [ ] **Step 4: Eseguire e rigenerare i golden**

```bash
go test ./internal/tui -run TestReportViewHeader -v
go test ./internal/tui -race
go test ./internal/tui -update
git diff internal/tui/testdata/
```
Expected: si muove **solo `palette_narrow.golden`** — è l'unico golden reso a una
larghezza minore del titolo. Nessun golden a 80 si muove. **Guardalo.** Se si
muove un golden a 80, il clamp scatta quando non dovrebbe.

- [ ] **Step 5: Gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui
git commit -m "fix(tui): clamp the report header to the terminal width (#143)"
```

---

### Task 3: inchiodare la colonna esposta come comportamento voluto (#143, prima metà)

**Nessuna modifica al codice di produzione.** La spec §3.3 chiude questa metà
della issue contro una misura: l'esposizione accanto al box è **una colonna** a
larghezza 40 e **zero** altrove, è il bordo destro della tabella — contenuto
legittimo dentro il terminale — e ritagliarla violerebbe il contratto di layering
della #59, rompendo tre test di `overlay_test.go`.

Quello che manca è un test che lo **dica**, così che chi vedrà quella colonna nel
golden trovi scritto perché c'è invece di ri-aprire la issue.

**Files:**
- Test: `internal/tui/overlay_test.go`

- [ ] **Step 1: Misurare, e confermare i numeri della spec**

Con una sonda usa-e-getta che poi cancelli, per ogni larghezza in
`{40, 50, 60, 80, 120}`, prendendo body, box, `x` e `y` dal percorso vero:

```go
m := goldenPaletteModel()
m.width = w
body := m.screenBody()
box, x, y := m.palette.layout(m.theme, m.width, m.height, strings.Count(body, "\n")+1)
```

misura **due** quantità distinte e riportale entrambe:

- la riga più larga di **tutto** il body, e quale indice ha;
- la riga più larga fra quelle che il box **copre davvero**, cioè con indice in
  `[y, y+len(boxLines))`, e quanto ne resta esposto a destra
  (`larghezza − (x + boxW)`, se positivo).

Expected, dalla spec §2.2: la riga più larga del body è sempre la **0** (il
titolo, 54 colonne) e sta **sopra** il box, che comincia a `y=2`; la riga coperta
più larga è sempre la **7** (la tabella, 39 colonne); l'esposizione accanto al box
è **1** a larghezza 40 e **0** a 50, 60, 80 e 120.

**Se i numeri non sono questi, fermati e riporta** invece di scrivere il test: la
spec ha già sbagliato una volta questa misura e la seconda versione va confermata,
non creduta.

- [ ] **Step 2: Il test che inchioda il comportamento**

Un test in `overlay_test.go` che asserisce che, a larghezza 40, la colonna del
body oltre il bordo destro del box **c'è** e vale esattamente una colonna, con un
commento che spiega: è il bordo destro della tabella, sta dentro il terminale, e
il contratto della #59 dice che il body fuori dal rettangolo del box sopravvive
verbatim. Nomina i tre test che cadrebbero se qualcuno decidesse di ritagliare.

Il test non ha un red-green — non corregge un difetto, fissa una scelta. La sua
prova è che fallisca se il comportamento cambia: **provalo** rendendo `composite`
un ritaglio (elimina il segmento destro), verifica che il tuo test nuovo
fallisca **e** che falliscano anche i tre test esistenti, poi ripristina. Metti
entrambi gli output nel report: sono la dimostrazione che il contratto è
protetto da più di un punto.

- [ ] **Step 3: Gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui
git commit -m "test(tui): pin the body column beside the overlay as intended (#143)"
```

---

### Task 4: CHANGELOG

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1** Sotto `## [Unreleased]` → `### Fixed`, una voce per issue,
  nello stile delle voci già presenti: comportamento osservato, non
  implementazione, e **niente garanzie assolute** — la tranche D ha dovuto
  correggere una voce che prometteva «the table now always fits the terminal
  width». Due voci:
  - il report non manda più a capo la propria intestazione su un terminale
    stretto (#143);
  - le cifre del burn-down budget si allineano fra le righe (#144).

  Non scrivere una voce per il Task 3: non cambia niente di osservabile.

- [ ] **Step 2** Gate completo, poi commit `docs: changelog for the layout follow-ups`.

---

## Self-review del piano

**Copertura:** spec §3.1 → Task 1; §3.2 → Task 2; §3.3 → Task 3; §4 (fuori
scope) non genera task.

**Dipendenze:** nessuna. La prima versione dichiarava `3` dipendente da `2`; la
misura corretta mostra che la riga coperta più larga è la tabella, quindi il
clamp del titolo non tocca il Task 3.

**Ogni test prescritto può passare contro il codice corretto**, che è la cosa che
la prima versione sbagliava due volte: la fixture del Task 1 ha `Billed` di pari
larghezza perché l'allineamento delle cifre è fuori scope, e l'asserzione sul caso
naturale guarda la **larghezza** e non un suffisso che l'implementazione emette
sempre.

**Ogni RED è osservabile:** Task 1 mette in staging il test che compila con la
firma attuale; Task 2 non introduce simboli nuovi; Task 3 non ha un RED per
costruzione, e lo dice, con la prova per mutazione al suo posto.

**Placeholder:** nessuno. Il Task 1 lascia una scelta con due vincoli espliciti
(i tre chiamanti di test, e `pctW = 0` a `width <= 0`); il Task 3 chiede una
conferma di misura prima di scrivere il test, e dice di fermarsi se la misura
smentisce la spec.
