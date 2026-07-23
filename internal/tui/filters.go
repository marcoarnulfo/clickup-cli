package tui

import (
	"slices"

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
	switch msg.String() {
	case "tab":
		fs.sec = (fs.sec + 1) % len(fs.sections)
		fs.row = 0
	case "shift+tab":
		fs.sec = (fs.sec - 1 + len(fs.sections)) % len(fs.sections)
		fs.row = 0
	case "up", "k":
		if fs.row > 0 {
			fs.row--
		}
	case "down", "j":
		if fs.row < len(cur.options)-1 {
			fs.row++
		}
	case " ", "space":
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
	case "a":
		if cur.radio {
			break // "all/none" doesn't apply to a single-choice dimension
		}
		all := allChosen(*cur)
		for _, o := range cur.options {
			cur.selected[o] = !all
		}
	case "enter":
		m.filterLists = fs.sections[0].selected
		m.filterTags = fs.sections[1].selected
		m.filterStatuses = fs.sections[2].selected
		m.filterBillable = billableFromSelection(fs.sections[3].selected)
		m.filtersScreen = fs
		if m.applyReport() {
			m.screen = screenReport
		}
		return m, nil
	case "esc":
		m.screen = screenReport
		return m, nil
	}
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

func (fs filtersModel) view() string {
	if fs.loadingStatuses {
		return styleTitle.Render("Loading statuses…")
	}
	b := styleTitle.Render("Filters") + "\n\n"
	for si, sec := range fs.sections {
		head := sec.title
		if si == fs.sec {
			head = styleAccent.Render("▸ " + sec.title)
		} else {
			head = "  " + head
		}
		b += head + "\n"
		if len(sec.options) == 0 {
			b += "    " + styleHelp.Render("(none)") + "\n"
		}
		for ri, o := range sec.options {
			box := "[ ]"
			if sec.selected[o] {
				box = "[x]"
			}
			line := "    " + box + " " + o
			if si == fs.sec && ri == fs.row {
				line = "    " + box + " " + styleAccent.Render(o)
			}
			b += line + "\n"
		}
	}
	b += "\n" + styleHelp.Render("Tab/⇧Tab section · ↑/↓ move · Space toggle · a: all/none · Enter: apply · Esc: cancel")
	return b
}
