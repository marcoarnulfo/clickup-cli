package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
)

func membersFixture() Model {
	mems := []clickup.Member{{ID: 1, Username: "alice"}, {ID: 2, Username: "bob"}}
	sel := map[int]bool{1: true, 2: true}
	return Model{
		screen:          screenMembers,
		teamMembers:     mems,
		selectedMembers: sel,
		membersScreen:   newMembers(mems, sel),
	}
}

func spaceKey() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}} }
func runeKey(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestMembersToggleAndConfirm(t *testing.T) {
	m := membersFixture()
	u, _ := m.updateMembers(spaceKey()) // toggle alice (idx 0) off
	m = u.(Model)
	if m.membersScreen.selected[1] {
		t.Error("alice should be deselected after space")
	}
	u, _ = m.updateMembers(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if m.screen != screenHome {
		t.Errorf("enter should return home, got %v", m.screen)
	}
	if m.selectedMembers[1] {
		t.Error("root selection should reflect alice deselected")
	}
}

func TestMembersAllNone(t *testing.T) {
	m := membersFixture()
	u, _ := m.updateMembers(runeKey("a")) // all selected -> clear
	m = u.(Model)
	if m.membersScreen.selected[1] || m.membersScreen.selected[2] {
		t.Error("'a' with all selected should clear all")
	}
	u, _ = m.updateMembers(runeKey("a")) // none -> select all
	m = u.(Model)
	if !m.membersScreen.selected[1] || !m.membersScreen.selected[2] {
		t.Error("'a' with none selected should select all")
	}
}

func TestMembersEscDiscards(t *testing.T) {
	m := membersFixture()
	u, _ := m.updateMembers(spaceKey()) // toggle alice off (on the copy)
	m = u.(Model)
	u, _ = m.updateMembers(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(Model)
	if m.screen != screenHome {
		t.Errorf("esc should return home, got %v", m.screen)
	}
	if !m.selectedMembers[1] {
		t.Error("esc must discard: root alice still selected")
	}
}

func TestMembersAllNoneEmptyRosterNoPanic(t *testing.T) {
	m := Model{
		screen:        screenMembers,
		membersScreen: newMembers(nil, map[int]bool{}),
	}
	u, _ := m.updateMembers(runeKey("a")) // no members: must be a no-op, not a panic
	m = u.(Model)
	if len(m.membersScreen.selected) != 0 {
		t.Errorf("selected = %v, want empty on an empty roster", m.membersScreen.selected)
	}
}

func TestMembersMsgDefaultsAll(t *testing.T) {
	m := Model{}
	u, _ := m.Update(membersMsg{members: []clickup.Member{{ID: 1, Username: "a"}, {ID: 2, Username: "b"}}})
	m = u.(Model)
	if m.screen != screenMembers {
		t.Errorf("screen = %v, want screenMembers", m.screen)
	}
	if !m.selectedMembers[1] || !m.selectedMembers[2] {
		t.Error("default selection should be all members")
	}
	if len(m.teamMembers) != 2 {
		t.Error("teamMembers should be cached")
	}
}
