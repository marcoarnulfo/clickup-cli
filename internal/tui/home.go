package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type homeModel struct {
	year  int
	month time.Month
	scope string
}

func newHome(year int, month time.Month) homeModel {
	return homeModel{year: year, month: month, scope: "me"}
}

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
	case "enter":
		m.home.year, m.home.month, m.home.scope = m.year, m.month, m.scope
		m.screen = screenLoading
		// loadEntriesCmd ricava da solo gli assignees del team quando scope=="team".
		return m, loadEntriesCmd(m.client, m.cfg.WorkspaceID, m.year, m.month, m.scope)
	}
	m.home.year, m.home.month, m.home.scope = m.year, m.month, m.scope
	return m, nil
}

func (h homeModel) view() string {
	title := styleTitle.Render("ClickUp Hours — Report mensile")
	sel := styleBox.Render(fmt.Sprintf("Mese: %s  ◂ %04d-%02d ▸    Scope: %s",
		styleAccent.Render(h.month.String()), h.year, int(h.month), styleAccent.Render(h.scope)))
	help := styleHelp.Render("◂/▸ cambia mese · t: me/team · Enter: genera report · q: esci")
	return title + "\n\n" + sel + "\n\n" + help
}
