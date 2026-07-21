package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
)

func TestHomeFOpensMembersInTeam(t *testing.T) {
	m := Model{scope: "team", screen: screenHome, demo: true}
	u, cmd := m.updateHome(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	m = u.(Model)
	if m.screen != screenMembers {
		t.Fatalf("f in team should open members, got %v", m.screen)
	}
	if cmd == nil {
		t.Fatal("expected a command to load members")
	}
	if _, ok := cmd().(membersMsg); !ok {
		t.Fatal("expected membersMsg from the load command")
	}
}

func TestHomeFNoopInMe(t *testing.T) {
	m := Model{scope: "me", screen: screenHome}
	u, _ := m.updateHome(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	m = u.(Model)
	if m.screen != screenHome {
		t.Errorf("f in me scope should be a no-op, got %v", m.screen)
	}
}

func TestHomeFUsesCache(t *testing.T) {
	mems := []clickup.Member{{ID: 1, Username: "a"}}
	m := Model{scope: "team", screen: screenHome, teamMembers: mems, selectedMembers: map[int]bool{1: true}}
	u, cmd := m.updateHome(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	m = u.(Model)
	if m.screen != screenMembers {
		t.Fatalf("expected members screen")
	}
	if cmd != nil {
		t.Error("cached members should not trigger a load command")
	}
	if len(m.membersScreen.members) != 1 {
		t.Error("members screen should use cached members")
	}
}

func TestHomeMembersNote(t *testing.T) {
	mems := []clickup.Member{{ID: 1}, {ID: 2}, {ID: 3}}
	m := Model{scope: "team", teamMembers: mems, selectedMembers: map[int]bool{1: true, 2: true}}
	if got := m.homeMembersNote(); got != "Members: 2/3" {
		t.Errorf("homeMembersNote = %q, want Members: 2/3", got)
	}
	m.scope = "me"
	if got := m.homeMembersNote(); got != "" {
		t.Errorf("me scope note = %q, want empty", got)
	}
}
