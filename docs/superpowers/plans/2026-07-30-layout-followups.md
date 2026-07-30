# Follow-up di layout: piano di implementazione

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Chiudere #144 e #143, i due follow-up della tranche D.

**Architecture:** Tre interventi indipendenti, in un ordine che non è
arbitrario: il Task 2 cambia i numeri che il Task 3 deve misurare.

**Spec:** `docs/superpowers/specs/2026-07-30-layout-followups-design.md`

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
- **Le larghezze si misurano rendendo, non calcolando.** Usa
  `lipgloss.Width(strings.Split(<render>, "\n")[0])`, o il ciclo su tutte le
  righe quando quella più larga non è la prima. La tranche D ha prodotto due
  numeri sbagliati perché sommati a mano: `reportAmountWidth` riprende spazio ad
  Amount come ultima risorsa, e l'aritmetica a mano non lo vede.
- **Un test scritto contro un bug non è attendibile finché non lo si è visto
  fallire contro quel bug**, e non deve asserire sotto il minimo che il codice
  corretto può raggiungere: nella tranche D tre test asserivano l'impossibile.
- **Ogni numero scritto in un commento va misurato.** La tranche D ha corretto
  quattordici commenti falsi.

## Helper esistenti (verificati)

- `truncateWidth(s string, cols int) string` e `cell(s string, cols int) string`
  — `internal/tui/width.go`. `cols <= 0` ritorna `""` in entrambi.
- `testTheme(dark bool) theme` — `theme_test.go:15`.
- `goldenReport()`, `goldenPaletteModel()`, `golden(t, name, out)` — `golden_test.go`.
- `newReport(r report.Report, note string, series []float64) reportModel`.
- `reportModel.view(th theme, width int) string` — **già riceve la larghezza**
  (`report.go:198`), chiamata da `app.go:980`.
- `renderBudgetBar(th theme, percentUsed float64) string` — `budget.go:60`.
  Chiamanti: produzione `budget.go:149`, test `budget_test.go:43` e
  `budget_bar_test.go:57`.
- `budgetLayout(lines []report.BudgetLine, width int) (nameW int, showRemaining bool)`
  — `budget.go:116`. Misura `pctW` internamente e oggi non lo espone.

## Ordine dei task

`1` e `2` sono indipendenti. **`3` dipende da `2`**: il clamp
dell'intestazione cambia la riga più larga del body, e quindi l'esposizione che
il Task 3 deve misurare.

---

### Task 1: allineare la percentuale del budget (#144)

**Files:**
- Modify: `internal/tui/budget.go` (`renderBudgetBar`, `budgetLayout`, `view`)
- Modify: `internal/tui/budget_test.go:43`, `internal/tui/budget_bar_test.go:57`
  (il terzo argomento)
- Test: `internal/tui/budget_test.go`
- Test: `internal/tui/testdata/budget.golden`

**Interfaces:**
- Produces: `renderBudgetBar(th theme, percentUsed float64, pctW int) string`;
  `budgetLayout` espone anche `pctW`.

- [ ] **Step 1: Write the failing test**

```go
// budgetLayout measures the widest percentage label across the real rows and
// uses it to size the name column, but the rendering never padded to it: a row
// at 62% started its figures one column left of a row at 104% (#144). The
// fixture needs percentages of DIFFERENT label widths — with two rows both
// under 100% the columns line up by accident and the test passes against the bug.
func TestBudgetViewAlignsFiguresAcrossRows(t *testing.T) {
	t.Parallel()
	th := testTheme(true)
	bm := newBudget([]report.BudgetLine{
		{ListName: "Website", Billed: 625, Budget: 1000, Currency: "EUR", Remaining: 375, PercentUsed: 62.5},
		{ListName: "Mobile app", Billed: 1040, Budget: 1000, Currency: "EUR", Remaining: -40, PercentUsed: 104},
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
// the columns after it hold. pctW <= 0 keeps the natural width, which is what
// the callers that are not testing alignment pass.
func TestRenderBudgetBarRightAlignsThePercentage(t *testing.T) {
	t.Parallel()
	th := testTheme(true)
	wide := renderBudgetBar(th, 104, 4)
	narrow := renderBudgetBar(th, 62.5, 4)
	if lipgloss.Width(wide) != lipgloss.Width(narrow) {
		t.Errorf("bars are %d and %d columns wide, want equal", lipgloss.Width(wide), lipgloss.Width(narrow))
	}
	if !strings.HasSuffix(narrow, " 62%") {
		t.Errorf("narrow = %q, want it to end in a right-aligned \" 62%%\"", narrow)
	}
	natural := renderBudgetBar(th, 62.5, 0)
	if !strings.HasSuffix(natural, "62%") || strings.HasSuffix(natural, " 62%") {
		t.Errorf("natural = %q, want the unpadded \"62%%\" when pctW <= 0", natural)
	}
}
```

