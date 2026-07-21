package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// homeModel è privo di stato: mese/anno/scope vivono sul root Model (unica fonte
// di verità); la view li riceve come argomenti.
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
		// loadEntriesCmd ricava da solo gli assignees del team quando scope=="team".
		return m, loadEntriesCmd(m.client, m.cfg.WorkspaceID, m.year, m.month, m.scope)
	}
	return m, nil
}

func (homeModel) view(year int, month time.Month, scope string) string {
	title := styleTitle.Render("ClickUp Hours — Report mensile")
	sel := styleBox.Render(fmt.Sprintf("Mese: %s  ◂ %04d-%02d ▸    Scope: %s",
		styleAccent.Render(month.String()), year, int(month), styleAccent.Render(scope)))
	help := styleHelp.Render("◂/▸ cambia mese · t: me/team · Enter: genera report · n: log ore · q: esci")
	return title + "\n\n" + sel + "\n\n" + help
}
