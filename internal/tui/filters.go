package tui

import (
	"slices"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// filterSection is one dimension of the Filters screen. radio marks a
// single-choice dimension (exactly one option selected at a time), as opposed
// to the default independent-checkboxes behavior.
type filterSection struct {
	title    string
	options  []string
	selected map[string]bool
	radio    bool
}

// Billable dimension option labels (#51). This maps onto the existing
// report.FilterCriteria.Billable *bool field: "All" -> nil, "Billable only"
// -> &true, "Non-billable only" -> &false. It is deliberately not a private
// pre-filter — see the task's binding note A3.
const (
	billableOptAll = "All"
	billableOptYes = "Billable only"
	billableOptNo  = "Non-billable only"
)

// billableSelection preselects the Billable section from the current
// criteria value, nil meaning "All".
func billableSelection(cur *bool) map[string]bool {
	sel := map[string]bool{billableOptAll: false, billableOptYes: false, billableOptNo: false}
	switch {
	case cur == nil:
		sel[billableOptAll] = true
	case *cur:
		sel[billableOptYes] = true
	default:
		sel[billableOptNo] = true
	}
	return sel
}

// billableFromSelection reads the Billable section's selection back into a
// report.FilterCriteria.Billable value.
func billableFromSelection(sel map[string]bool) *bool {
	if sel[billableOptYes] {
		b := true
		return &b
	}
	if sel[billableOptNo] {
		b := false
		return &b
	}
	return nil
}

type filtersModel struct {
	sections        []filterSection // [Lists, Tags, Statuses, Billable]
	sec             int             // active section index
	row             int             // active row within the section
	top             int             // first visible visual row (#28)
	loadingStatuses bool
}

// newFilters builds the screen from the entries' lists/tags/statuses, preselecting
// from the current criteria (copied defensively so Esc can discard). billable
// preselects the fourth (radio) section; nil means "All".
func newFilters(entries []report.TimeEntry, lists, tags, statuses map[string]bool, billable *bool) filtersModel {
	listOpts := map[string]bool{}
	tagOpts := map[string]bool{}
	statusOpts := map[string]bool{}
	for _, e := range entries {
		if e.ListName != "" {
			listOpts[e.ListName] = true
		}
		for _, t := range e.Tags {
			tagOpts[t] = true
		}
		if e.Status != "" {
			statusOpts[e.Status] = true
		}
	}
	return filtersModel{
		sections: []filterSection{
			{title: "Lists", options: sortedKeys(listOpts), selected: copyBool(lists)},
			{title: "Tags", options: sortedKeys(tagOpts), selected: copyBool(tags)},
			{title: "Statuses", options: sortedKeys(statusOpts), selected: copyBool(statuses)},
			{title: "Billable", options: []string{billableOptAll, billableOptYes, billableOptNo},
				selected: billableSelection(billable), radio: true},
		},
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func copyBool(m map[string]bool) map[string]bool {
	out := make(map[string]bool, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (m Model) updateFilters(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	fs := m.filtersScreen
	if fs.loadingStatuses {
		return m, nil
	}
	cur := &fs.sections[fs.sec]
	k := keysFor(m)
	switch {
	case key.Matches(msg, k.NextField):
		fs.sec = (fs.sec + 1) % len(fs.sections)
		fs.row = 0
	case key.Matches(msg, k.PrevField):
		fs.sec = (fs.sec - 1 + len(fs.sections)) % len(fs.sections)
		fs.row = 0
	case key.Matches(msg, k.Up):
		if fs.row > 0 {
			fs.row--
		}
	case key.Matches(msg, k.Down):
		if fs.row < len(cur.options)-1 {
			fs.row++
		}
	case key.Matches(msg, k.ToggleItem):
		if len(cur.options) > 0 {
			opt := cur.options[fs.row]
			if cur.radio {
				// Exactly one option selected at a time: clear the others.
				for _, o := range cur.options {
					cur.selected[o] = false
				}
				cur.selected[opt] = true
			} else {
				cur.selected[opt] = !cur.selected[opt]
			}
		}
	case key.Matches(msg, k.SelectAll):
		if cur.radio {
			break // "all/none" doesn't apply to a single-choice dimension
		}
		all := allChosen(*cur)
		for _, o := range cur.options {
			cur.selected[o] = !all
		}
	case key.Matches(msg, k.Confirm):
		m.filterLists = fs.sections[0].selected
		m.filterTags = fs.sections[1].selected
		m.filterStatuses = fs.sections[2].selected
		m.filterBillable = billableFromSelection(fs.sections[3].selected)
		m.filtersScreen = fs
		if m.applyReport() {
			m = m.pop()
		}
		return m, nil
	case key.Matches(msg, k.Back):
		m = m.pop()
		return m, nil
	}
	fs.top = scrollWindow(filtersCursorRow(fs), fs.top, filtersContentRows(filtersRows(m.height)))
	m.filtersScreen = fs
	return m, nil
}

// allChosen reports whether every option in a section is selected.
func allChosen(s filterSection) bool {
	if len(s.options) == 0 {
		return false
	}
	for _, o := range s.options {
		if !s.selected[o] {
			return false
		}
	}
	return true
}

// filtersRows is how many lines view() may emit in total, title included. The
// subtraction accounts for the blank line and the footer that View always
// appends; the floor says the screen shrinks but never vanishes.
func filtersRows(height int) int {
	return max(6, height-2)
}

// filtersTitleRows is the title block's physical line cost, measured (not
// assumed): th.Title.Render("Filters") itself renders as TWO lines, not one —
// its MarginBottom(1) pads out a second, space-filled line below "Filters" —
// and view() adds one further blank line after that. Three, not two.
const filtersTitleRows = 3

// filtersContentRows is how many of filtersVisualRows' rows view(th, rows) can
// actually show out of a total budget of rows: rows, minus the title block
// (filtersTitleRows), minus one more for the line-counting convention
// strings.Count(s, "\n")+1 uses (the tests measure rows this way, and it counts
// the string's own trailing newline as one further line). updateFilters must
// scroll to the same window this slices, or the cursor tracks a window wider
// than what actually renders.
func filtersContentRows(rows int) int {
	return max(rows-filtersTitleRows-1, 0)
}

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

// filtersVisualRows renders the screen's CONTENT as one flat slice of visual
// rows: a section header, then its options (or a single "(none)"), for each
// section. The title is not part of the slice. The window in view() is taken
// over THIS slice, so the cursor's index and the rendering can never disagree
// about what row something is on.
func filtersVisualRows(fs filtersModel, th theme) []string {
	rows := make([]string, 0, len(fs.sections)*2)
	for si, sec := range fs.sections {
		head := sec.title
		if si == fs.sec {
			head = th.Accent.Render("▸ " + sec.title)
		} else {
			head = "  " + head
		}
		rows = append(rows, head)
		if len(sec.options) == 0 {
			rows = append(rows, "    "+th.Help.Render("(none)"))
		}
		for ri, o := range sec.options {
			box := "[ ]"
			if sec.selected[o] {
				box = "[x]"
			}
			line := "    " + box + " " + o
			if si == fs.sec && ri == fs.row {
				line = "    " + box + " " + th.Accent.Render(o)
			}
			rows = append(rows, line)
		}
	}
	return rows
}

func (fs filtersModel) view(th theme, rows int) string {
	if fs.loadingStatuses {
		return th.Title.Render("Loading statuses…")
	}
	visual := filtersVisualRows(fs, th)
	window := filtersContentRows(rows)
	top := min(fs.top, max(0, len(visual)-window))
	end := min(len(visual), top+window)

	b := th.Title.Render("Filters") + "\n\n"
	for _, line := range visual[top:end] {
		b += line + "\n"
	}
	return b
}
