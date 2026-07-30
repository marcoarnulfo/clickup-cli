package tui

import (
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/marcoarnulfo/clickup-cli/internal/fuzzy"
)

// Geometry. See the spec's section 8.2.
const (
	paletteWidth    = 52 // preferred box width, borders included
	paletteMinWidth = 24 // never narrower, even on a narrow terminal
	paletteMaxRows  = 8  // action rows visible at once
	paletteChrome   = 4  // top border + query row + separator + bottom border
	paletteTopY     = 2  // body row the top border sits on
	paletteTitle    = "Command palette"
)

// paletteItem is one filtered row: the action, the rune indices fuzzy.Match
// hit (which drive the highlighting, not just the ranking) and its score.
type paletteItem struct {
	a     action
	idx   []int
	score int
}

// paletteModel is the command palette's state (#71).
//
// The query is a plain string with append and backspace rather than a
// textinput.Model: that type derives its styles from lipgloss's default
// renderer — the same reason footer.go refuses help.New() — and would add three
// fields of editing state a twenty-item filter never uses.
//
// There is no cached copy of every action. refreshPalette rebuilds from
// paletteActions on each keystroke: a fuzzy match over twenty short strings
// costs nothing, and a copy that is never kept cannot go stale.
type paletteModel struct {
	query string
	items []paletteItem
	idx   int // selected row
	top   int // first visible row
}

// openPalette raises the overlay. It does not touch m.nav: an overlay is not a
// place you navigated to, so closing it is not a pop().
//
// helpAll is cleared because the expanded footer is several lines tall, and
// paletteRows subtracts a fixed two rows for the blank line and the footer.
// Dismissing it is also right on its own terms: the palette and full help
// answer the same question, and showing both at once answers it twice.
func (m Model) openPalette() Model {
	m.overlay = overlayPalette
	m.helpAll = false
	m.palette = paletteModel{}
	return m.refreshPalette()
}

func (m Model) closePalette() Model {
	m.overlay = overlayNone
	m.palette = paletteModel{}
	return m
}

// refreshPalette rebuilds the filtered, ranked list for the current query.
func (m Model) refreshPalette() Model {
	all := paletteActions(m)
	items := make([]paletteItem, 0, len(all))
	for _, a := range all {
		score, idx, ok := fuzzy.Match(m.palette.query, a.label)
		if !ok {
			continue
		}
		items = append(items, paletteItem{a: a, idx: idx, score: score})
	}
	// Stable, so an empty query (every score 0) keeps paletteActions' order:
	// screen commands above the navigation rows.
	slices.SortStableFunc(items, func(a, b paletteItem) int { return b.score - a.score })
	m.palette.items = items
	m.palette.idx = 0
	m.palette.top = 0
	return m
}

// updateOverlay handles every key while an overlay is open. Update routes here
// before it checks Quit, so nothing below can be reached by a query character.
func (m Model) updateOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := keysFor(m) // == paletteKeys: the handler and the footer read one source
	switch {
	case key.Matches(msg, k.Back), key.Matches(msg, k.Palette):
		return m.closePalette(), nil

	case key.Matches(msg, k.Confirm):
		if len(m.palette.items) == 0 {
			return m, nil
		}
		run := m.palette.items[m.palette.idx].a.run
		// Close FIRST: the action changes screen, and a palette still raised
		// would be composited over the screen it just opened.
		return run(m.closePalette())

	case key.Matches(msg, k.Up):
		if m.palette.idx > 0 {
			m.palette.idx--
		}
		m.palette = scrollPalette(m.palette, paletteRows(m.height))
		return m, nil

	case key.Matches(msg, k.Down):
		if m.palette.idx < len(m.palette.items)-1 {
			m.palette.idx++
		}
		m.palette = scrollPalette(m.palette, paletteRows(m.height))
		return m, nil

	// Backspace is text editing, which none of the ten textinput screens
	// advertises either, so it is a type check rather than a keymap binding.
	case msg.Type == tea.KeyBackspace:
		if r := []rune(m.palette.query); len(r) > 0 {
			m.palette.query = string(r[:len(r)-1])
			m = m.refreshPalette()
		}
		return m, nil
	}

	// bubbletea reclassifies a lone space as KeySpace while still carrying it in
	// Runes (key.go:698-702), so a query could not contain a space without this
	// second type. Alt-modified runes are chords, not text.
	if (msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace) && !msg.Alt {
		m.palette.query += string(msg.Runes)
		m = m.refreshPalette()
	}
	return m, nil
}

// scrollPalette moves the visible window so idx is always inside it, on the
// shared idiom in scrollWindow.
func scrollPalette(p paletteModel, rows int) paletteModel {
	p.top = scrollWindow(p.idx, p.top, rows)
	return p
}

// scrollWindow moves a visible window of `rows` rows so idx stays inside it, and
// returns the new top. Shared by the palette and the Filters screen (#28): the
// palette had this from the start, and Filters now uses the same idiom instead
// of a second one.
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

// paletteRows is how many action rows fit. The subtraction accounts for the box
// chrome, the rows above it, and the blank line plus footer View always appends.
// The floor of 3 says the palette shrinks on a short terminal but never vanishes.
func paletteRows(height int) int {
	if height <= 0 {
		return paletteMaxRows
	}
	return max(3, min(paletteMaxRows, height-paletteChrome-paletteTopY-2))
}

