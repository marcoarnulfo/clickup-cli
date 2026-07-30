# Tranche D — layout & correttezza: piano di implementazione

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Far diventare vere le invarianti di layout che i commenti già
dichiarano, chiudendo #137, #136, #135, #141, #138 (punto 1) e le caselle aperte
della #28.

**Architecture:** Una primitiva sola (`ansi.Truncate`) sotto due helper nuovi in
`internal/tui/width.go`; da lì scendono la migrazione dei call site, il fix di
`clampWidth`, la larghezza misurata delle colonne numeriche e la view budget
width-aware. Poi due interventi indipendenti sulla #28: il range pinnato con
trasporto nel messaggio, e lo scroll dei Filters su una finestra condivisa col
palette.

**Tech Stack:** Go 1.26, bubbletea v1.3.10, bubbles v1.0.0, lipgloss v1.1.0,
`charmbracelet/x/ansi` v0.11.6 (già dipendenza diretta).

**Spec:** `docs/superpowers/specs/2026-07-30-tranche-d-layout-correctness-design.md`

## Global Constraints

- `internal/report` e `internal/duration` restano **puri**: nessun I/O, nessun
  import di `internal/config`, `internal/clickup`, `internal/tui`.
- Nessuna funzione di stile in produzione chiama `lipgloss.NewStyle()`: gli
  stili vengono dal renderer iniettato del `theme`.
- Mai chiamare l'API ClickUp reale. Il comportamento di rete si esercita solo
  con `httptest` e il base URL del client puntato lì (segui il pattern dei test
  già presenti in `internal/clickup`).
- Golden rigenerati con `go test ./internal/tui -update`, **mai** a mano.
- Gate prima di ogni commit, tutti e cinque puliti: `gofmt -l .`,
  `go vet ./...`, `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`,
  `go build ./...`, `go test ./... -race`.
- Tutto ciò che vive nel repo è in **inglese**: codice, identificatori,
  commenti, stringhe UI, nomi e messaggi dei test, messaggi di commit.
