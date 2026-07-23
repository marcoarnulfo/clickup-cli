package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
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

// #38: an inline error routed back to Home must be visible in the view...
func TestHomeViewRendersErrText(t *testing.T) {
	m := homeModel{errText: "Error: boom"}
	out := m.view("This month", "me", "")
	if !strings.Contains(out, "Error: boom") {
		t.Fatalf("home view should render errText, got:\n%s", out)
	}
}

// ...and must not linger once the user retries.
func TestHomeEnterClearsErrText(t *testing.T) {
	m := Model{scope: "me", screen: screenHome, demo: true, now: time.Now}
	m.home = homeModel{errText: "Error: boom"}
	u, _ := m.updateHome(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if m.home.errText != "" {
		t.Fatalf("home.errText should be cleared before dispatching a new load, got %q", m.home.errText)
	}
}

// #4: 'w' switches Home's period to the current ISO week, computed from the
// injected clock via ISOWeek() and the Model's single resolved location —
// never time.Now() and never a second location (binding note).
func TestHomeWTogglesToCurrentISOWeek(t *testing.T) {
	fixedNow := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	m := Model{scope: "me", screen: screenHome, now: func() time.Time { return fixedNow }, loc: time.UTC}
	u, _ := m.updateHome(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	m = u.(Model)

	wantY, wantW := fixedNow.ISOWeek()
	wantStart, wantEnd := report.WeekRange(wantY, wantW, time.UTC)
	gotStart, gotEnd := m.currentRange()
	if !gotStart.Equal(wantStart) || !gotEnd.Equal(wantEnd) {
		t.Errorf("currentRange after w = [%v,%v), want [%v,%v)", gotStart, gotEnd, wantStart, wantEnd)
	}
}

// Pressing 'w' again returns to the month period.
func TestHomeWTogglesBackToMonth(t *testing.T) {
	fixedNow := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	m := Model{
		scope: "me", screen: screenHome, preset: report.PresetThisMonth,
		year: 2026, month: time.July, now: func() time.Time { return fixedNow }, loc: time.UTC,
	}
	u, _ := m.updateHome(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	m = u.(Model)
	u, _ = m.updateHome(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	m = u.(Model)

	wantStart, wantEnd := report.MonthRange(2026, time.July, time.UTC)
	gotStart, gotEnd := m.currentRange()
	if !gotStart.Equal(wantStart) || !gotEnd.Equal(wantEnd) {
		t.Errorf("currentRange after w,w = [%v,%v), want month range [%v,%v)", gotStart, gotEnd, wantStart, wantEnd)
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