// paletteBoxWidth is the box's total width, borders included.
func paletteBoxWidth(width int) int {
	if width <= 0 {
		return paletteWidth
	}
	// On a terminal too narrow even for the floor, the box keeps its 24 columns
	// and overflows to the right: a box that spills is readable, a box squeezed
	// below its floor is not.
	return max(paletteMinWidth, min(paletteWidth, width-4))
}

// layout renders the box and returns the cell its top-left corner goes in.
//
// bodyLines is how many lines the screen underneath occupies, and it is not
// decoration. y is a coordinate in the BODY, but bubbletea's standard renderer
// keeps only the LAST height lines when a view overflows
// (standard_renderer.go:186-188). On a report grouped by task with fifty
// buckets, a box pinned at y=2 scrolls off the top and the user types into a
// palette they cannot see — on exactly the screen where it is most useful. So
// the box moves down by however much the body will be scrolled up.
func (p paletteModel) layout(th theme, width, height, bodyLines int) (string, int, int) {
	boxW := paletteBoxWidth(width)
	x := 0
	if width > boxW {
		x = (width - boxW) / 2
	}
	y := paletteTopY
	if height > 0 {
		if overflow := bodyLines + 2 - height; overflow > 0 { // +2: blank line + footer
			y += overflow
		}
	}
	return p.box(th, boxW, paletteRows(height)), x, y
}

// box builds the frame a line at a time rather than through th.Box.
//
// For an overlay the rendered width is not decoration: it is the number
// composite uses to decide where the body resumes. lipgloss's own border and
// padding arithmetic already cost this package one bug (the report table's
// amputated right border, #66), so here every line is boxW cells wide by
// construction and a test checks it.
func (p paletteModel) box(th theme, boxW, rows int) string {
	innerW := boxW - 4
	var b strings.Builder

	// th.Accent, never th.Title: Title carries MarginBottom(1) and would inject
	// a blank line into the middle of the overlay.
	dashes := boxW - lipgloss.Width(paletteTitle) - 5
	b.WriteString(th.Border.Render("╭─ ") + th.Accent.Render(paletteTitle) +
		th.Border.Render(" "+strings.Repeat("─", dashes)+"╮") + "\n")

	q := truncateWidth("> "+p.query, innerW)
	b.WriteString(paletteLine(th, th.Cell.Render(q), innerW-lipgloss.Width(q)))
	b.WriteString(th.Border.Render("├"+strings.Repeat("─", boxW-2)+"┤") + "\n")

	if len(p.items) == 0 {
		msg := truncateWidth("no matching action", innerW)
		b.WriteString(paletteLine(th, th.Help.Render(msg), innerW-lipgloss.Width(msg)))
	}
	for i := p.top; i < len(p.items) && i < p.top+rows; i++ {
		b.WriteString(paletteLine(th, paletteRow(th, p.items[i], i == p.idx, innerW), 0))
	}

	b.WriteString(th.Border.Render("╰" + strings.Repeat("─", boxW-2) + "╯"))
	return b.String()
}

// paletteLine wraps already-styled content in the side borders, padding it out
// by pad cells so every line is the same width.
func paletteLine(th theme, content string, pad int) string {
	if pad > 0 {
		content += th.Cell.Render(strings.Repeat(" ", pad))
	}
	return th.Border.Render("│ ") + content + th.Border.Render(" │") + "\n"
}

// paletteRow renders one action's inner content, exactly innerW cells wide.
func paletteRow(th theme, it paletteItem, selected bool, innerW int) string {
	cursor := "  "
	style := th.Cell
	if selected {
		cursor = "▸ " // the marker members.go, export.go and filters.go already use
		style = th.Accent
	}

	hintW := lipgloss.Width(it.a.hint)
	labelW := innerW - 2 - hintW
	if hintW > 0 {
		labelW -= 2 // two spaces between the label and the key
	}
	label := truncateWidth(it.a.label, max(labelW, 1))
	gap := innerW - 2 - lipgloss.Width(label) - hintW

	out := style.Render(cursor) + highlight(th, label, it.idx)
	if gap > 0 {
		out += th.Cell.Render(strings.Repeat(" ", gap))
	}
	return out + th.Help.Render(it.a.hint)
}

// highlight renders label with the runes at idx in th.Accent and the rest in
// th.Cell.
//
// idx indexes the FULL label, because that is what fuzzy.Match was given.
// Truncation happens first (highlighting first would leave escape sequences to
// be cut in half), so indices past the shortened label are dropped here, and a
// trailing ellipsis never lights up.
func highlight(th theme, label string, idx []int) string {
	r := []rune(label)
	limit := len(r)
	if limit > 0 && r[limit-1] == '…' {
		limit--
	}
	hit := make(map[int]bool, len(idx))
	for _, i := range idx {
		if i < limit {
			hit[i] = true
		}
	}

	var b strings.Builder
	// Runs, not runes: one Render per stretch keeps the escape sequences down to
	// a handful per row instead of one pair per character.
	for i := 0; i < len(r); {
		j := i
		for j < len(r) && hit[j] == hit[i] {
			j++
		}
		seg := string(r[i:j])
		if hit[i] {
			b.WriteString(th.Accent.Render(seg))
		} else {
			b.WriteString(th.Cell.Render(seg))
		}
		i = j
	}
	return b.String()
}