- Conventional Commits. **MAI** `Co-Authored-By`.
- **Un test scritto contro un bug specifico non è attendibile finché non lo si è
  visto fallire contro quel bug.** Ogni task che chiude un difetto allega il
  transcript del RED. Unica eccezione, dichiarata nella spec: il Task 3 (#141),
  behaviorally inert e invisibile alla suite per costruzione.
- **Un RED va visto, non dedotto.** Se un test nuovo non compila perché nomina
  una funzione che ancora non esiste, l'intero package non compila e nessun
  numero comportamentale appare: `FAIL [build failed]` non è un RED. Dove serve,
  gli step qui sotto **separano** i test comportamentali (che girano contro il
  codice attuale) da quelli che nominano simboli nuovi.
- **Le larghezze si misurano rendendo, non calcolando.** Sommare a mano le
  colonne che si crede la tabella userà ha già prodotto due numeri sbagliati in
  questa tranche: `reportAmountWidth` riprende spazio ad Amount come ultima
  risorsa, e `lipgloss/table` dimensiona ogni colonna anche sull'header. Misura
  con `lipgloss.Width(strings.Split(reportTable(...), "\n")[0])`.
- Fixture obbligatorie: ogni test di larghezza nasce con **rune larghe** o con
  la **congiunzione** che il difetto richiede. Un test a label corte e cifre
  normali passa contro metà dei bug di questa tranche.

## Helper esistenti (verificati — usali con queste firme esatte)

- `newTestModel() Model` — `log_test.go:31`. **Nessun parametro.**
- `newTestModelOnReport() Model` — `log_test.go:36`.
- `testTheme(dark bool) theme` — `theme_test.go:15`.
- `goldenEntries()`, `goldenReport()`, `goldenModel()` — `golden_test.go`.
  `goldenReport()` è `GroupByList`; `goldenModel()` fissa `loc = time.UTC`.
- `golden(t, name, out)` — `golden_test.go`.
- Il campo del Model è **`m.filtersScreen`** (`app.go:179`), non `m.filters`.
- `newFilters(entries []report.TimeEntry, lists, tags, statuses map[string]bool, billable *bool) filtersModel`
  — `filters.go:70`. **Le opzioni delle sezioni Lists/Tags/Statuses derivano
  dalle `entries`**, non dalle mappe: le mappe sono solo lo stato *selected*.
  Con `entries == nil` tutte le sezioni tranne Billable hanno zero opzioni.

## Dipendenze fra i task

`1 → 2 → {3, 4, 5, 6}`, e `7 → 9`. Il Task 8 è indipendente.

Il Task 9 **dipende dal Task 7**: estrae `rebuildReport` dagli stessi tre
blocchi di rebuild (`report.go:83`, `app.go:719`, `rates.go:855`) in cui il Task
7 sostituisce `currentRange()` con `activeRange()` e in cui l'handler passa a
`msg.start`/`msg.end`. Eseguito prima, il Task 9 farebbe puntare la lista di
siti del Task 7 a righe che non esistono più.

Task 4 prima di Task 5: entrambi toccano `report_table.go`, e il Task 4 crea
golden nuovi che il Task 5 poi rigenera.

---

### Task 1: `width.go` — le due primitive

**Files:**
- Create: `internal/tui/width.go`
- Create: `internal/tui/width_test.go`

**Interfaces:**
- Produces: `truncateWidth(s string, cols int) string`, `cell(s string, cols int) string`.
  Ogni task successivo usa questi due e nient'altro per far stare una stringa
  in una colonna.

- [ ] **Step 1: Write the failing test**

In `internal/tui/width_test.go`:

```go
package tui

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// Display width and rune count agree only for ASCII, which is why every fixture
// here that matters is not ASCII.
func TestTruncateWidth(t *testing.T) {
	tests := []struct {
		name string
		s    string
		cols int
		want string
	}{
		{"empty", "", 5, ""},
		{"fits", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"cuts ascii", "hello world", 5, "hell…"},
		{"zero cols", "hello", 0, ""},
		{"negative cols", "hello", -3, ""},
		{"one col fits", "x", 1, "x"},
		{"one col cuts", "xy", 1, "…"},
		// A wide cluster is dropped whole rather than split, so the result can
		// land NARROWER than cols. That is what makes the pad in cell() load-bearing.
		{"wide runes cut on the boundary", "日本語のリスト", 2, "…"},
		{"wide runes cut inside", "日本語のリスト", 5, "日本…"},
		{"emoji", "🚀🚀🚀🚀", 3, "🚀…"},
		{"mixed", "ab🚀cd", 5, "ab🚀…"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateWidth(tc.s, tc.cols)
			if got != tc.want {
				t.Errorf("truncateWidth(%q, %d) = %q, want %q", tc.s, tc.cols, got, tc.want)
			}
			if w := lipgloss.Width(got); tc.cols > 0 && w > tc.cols {
				t.Errorf("truncateWidth(%q, %d) rendered %d columns, want at most %d", tc.s, tc.cols, w, tc.cols)
			}
		})
	}
}

// cell must produce EXACTLY cols columns, which is the half that fmt's "%-Ns"
// gets wrong: fmt pads by rune count, so a 7-rune 14-column string padded to
// "%-10s" comes out 17 columns wide.
func TestCellIsExactlyColsWide(t *testing.T) {
	for _, s := range []string{"", "x", "hello", "hello world", "日本語のリスト", "🚀🚀🚀🚀", "ab🚀cd"} {
		for _, cols := range []int{1, 2, 5, 10, 24} {
			got := cell(s, cols)
			if w := lipgloss.Width(got); w != cols {
				t.Errorf("cell(%q, %d) rendered %d columns, want exactly %d (%q)", s, cols, w, cols, got)
			}
		}
	}
}

func TestCellNonPositiveCols(t *testing.T) {
	for _, cols := range []int{0, -1, -12} {
		if got := cell("日本語", cols); got != "" {
			t.Errorf("cell(%q, %d) = %q, want %q", "日本語", cols, got, "")
		}
	}
}

// The migration in Task 2 must be a no-op for ASCII content at cols >= 2: that
// is what keeps every existing golden green, so the wide-rune tests are the only
// thing that distinguishes before from after. cols == 1 is the one documented
// divergence — the old rune cut returned "…" even when the input fit.
func TestCellMatchesTheOldPaddedCutForASCII(t *testing.T) {
	for _, s := range []string{"", "x", "Website", "Website redesign", "Mobile app"} {
		for _, cols := range []int{2, 5, 20, 24, 40} {
			want := fmt.Sprintf("%-*s", cols, oldRuneCutForTest(s, cols))
			if got := cell(s, cols); got != want {
				t.Errorf("cell(%q, %d) = %q, want %q (the pre-migration rendering)", s, cols, got, want)
			}
		}
	}
}

// oldRuneCutForTest is the rune-based cut this tranche deletes, kept in the test
// file only, as the reference the ASCII equivalence is measured against.
func oldRuneCutForTest(s string, n int) string {
	if s == "" {
		return ""
	}
	if n <= 1 {
		return "…"
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// The report footer cuts styled strings, so the cut must survive escapes.
// TestMain pins termenv.Ascii for this package, so a theme style renders without
// escapes: the fixture writes them by hand instead.
func TestTruncateWidthIsANSIAware(t *testing.T) {
	const styled = "\x1b[1mabcdef\x1b[0m"
	got := truncateWidth(styled, 3)
	if want := "\x1b[1mab…\x1b[0m"; got != want {
		t.Errorf("truncateWidth(%q, 3) = %q, want %q", styled, got, want)
	}
	if w := lipgloss.Width(got); w != 3 {
		t.Errorf("truncated styled string rendered %d columns, want 3", w)
	}
}

// A style left OPEN by the caller is not closed by the cut: the escape survives
// and bleeds past it. Nothing in production feeds an unterminated style today
// (help.View closes its own), and this test exists so that a caller who starts
// doing it finds out here rather than on a user's terminal.
func TestTruncateWidthDoesNotCloseAnOpenStyle(t *testing.T) {
	got := truncateWidth("\x1b[31mabcdef", 3)
	if want := "\x1b[31mab…"; got != want {
		t.Errorf("truncateWidth of an unterminated style = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/tui -run 'TestTruncateWidth|TestCell' -v`
Expected: `FAIL [build failed]` — `undefined: truncateWidth`, `undefined: cell`.
Qui il build-failure **è** il RED corretto: le funzioni non esistono ancora e non
c'è alcun difetto da esercitare.

- [ ] **Step 3: Write the implementation**

`internal/tui/width.go`:

```go
package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// This file is the only sanctioned way to fit a string into a column.
//
// Every layout in this package is measured in DISPLAY COLUMNS, but Go counts
// runes in two places that used to be relied on: a rune-sliced cut, and fmt's
// "%-Ns" padding (fmt's width is a rune count, so a 7-rune 14-column string
// padded to 10 comes out 17 columns wide). The two agree only for ASCII, and
// ClickUp list and task names are exactly where emoji and CJK live.
//
// Both helpers take a SINGLE LINE. ansi.Truncate lets a "\n" through, so a
// multi-line input would produce a "cell" whose padding applies to the last line
// only; no call site passes one.

// truncateWidth cuts s to at most cols display columns, the ellipsis included.
// cols <= 0 returns "".
func truncateWidth(s string, cols int) string {
	if cols <= 0 {
		return ""
	}
	return ansi.Truncate(s, cols, "…")
}

// cell renders s into exactly cols display columns: truncated when too wide,
// space-padded when too narrow. It replaces the truncate(s, N) + "%-Ns" pair,
// where both halves counted runes.
//
// The pad is not cosmetic. ansi.Truncate drops a wide cluster whole rather than
// splitting it, so a cut can land NARROWER than cols ("日本語のリスト" cut to 2
// gives "…", one column). Without the measured pad the column misaligns anyway,
// just in the other direction.
func cell(s string, cols int) string {
	s = truncateWidth(s, cols)
	if pad := cols - lipgloss.Width(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/tui -run 'TestTruncateWidth|TestCell' -v`
Expected: PASS, tutti i sottotest.

- [ ] **Step 5: Full gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui/width.go internal/tui/width_test.go
git commit -m "feat(tui): add display-width truncate and cell helpers (#135)"
```

---

### Task 2: migrare i call site, eliminare `truncate` e `shaveToWidth`

**Files:**
- Modify: `internal/tui/report.go` (elimina `truncate`, righe 270-282; aggiorna il commento a riga 269 che cita `shaveToWidth`)
- Modify: `internal/tui/report_table.go` (elimina `shaveToWidth`, righe 224-259; migra i 2 usi a 209-210; aggiorna i commenti a 142 e 195 che citano `truncate`)
- Modify: `internal/tui/palette.go` (**3 usi di `shaveToWidth`**: 214, 219, 253)
- Modify: `internal/tui/budget.go:75`
- Modify: `internal/tui/rates_view.go:53, 57, 76, 82, 92, 93, 143, 147, 149`
- Modify: `internal/tui/entries.go:583, 593, 643, 648`
- Modify: `internal/tui/log.go:540`
- Test: `internal/tui/width_test.go` (aggiunte)

**Interfaces:**
- Consumes: `truncateWidth`, `cell` (Task 1).
- Produces: `truncate` e `shaveToWidth` **non esistono più**.

**Attenzione, è il punto in cui questo task si rompe se letto in fretta:**
`shaveToWidth` ha **5 chiamanti**, non 2. Tre sono in `palette.go` e non hanno
niente a che vedere con la tabella report. Cancellare la funzione senza migrarli
non compila. Verifica prima di iniziare:

```bash
grep -rn "shaveToWidth\|truncate(" internal/tui/*.go | grep -v _test
```

**Regola meccanica:** coppia `truncate(s,N)` + verbo `%-Ns` → `cell(s, N)` con
`%s`; sito senza pad → `truncateWidth(s, N)`. `shaveToWidth(s, w)` →
`truncateWidth(s, w)` in tutti e cinque i siti: la shave-loop faceva a mano, un
rune per giro, ciò che `ansi.Truncate` fa in un passo.

- [ ] **Step 1: See the defect fail, before touching any call site**

Il test sull'helper non è il RED di questo task: `cell` è già corretta dal Task
1. Il RED è sulla **coppia vecchia**. Aggiungi questo test temporaneo, eseguilo,
allega il transcript, poi cancellalo:

```go
func TestTemporaryProofTheOldPairMisaligns(t *testing.T) {
	const wide = "日本語のリストの名前がとても長い場合"
	old := fmt.Sprintf("%-24s", oldRuneCutForTest(wide, 24))
	t.Logf("the old cut+pad rendered %d columns, want 24", lipgloss.Width(old))
	if lipgloss.Width(old) == 24 {
		t.Fatal("the old pair did NOT misalign — the fixture is wrong, fix the fixture")
	}
}
```

Run: `go test ./internal/tui -run TestTemporaryProofTheOldPairMisaligns -v`
Expected: PASS con un numero **diverso da 24** nel log (misurato: 42). Allega il
log, poi rimuovi il test.

- [ ] **Step 2: Add the permanent regression test**

```go
// The rates list, the entries browser and the budget screen all pair a cut with
// a fixed-width column. With a wide-rune list name the row used to render wider
// than its own column, which is what pushed those screens past the terminal
// edge. The fixture is deliberately CJK: an ASCII name passes against the bug.
func TestFixedWidthRowsHoldTheirColumnWithWideRunes(t *testing.T) {
	const wide = "日本語のリストの名前がとても長い場合"
	if lipgloss.Width(wide) == len([]rune(wide)) {
		t.Fatalf("fixture is not wide: %d runes, %d columns", len([]rune(wide)), lipgloss.Width(wide))
	}
	for _, cols := range []int{20, 24, 30} {
		if w := lipgloss.Width(cell(wide, cols)); w != cols {
			t.Errorf("cell(wide, %d) rendered %d columns, want %d", cols, w, cols)
		}
	}
}
```

- [ ] **Step 3: Migrate the sites**

`internal/tui/budget.go:74-76` — swap meccanico (il Task 6 riscriverà la riga):

```go
		rows.WriteString(fmt.Sprintf("%s %s  %.2f / %.2f %s (remaining %.2f)\n",
			cell(l.ListName, 24), renderBudgetBar(th, l.PercentUsed), l.Billed, l.Budget, l.Currency, l.Remaining))
```

`internal/tui/rates_view.go`:

```go
// :52-53
		line := fmt.Sprintf("%s %10.2f %-5s %10s  %s",
			cell(r.name, 24), rate, rt.effectiveCurrency(r.listID), moneyOrDash(bud, hasBud), tag)
// :57 — a sentence, not a column
	note := fmt.Sprintf("Effective for %s: %.2f %s", truncateWidth(sel.name, 24), rt.rateFor(sel.listID), rt.effectiveCurrency(sel.listID))
// :76
		line := fmt.Sprintf("%s %10.2f  %s", cell(fmt.Sprintf("%s (%d)", mr.name, mr.id), 30), rate, tag)
// :82 — a sentence
		note = fmt.Sprintf("%s is overridden on %d list(s) by a (list,member) rate.", truncateWidth(sel.name, 24), n)
// :91-94
		line := fmt.Sprintf("%s %s %10.2f  %.2f (%s)",
			cell(rt.listName(o.listID), 20),
			cell(fmt.Sprintf("%s (%d)", rt.memberName(o.member), o.member), 22),
			o.rate, below, src)
// :143 — no padding, the row is the whole line
			b += billingRow(th, i == rt.draft.idx, truncateWidth(r.name, 40))
// :147 — a sentence
	b := th.Help.Render(fmt.Sprintf("New override on %s — choose the member:", truncateWidth(rt.listName(rt.draft.listID), 24))) + "\n"
// :149 — no padding
		b += billingRow(th, i == rt.draft.idx, truncateWidth(fmt.Sprintf("%s (%d)", mr.name, mr.id), 40))
```

Le **tre** righe di header letterali di questo file — `rates_view.go:42`
(`%-24s`), `:66` (`%-30s`), `:87` (`%-20s %-22s`) — restano con i verbi `%-Ns`,
e lo stesso vale per il `%-5s` sulla valuta a riga 52. Aggiungi a ciascuna delle
tre righe di header un commento di una riga che dica perché: sono letterali
ASCII (e codici ISO), dove rune e colonne coincidono per costruzione, e restano
allineate con le righe dati perché `cell(...,N)` produce esattamente N colonne.

`internal/tui/entries.go`:

```go
// :583 — a sentence
			"Delete " + th.Accent.Render(truncateWidth(e.TaskName, 40)) + " (" +
// :593 — a title
		b := th.Title.Render("Tags") + "  " + th.Accent.Render(truncateWidth(tagPickerTaskName(es), 40)) + "\n\n"
// :642-643
		line := fmt.Sprintf("%s  %s %6s %s  %s",
			when, cell(e.TaskName, 24), duration.FormatHours(e.Duration), bill, owner)
// :648 — appended, no column
			line += "  " + truncateWidth(tagBadges(e.EntryTags), 20)
```

`internal/tui/log.go:540`:

```go
			line := truncateWidth(tk.Name, 40)
```

`internal/tui/palette.go` — i tre siti, sostituzione diretta:

```go
// :214
	q := truncateWidth("> "+p.query, innerW)
// :219
		msg := truncateWidth("no matching action", innerW)
// :253
	label := truncateWidth(it.a.label, max(labelW, 1))
```

`internal/tui/report_table.go:209-210` — sostituzione diretta, poi elimina
`shaveToWidth` (224-259):

```go
		rows[i][0] = truncateWidth(rows[i][0], itemW)
		rows[i][3] = truncateWidth(rows[i][3], amountW)
```

Elimina `truncate` (`report.go:270-282`) e aggiorna i tre commenti che nominano
funzioni che non esistono più: `report.go:269` (cita `shaveToWidth`),
`report_table.go:142` («truncate's own guard (n <= 1) is what makes that safe» —
la guardia nuova è `cols <= 0 → ""`, che è **diversa**, quindi il commento va
riscritto, non solo rinominato) e `report_table.go:195` («letting truncate cut
it»).

- [ ] **Step 4: Run the full suite and regenerate nothing**

Run: `go test ./internal/tui -race`
Expected: **PASS senza rigenerare alcun golden**, compresi i golden `palette_*`.
Se un golden si muove, non rigenerarlo: significa che un sito è stato migrato con
la larghezza sbagliata, o che un `%-Ns` è rimasto accoppiato a un `cell`. Riporta
nel report ogni golden che si è mosso e perché.

- [ ] **Step 5: Full gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add -A internal/tui
git commit -m "refactor(tui): truncate and pad by display width, not rune count (#135)"
```

---

### Task 3: `clampWidth` senza il renderer di default (#141)

**Files:**
- Modify: `internal/tui/footer.go:56-63`
- Test: `internal/tui/footer_test.go`

**Interfaces:**
- Consumes: `truncateWidth` (Task 1).

**Nota di processo:** è l'unico task della tranche **senza** red-green, e la spec
lo dichiara. Il difetto è behaviorally inert: `MaxWidth` e `ansi.Truncate`
tagliano in modo indipendente dal profilo colore, e `TestMain` fissa
`termenv.Ascii`, quindi nessun test comportamentale può fallire contro di esso.
La prova è il grep più il test anti-bleed. Non inventare un red-green: se ti
sembra di averne trovato uno, stai testando altro.

- [ ] **Step 1: Write the tests, and run them against the CURRENT code first**

```go
// clampWidth must not build a style on lipgloss's default renderer: that is the
// one thing the injected-renderer discipline exists to prevent, and this file's
// own opening comment says so about help.New().
//
// This is invisible to the suite by construction: TestMain pins the default
// renderer to termenv.Ascii, so a style built on it renders identically to one
// built on the injected renderer, and no golden or assertion can tell them
// apart. Do not add a test that claims otherwise — pin the truncation instead,
// and keep the grep in the checklist.
func TestClampWidthNeverExceedsTheTerminal(t *testing.T) {
	th := testTheme(true)
	long := "↑/↓ move · enter run · esc close · ctrl+c force quit · ? help · g group · e export"
	for _, width := range []int{20, 40, 66, 80} {
		got := clampWidth(th, long, width)
		if w := lipgloss.Width(got); w > width {
			t.Errorf("clampWidth(width=%d) rendered %d columns", width, w)
		}
		if !strings.HasSuffix(got, "…") {
			t.Errorf("clampWidth(width=%d) = %q, want a visible ellipsis", width, got)
		}
	}
}

// The footer's input comes from help.View, which closes its own styles. If a
// caller ever passes an unterminated one, ansi.Truncate will NOT close it and
// the style bleeds past the cut — this pins the boundary of what clampWidth
// promises, so the discovery happens here and not on a user's terminal.
func TestClampWidthDoesNotCloseAnOpenStyle(t *testing.T) {
	th := testTheme(true)
	got := clampWidth(th, "\x1b[31m"+strings.Repeat("x", 40), 10)
	if !strings.Contains(got, "\x1b[31m") {
		t.Fatalf("expected the caller's escape to survive, got %q", got)
	}
	if strings.Count(got, "\x1b[0m") != 0 {
		t.Errorf("clampWidth closed a style it did not open: %q", got)
	}
}
```

Run: `go test ./internal/tui -run TestClampWidth -v`
Expected: il primo PASS (il comportamento non cambia con questo task). Il secondo
documenta cosa fa `MaxWidth` **oggi** con uno stile aperto. **Riporta l'esito di
entrambi nel report prima di cambiare il codice**, così la differenza
prima/dopo è registrata invece di essere assunta.

- [ ] **Step 2: Replace the implementation**

```go
	// ansi.Truncate cuts ANSI-aware without a renderer: MaxWidth would do the
	// same cut, but only by building a style on lipgloss's DEFAULT renderer —
	// the one this file's opening comment refuses help.New() over. The ellipsis
	// is added separately, styled from the theme, so the cut is visible rather
	// than looking like a footer that simply ends.
	//
	// This is invisible to the tests by construction: TestMain pins the default
	// renderer to termenv.Ascii, so both spellings render identically and no
	// golden can tell them apart. The guard is the grep, not the suite.
	return ansi.Truncate(s, width-1, "") + th.Help.Render("…")
```

- [ ] **Step 3: Run the tests, then the grep that is this task's real proof**

```bash
go test ./internal/tui -run TestClampWidth -v
go test ./internal/tui -race
grep -rn "lipgloss.NewStyle()" internal/ --include="*.go" | grep -v "_test.go"
```
Expected: test PASS, golden fermi; il grep restituisce **solo** il commento a
`report_table.go:163`, nessuna occorrenza di codice. Allega l'output del grep: è
la chiusura della seconda casella della #141.

- [ ] **Step 4: Full gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui/footer.go internal/tui/footer_test.go
git commit -m "refactor(tui): cut the footer with ansi.Truncate, not a default-renderer style (#141)"
```

---

### Task 4: la riga `Total` duplicata (#137)

**Files:**
- Modify: `internal/tui/report_table.go` (`reportRows`, righe 36-73)
- Test: `internal/tui/report_table_test.go`
- Test: `internal/tui/golden_test.go` (**due golden nuovi**)
- Test: `internal/tui/testdata/report_total.golden`, `report_total_multicurrency.golden` (nuovi)

**Interfaces:**
- Produces: `reportRows` non emette più la riga bucket sotto `GroupByTotal` con
  un bucket unico; `firstTotal` in quel caso è 0.

**Nota sui golden:** **nessun golden esistente si muove.** Tutti i golden del
report girano su `goldenReport()`, che è `GroupByList`, e in `testdata/` non
esiste alcuna riga bucket `Total` (verificalo: `grep -rln "^│ Total" internal/tui/testdata/`
non trova niente). I due golden di questa forma vanno **creati**.

- [ ] **Step 1: Write the failing test**

```go
// Under GroupByTotal — the grouping the report loads with — the single bucket IS
// the totals row: same hours, same billed, and the same amounts, because one
// bucket collects every entry. The table used to show "Total" directly above
// "TOTAL", differing only in case and color (#137).
func TestReportRowsSuppressesTheBucketUnderTotalGrouping(t *testing.T) {
	r := report.Report{
		GroupBy:         report.GroupByTotal,
		DefaultCurrency: "EUR",
		Buckets: []report.Bucket{{Label: "Total", Hours: 15.5, BilledHours: 12.5,
			Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 625}}}},
		CurrencySubtotals: []report.CurrencySubtotal{{Currency: "EUR", Hours: 15.5, BilledHours: 12.5, Amount: 625}},
		TotalHours:        15.5, BilledHours: 12.5, TotalAmount: 625,
	}
	rows, firstTotal := reportRows(r)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1 (the TOTAL row alone): %v", len(rows), rows)
	}
	if firstTotal != 0 {
		t.Errorf("firstTotal = %d, want 0 (the TOTAL row is the first row)", firstTotal)
	}
	if rows[0][0] != "TOTAL" {
		t.Errorf("rows[0][0] = %q, want %q", rows[0][0], "TOTAL")
	}
}

// Multi-currency keeps its shape: TOTAL with an empty Amount, then one subtotal
// row per currency. The suppressed bucket carried "A + B", which is the same
// information those subtotal rows carry.
func TestReportRowsSuppressesTheBucketUnderTotalGroupingMultiCurrency(t *testing.T) {
	r := report.Report{
		GroupBy:         report.GroupByTotal,
		DefaultCurrency: "EUR",
		Buckets: []report.Bucket{{Label: "Total", Hours: 18, BilledHours: 17,
			Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 337.5}, {Currency: "USD", Amount: 512.5}}}},
		CurrencySubtotals: []report.CurrencySubtotal{
			{Currency: "EUR", Hours: 7.75, BilledHours: 6.75, Amount: 337.5},
			{Currency: "USD", Hours: 10.25, BilledHours: 10.25, Amount: 512.5},
		},
		TotalHours: 18, BilledHours: 17,
	}
	rows, firstTotal := reportRows(r)
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3 (TOTAL + two subtotals): %v", len(rows), rows)
	}
	if firstTotal != 0 {
		t.Errorf("firstTotal = %d, want 0", firstTotal)
	}
	if rows[0][3] != "" {
		t.Errorf("TOTAL Amount = %q, want empty (the subtotals carry the figures)", rows[0][3])
	}
}

// Every other grouping keeps its bucket rows: the suppression is about the
// degenerate one-bucket case of GroupByTotal, not about totals in general.
func TestReportRowsKeepsBucketsUnderOtherGroupings(t *testing.T) {
	r := report.Report{
		GroupBy:           report.GroupByList,
		DefaultCurrency:   "EUR",
		Buckets:           []report.Bucket{{Label: "Website", Hours: 12.5, BilledHours: 12.5}},
		CurrencySubtotals: []report.CurrencySubtotal{{Currency: "EUR", Hours: 12.5, BilledHours: 12.5, Amount: 625}},
		TotalHours:        12.5, BilledHours: 12.5, TotalAmount: 625,
	}
	rows, firstTotal := reportRows(r)
	if len(rows) != 2 || firstTotal != 1 {
		t.Fatalf("got %d rows firstTotal=%d, want 2 rows firstTotal=1: %v", len(rows), firstTotal, rows)
	}
}
```

- [ ] **Step 2: Run to verify the first two fail**

Run: `go test ./internal/tui -run TestReportRows -v`
Expected: i primi due FAIL con `got 2 rows, want 1` e `got 4 rows, want 3`; il
terzo PASS. Tutti e tre compilano contro il codice attuale, quindi questo è un
RED comportamentale vero. **Allega il transcript.**

- [ ] **Step 3: Implement**

In `reportRows`, sostituisci il ciclo sui bucket (righe 38-45) con:

```go
	// Under GroupByTotal the single bucket IS the totals row: it collects every
	// entry, so its hours, billed hours and Amounts equal the totals and the
	// currency subtotals exactly — not approximately, so there is not even the
	// PerDay rounding caveat that applies at finer groupings. Emitting both put
	// "Total" directly above "TOTAL", differing only in case and color (#137).
	if !(r.GroupBy == report.GroupByTotal && len(r.Buckets) == 1) {
		for _, b := range r.Buckets {
			rows = append(rows, []string{
				b.Label,
				fmt.Sprintf("%.2f", b.Hours),
				fmt.Sprintf("%.2f", b.BilledHours),
				formatAmounts(b.Amounts, r.DefaultCurrency),
			})
		}
	}
```

`firstTotal := len(rows)` resta com'è: diventa 0 da solo, e con `firstTotal == 0`
lo `StyleFunc` colora già la riga 0 come riga totali. `len(Buckets) == 0` non
arriva qui: `report.go:208` intercetta con «No hours to show.».

- [ ] **Step 4: Add the two new goldens**

In `golden_test.go`, accanto agli altri, seguendo il pattern del file:

```go
// The Total grouping is what the report loads with, so it is the first table a
// user (and the README GIF) ever sees. Two goldens: one currency, and the
// multi-currency shape where TOTAL's Amount is empty and the subtotals carry
// the figures (#137).
func TestGoldenReportTotalGrouping(t *testing.T) { ... }
func TestGoldenReportTotalGroupingMultiCurrency(t *testing.T) { ... }
```

Costruisci le due `report.Report` con le stesse fixture dello Step 1 e rendi con
`reportTable(testTheme(true), r, 80)`. Poi:

```bash
go test ./internal/tui -race        # i due golden nuovi FAIL: il file non esiste
go test ./internal/tui -update
git status --short internal/tui/testdata/
```
Expected: **due file nuovi**, nessun file esistente modificato. Se un golden
esistente si muove, fermati e riporta: significa che la soppressione ha colpito
una forma che non doveva. **Guarda i due golden nuovi con i tuoi occhi** e
riportali nel report.

- [ ] **Step 5: Full gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui
git commit -m "fix(tui): stop repeating the total row under the Total grouping (#137)"
```

---

### Task 5: le colonne numeriche misurate (#138, punto 1)

**Files:**
- Modify: `internal/tui/report_table.go` (`reportItemWidth`, `reportAmountWidth`,
  elimina `reportNumWidth` e aggiorna il commento del blocco `const`, righe 11-23)
- Test: `internal/tui/report_table_test.go`

**Interfaces:**
- Produces: `reportNumWidths(rows [][]string) (hours, billed int)`.
  `reportItemWidth` e `reportAmountWidth` mantengono la firma attuale.

**I due fatti che decidono l'implementazione:**

1. In `lipgloss/table` **l'header determina la larghezza di colonna**. Misurato:
   celle `"12.50"` con header `"H"` → 7 colonne, con header `"Billed"` → 8.
   Quindi la misura **deve** includere gli header, come `reportItemWidth` già fa
   per Item e Amount (righe 102-103). Misurando solo i dati il budget assume
   5+5=10 mentre il renderer usa 5+6=11, e la tabella sfora di 1.
2. **La tabella non può stare in 40 colonne** con cifre a 10 caratteri: il minimo
   post-fix è `chrome(10) + floor Item + 10 + 10 + header Amount(6)` = 48 con
   label lunghe, 43 con label corte. Un test che asserisce `≤ 40` fallisce
   contro il codice corretto.

Sweep misurato del codice **attuale** (larghezza resa, `!` = sfora):

| width | label lunga + cifre larghe | label corta + cifre larghe |
|---|---|---|
| 40 | 48 ! | 44 ! |
| 44 | 48 ! | 47 ! |
| 48 | 52 ! | 47 ✓ |
| 60 | 64 ! | 47 ✓ |
| 80 | 84 ! | 47 ✓ |
| 100 | 86 ! | 47 ✓ |

Da qui le larghezze dei test: **60, 80, 100** per la label lunga, e **44** per la
label corta — l'unica finestra in cui quella direzione è rossa prima (47 in 44) e
verde dopo (43 in 44).

- [ ] **Step 1: Write the two behavioral tests ONLY**

Non scrivere ancora il test su `reportNumWidths`: nominerebbe un simbolo
inesistente, il package non compilerebbe e i due RED qui sotto non si vedrebbero.

```go
// reportNumWidth reserved 8 columns for each of Hours and Billed but nothing
// enforced it: a wider value pushed the table past the terminal edge by exactly
// 2 x (width - 8). The overflow needs BOTH factors — a long label (so Item is
// budget-bound rather than capped by its own content) and numbers wider than 8.
// A test missing either one passes against the bug: measured, a long label with
// ordinary hours renders 55 columns at width 60.
//
// 40 and 44 are deliberately absent: the fix cannot fit this table in fewer than
// 48 columns (chrome 10 + Item floor 12 + 10 + 10 + Amount header 6), so
// asserting there would fail against correct code.
func TestReportTableNeverExceedsWidthWithLongLabelAndWideNumbers(t *testing.T) {
	th := testTheme(true)
	const longLabel = "Website redesign and content migration backlog"
	rep := reportWithHours(longLabel, 1234567.5)
	for _, width := range []int{60, 80, 100} {
		got := lipgloss.Width(strings.Split(reportTable(th, rep, width), "\n")[0])
		if got > width {
			t.Errorf("width %d: table rendered %d columns", width, got)
		}
	}
}

// The other direction: short labels leave Item capped by its own content, but
// wide numbers still push the table past a narrow terminal. 44 is the width
// where the current code renders 47 (overflowing by 3) and the fix renders 43.
func TestReportTableNeverExceedsWidthWithShortLabelAndWideNumbers(t *testing.T) {
	th := testTheme(true)
	rep := reportWithHours("Website", 1234567.5)
	if got := lipgloss.Width(strings.Split(reportTable(th, rep, 44), "\n")[0]); got > 44 {
		t.Errorf("table rendered %d columns in a 44-column terminal", got)
	}
}

// reportWithHours builds a one-bucket report whose Hours and Billed carry the
// given value, so a test can dial the numeric width independently of the label.
func reportWithHours(label string, hours float64) report.Report {
	return report.Report{
		GroupBy: report.GroupByList, DefaultCurrency: "EUR",
		Buckets: []report.Bucket{{Label: label, Hours: hours, BilledHours: hours,
			Amounts: []report.CurrencyAmount{{Currency: "EUR", Amount: 625}}}},
		CurrencySubtotals: []report.CurrencySubtotal{{Currency: "EUR", Hours: hours, BilledHours: hours, Amount: 625}},
		TotalHours:        hours, BilledHours: hours, TotalAmount: 625,
	}
}
```

- [ ] **Step 2: Run to see the RED, with the numbers from the sweep**

Run: `go test ./internal/tui -run TestReportTableNeverExceedsWidthWith -v`
Expected: il primo FAIL tre volte — **64** a width 60, **84** a 80, **86** a 100;
il secondo FAIL una volta — **47** in un terminale da 44. Se i numeri non sono
questi, fermati: la fixture non è quella misurata dalla spec e il test non sta
esercitando il difetto che credi. **Allega il transcript.**

- [ ] **Step 3: Implement, and add the header test alongside it**

```go
// reportNumWidths measures the Hours and Billed columns from the rows, HEADERS
// INCLUDED. The headers are not decoration in this arithmetic: lipgloss/table
// sizes a column from every row it is given, the header row among them, so a
// "Billed" header holds 6 columns even when every value is 5. reportNumWidth
// used to reserve a flat 8 for each and nothing enforced it — a wider value
// simply pushed the table past the terminal edge by 2 x (width - 8) (#138).
func reportNumWidths(rows [][]string) (hours, billed int) {
	hours = lipgloss.Width(reportHeaders[1])
	billed = lipgloss.Width(reportHeaders[2])
	for _, row := range rows {
		hours = max(hours, lipgloss.Width(row[1]))
		billed = max(billed, lipgloss.Width(row[2]))
	}
	return hours, billed
}
```

In `reportItemWidth` e `reportAmountWidth`, sostituisci `2*reportNumWidth` con
`hours + billed` presi da `reportNumWidths(rows)`. Elimina la costante
`reportNumWidth` e riscrivi la parte del commento del blocco `const` che la
descrive.

E il test che inchioda la parte header:

```go
// Measuring the data only would make the budget assume 10 where the renderer
// uses 11, and the table would overflow by 1 — the same bug in a smaller size.
func TestReportNumWidthsIncludeTheHeaders(t *testing.T) {
	rows := [][]string{{"Website", "1.00", "1.00", "5.00 EUR"}}
	hours, billed := reportNumWidths(rows)
	if hours != lipgloss.Width(reportHeaders[1]) || billed != lipgloss.Width(reportHeaders[2]) {
		t.Errorf("reportNumWidths = (%d, %d), want the header widths (%d, %d)",
			hours, billed, lipgloss.Width(reportHeaders[1]), lipgloss.Width(reportHeaders[2]))
	}
}
```

- [ ] **Step 4: Run, then regenerate the goldens**

```bash
go test ./internal/tui -run 'TestReportTable|TestReportNumWidths|TestReportItemWidth|TestReportAmountWidth' -v
go test ./internal/tui -race
go test ./internal/tui -update
git diff internal/tui/testdata/
```
Expected: dove Item è vincolato dal budget (label lunghe: `report`,
`report_narrow`) Item guadagna **5** colonne — non 6: 16 riservate → 11 misurate
(`"Hours"`=5 + `"Billed"`=6). Dove le label sono corte l'output è identico.
**Guarda il diff dei golden con i tuoi occhi**: se un golden guadagna 6 colonne
invece di 5, la misura non include gli header.

- [ ] **Step 5: Full gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui
git commit -m "fix(tui): measure the report's numeric columns instead of reserving them (#138)"
```

---

### Task 6: il budget in 80 colonne (#136)

**Files:**
- Modify: `internal/tui/budget.go`
- Modify: `internal/tui/golden_test.go:198, 203` — `TestGoldenBudget` e
  `TestGoldenBudgetEmpty` chiamano `view(testTheme(true))` e vanno aggiornati
- Modify: il chiamante di `budgetModel.view` in `internal/tui` (cercalo:
  `grep -rn "budgetScreen.view" internal/tui`)
- Test: `internal/tui/budget_test.go`
- Test: `internal/tui/testdata/budget.golden`, `budget_empty.golden`

**Interfaces:**
- Consumes: `cell` (Task 1).
- Produces: `budgetModel.view(th theme, width int) string`;
  `budgetLayout(lines []report.BudgetLine, width int) (nameW int, showRemaining bool)`;
  `budgetFigures(l report.BudgetLine, withRemaining bool) string`.

**Larghezza dei golden, fissata qui e non lasciata all'implementatore:**
entrambi i golden rendono a **width 80** — la larghezza della issue #136. Con
`view(th, 0)` il golden non cambierebbe affatto (il fallback tiene il layout
naturale), che è l'opposto di quello che questo task deve dimostrare.

**La soglia di degrado, misurata:** la riga richiede almeno
`4 + 1 + budgetBarWidth + 1 + pctW + 2 + figuresW(senza remaining) + 12`
colonne, cioè **63** con le cifre del golden e **65** con importi a quattro cifre
e percentuale sopra 100. Sotto quella soglia sfora, per design. Quindi:
- fixture a una riga (cifre del golden): sta in 63, 65, 80, 100; **sfora a 60**;
- fixture a due righe (con 1040.00 e 104%): sta in 65, 80, 100; **sfora a 60 e 63**.

I test qui sotto asseriscono `≤ width` solo alle larghezze in cui la fixture ci
sta. Non è indulgenza: è la differenza fra un test che verifica il fix e un test
che pretende l'impossibile.

- [ ] **Step 1: Run the bar test against the CURRENT code, in isolation**

`renderBudgetBar` ha già la firma giusta, quindi il difetto #136.2 si può vedere
fallire **prima** di toccare la firma di `view`. Metti questo test in un file a
sé (`internal/tui/budget_bar_test.go`) così non venga trascinato giù dagli errori
di compilazione dei test dello Step 2:

```go
// The label rounded while the fill truncated, so from 99.5% up the label read
// "100%" over 19 of 20 blocks. Flooring the LABEL is the fix, not rounding the
// fill: a rounded fill would show a full bar from 97.5%, and a full bar means
// the budget is spent. The label stays unclamped above 100 — that is why this
// screen does not use bubbles/progress.
func TestBudgetBarLabelAgreesWithTheFill(t *testing.T) {
	th := testTheme(true)
	for _, pct := range []float64{0, 50, 97.6, 99.4, 99.5, 99.6, 99.9, 100, 104.7} {
		bar := renderBudgetBar(th, pct)
		full := strings.Count(bar, string(gaugeFull))
		wantFull := int(min(max(pct, 0), 100) / 100 * budgetBarWidth)
		if full != wantFull {
			t.Errorf("pct=%.1f: %d full blocks, want %d (%q)", pct, full, wantFull, bar)
		}
		// The label must state the floored percentage, so it can never claim a
		// milestone the bar has not reached.
		wantLabel := fmt.Sprintf("%.0f%%", math.Floor(pct))
		if !strings.HasSuffix(bar, " "+wantLabel) {
			t.Errorf("pct=%.1f: %q does not end in %q", pct, bar, wantLabel)
		}
	}
}
```

Run: `go test ./internal/tui -run TestBudgetBarLabelAgreesWithTheFill -v`
Expected: **FAIL** su 99,5 / 99,6 / 99,9 (label `100%` invece di `99%`) e su
104,7 (label `105%` invece di `104%`). **Allega il transcript**: è il RED di
#136.2.

- [ ] **Step 2: Prove the overflow against the current code, then write the rest**

Sonda temporanea (la firma di `view` è ancora quella vecchia): eseguila, allega
l'output, cancellala.

```go
func TestTemporaryProofBudgetOverflows80(t *testing.T) {
	th := testTheme(true)
	bm := newBudget([]report.BudgetLine{
		{ListName: "Website", Billed: 625, Budget: 1000, Currency: "EUR", Remaining: 375, PercentUsed: 62.5},
	})
	for _, line := range strings.Split(bm.view(th), "\n") {
		t.Logf("%3d cols %q", lipgloss.Width(line), line)
	}
}
```
Expected nel log: una riga da **94** colonne.

Poi i test definitivi, in `budget_test.go`:

```go
// The budget line is unbounded: it grows with the figures. Measured before the
// fix, testdata/budget.golden is 94 columns (90 of content + 4 of th.Box), and
// 80 columns is a bare terminal and a split tmux pane (#136).
//
// The widths are the ones this fixture can actually fit: the row needs
// box(4) + 1 + gauge(20) + 1 + pct + 2 + figures + name-floor(12), which is 65
// for these two lines. Asserting at 60 would demand the impossible — below the
// threshold the row overflows by design, and the spec says so.
func TestBudgetViewNeverExceedsWidth(t *testing.T) {
	th := testTheme(true)
	bm := newBudget([]report.BudgetLine{
		{ListName: "Website", Billed: 625, Budget: 1000, Currency: "EUR", Remaining: 375, PercentUsed: 62.5},
		{ListName: "Mobile app", Billed: 1040, Budget: 1000, Currency: "EUR", Remaining: -40, PercentUsed: 104},
	})
	for _, width := range []int{65, 80, 100, 120} {
		for _, line := range strings.Split(bm.view(th, width), "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Errorf("width %d: line rendered %d columns: %q", width, w, line)
			}
		}
	}
}

// Small figures let the row fit a narrow terminal, which is what proves the
// layout is measured rather than reserved: same code, more room, wider name.
func TestBudgetViewFitsNarrowTerminalsWithSmallFigures(t *testing.T) {
	th := testTheme(true)
	bm := newBudget([]report.BudgetLine{
		{ListName: "Website", Billed: 12.5, Budget: 20, Currency: "EUR", Remaining: 7.5, PercentUsed: 62.5},
	})
	for _, width := range []int{60, 80} {
		for _, line := range strings.Split(bm.view(th, width), "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Errorf("width %d: line rendered %d columns: %q", width, w, line)
			}
		}
	}
}

// A wide-rune list name must not push the row past its column: the name column
// is the one that absorbs the slack, so it is also the one that used to
// misalign. An ASCII fixture passes against the bug.
func TestBudgetViewHoldsItsWidthWithWideRunes(t *testing.T) {
	th := testTheme(true)
	bm := newBudget([]report.BudgetLine{
		{ListName: strings.Repeat("🚀", 24), Billed: 625, Budget: 1000, Currency: "EUR", Remaining: 375, PercentUsed: 62.5},
	})
	for _, line := range strings.Split(bm.view(th, 80), "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("line rendered %d columns: %q", w, line)
		}
	}
}

// "(remaining X)" is the most redundant field on the row (it is budget minus
// billed), so it is what gives way when the name column would fall below the
// floor. One pass: compute with it, and if the name is too narrow, recompute
// without it and never put it back. Measured for this fixture at width 80: with
// remaining the name would get 10 columns, under the floor of 12, so remaining
// goes and the name gets 29.
func TestBudgetLayoutDropsRemainingBeforeStarvingTheName(t *testing.T) {
	lines := []report.BudgetLine{
		{ListName: "Website", Billed: 625, Budget: 1000, Currency: "EUR", Remaining: 375, PercentUsed: 62.5},
	}
	nameW, showRemaining := budgetLayout(lines, 80)
	if showRemaining {
		t.Errorf("at width 80 remaining should have been dropped; name column is %d", nameW)
	}
	if nameW != 29 {
		t.Errorf("nameW = %d, want 29", nameW)
	}
	// A roomy terminal keeps remaining AND a wider name.
	wideName, wideRem := budgetLayout(lines, 100)
	if !wideRem {
		t.Errorf("at width 100 remaining should fit; name column is %d", wideName)
	}
}

// Before the first WindowSizeMsg the width is 0 and nothing is sized against it:
// the screen keeps its natural layout, the same fallback reportItemWidth has.
func TestBudgetLayoutFallsBackBeforeTheFirstWindowSize(t *testing.T) {
	nameW, showRemaining := budgetLayout([]report.BudgetLine{{ListName: "Website"}}, 0)
	if nameW != budgetNameWidth || !showRemaining {
		t.Errorf("budgetLayout(width=0) = (%d, %v), want (%d, true)", nameW, showRemaining, budgetNameWidth)
	}
}
```

Run: `go test ./internal/tui -run TestBudget -v`
Expected: `FAIL [build failed]` sulla firma di `view` e su `budgetLayout` —
atteso, sono simboli nuovi. Il RED comportamentale di questo task è quello dello
Step 1 più il log della sonda.

- [ ] **Step 3: Implement**

```go
const (
	// boxChrome is what th.Box spends on itself: a rounded border plus one
	// column of padding on each side. Measured, not assumed:
	// th.Box.Render("12345") renders 9 columns.
	boxChrome = 4
	// budgetNameWidth is the natural list-name column, used before the first
	// WindowSizeMsg — the same "nothing is sized yet" fallback reportItemWidth has.
	budgetNameWidth = 24
	// budgetMinNameWidth is where a list name stops being a name, mirroring
	// reportMinItemWidth's floor on the report table.
	budgetMinNameWidth = 12
)

// budgetFigures renders the money part of a row. remaining is the most redundant
// field on the line — it is Budget minus Billed — so it is what gives way on a
// narrow terminal.
func budgetFigures(l report.BudgetLine, withRemaining bool) string {
	if withRemaining {
		return fmt.Sprintf("%.2f / %.2f %s (remaining %.2f)", l.Billed, l.Budget, l.Currency, l.Remaining)
	}
	return fmt.Sprintf("%.2f / %.2f %s", l.Billed, l.Budget, l.Currency)
}

// budgetLayout resolves the list-name column and whether "(remaining …)" fits.
//
// Everything except the name is MEASURED from the real rows, because none of it
// is fixed-width: the figures grow with the amounts and the percentage grows past
// 100. Reserving a constant for them is exactly how this screen came to render 94
// columns into an 80-column terminal.
//
// One pass, in this order — the circularity (figures depend on the drop, the drop
// depends on the name, the name depends on the figures) is resolved here and not
// left to the caller:
//
//  1. compute with remaining; if the name column clears the floor, done;
//  2. otherwise recompute without remaining, and never put it back;
//  3. if the name is still under the floor, the floor wins and the row
//     overflows. That happens below boxChrome + 1 + budgetBarWidth + 1 + pctW +
//     2 + figures + budgetMinNameWidth — around 63 to 65 columns with realistic
//     amounts. Below it no split saves the row, and pretending otherwise would
//     mean a name column too narrow to read. Accepted and explicit, the same
//     trade-off the report table makes below its own floor.
func budgetLayout(lines []report.BudgetLine, width int) (nameW int, showRemaining bool) {
	if width <= 0 { // before the first WindowSizeMsg nothing is sized against it
		return budgetNameWidth, true
	}
	pctW := 0
	for _, l := range lines {
		pctW = max(pctW, lipgloss.Width(fmt.Sprintf("%.0f%%", math.Floor(l.PercentUsed))))
	}
	widestFigures := func(withRemaining bool) int {
		w := 0
		for _, l := range lines {
			w = max(w, lipgloss.Width(budgetFigures(l, withRemaining)))
		}
		return w
	}
	fixed := 1 + budgetBarWidth + 1 + pctW + 2
	usable := width - boxChrome

	if nameW = usable - fixed - widestFigures(true); nameW >= budgetMinNameWidth {
		return nameW, true
	}
	return max(usable-fixed-widestFigures(false), budgetMinNameWidth), false
}
```

`view` diventa:

```go
func (bm budgetModel) view(th theme, width int) string {
	title := th.Title.Render("Budget burn-down")
	if len(bm.lines) == 0 {
		return title + "\n\n" + th.Box.Render("No budgets configured.")
	}
	nameW, showRemaining := budgetLayout(bm.lines, width)
	var rows strings.Builder
	for _, l := range bm.lines {
		rows.WriteString(fmt.Sprintf("%s %s  %s\n",
			cell(l.ListName, nameW), renderBudgetBar(th, l.PercentUsed), budgetFigures(l, showRemaining)))
	}
	body := th.Box.Render(strings.TrimRight(rows.String(), "\n"))
	return title + "\n\n" + body
}
```

E in `renderBudgetBar`, l'ultima riga:

```go
	// The fill truncates (int()), so the label FLOORS to match. Rounding the
	// label instead is what made 99.5% read "100%" over 19 of 20 blocks; and
	// rounding the FILL to match a rounded label would be worse — it would show
	// a full bar from 97.5%, and a full bar means the budget is spent. Above 100
	// the label stays unclamped: that asymmetry is the whole point of this
	// function (see the doc comment).
	return fmt.Sprintf("%s %.0f%%", bar, math.Floor(percentUsed))
```

Aggiorna i chiamanti: quello di produzione, e `golden_test.go:198,203`, che
diventano `newBudget(lines).view(testTheme(true), 80)` e
`newBudget(nil).view(testTheme(true), 80)`.

- [ ] **Step 4: Run, then regenerate the goldens**

```bash
go test ./internal/tui -run TestBudget -v
go test ./internal/tui -race
go test ./internal/tui -update
git diff internal/tui/testdata/budget.golden
```
Expected: test PASS; `budget.golden` scende da 94 a 80 colonne e perde
`(remaining …)`, con il nome a 29 colonne. `budget_empty.golden` non cambia (il
ramo vuoto ignora la larghezza). **Guardali** e riporta la forma nuova.

- [ ] **Step 5: Full gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui
git commit -m "fix(tui): fit the budget screen in 80 columns and make its label agree with its bar (#136)"
```

---

### Task 7: il range pinnato (#28)

**Files:**
- Modify: `internal/tui/app.go` (`entriesMsg` a riga 54, `loadEntriesCmd` a 380-392,
  handler `entriesMsg` a ~699-727, campi del `Model` a ~125-150, nuovo `activeRange`)
- Modify: `internal/tui/entries.go` (`entriesReloadedMsg` a 23-26, `reloadForBrowser`
  a ~563, `updateEntryCmd` a ~436)
- Modify: `internal/tui/demo.go:180` (`demoEntriesCmd` produce `entriesMsg`)
- Modify: `internal/tui/report.go:67, 81, 127, 184`, `internal/tui/rates.go:854`
- Test: `internal/tui/range_pin_test.go` (nuovo)

**Interfaces:**
- Produces: `Model.loadedStart`, `Model.loadedEnd`,
  `Model.activeRange() (start, end time.Time)`. `entriesMsg` e
  `entriesReloadedMsg` portano `start, end time.Time`.

**Il punto che fa la differenza:** il pin non si ricalcola, si **trasporta**.
Ripinnare chiamando `currentRange()` nell'handler ricalcola il range *dopo* il
load: un load alle 23:59:59 su R1 il cui messaggio arriva alle 00:00:01
pinnerebbe R2 ≠ R1, e il pin nascerebbe disallineato dalle entry che descrive —
cioè proprio il drift che esiste per eliminare.

**Il fuso orario è parte della fixture, non un dettaglio.** `newTestModel()` non
imposta la timezone, quindi `m.loc == time.Local` e `RangeForPreset` risolve la
mezzanotte **locale**. Su una macchina a UTC+2 i due istanti attorno alla
mezzanotte UTC cadono nello stesso giorno locale e il range non si muove: il test
passerebbe in CI (UTC) e fallirebbe sulla macchina di chi sviluppa. Ogni test di
questo task fissa `m.loc = time.UTC`, come fa `goldenModel()`.

- [ ] **Step 1: Write the failing test**

```go
// A relative preset re-resolved time.Now() on every rebuild, so regrouping after
// midnight relabeled the report with a range the loaded entries never covered.
// The range is pinned by the load and carried in the message, so a rebuild
// cannot drift away from its own data (#28).
//
// m.loc is pinned to UTC on purpose: with time.Local the two instants below fall
// on the same local day east of Greenwich, the fixture stops exercising the
// drift, and the test would pass in CI while failing on a developer's machine.
func TestRebuildKeepsTheRangeTheEntriesWereLoadedFor(t *testing.T) {
	before := time.Date(2026, time.July, 24, 23, 59, 59, 0, time.UTC)
	after := time.Date(2026, time.July, 25, 0, 0, 1, 0, time.UTC)
	now := before
	m := newTestModel()
	m.loc = time.UTC
	m.preset = report.PresetLast7d
	m.now = func() time.Time { return now }

	start, end := m.currentRange()
	mm, _ := m.Update(entriesMsg{entries: goldenEntries(), start: start, end: end})
	m = mm.(Model)
	if !m.report.Start.Equal(start) {
		t.Fatalf("report.Start = %v, want the loaded range start %v", m.report.Start, start)
	}

	now = after // midnight passes; last_7d would now resolve one day later
	if newStart, _ := m.currentRange(); newStart.Equal(start) {
		t.Fatal("the fixture does not exercise the drift: currentRange did not move across midnight")
	}
	if !m.applyReport() {
		t.Fatal("applyReport returned false")
	}
	if !m.report.Start.Equal(start) {
		t.Errorf("after midnight report.Start = %v, want the pinned %v", m.report.Start, start)
	}
}

// Home's label describes the range the NEXT load will fetch, so it must stay
// fresh: month, week mode and preset all change on Home WITHOUT a reload, and a
// pinned label would freeze while the user navigates.
func TestHomeLabelFollowsTheSelectionNotThePin(t *testing.T) {
	m := newTestModel()
	m.loc = time.UTC
	m.now = func() time.Time { return time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC) }
	start, end := m.currentRange()
	mm, _ := m.Update(entriesMsg{entries: goldenEntries(), start: start, end: end})
	m = mm.(Model)
	pinned := m.rangeLabel()

	m.month-- // Home's PrevMonth, which triggers no reload
	if got := m.rangeLabel(); got == pinned {
		t.Errorf("rangeLabel stayed %q after changing month: the label must describe the next load", got)
	}
}

// Nothing is pinned before the first load, and the fallback is reachable in the
// live flow: the rates screen opens from Home and rebuilds the report there.
func TestActiveRangeFallsBackBeforeAnyLoad(t *testing.T) {
	m := newTestModel()
	m.loc = time.UTC
	wantStart, wantEnd := m.currentRange()
	gotStart, gotEnd := m.activeRange()
	if !gotStart.Equal(wantStart) || !gotEnd.Equal(wantEnd) {
		t.Errorf("activeRange() = (%v, %v), want currentRange() (%v, %v)", gotStart, gotEnd, wantStart, wantEnd)
	}
}

// A browser reload is a load: it must refresh the pin, not leave a stale one.
func TestBrowserReloadRepinsTheRange(t *testing.T) {
	m := newTestModel()
	m.loc = time.UTC
	old := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	m.loadedStart, m.loadedEnd = old, old.AddDate(0, 1, 0)
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	mm, _ := m.Update(entriesReloadedMsg{entries: goldenEntries(), status: "ok", start: start, end: end})
	m = mm.(Model)
	if got, _ := m.activeRange(); !got.Equal(start) {
		t.Errorf("activeRange() = %v after a browser reload, want the reloaded %v", got, start)
	}
}
```

- [ ] **Step 2: Stage the RED so it is observable**

I test non compilano finché `entriesMsg` non ha i campi e `activeRange` non
esiste — atteso. Per **vedere** il difetto:

1. aggiungi **solo** i campi `start, end` ai due messaggi, i campi
   `loadedStart/loadedEnd` al Model, e `activeRange()` (che a questo punto
   ritorna sempre il fallback, perché niente pinna ancora);
2. fai passare `start`/`end` dai due cmd ai messaggi;
3. **non** toccare ancora i sei siti di rebuild né gli handler;
4. Run: `go test ./internal/tui -run TestRebuildKeepsTheRange -v`

Expected: FAIL con `after midnight report.Start = 2026-07-19 …, want the pinned
2026-07-18 …` (un giorno di scarto). **Allega il transcript.** Se passa, il pin
non è la causa e la fixture non esercita il drift: correggi la fixture.

- [ ] **Step 3: Implement**

`app.go:54`:

```go
	// entriesMsg carries the range the load resolved, not just the entries: the
	// handler pins that pair, so a rebuild after midnight cannot relabel the
	// report with a range these entries never covered (#28). Re-deriving the
	// range in the handler would defeat the point — a load at 23:59:59 whose
	// message lands at 00:00:01 would pin the wrong day.
	entriesMsg struct {
		entries    []report.TimeEntry
		start, end time.Time
	}
```

`entries.go:23-26`, uguale con il commento accorciato. Produttori:
`loadEntriesCmd` (`app.go:390`), `demoEntriesCmd` (`demo.go:180`) e
`reloadForBrowser` (`entries.go:564`) hanno già `start`/`end` in scope.

`Model`, accanto agli altri campi di periodo:

```go
	// loadedStart/loadedEnd are the range the currently loaded entries were
	// fetched for, pinned from the pair the load itself resolved (#28). Zero
	// means nothing has been loaded yet.
	loadedStart, loadedEnd time.Time
```

```go
// activeRange is the range the LOADED entries actually cover: the pinned pair
// when a load has happened, else what the next load would use.
//
// The split matters because a relative preset moves under you. Every rebuild
// over already-loaded entries goes through here, so it describes its own data.
// Every surface that describes the NEXT load — Home's label, and the loads
// themselves — uses currentRange() instead: Home changes month, week mode and
// preset WITHOUT reloading, so a pinned label would freeze while the user
// navigates.
func (m Model) activeRange() (start, end time.Time) {
	if !m.loadedStart.IsZero() {
		return m.loadedStart, m.loadedEnd
	}
	return m.currentRange()
}
```

Nei due handler, **prima** di ricostruire:
`m.loadedStart, m.loadedEnd = msg.start, msg.end`, e usa `msg.start, msg.end`
per la `report.Build` immediata dell'handler `entriesMsg`.

Sostituisci `m.currentRange()` con `m.activeRange()` **solo** in questi sei
punti: `entries.go:436` (`updateEntryCmd` — «moved outside the current range»
deve riferirsi al range delle entry a schermo), `report.go:67` (`dailySeries`),
`report.go:81`, `report.go:127`, `report.go:184`, `rates.go:854`.

Lascia `currentRange()` a `home.go:99`, `app.go:287`, `entries.go:564`. Aggiungi
a `rangeLabel` un commento di una riga che dica perché è fresco.

- [ ] **Step 4: Run**

Run: `go test ./internal/tui -race`
Expected: PASS. Nessun golden si muove: il pin non cambia il rendering, solo
quale range viene usato.

- [ ] **Step 5: Full gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui
git commit -m "fix(tui): pin the resolved range to the loaded entries (#28)"
```

---

### Task 8: lo scroll dei Filters (#28)

**Files:**
- Modify: `internal/tui/palette.go` (`scrollPalette` a 139-151)
- Modify: `internal/tui/filters.go` (`filtersModel`, `updateFilters`, `view`)
- Modify: `internal/tui/golden_test.go:219` — `TestGoldenFilters` chiama
  `view(testTheme(true))` e va aggiornato
- Modify: il chiamante di produzione di `filtersModel.view`
- Test: `internal/tui/filters_test.go`
- Test: `internal/tui/testdata/filters.golden`, più un golden nuovo per la
  vista finestrata

**Interfaces:**
- Produces: `scrollWindow(idx, top, rows int) int`; `filtersModel.top`;
  `filtersCursorRow(fs filtersModel) int`;
  `filtersVisualRows(fs filtersModel, th theme) []string`;
  `filtersRows(height int) int`; `filtersModel.view(th theme, rows int) string`.

**Il contratto, fissato qui perché due letture producono due UI diverse:**
`view(th, rows)` emette **al più `rows` righe totali**, titolo e riga vuota
compresi; la finestra sulle righe di contenuto è quindi `rows - 2`.
`filtersRows(height)` sottrae solo la riga vuota e il footer che `View` aggiunge
sempre — **non** il titolo, che è dentro il budget — con un floor di 6 perché la
schermata si restringe ma non svanisce. `filtersVisualRows` produce **solo le
righe di contenuto**, senza titolo.

**La proiezione, decisa nella spec:** la finestra è sulle **righe visuali
dell'intera vista** (header di sezione inclusi), non sulle opzioni della sola
sezione attiva. L'overflow di cui si lamenta la #28 è quello della vista intera,
e una finestra per-sezione lascerebbe fuori schermo gli header delle sezioni
successive. L'indice del cursore è la riga visuale su cui sta `(sec, row)`.

**Le fixture: le opzioni vengono dalle `entries`.** `newFilters` costruisce
`options` da `entries` (`filters.go:74-84`); le mappe che riceve sono solo lo
stato *selected*. Passare 40 chiavi come `lists` non crea 40 opzioni: crea
**zero** opzioni, un cursore che non si muove, e un test che passa contro
qualunque implementazione, compresa una che non scrolla. Costruisci le entry:

```go
// filtersFixture builds entries whose list names produce n Lists options, plus
// one tag and one status, so the Filters screen has something to scroll.
func filtersFixture(n int) []report.TimeEntry {
	entries := make([]report.TimeEntry, 0, n)
	for i := range n {
		entries = append(entries, report.TimeEntry{
			ListName: fmt.Sprintf("List %02d", i),
			Tags:     []string{"backend"},
			Status:   "done",
		})
	}
	return entries
}
```

- [ ] **Step 1: Write the failing test**

```go
// The Filters screen had no window at all: many lists or tags simply ran off the
// bottom of a short terminal, with no way to reach them (#28).
func TestFiltersViewNeverExceedsItsRowBudget(t *testing.T) {
	th := testTheme(true)
	fs := newFilters(filtersFixture(40), nil, nil, nil, nil)
	for _, rows := range []int{6, 10, 20} {
		if got := strings.Count(fs.view(th, rows), "\n") + 1; got > rows {
			t.Errorf("row budget %d: view rendered %d lines", rows, got)
		}
	}
}

// The window must follow the cursor. Asserting on the rendered cursor is not an
// option: under termenv.Ascii the active row is byte-identical to the others
// (th.Accent renders nothing), and "▸" marks the section header, which scrolls
// out. So assert on the window itself, which is what the feature actually is.
func TestFiltersWindowFollowsTheCursor(t *testing.T) {
	m := newTestModel()
	m.filtersScreen = newFilters(filtersFixture(40), nil, nil, nil, nil)
	m.screen = screenFilters
	m.height = 12
	for range 30 { // walk down well past the first window
		mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = mm.(Model)
	}
	fs := m.filtersScreen
	if fs.row == 0 && fs.sec == 0 {
		t.Fatal("the cursor never moved: the fixture has no options to walk")
	}
	row, rows := filtersCursorRow(fs), filtersRows(m.height)-2
	if row < fs.top || row >= fs.top+rows {
		t.Errorf("cursor at visual row %d is outside the window [%d, %d)", row, fs.top, fs.top+rows)
	}
}

// scrollWindow is the palette's own scrolling, extracted so both screens share
// one idiom. Same behavior, now with a name.
func TestScrollWindow(t *testing.T) {
	tests := []struct {
		name                string
		idx, top, rows, want int
	}{
		{"inside stays", 3, 2, 5, 2},
		{"above pulls up", 1, 4, 5, 1},
		{"below pushes down", 9, 2, 5, 5},
		{"first row", 0, 0, 5, 0},
		{"zero rows", 7, 3, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := scrollWindow(tc.idx, tc.top, tc.rows); got != tc.want {
				t.Errorf("scrollWindow(%d, %d, %d) = %d, want %d", tc.idx, tc.top, tc.rows, got, tc.want)
			}
		})
	}
}

// The cursor's visual row must account for section headers and the "(none)"
// placeholder an empty section still renders, or the window scrolls to the wrong
// place. With two Lists options, Lists row 1 is visual row 2 (header + first
// option); the Tags section that follows starts after header + 2 options.
func TestFiltersCursorRowCountsHeadersAndOptions(t *testing.T) {
	fs := newFilters(filtersFixture(2), nil, nil, nil, nil)
	fs.sec, fs.row = 0, 1
	if got := filtersCursorRow(fs); got != 2 {
		t.Errorf("filtersCursorRow at Lists row 1 = %d, want 2", got)
	}
	fs.sec, fs.row = 1, 0
	if got := filtersCursorRow(fs); got != 4 {
		t.Errorf("filtersCursorRow at Tags row 0 = %d, want 4", got)
	}
}
```

Prima di eseguire, **verifica i due numeri 2 e 4 contro la struttura reale**:
`filtersFixture(2)` produce 2 opzioni in Lists (`List 00`, `List 01`), 1 in Tags
(`backend`), 1 in Statuses (`done`), 3 in Billable. Se `newFilters` ordina o
raggruppa diversamente da come credi, aggiusta i numeri attesi al comportamento
reale — non il comportamento ai numeri.

- [ ] **Step 2: Stage the RED**

Aggiungi **solo** la firma `view(th theme, rows int)` che ignora `rows`, più
`filtersRows` e `filtersCursorRow`, senza finestrare niente. Poi:

Run: `go test ./internal/tui -run TestFiltersViewNeverExceedsItsRowBudget -v`
Expected: FAIL su tutti e tre i budget, con un numero di righe pari alla vista
intera (con 40 liste: 2 header+opzioni per quattro sezioni — conta quante escono
e riportalo). **Allega il transcript.**

- [ ] **Step 3: Implement**

In `palette.go`, estrai:

```go
// scrollWindow moves a visible window of `rows` rows so idx stays inside it, and
// returns the new top. Shared by the palette and the Filters screen, so there is
// one scrolling idiom in this package rather than two (#28).
func scrollWindow(idx, top, rows int) int {
	if rows <= 0 {
		return 0
	}
	if idx < top {
		return idx
	}
	if idx >= top+rows {
		return idx - rows + 1
	}
	return top
}
```

`scrollPalette` diventa `p.top = scrollWindow(p.idx, p.top, rows); return p`.
Sposta la nota sulla #28 dal commento di `scrollPalette` a quello di
`scrollWindow`, aggiornandola: adesso i Filters ce l'hanno.

In `filters.go`, ristruttura `view` in due passi — costruire tutte le righe
visuali, poi finestrarle. È anche ciò che rende la proiezione testabile:

```go
// filtersVisualRows renders the screen's CONTENT as one flat slice of visual
// rows: a section header, then its options (or a single "(none)"), for each
// section. The title is not part of the slice. The window in view() is taken
// over THIS slice, so the cursor's index and the rendering can never disagree
// about what row something is on.
func filtersVisualRows(fs filtersModel, th theme) []string

// filtersCursorRow is the visual row (sec, row) sits on — the index
// filtersVisualRows puts it at. Headers and the "(none)" placeholder count.
func filtersCursorRow(fs filtersModel) int {
	n := 0
	for si, sec := range fs.sections {
		if si == fs.sec {
			return n + 1 + fs.row // this section's header, then row
		}
		n += 1 + max(1, len(sec.options)) // header + options, or the "(none)" row
	}
	return n
}

// filtersRows is how many lines view() may emit in total, title included. The
// subtraction accounts for the blank line and the footer that View always
// appends; the floor says the screen shrinks but never vanishes.
func filtersRows(height int) int {
	return max(6, height-2)
}
```

`filtersModel` prende `top int`; `updateFilters` chiude ogni movimento del
cursore con
`fs.top = scrollWindow(filtersCursorRow(fs), fs.top, filtersRows(m.height)-2)`,
e scrive indietro `m.filtersScreen = fs` **prima** del return, come vuole il
pattern del pacchetto.

- [ ] **Step 4: Run and add a golden**

```bash
go test ./internal/tui -run 'TestFilters|TestScrollWindow' -v
go test ./internal/tui -race
go test ./internal/tui -update
```
Aggiorna `golden_test.go:219` in `newFilters(goldenEntries(), nil, nil, nil, nil).view(testTheme(true), 24)`
— 24 righe, abbastanza per la vista intera delle fixture golden, così
`filters.golden` non cambia. Poi aggiungi un golden nuovo per la vista
finestrata: `filtersFixture(40)` reso con `rows = 12`. **Guardalo**: deve
mostrare il taglio in fondo e il cursore dentro.

- [ ] **Step 5: Full gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/tui
git commit -m "feat(tui): scroll the Filters screen on the palette's shared window (#28)"
```

---

### Task 9: il resto della #28 — `task_tags`, micro-nit, densità demo

> **Dipende dal Task 7**: estrae `rebuildReport` dagli stessi blocchi che il
> Task 7 modifica. Non eseguirlo prima.

**Files:**
- Modify: `internal/clickup/timeentries.go` (`rawEntry.TaskTags`, `rawEntry.Tags`)
- Test: `internal/clickup/timeentries_test.go`
- Modify: `internal/report/filter.go` (`Filter`)
- Modify: `internal/tui/report.go` (`applyReport` → `rebuildReport`),
  `internal/tui/app.go`, `internal/tui/rates.go`
- Test: `internal/tui/demo_test.go`

**Interfaces:**
- Produces: `clickup.tagName` con `UnmarshalJSON`;
  `Model.rebuildReport(groupBy string) bool`.

- [ ] **Step 1: Write the failing test**

`internal/clickup/timeentries_test.go` — segui il pattern httptest già presente
nel file per costruire il client e puntarne il base URL:

```go
// ClickUp's documented example returns task_tags as objects
// ({name, tag_fg, tag_bg, creator}), but the reference's generated schema for
// the date-range endpoint types it as an array of strings. A bare string used to
// fail the WHOLE decode — "cannot unmarshal string into Go struct field
// rawEntry.data.task_tags" — losing every entry, not just the tags (#28).
func TestTimeEntriesAcceptsBothTagShapes(t *testing.T) {
	tests := []struct {
		name string
		tags string
		want []string
	}{
		{"objects, as documented", `[{"name":"urgent"},{"name":"frontend"}]`, []string{"urgent", "frontend"}},
		{"bare strings, as the generated schema types it", `["urgent","frontend"]`, []string{"urgent", "frontend"}},
		{"mixed", `[{"name":"urgent"},"frontend"]`, []string{"urgent", "frontend"}},
		{"empty", `[]`, nil},
		{"null", `null`, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// ... httptest server returning one entry whose task_tags is tc.tags,
			// following this file's existing fixture shape ...
			if !slices.Equal(got[0].Tags, tc.want) {
				t.Errorf("Tags = %v, want %v", got[0].Tags, tc.want)
			}
		})
	}
}
```

`slices.Equal` tratta `nil` e `[]string{}` come uguali, quindi i casi
`empty`/`null` passano qualunque delle due cose produca `toTimeEntry`: non è una
differenza da inseguire.

`internal/tui/demo_test.go`:

```go
// Demo mode used to be near-empty for the non-month presets because the fixtures
// lived on fixed month days; #4 made them day offsets wrapped modulo the range
// span. This pins that, so the presets stay demoable (#28). It is a
// regression-guard on a box that is ALREADY closed: it must pass on the first
// run, and a failure means #4's wrapping regressed.
func TestDemoEntriesPopulateEveryPreset(t *testing.T) {
	now := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	for _, preset := range []string{report.PresetThisMonth, report.PresetLast7d, report.PresetLast30d, report.PresetThisWeek} {
		t.Run(preset, func(t *testing.T) {
			start, end := report.RangeForPreset(preset, 2026, time.July, now, time.UTC)
			entries := demoEntries(start, end)
			if len(entries) != 8 {
				t.Errorf("got %d entries, want all 8 fixtures inside the range", len(entries))
			}
			for _, e := range entries {
				if e.Start.Before(start) || !e.Start.Before(end) {
					t.Errorf("entry %s at %v falls outside [%v, %v)", e.ID, e.Start, start, end)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run to verify the tag test fails**

Run: `go test ./internal/clickup -run TestTimeEntriesAcceptsBothTagShapes -v`
Expected: `bare strings` e `mixed` FAIL con
`json: cannot unmarshal string into Go struct field`. **Allega il transcript.**

Run: `go test ./internal/tui -run TestDemoEntriesPopulateEveryPreset -v`
Expected: PASS al primo colpo. Dichiaralo nel report come regression-guard, non
come fix.

- [ ] **Step 3: Implement**

`internal/clickup/timeentries.go`:

```go
// tagName is a tag as ClickUp returns it on a time entry. The documented example
// is an object ({name, tag_fg, tag_bg, creator}), but the API reference's
// generated schema for the date-range endpoint types task_tags as an array of
// strings. A bare string used to fail the WHOLE TimeEntries decode, not just the
// tags, so both shapes are accepted and the assumption is gone rather than
// annotated (#28). A live confirmation against a real workspace is tracked in
// #129.
type tagName struct{ Name string }

func (t *tagName) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		t.Name = s
		return nil
	}
	var o struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return fmt.Errorf("tagName: unhandled value: %s", b)
	}
	t.Name = o.Name
	return nil
}
```

`rawEntry.TaskTags` e `rawEntry.Tags` diventano `[]tagName`; i due cicli in
`toTimeEntry` restano identici.

`internal/report/filter.go` — `Filter` calcola i tre conteggi una volta e deriva
l'emptiness da quelli (`nL+nT+nS == 0 && c.Billable == nil`) invece di chiamare
`Empty()` e poi ricontare. **`Empty()` resta esattamente com'è**: è API esportata,
usata da `internal/cli/report.go:139` e `internal/tui/app.go:369`, e memoizzare
conteggi sulla struct la esporrebbe a hazard di copia. Il comportamento non
cambia: se un test esistente si muove, hai cambiato semantica — torna indietro.

`internal/tui` — estrai il blocco triplicato:

```go
// rebuildReport rebuilds m.report and m.rep for the given grouping, over the
// visible entries and the pinned range. It returns false when the timezone or
// the pricing rules failed to parse, in which case the caller must not overwrite
// the error screen those helpers already routed to. It is the single rebuild
// used by applyReport, by the entries handler and by the grouping and rates
// changes, which each carried their own copy.
func (m *Model) rebuildReport(groupBy string) bool
```

Chiamanti: `report.go:83`, `app.go:719`, `rates.go:855`, e `applyReport` stesso,
che resta come wrapper che risolve il grouping di default. I tre chiamanti sono
metodi a ricevitore **valore**: `m.rebuildReport(g)` su un ricevitore valore
compila (la copia locale è indirizzabile) e muta quella copia, che è esattamente
ciò che serve dato che poi la ritornano.

**Attenzione all'handler `entriesMsg`**: dopo il Task 7 usa `msg.start`/`msg.end`
e non `activeRange()`. Se lo fai passare per `rebuildReport`, il pin va scritto
**prima** della chiamata, così `activeRange()` dentro `rebuildReport` legge già
la coppia nuova. Verifica con `TestRebuildKeepsTheRangeTheEntriesWereLoadedFor`.

- [ ] **Step 4: Run**

```bash
go test ./internal/clickup ./internal/report ./internal/tui -race
```
Expected: PASS, nessun golden mosso.

- [ ] **Step 5: Full gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add internal/
git commit -m "fix(clickup): accept both documented shapes of task_tags (#28)"
```

---

### Task 10: documentazione e chiusura

**Files:**
- Modify: `CHANGELOG.md`
- Modify: `README.md`, `README.it.md` (solo se mostrano la riga budget)

- [ ] **Step 1: CHANGELOG**

Sotto `## [Unreleased]`, in inglese, una riga per issue chiusa.

- [ ] **Step 2: Verifica che i README non mentano**

```bash
grep -n "remaining\|budget\|burn-down" README.md README.it.md | head -20
```
La tabella dei tasti non cambia (nessun tasto nuovo). Se un README mostra un
esempio della riga budget, aggiornalo alla forma a 80 colonne.

- [ ] **Step 3: Full gate + commit**

```bash
gofmt -l . && go vet ./... && go run honnef.co/go/tools/cmd/staticcheck@latest ./... && go build ./... && go test ./... -race
git add CHANGELOG.md README.md README.it.md
git commit -m "docs: changelog for v1.9 tranche D"
```

---

## Self-review del piano

**Copertura della spec.** §4.1 → Task 1. §4.2 → Task 2. §4.3 → Task 3. §4.4 →
Task 4. §4.5 → Task 6. §4.6 → Task 5. §4.7 → Task 7 (range) e Task 8 (scroll).
§4.8 → Task 9. §6 è vincolo globale. §5 (fuori scope) non genera task.

**Ordine e dipendenze.** `1 → 2 → {3,4,5,6}`; `4` prima di `5`; `7 → 9`; `8`
indipendente. La dipendenza `7 → 9` è reale e va rispettata: il Task 9 riscrive
le stesse righe in cui il Task 7 mette `activeRange()` e `msg.start`.

**Firme.** Gli helper esistenti sono elencati in cima con la firma verificata nel
repo — in particolare `newTestModel()` **senza parametri** e il campo
`m.filtersScreen`. `newFilters` deriva le opzioni dalle **entries**, ed è per
questo che il Task 8 porta la sua `filtersFixture`. Nessun nome definito in un
task è usato con un'ortografia diversa in un altro.

**RED osservabili.** Task 2, 5, 6, 7 e 8 separano i test comportamentali (che
girano contro il codice attuale) da quelli che nominano simboli nuovi, perché un
`FAIL [build failed]` non è un RED. Task 1 è l'eccezione dichiarata: là il
build-failure è tutto ciò che può esistere, non c'è difetto da esercitare.

**Larghezze.** Ogni larghezza asserita nei test viene dallo sweep misurato del
Task 5 e dalla soglia misurata del Task 6. Nessun test pretende una larghezza
sotto il minimo del codice corretto: era l'errore che rendeva impossibili tre
test della versione precedente di questo piano.

**Placeholder.** Nessuno nel codice da scrivere. Tre punti rimandano
deliberatamente alla verifica dell'implementatore contro il repo, e lo dicono: il
pattern httptest di `internal/clickup` (Task 9 Step 1), i numeri 2 e 4 di
`filtersCursorRow` (Task 8 Step 1), e il conteggio esatto delle righe nel RED del
Task 8 Step 2. Sono verifiche, non decisioni lasciate aperte.
