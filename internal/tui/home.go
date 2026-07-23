package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// homeModel is mostly stateless: month/year/scope live on the root Model
// (single source of truth); the view receives them as arguments. errText
// holds the message from a retryableErrMsg routed back to Home (#38), shown
// inline until the next load attempt clears it.
type homeModel struct {
	errText string
}

func newHome() homeModel { return homeModel{} }

// periodMode values (#4): periodModeMonth (the zero value) follows the active
// preset/month-year as before; periodModeWeek overrides it with the current
// ISO week (see Model.currentRange).
const (
	periodModeMonth = ""
	periodModeWeek  = "week"
)

func (m Model) updateHome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "h":
		if m.preset != report.PresetThisMonth || m.periodMode == periodModeWeek {
			break
		}
		m.month--
		if m.month < time.January {
			m.month = time.December
			m.year--
		}
	case "right", "l":
		if m.preset != report.PresetThisMonth || m.periodMode == periodModeWeek {
			break
		}
		m.month++
		if m.month > time.December {
			m.month = time.January
			m.year++
		}
	case "w":
		if m.periodMode == periodModeWeek {
			m.periodMode = periodModeMonth
		} else {
			m.periodMode = periodModeWeek
		}
	case "d":
		m.rangeScreen = newRange(m.preset)
		m.screen = screenRange
		return m, nil
	case "t":
		if m.scope == "me" {
			m.scope = "team"
		} else {
			m.scope = "me"
		}
	case "n":
		m.logScreen = newLog(m.entries, m.cfg)
		m.screen = screenLog
	case "f":
		if m.scope != "team" {
			break
		}
		if len(m.teamMembers) > 0 {
			m.membersScreen = newMembers(m.teamMembers, m.selectedMembers)
			m.screen = screenMembers
			return m, nil
		}
		m.membersScreen = membersModel{loading: true}
		m.screen = screenMembers
		if m.demo {
			return m, demoMembersCmd()
		}
		return m, loadMembersCmd(m.client, m.cfg.WorkspaceID, screenHome)
	case "enter":
		m.home.errText = "" // clear any previous inline error before retrying
		m.screen = screenLoading
		// loadEntriesCmd derives the team assignees on its own when scope=="team".
		return m, m.reloadEntriesCmd(screenHome)
	}
	return m, nil
}

// homeMembersNote returns "Members: k/n" for the team scope when members are
// known, else "". An empty selection counts as all (k = n).
func (m Model) homeMembersNote() string {
	if m.scope != "team" || len(m.teamMembers) == 0 {
		return ""
	}
	k := len(m.selectedAssignees())
	if k == 0 {
		k = len(m.teamMembers) // empty selection = all
	}
	return fmt.Sprintf("Members: %d/%d", k, len(m.teamMembers))
}

// rangeLabel returns a short label for the active range shown on Home.
func (m Model) rangeLabel() string {
	start, end := m.currentRange()
	return report.PeriodLabel(start, end)
}

func (m homeModel) view(rangeLabel, scope, membersNote string) string {
	title := styleTitle.Render("ClickUp Hours — Report")
	scopeStr := styleAccent.Render(scope)
	if membersNote != "" {
		scopeStr += " · " + membersNote
	}
	sel := styleBox.Render(fmt.Sprintf("Range: %s    Scope: %s",
		styleAccent.Render(rangeLabel), scopeStr))
	help := "d: range · ◂/▸ change month (this_month only) · w: this week/month · t: me/team · "
	if scope == "team" {
		help += "f: select members · " // only active in team scope
	}
	help += "Enter: generate report · n: log hours · q: quit"
	out := title + "\n\n" + sel + "\n\n" + styleHelp.Render(help)
	if m.errText != "" {
		out += "\n\n" + styleErr.Render(m.errText)
	}
	return out
}
