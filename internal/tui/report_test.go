package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func TestNextGroupByTeamIncludesMember(t *testing.T) {
	if got := nextGroupBy(report.GroupByDay, "team"); got != report.GroupByMember {
		t.Errorf("team: day -> %q, want member", got)
	}
	if got := nextGroupBy(report.GroupByMember, "team"); got != report.GroupByTotal {
		t.Errorf("team: member -> %q, want total", got)
	}
}

func TestNextGroupByMeSkipsMember(t *testing.T) {
	if got := nextGroupBy(report.GroupByDay, "me"); got != report.GroupByTotal {
		t.Errorf("me: day -> %q, want total", got)
	}
}

// TestReportCycleGroupByTeamViaUpdate drives the 'g' key through Update() to
// verify the team cycle reaches the member grouping.
func TestReportCycleGroupByTeamViaUpdate(t *testing.T) {
	m := Model{scope: "team", screen: screenReport, now: time.Now}
	m.report = report.Report{GroupBy: report.GroupByDay}
	m.rep = newReport(m.report, "")
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m = u.(Model)
	if m.report.GroupBy != report.GroupByMember {
		t.Errorf("team g from day -> %q, want member", m.report.GroupBy)
	}
}

func TestMemberFilterNotePartial(t *testing.T) {
	m := Model{
		scope:           "team",
		teamMembers:     make([]clickup.Member, 3), // 3 members total
		selectedMembers: map[int]bool{1: true, 2: true},
	}
	if got := m.memberFilterNote(); got != " (2/3 members)" {
		t.Errorf("memberFilterNote = %q, want ' (2/3 members)'", got)
	}
}
