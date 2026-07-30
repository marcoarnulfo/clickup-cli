package tui

import (
	"slices"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
)

// TestListBrowserKeyLabels pins the label set updateListBrowser accepts at
// every drill-down level (only the action taken differs, not the labels).
// Quit stays off today (see TestQuitBindingPerScreen).
func TestListBrowserKeyLabels(t *testing.T) {
	t.Parallel()
	m := browserFixture(screenLog)
	want := []string{"?", "ctrl+c", "ctrl+p", "down", "enter", "esc", "j", "k", "up"}
	if got := enabledLabels(keysFor(m)); !slices.Equal(got, want) {
		t.Errorf("list browser labels = %v, want %v", got, want)
	}
}

// browserFixture builds a Model on screenListBrowser with nav seeded to the
// chain a real open implies (Home -> Report -> origin), so pop() and the
// selectBrowsedList discriminator (which reads the top of nav) see the same
// parent a live goTo(screenListBrowser) from origin would have pushed.
func browserFixture(origin screen) Model {
	m := Model{screen: screenListBrowser, nav: []screen{screenHome, screenReport, origin}}
	m.browserContents = map[string]browserSpaceContents{
		"s1": {
			folders:    []clickup.Folder{{ID: "f1", Name: "Backend", Lists: []clickup.List{{ID: "l1", Name: "API"}}}},
			folderless: []clickup.List{{ID: "l9", Name: "Roadmap"}},
		},
	}
	m.browserScreen = listBrowserModel{
		level:  browseSpaces,
		spaces: []clickup.Space{{ID: "s1", Name: "Engineering"}},
	}
	return m
}

func enter() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyEnter} }
func esc() tea.KeyMsg   { return tea.KeyMsg{Type: tea.KeyEsc} }

func TestBrowserDrillDownToFolderList(t *testing.T) {
	m := browserFixture(screenRates)
	// spaces -> enter space s1 (contents cached) -> space contents
	u, _ := m.updateListBrowser(enter())
	m = u.(Model)
	if m.browserScreen.level != browseSpaceContents {
		t.Fatalf("level = %v, want space contents", m.browserScreen.level)
	}
	// idx 0 = folder "Backend" -> enter -> folder lists
	u, _ = m.updateListBrowser(enter())
	m = u.(Model)
	if m.browserScreen.level != browseFolderLists || len(m.browserScreen.folderLists) != 1 {
		t.Fatalf("level = %v folderLists = %+v", m.browserScreen.level, m.browserScreen.folderLists)
	}
	// enter the list -> rates origin adds a row and returns to rates
	u, _ = m.updateListBrowser(enter())
	m = u.(Model)
	if m.screen != screenRates {
		t.Fatalf("screen = %v, want rates", m.screen)
	}
	found := false
	for _, row := range m.ratesScreen.rows {
		if row.listID == "l1" {
			found = true
		}
	}
	if !found {
		t.Error("selected list should be added to rates rows")
	}
}

func TestBrowserSelectFolderlessListForLog(t *testing.T) {
	m := browserFixture(screenLog)
	m.client = clickup.New("tok")        // listTasksCmd needs a client
	u, _ := m.updateListBrowser(enter()) // into space contents
	m = u.(Model)
	// idx 1 = folderless list "Roadmap" (after the single folder)
	m.browserScreen.idx = 1
	u, cmd := m.updateListBrowser(enter())
	m = u.(Model)
	if m.screen != screenLog || !m.logScreen.loading {
		t.Fatalf("screen=%v loading=%v, want log+loading", m.screen, m.logScreen.loading)
	}
	if cmd == nil {
		t.Fatal("expected listTasksCmd for the chosen list")
	}
}

