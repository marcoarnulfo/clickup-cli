package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
)

// membersModel is the team-member selection screen. Its `selected` set is a
// defensive copy of the root's, so Esc can discard changes without touching root.
type membersModel struct {
	members  []clickup.Member
	selected map[int]bool
	idx      int
	loading  bool
}

// newMembers builds the screen from the workspace members and the current
// selection (copied defensively).
func newMembers(members []clickup.Member, selected map[int]bool) membersModel {
	sel := make(map[int]bool, len(selected))
	for id, on := range selected {
		sel[id] = on
	}
	return membersModel{members: members, selected: sel}
}

// allSelected reports whether every member is currently selected.
func (mm membersModel) allSelected() bool {
	if len(mm.members) == 0 {
		return false
	}
	for _, mem := range mm.members {
		if !mm.selected[mem.ID] {
			return false
		}
	}
	return true
}

func (m Model) updateMembers(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	mm := m.membersScreen
	if mm.loading {
		return m, nil
	}
	switch msg.String() {
	case "up", "k":
		if mm.idx > 0 {
			mm.idx--
		}
	case "down", "j":
		if mm.idx < len(mm.members)-1 {
			mm.idx++
		}
	case " ", "space":
		if len(mm.members) > 0 {
			id := mm.members[mm.idx].ID
			mm.selected[id] = !mm.selected[id]
		}
	case "a":
		on := !mm.allSelected() // all selected -> clear; else select all
		for _, mem := range mm.members {
			mm.selected[mem.ID] = on
		}
	case "enter":
		m.selectedMembers = mm.selected
		m.membersScreen = mm
		m.screen = screenHome
		return m, nil
	case "esc":
		m.screen = screenHome // discard: don't write mm back to root
		return m, nil
	}
	m.membersScreen = mm
	return m, nil
}

func (mm membersModel) view() string {
	if mm.loading {
		return styleTitle.Render("Loading members…")
	}
	b := styleTitle.Render("Team members") + "\n\n"
	if len(mm.members) == 0 {
		b += styleHelp.Render("No members in this workspace.") + "\n"
	}
	for i, mem := range mm.members {
		box := "[ ]"
		if mm.selected[mem.ID] {
			box = "[x]"
		}
		cursor := "  "
		line := box + " " + mem.Username
		if i == mm.idx {
			cursor = "▸ "
			line = styleAccent.Render(line)
		}
		b += cursor + line + "\n"
	}
	b += "\n" + styleHelp.Render("↑/↓ move · Space toggle · a: all/none · Enter: confirm · Esc: cancel")
	return b
}
