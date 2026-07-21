package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// homeModel is stateless: month/year/scope live on the root Model (single source
// of truth); the view receives them as arguments.
type homeModel struct{}

func newHome() homeModel { return homeModel{} }

func (m Model) updateHome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "h":
		m.month--
		if m.month < time.January {
			m.month = time.December
			m.year--
		}
	case "right", "l":
		m.month++
		if m.month > time.December {
			m.month = time.January
			m.year++
		}
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
		return m, loadMembersCmd(m.client, m.cfg.WorkspaceID)
	case "enter":
		m.screen = screenLoading
		// loadEntriesCmd derives the team assignees on its own when scope=="team".
		return m, m.reloadEntriesCmd()
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

func (homeModel) view(year int, month time.Month, scope, membersNote string) string {
	title := styleTitle.Render("ClickUp Hours — Monthly report")
	scopeStr := styleAccent.Render(scope)
	if membersNote != "" {
		scopeStr += " · " + membersNote
	}
	sel := styleBox.Render(fmt.Sprintf("Month: %s  ◂ %04d-%02d ▸    Scope: %s",
		styleAccent.Render(month.String()), year, int(month), scopeStr))
	help := "◂/▸ change month · t: me/team · "
	if scope == "team" {
		help += "f: select members · " // only active in team scope
	}
	help += "Enter: generate report · n: log hours · q: quit"
	return title + "\n\n" + sel + "\n\n" + styleHelp.Render(help)
}