func TestBrowserEscGoesUpThenBackToOrigin(t *testing.T) {
	m := browserFixture(screenLog)
	u, _ := m.updateListBrowser(enter()) // spaces -> contents
	m = u.(Model)
	u, _ = m.updateListBrowser(esc()) // contents -> spaces
	m = u.(Model)
	if m.browserScreen.level != browseSpaces {
		t.Fatalf("esc should go up to spaces, got %v", m.browserScreen.level)
	}
	u, _ = m.updateListBrowser(esc()) // spaces -> back to origin (log)
	m = u.(Model)
	if m.screen != screenLog {
		t.Fatalf("esc at top should return to origin, got %v", m.screen)
	}
}

func TestSpacesMsgPopulatesAndDemoCmds(t *testing.T) {
	m := Model{screen: screenListBrowser}
	m.browserScreen = listBrowserModel{loading: true}
	u, _ := m.Update(spacesMsg{spaces: []clickup.Space{{ID: "s1", Name: "Eng"}}})
	m = u.(Model)
	if m.browserScreen.loading || len(m.browserScreen.spaces) != 1 || len(m.browserSpaces) != 1 {
		t.Errorf("spacesMsg did not populate/cache: %+v", m.browserScreen)
	}
	if _, ok := demoSpacesCmd()().(spacesMsg); !ok {
		t.Error("demoSpacesCmd should produce spacesMsg")
	}
	if _, ok := demoSpaceContentsCmd("s1")().(spaceContentsMsg); !ok {
		t.Error("demoSpaceContentsCmd should produce spaceContentsMsg")
	}
}

func TestOpenListBrowserCacheHitAndMiss(t *testing.T) {
	// cache hit: browserSpaces already populated -> opens directly, no command.
	// openListBrowser no longer takes an origin: it's whatever screen the
	// Model was on (goTo pushes it), so the fixture is ON that screen.
	hit := Model{screen: screenRates, browserSpaces: []clickup.Space{{ID: "s1", Name: "Eng"}}}
	u, cmd := hit.openListBrowser()
	hit = u
	if hit.screen != screenListBrowser || len(hit.browserScreen.spaces) != 1 || cmd != nil {
		t.Fatalf("cache hit: screen=%v spaces=%d cmd=%v", hit.screen, len(hit.browserScreen.spaces), cmd)
	}
	if len(hit.nav) == 0 || hit.nav[len(hit.nav)-1] != screenRates {
		t.Errorf("nav = %v, want top screenRates", hit.nav)
	}
	// cache miss in demo mode: loading + a command that yields spacesMsg.
	miss := Model{screen: screenLog, demo: true}
	u2, cmd2 := miss.openListBrowser()
	miss = u2
	if miss.screen != screenListBrowser || !miss.browserScreen.loading || cmd2 == nil {
		t.Fatalf("cache miss: screen=%v loading=%v cmd=%v", miss.screen, miss.browserScreen.loading, cmd2)
	}
	if len(miss.nav) == 0 || miss.nav[len(miss.nav)-1] != screenLog {
		t.Errorf("nav = %v, want top screenLog", miss.nav)
	}
	if _, ok := cmd2().(spacesMsg); !ok {
		t.Error("cache miss should load spaces (demo)")
	}
}

func TestSpaceContentsMsgPopulatesCache(t *testing.T) {
	m := Model{screen: screenListBrowser}
	m.browserScreen = listBrowserModel{spaceID: "s1", loading: true, level: browseSpaces}
	u, _ := m.Update(spaceContentsMsg{
		spaceID:    "s1",
		folders:    []clickup.Folder{{ID: "f1", Name: "F", Lists: []clickup.List{{ID: "l1", Name: "L"}}}},
		folderless: []clickup.List{{ID: "l9", Name: "R"}},
	})
	m = u.(Model)
	if _, ok := m.browserContents["s1"]; !ok {
		t.Error("spaceContentsMsg should cache the contents")
	}
	if m.browserScreen.level != browseSpaceContents || m.browserScreen.loading {
		t.Errorf("browser not advanced: level=%v loading=%v", m.browserScreen.level, m.browserScreen.loading)
	}
	if len(m.browserScreen.folders) != 1 || len(m.browserScreen.folderless) != 1 {
		t.Errorf("contents not applied: %+v", m.browserScreen)
	}
}