- [ ] **Step 2: See the RED**

I due test nominano `renderBudgetBar` con tre argomenti, quindi il package non
compila: `FAIL [build failed]` non è un RED. Per **vedere** il difetto, aggiungi
solo `TestBudgetViewAlignsFiguresAcrossRows` (che usa la firma attuale di `view`)
ed eseguilo.

Run: `go test ./internal/tui -run TestBudgetViewAlignsFiguresAcrossRows -v`
Expected: FAIL, con due colonne diverse. **Riporta i due numeri** nel report:
sono la misura del difetto.

- [ ] **Step 3: Implement**

`renderBudgetBar` prende `pctW` e allinea a destra:

```go
// pctW is the width the caller measured for the widest percentage label across
// all rows; the label is right-aligned into it so every row's figures start at
// the same column (#144). pctW <= 0 means natural width — the callers that are
// not testing alignment pass 0.
func renderBudgetBar(th theme, percentUsed float64, pctW int) string {
	...
	label := fmt.Sprintf("%.0f%%", math.Floor(percentUsed))
	if pctW > lipgloss.Width(label) {
		label = strings.Repeat(" ", pctW-lipgloss.Width(label)) + label
	}
	return fmt.Sprintf("%s %s", bar, label)
}
```

Non toccare `math.Floor`: il riempimento tronca, quindi la label deve
arrotondare per difetto o rivendicherebbe una soglia che la barra non ha
raggiunto (#136).

`budgetLayout` espone il `pctW` che già misura, invece di lasciarlo interno. La
firma diventa `(nameW int, pctW int, showRemaining bool)` — oppure estrai la
misura in un helper che sia `budgetLayout` sia `view` chiamano. Scegli tu, ma la
quantità che `view` passa a `renderBudgetBar` deve essere **la stessa** che
l'aritmetica di `budgetLayout` ha usato, non una ricalcolata: se divergono, la
riga sfora di quanto divergono. Se estrai un helper, verifica che entrambi lo
chiamino e non ne resti una copia.

`view` passa `pctW` alla `renderBudgetBar`. I due call site di test passano `0`.

- [ ] **Step 4: Run and regenerate the golden**

```bash
go test ./internal/tui -run 'TestBudget|TestRenderBudgetBar' -v
go test ./internal/tui -race
go test ./internal/tui -update
git diff internal/tui/testdata/
```
Expected: `budget.golden` ha una riga sola con `62%`, quindi `pctW` è 3 e non
cambia niente — **verificalo**, non assumerlo. Se cambia, guarda cosa è
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
- Test: i golden stretti che mostrano l'intestazione

**Interfaces:**
- Consumes: `truncateWidth` (`width.go`).

- [ ] **Step 1: Write the failing test**

```go
// The report table became width-aware in #66 and the budget screen in #136, but
// the header line above them never did: measured, it renders 68 columns on both
// a 40- and a 60-column terminal (the member note is what pushes it past the 54
// it measures without one). Line 1 is the title's own MarginBottom padding and
// overflows with it, which is why the text is truncated BEFORE th.Title.Render
// rather than after — truncating the rendered string would leave that line long.
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

- [ ] **Step 2: See the RED**

Run: `go test ./internal/tui -run TestReportViewHeader -v`
Expected: il primo FAIL su width 40 e 60, riportando **68** colonne per la riga
0 e per la riga 1 — quattro errori in tutto. Il secondo PASS (oggi il titolo non
è troncato, quindi non può sparire). **Allega il transcript**: se i numeri non
sono 68, la fixture non è quella misurata dalla spec.

- [ ] **Step 3: Implement**

In `reportModel.view`, prima del `Render`:

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

- [ ] **Step 4: Run and regenerate the goldens**

```bash
go test ./internal/tui -run TestReportViewHeader -v
go test ./internal/tui -race
go test ./internal/tui -update
git diff internal/tui/testdata/
```
Expected: si muovono i golden resi a una larghezza minore del titolo —
`palette_narrow` fra questi. Quelli a 80 non si muovono (68 ≤ 80). **Guarda ogni
golden cambiato** e riporta quali e come. Se un golden a 80 si muove, fermati:
significa che il clamp scatta quando non dovrebbe.

- [ ] **Step 5: Gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui
git commit -m "fix(tui): clamp the report header to the terminal width (#143)"
```

---

### Task 3: rimisurare l'esposizione accanto al box, e decidere (#143, prima metà)

> **Dipende dal Task 2.** Il clamp dell'intestazione cambia la riga più larga
> del body, che è la quantità da misurare qui. Non eseguirlo prima.

**Files:**
- Modify: `internal/tui/overlay.go` (`composite`) — **solo se la misura lo dice**
- Test: `internal/tui/overlay_test.go`
- Test: `internal/tui/testdata/palette_narrow.golden`

- [ ] **Step 1: Misurare, prima di scrivere codice**

Con una sonda usa-e-getta che poi cancelli, misura per ogni larghezza in
`{40, 50, 60, 80, 120}`: la riga più larga del body, la larghezza del box, il suo
`x`, e quante colonne del body restano esposte a destra del box
(`widestBodyLine - (x + boxW)`, se positivo). Prendi body, box, `x` e `y` dal
percorso vero:

```go
m := goldenPaletteModel()
m.width = w
body := m.screenBody()
box, x, y := m.palette.layout(m.theme, m.width, m.height, strings.Count(body, "\n")+1)
```

Prima del Task 2 la misura era: 16 colonne esposte a larghezza 40, zero a ogni
larghezza da 60 in su.

**Riporta la tabella nel report, e poi segui la misura:**

- **Se l'esposizione massima è ≤ 1 colonna** a ogni larghezza: implementa il
  ritaglio (Step 2). È il caso atteso.
- **Se è > 1 colonna** a qualche larghezza: **fermati**, riporta la tabella e
  quale contenuto è esposto, e non implementare niente. Vuol dire che il Task 2
  non ha coperto tutta la causa e la decisione torna all'umano.

- [ ] **Step 2: Il ritaglio, solo se la misura lo autorizza**

Il segmento destro di `composite` non viene disegnato quando ciò che esporrebbe
è contenuto che sfonda comunque il terminale. Attenzione: `composite` non conosce
la larghezza del terminale, conosce solo body e box. Il ritaglio quindi non può
essere «oltre il terminale», deve essere «oltre il bordo destro del box» — che è
la stessa cosa solo quando il body non è più largo del terminale, cioè dopo il
Task 2.

Scrivi il perché nel commento, e scrivi **anche** il caso in cui questa scelta
sarebbe sbagliata: se un giorno il body tornasse a essere più largo del
terminale, il ritaglio nasconderebbe contenuto invece di detriti.

Il test deve inchiodare che il fondo esposto **non** compare, con una fixture in
cui prima compariva. Provalo per mutazione: rimetti il segmento destro e verifica
che il test fallisca.

- [ ] **Step 3: Golden e gate**

```bash
go test ./internal/tui -race
go test ./internal/tui -update
git diff internal/tui/testdata/palette_narrow.golden
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
```
**Guarda il golden.** Deve mostrare il box senza i caratteri di cornice spuri a
destra.

- [ ] **Step 4: Commit**

```bash
git add internal/tui
git commit -m "fix(tui): stop drawing the background past the overlay's right edge (#143)"
```

---

### Task 4: CHANGELOG

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1** Sotto `## [Unreleased]`, sezione `### Fixed`, una voce per issue
  chiusa, nello stile delle voci già presenti: comportamento osservato, non
  implementazione, e **niente garanzie assolute** — la tranche D ne ha dovuta
  correggere una che prometteva «the table now always fits the terminal width».
  Se il Task 3 si è fermato senza implementare, la voce sulla #143 copre solo il
  clamp dell'intestazione e la issue resta aperta.

- [ ] **Step 2** Gate completo, poi commit `docs: changelog for the layout follow-ups`.

---

## Self-review del piano

**Copertura:** spec §3.1 → Task 1; §3.2 → Task 2; §3.3 → Task 3; §5 è vincolo
globale; §4 (fuori scope) non genera task.

**Dipendenze:** `3` dipende da `2` ed è dichiarato in testa al task, non solo
qui. `1` è indipendente da entrambi.

**Il RED di ogni task è osservabile:** il Task 1 mette in staging il test che
compila contro la firma attuale; il Task 2 non ha simboli nuovi, quindi il RED è
diretto; il Task 3 non ha un RED perché è una misura che decide se c'è un
difetto da correggere — e il suo test nasce dopo la misura, con la prova per
mutazione al posto del red-green.

**Le larghezze asserite** vengono tutte da §2 della spec, misurata rendendo. Il
Task 2 asserisce a 40, 60 e 80: le prime due sono le larghezze in cui oggi sfora,
la terza è il controllo che il clamp non scatti quando non serve.

**Placeholder:** nessuno. Il Task 1 lascia una scelta esplicita
all'implementatore (esporre `pctW` da `budgetLayout` o estrarre un helper) con il
vincolo che la decide: la quantità passata a `renderBudgetBar` deve essere la
stessa usata dall'aritmetica. Il Task 3 è deliberatamente condizionale, e la
condizione è un numero misurato, non un giudizio.
