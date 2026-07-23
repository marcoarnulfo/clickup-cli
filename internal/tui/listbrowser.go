package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
)

type browseLevel int

const (
	browseSpaces browseLevel = iota
	browseSpaceContents
	browseFolderLists
)

// browserSpaceContents is the cached content of one space.
type browserSpaceContents struct {
	folders    []clickup.Folder
	folderless []clickup.List
}

// listBrowserModel is the shared Space→Folder→List drill-down. origin records
// who opened it, so a selected list is routed back to the right caller.
type listBrowserModel struct {
	origin  screen // screenLog | screenRates
	level   browseLevel
	idx     int
	loading bool

	spaces []clickup.Space

	spaceID    string
	spaceName  string
	folders    []clickup.Folder
	folderless []clickup.List

	folderName  string
	folderLists []clickup.List
}

// itemCount is the number of rows at the current level.
func (bs listBrowserModel) itemCount() int {
	switch bs.level {
	case browseSpaces:
		return len(bs.spaces)
	case browseSpaceContents:
		return len(bs.folders) + len(bs.folderless)
	default: // browseFolderLists
		return len(bs.folderLists)
	}
}

func (m Model) updateListBrowser(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	bs := m.browserScreen
	if bs.loading {
		return m, nil
	}
	switch msg.String() {
	case "up", "k":
		if bs.idx > 0 {
			bs.idx--
		}
	case "down", "j":
		if bs.idx < bs.itemCount()-1 {
			bs.idx++
		}
	case "enter":
		return m.browserEnter(bs)
	case "esc":
		switch bs.level {
		case browseFolderLists:
			bs.level = browseSpaceContents
			bs.idx = 0
		case browseSpaceContents:
			bs.level = browseSpaces
			bs.idx = 0
		default: // browseSpaces -> back to whoever opened the browser
			m.screen = bs.origin
			return m, nil
		}
	}
	m.browserScreen = bs
	return m, nil
}

// browserEnter handles Enter at the current level (drill down or select a list).
func (m Model) browserEnter(bs listBrowserModel) (tea.Model, tea.Cmd) {
	switch bs.level {
	case browseSpaces:
		if bs.idx >= len(bs.spaces) {
			return m, nil
		}
		sp := bs.spaces[bs.idx]
		bs.spaceID, bs.spaceName = sp.ID, sp.Name
		if c, ok := m.browserContents[sp.ID]; ok {
			bs.folders, bs.folderless = c.folders, c.folderless
			bs.level = browseSpaceContents
			bs.idx = 0
			m.browserScreen = bs
			return m, nil
		}
		bs.loading = true
		m.browserScreen = bs
		return m, m.spaceContentsCmd(sp.ID)
	case browseSpaceContents:
		if bs.idx < len(bs.folders) {
			f := bs.folders[bs.idx]
			bs.folderName, bs.folderLists = f.Name, f.Lists
			bs.level = browseFolderLists
			bs.idx = 0
			m.browserScreen = bs
			return m, nil
		}
		li := bs.idx - len(bs.folders)
		if li >= len(bs.folderless) {
			return m, nil
		}
		l := bs.folderless[li]
		m.browserScreen = bs
		cmd := m.selectBrowsedList(l.ID, l.Name)
		return m, cmd
	default: // browseFolderLists
		if bs.idx >= len(bs.folderLists) {
			return m, nil
		}
		l := bs.folderLists[bs.idx]
		m.browserScreen = bs
		cmd := m.selectBrowsedList(l.ID, l.Name)
		return m, cmd
	}
}

// selectBrowsedList routes a chosen list back to whoever opened the browser.
func (m *Model) selectBrowsedList(id, name string) tea.Cmd {
	if m.browserScreen.origin == screenRates {
		rt := m.ratesScreen
		found := -1
		for i, row := range rt.rows {
			if row.listID == id {
				found = i
				break
			}
		}
		if found < 0 {
			rt.rows = append(rt.rows, rateRow{listID: id, name: name})
			found = len(rt.rows) - 1
		}
		rt.idx = found
		rt.sec = secLists // the browsed list is a row of the Lists section
		m.ratesScreen = rt
		m.screen = screenRates
		return nil
	}
	// screenLog: enter the normal task-pick flow for the chosen list.
	m.logScreen.loading = true
	m.logScreen.step = logListPick
	m.screen = screenLog
	return m.tasksCmd(id)
}

func (bs listBrowserModel) view() string {
	if bs.loading {
		return styleTitle.Render("Loading…")
	}
	b := styleTitle.Render("Browse workspace lists") + "\n"
	switch bs.level {
	case browseSpaces:
		b += styleHelp.Render("Spaces") + "\n\n"
		for i, s := range bs.spaces {
			b += browserRow(s.Name, i == bs.idx)
		}
	case browseSpaceContents:
		b += styleHelp.Render(bs.spaceName) + "\n\n"
		row := 0
		for _, f := range bs.folders {
			b += browserRow("📁 "+f.Name, row == bs.idx)
			row++
		}
		for _, l := range bs.folderless {
			b += browserRow("🗒 "+l.Name, row == bs.idx)
			row++
		}
	default: // browseFolderLists
		b += styleHelp.Render(bs.spaceName+" / "+bs.folderName) + "\n\n"
		for i, l := range bs.folderLists {
			b += browserRow(l.Name, i == bs.idx)
		}
	}
	if bs.itemCount() == 0 {
		b += styleHelp.Render("(empty)") + "\n"
	}
	b += "\n" + styleHelp.Render("↑/↓ move · Enter: open/select · Esc: up / back")
	return b
}

func browserRow(label string, sel bool) string {
	if sel {
		return "▸ " + styleAccent.Render(label) + "\n"
	}
	return "  " + label + "\n"
}
