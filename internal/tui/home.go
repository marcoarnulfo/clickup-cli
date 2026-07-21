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
	case "enter":
		m.screen = screenLoading
		// loadEntriesCmd derives the team assignees on its own when scope=="team".
		return m, m.reloadEntriesCmd()
	}
	return m, nil
}

func (homeModel) view(year int, month time.Month, scope string) string {
	title := styleTitle.Render("ClickUp Hours — Monthly report")
	sel := styleBox.Render(fmt.Sprintf("Month: %s  ◂ %04d-%02d ▸    Scope: %s",
		styleAccent.Render(month.String()), year, int(month), styleAccent.Render(scope)))
	help := styleHelp.Render("◂/▸ change month · t: me/team · Enter: generate report · n: log hours · q: quit")
	return title + "\n\n" + sel + "\n\n" + help
}
