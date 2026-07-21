# Workspace List Browser Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the user pick ANY workspace list (not just report ∪ config) in both the guided log-hours picker and the per-list rates screen, via a lazy Space→Folder→List drill-down browser.

**Architecture:** A new ClickUp client returns spaces, and a space's folders (with their lists inline) plus folderless lists. A shared `listBrowserModel` screen drills down lazily (spaces → space contents → folder lists), caches per session, and on selecting a list routes the `(id, name)` back to whoever opened it (log or rates) via an `origin` field.

**Tech Stack:** Go 1.26, bubbletea/lipgloss, `net/http`+`httptest`, standard `testing`.

## Global Constraints

- Go **1.26**; no new third-party dependencies.
- Everything in the repo is written in **English**: identifiers, comments, test messages, UI strings, commit messages. (Design docs under `docs/superpowers/` stay in Italian.)
- `internal/report`/`internal/duration` stay pure (this feature lives in `clickup`/`tui`).
- **Conventional Commits**. **NEVER** add a `Co-Authored-By` trailer.
- Before each commit: `gofmt -l .` (empty), `go vet ./...`, `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`, `go test ./... -race` — all clean/green. staticcheck is a hard gate (a written-but-unread field / a lone const in a used iota group are fine; an unused type/func is not).
- TUI convention: `Model` is a value receiver; write sub-models back explicitly before `return`. Screens are `updateX`/`view`. Async work is a `tea.Cmd` returning a typed msg handled in `Update`. Pointer-receiver methods (`applyReport`, `assignStatuses`) exist as a precedent for helpers that mutate `m`.
- Client: `Authorization: <token>` (no Bearer); `c.get(ctx, path, query, &out)`; `httptest` for tests.

---

## File Structure

- `internal/clickup/space.go` (new) — `Space`/`Folder`/`List` types + `Spaces` + `SpaceContents`.
- `internal/tui/listbrowser.go` (new) — `listBrowserModel`, drill-down `updateListBrowser`/`view`, selection routing.
- `internal/tui/app.go` — `screenListBrowser`, msgs, fetch cmds, cache fields, wiring, `openListBrowser`, `spacesCmd`/`spaceContentsCmd`.
- `internal/tui/log.go` — a "Browse all workspace lists…" entry in the guided picker.
- `internal/tui/rates.go` — a `b` key that opens the browser.
- `internal/tui/demo.go` — `demoSpaces`/`demoSpaceContents` + their cmds.
- Tests alongside; `README.md`/`README.it.md` in the last task.

---

### Task 1: clickup — Spaces + SpaceContents

**Files:**
- Create: `internal/clickup/space.go`, `internal/clickup/space_test.go`

**Interfaces:**
- Produces: `Space{ID,Name}`, `List{ID,Name}`, `Folder{ID,Name,Lists []List}`; `Spaces(ctx, teamID) ([]Space, error)`; `SpaceContents(ctx, spaceID) (folders []Folder, folderless []List, err error)`.

- [ ] **Step 1: Write the failing tests**

Create `internal/clickup/space_test.go`:

```go
package clickup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSpaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/team/900/space" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"spaces":[{"id":"s1","name":"Engineering"},{"id":"s2","name":"Marketing"}]}`))
	}))
	defer srv.Close()
	c := New("tok")
	c.BaseURL = srv.URL
	spaces, err := c.Spaces(context.Background(), "900")
	if err != nil {
		t.Fatal(err)
	}
	if len(spaces) != 2 || spaces[0].ID != "s1" || spaces[1].Name != "Marketing" {
		t.Errorf("spaces = %+v", spaces)
	}
}

func TestSpaceContents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/space/s1/folder":
			_, _ = w.Write([]byte(`{"folders":[{"id":"f1","name":"Backend","lists":[{"id":"l1","name":"API"},{"id":"l2","name":"Auth"}]}]}`))
		case "/space/s1/list":
			_, _ = w.Write([]byte(`{"lists":[{"id":"l9","name":"Roadmap"}]}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	c := New("tok")
	c.BaseURL = srv.URL
	folders, folderless, err := c.SpaceContents(context.Background(), "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 1 || folders[0].Name != "Backend" || len(folders[0].Lists) != 2 || folders[0].Lists[0].Name != "API" {
		t.Errorf("folders = %+v", folders)
	}
	if len(folderless) != 1 || folderless[0].ID != "l9" {
		t.Errorf("folderless = %+v", folderless)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/clickup/ -run 'TestSpaces|TestSpaceContents' -v` → FAIL (`undefined: (*Client).Spaces`).

- [ ] **Step 3: Write minimal implementation**

Create `internal/clickup/space.go`:

```go
package clickup

import "context"

// Space is a ClickUp space within a workspace.
type Space struct {
	ID   string
	Name string
}

// List is a ClickUp list (the leaf that holds tasks/time).
type List struct {
	ID   string
	Name string
}

// Folder is a ClickUp folder within a space; its lists are returned inline.
type Folder struct {
	ID    string
	Name  string
	Lists []List
}

// Spaces returns the spaces of a workspace (GET /team/{id}/space).
func (c *Client) Spaces(ctx context.Context, teamID string) ([]Space, error) {
	var resp struct {
		Spaces []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"spaces"`
	}
	if err := c.get(ctx, "/team/"+teamID+"/space", nil, &resp); err != nil {
		return nil, err
	}
	out := make([]Space, 0, len(resp.Spaces))
	for _, s := range resp.Spaces {
		out = append(out, Space{ID: s.ID, Name: s.Name})
	}
	return out, nil
}

// SpaceContents returns a space's folders (with their lists inline) and its
// folderless lists (GET /space/{id}/folder + GET /space/{id}/list).
func (c *Client) SpaceContents(ctx context.Context, spaceID string) ([]Folder, []List, error) {
	var fresp struct {
		Folders []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Lists []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"lists"`
		} `json:"folders"`
	}
	if err := c.get(ctx, "/space/"+spaceID+"/folder", nil, &fresp); err != nil {
		return nil, nil, err
	}
	folders := make([]Folder, 0, len(fresp.Folders))
	for _, f := range fresp.Folders {
		lists := make([]List, 0, len(f.Lists))
		for _, l := range f.Lists {
			lists = append(lists, List{ID: l.ID, Name: l.Name})
		}
		folders = append(folders, Folder{ID: f.ID, Name: f.Name, Lists: lists})
	}

	var lresp struct {
		Lists []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"lists"`
	}
	if err := c.get(ctx, "/space/"+spaceID+"/list", nil, &lresp); err != nil {
		return nil, nil, err
	}
	folderless := make([]List, 0, len(lresp.Lists))
	for _, l := range lresp.Lists {
		folderless = append(folderless, List{ID: l.ID, Name: l.Name})
	}
	return folders, folderless, nil
}
```

- [ ] **Step 4: Run tests** → `go test ./internal/clickup/ -race` PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/clickup/space.go internal/clickup/space_test.go
git commit -m "feat(clickup): list spaces and space contents (folders + folderless lists)"
```

---

### Task 2: the shared list-browser screen (drill-down + fetch + cache + selection)

**Files:**
- Create: `internal/tui/listbrowser.go`, `internal/tui/listbrowser_test.go`
- Modify: `internal/tui/app.go` (screen const, fields, msgs, cmds, handlers, routeKey, View, helpers), `internal/tui/demo.go` (demo data + cmds)

**Interfaces:**
- Consumes: `clickup.Space/Folder/List`, `clickup.Spaces/SpaceContents` (Task 1), existing `listTasksCmd`, `rateRow`.
- Produces:
  - `screenListBrowser` (screen const); `Model.browserScreen listBrowserModel`; `Model.browserSpaces []clickup.Space`; `Model.browserContents map[string]browserSpaceContents`.
  - `spacesMsg struct{ spaces []clickup.Space }`; `spaceContentsMsg struct{ spaceID string; folders []clickup.Folder; folderless []clickup.List }`.
  - `func (m Model) openListBrowser(origin screen) (Model, tea.Cmd)`; `func (m *Model) selectBrowsedList(id, name string) tea.Cmd`; `func (m Model) spacesCmd() tea.Cmd`; `func (m Model) spaceContentsCmd(spaceID string) tea.Cmd`.
  - `func (m Model) updateListBrowser(msg tea.KeyMsg) (tea.Model, tea.Cmd)`.

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/listbrowser_test.go`:

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
)

func browserFixture(origin screen) Model {
	m := Model{screen: screenListBrowser}
	m.browserContents = map[string]browserSpaceContents{
		"s1": {
			folders:    []clickup.Folder{{ID: "f1", Name: "Backend", Lists: []clickup.List{{ID: "l1", Name: "API"}}}},
			folderless: []clickup.List{{ID: "l9", Name: "Roadmap"}},
		},
	}
	m.browserScreen = listBrowserModel{
		origin: origin,
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
	m.client = clickup.New("tok") // listTasksCmd needs a client
	u, _ := m.updateListBrowser(enter())          // into space contents
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
	m.browserScreen = listBrowserModel{origin: screenLog, loading: true}
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
	hit := Model{browserSpaces: []clickup.Space{{ID: "s1", Name: "Eng"}}}
	u, cmd := hit.openListBrowser(screenRates)
	hit = u
	if hit.screen != screenListBrowser || len(hit.browserScreen.spaces) != 1 || cmd != nil {
		t.Fatalf("cache hit: screen=%v spaces=%d cmd=%v", hit.screen, len(hit.browserScreen.spaces), cmd)
	}
	if hit.browserScreen.origin != screenRates {
		t.Errorf("origin = %v, want rates", hit.browserScreen.origin)
	}
	// cache miss in demo mode: loading + a command that yields spacesMsg.
	miss := Model{demo: true}
	u2, cmd2 := miss.openListBrowser(screenLog)
	miss = u2
	if miss.screen != screenListBrowser || !miss.browserScreen.loading || cmd2 == nil {
		t.Fatalf("cache miss: screen=%v loading=%v cmd=%v", miss.screen, miss.browserScreen.loading, cmd2)
	}
	if _, ok := cmd2().(spacesMsg); !ok {
		t.Error("cache miss should load spaces (demo)")
	}
}

func TestSpaceContentsMsgPopulatesCache(t *testing.T) {
	m := Model{screen: screenListBrowser}
	m.browserScreen = listBrowserModel{origin: screenLog, spaceID: "s1", loading: true, level: browseSpaces}
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
```

- [ ] **Step 2: Run to verify it fails**

Run: `go build ./... 2>&1 | head` → many `undefined:` (browser types/funcs).

- [ ] **Step 3: Write minimal implementation**

Create `internal/tui/listbrowser.go`:

```go
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
		m.ratesScreen = rt
		m.screen = screenRates
		return nil
	}
	// screenLog: enter the normal task-pick flow for the chosen list.
	m.logScreen.loading = true
	m.logScreen.step = logListPick
	m.screen = screenLog
	return listTasksCmd(m.client, id)
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
```

In `internal/tui/app.go`:

(a) Append `screenListBrowser` to the `screen` iota. Also add it to the global `q`-quit exemption in `Update` (the `if msg.String() == "q" && m.screen != screenSetup && m.screen != screenRates && m.screen != screenRange` condition) → append `&& m.screen != screenListBrowser`, so opening the browser from the rates screen (via `b`) doesn't let a stray `q` quit and discard unsaved rate overrides.

(b) Add to the `type (...)` msg block:
```go
	spacesMsg        struct{ spaces []clickup.Space }
	spaceContentsMsg struct {
		spaceID    string
		folders    []clickup.Folder
		folderless []clickup.List
	}
```

(c) Add root fields:
```go
	browserScreen   listBrowserModel
	browserSpaces   []clickup.Space
	browserContents map[string]browserSpaceContents
```

(d) Helpers (near the other cmds):
```go
// spacesCmd / spaceContentsCmd pick the demo or real source.
func (m Model) spacesCmd() tea.Cmd {
	if m.demo {
		return demoSpacesCmd()
	}
	return loadSpacesCmd(m.client, m.cfg.WorkspaceID)
}

func (m Model) spaceContentsCmd(spaceID string) tea.Cmd {
	if m.demo {
		return demoSpaceContentsCmd(spaceID)
	}
	return loadSpaceContentsCmd(m.client, spaceID)
}

// openListBrowser opens the shared list browser on behalf of `origin`.
func (m Model) openListBrowser(origin screen) (Model, tea.Cmd) {
	bs := listBrowserModel{origin: origin}
	m.screen = screenListBrowser
	if len(m.browserSpaces) > 0 {
		bs.spaces = m.browserSpaces
		m.browserScreen = bs
		return m, nil
	}
	bs.loading = true
	m.browserScreen = bs
	return m, m.spacesCmd()
}

// loadSpacesCmd / loadSpaceContentsCmd fetch in the background.
func loadSpacesCmd(c *clickup.Client, teamID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		spaces, err := c.Spaces(ctx, teamID)
		if err != nil {
			return errMsg{err: err}
		}
		return spacesMsg{spaces: spaces}
	}
}

func loadSpaceContentsCmd(c *clickup.Client, spaceID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		folders, folderless, err := c.SpaceContents(ctx, spaceID)
		if err != nil {
			return errMsg{err: err}
		}
		return spaceContentsMsg{spaceID: spaceID, folders: folders, folderless: folderless}
	}
}
```

(e) `Update` handlers:
```go
	case spacesMsg:
		m.browserSpaces = msg.spaces
		bs := m.browserScreen
		bs.spaces = msg.spaces
		bs.loading = false
		bs.level = browseSpaces
		bs.idx = 0
		m.browserScreen = bs
		m.screen = screenListBrowser
		return m, nil

	case spaceContentsMsg:
		if m.browserContents == nil {
			m.browserContents = map[string]browserSpaceContents{}
		}
		m.browserContents[msg.spaceID] = browserSpaceContents{folders: msg.folders, folderless: msg.folderless}
		bs := m.browserScreen
		if bs.spaceID == msg.spaceID {
			bs.folders = msg.folders
			bs.folderless = msg.folderless
			bs.loading = false
			bs.level = browseSpaceContents
			bs.idx = 0
			m.browserScreen = bs
		}
		return m, nil
```

(f) `routeKey`: `case screenListBrowser: return m.updateListBrowser(msg)`.

(g) `View`: `case screenListBrowser: return m.browserScreen.view()`.

In `internal/tui/demo.go`, add:
```go
// demoSpaces / demoSpaceContents are fake workspace data for demo mode.
func demoSpaces() []clickup.Space {
	return []clickup.Space{{ID: "s-web", Name: "Web"}, {ID: "s-mobile", Name: "Mobile"}}
}

func demoSpaceContents(spaceID string) ([]clickup.Folder, []clickup.List) {
	switch spaceID {
	case "s-web":
		return []clickup.Folder{{ID: "f-site", Name: "Website", Lists: []clickup.List{{ID: "web", Name: "Website"}}}},
			[]clickup.List{{ID: "web-misc", Name: "Misc"}}
	case "s-mobile":
		return []clickup.Folder{{ID: "f-app", Name: "App", Lists: []clickup.List{{ID: "mobile", Name: "Mobile app"}}}}, nil
	default:
		return nil, nil
	}
}

func demoSpacesCmd() tea.Cmd {
	return func() tea.Msg { return spacesMsg{spaces: demoSpaces()} }
}

func demoSpaceContentsCmd(spaceID string) tea.Cmd {
	return func() tea.Msg {
		folders, folderless := demoSpaceContents(spaceID)
		return spaceContentsMsg{spaceID: spaceID, folders: folders, folderless: folderless}
	}
}
```

- [ ] **Step 4: Run the full suite** → `go build ./...`, `go test ./... -race`, `gofmt -l .`, `go vet ./...`, `staticcheck ./...` — all clean/green.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/listbrowser.go internal/tui/listbrowser_test.go internal/tui/app.go internal/tui/demo.go
git commit -m "feat(tui): shared workspace list browser (space/folder/list drill-down)"
```

---

### Task 3: log-hours integration — "Browse all workspace lists…" entry

**Files:**
- Modify: `internal/tui/log.go`
- Test: `internal/tui/log_test.go`

**Interfaces:**
- Consumes: `openListBrowser`, `screenListBrowser` (Task 2), existing `logListPick`/`listTasksCmd`.

- [ ] **Step 1: Write the failing test**

Add to `internal/tui/log_test.go`:

```go
func TestLogBrowseEntryOpensBrowser(t *testing.T) {
	m := Model{screen: screenLog, demo: true}
	m.logScreen = newLog([]report.TimeEntry{{ListID: "a", ListName: "A"}}, config.Config{})
	m.logScreen.step = logListPick
	// move down onto the "Browse all…" row (index len(lists) == 1)
	u, _ := m.updateLog(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = u.(Model)
	if m.logScreen.listIdx != 1 {
		t.Fatalf("listIdx = %d, want 1 (browse row)", m.logScreen.listIdx)
	}
	u, _ = m.updateLog(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if m.screen != screenListBrowser || m.browserScreen.origin != screenLog {
		t.Fatalf("browse row should open the browser for log; screen=%v origin=%v", m.screen, m.browserScreen.origin)
	}
}
```

(`log_test.go` already imports `report`, `config`, `tea`.)

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/tui/ -run TestLogBrowseEntryOpensBrowser -v` → FAIL (Enter on the browse row doesn't open the browser yet).

- [ ] **Step 3: Write minimal implementation**

In `internal/tui/log.go` `updateLog`, replace the `case logListPick:` block with a version that has a trailing "Browse" row at index `len(lg.lists)`:

```go
	case logListPick:
		browseIdx := len(lg.lists) // trailing "Browse all workspace lists…" row
		switch msg.String() {
		case "up", "k":
			if lg.listIdx > 0 {
				lg.listIdx--
			}
		case "down", "j":
			if lg.listIdx < browseIdx {
				lg.listIdx++
			}
		case "enter":
			if lg.loading {
				break
			}
			if lg.listIdx == browseIdx {
				m.logScreen = lg
				return m.openListBrowser(screenLog)
			}
			if len(lg.lists) > 0 {
				lg.loading = true
				m.logScreen = lg
				return m, listTasksCmd(m.client, lg.lists[lg.listIdx].id)
			}
		}
		m.logScreen = lg
		return m, nil
```

And in `view`, replace the `case logListPick:` list rendering so the browse row is always shown (drop the old "No known lists" message — the browse row covers the empty case):

```go
	case logListPick:
		if lg.loading {
			b += styleHelp.Render("Loading tasks…") + "\n\n"
		}
		b += "Choose the list:\n\n"
		for i, l := range lg.lists {
			cursor := "  "
			line := l.name
			if i == lg.listIdx {
				cursor = "▸ "
				line = styleAccent.Render(line)
			}
			b += cursor + line + "\n"
		}
		browseLine := "🔍 Browse all workspace lists…"
		if lg.listIdx == len(lg.lists) {
			b += "▸ " + styleAccent.Render(browseLine) + "\n"
		} else {
			b += "  " + browseLine + "\n"
		}
		b += "\n" + styleHelp.Render("↑/↓ select · Enter: open tasks / browse")
```

- [ ] **Step 4: Run the full suite** → all clean/green.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/log.go internal/tui/log_test.go
git commit -m "feat(tui): browse all workspace lists from the guided log picker"
```

---

### Task 4: rates integration — `b` opens the browser

**Files:**
- Modify: `internal/tui/rates.go`
- Test: `internal/tui/rates_test.go` (new if absent, else extend)

**Interfaces:**
- Consumes: `openListBrowser`, `screenListBrowser` (Task 2); `selectBrowsedList`'s rates path already adds/selects a `rateRow`.

- [ ] **Step 1: Write the failing test**

Create/extend `internal/tui/rates_test.go`:

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
)

func TestRatesBOpensBrowser(t *testing.T) {
	m := Model{screen: screenRates, demo: true}
	m.ratesScreen = newRates(nil, config.Config{})
	u, _ := m.updateRates(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	m = u.(Model)
	if m.screen != screenListBrowser || m.browserScreen.origin != screenRates {
		t.Fatalf("'b' should open the browser for rates; screen=%v origin=%v", m.screen, m.browserScreen.origin)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/tui/ -run TestRatesBOpensBrowser -v` → FAIL (`b` does nothing yet).

- [ ] **Step 3: Write minimal implementation**

In `internal/tui/rates.go` `updateRates`, in the NON-editing key switch (the `switch msg.String()` after the `if rt.editing { … }` block), add a `b` case before `case "esc"`:

```go
	case "b":
		m.ratesScreen = rt
		return m.openListBrowser(screenRates)
```

And add `b: browse` to the rates help line in `view` (e.g. `"↑/↓ select · Enter: edit · d: use default · b: browse lists · s: save · Esc: cancel"`).

- [ ] **Step 4: Run the full suite** → all clean/green. (When the browser returns a list for rates, `selectBrowsedList` appends a `rateRow` and selects it — verified by Task 2's `TestBrowserDrillDownToFolderList`.)

- [ ] **Step 5: Commit**

```bash
git add internal/tui/rates.go internal/tui/rates_test.go
git commit -m "feat(tui): browse all workspace lists from the rates screen"
```

---

### Task 5: docs

**Files:**
- Modify: `README.md`, `README.it.md`

- [ ] **Step 1: Update both READMEs**

Document the workspace list browser, in sync EN/IT:
- **Log hours** section: note the guided picker now has a "Browse all workspace lists…" entry that drills down Space → Folder → List, so you can log time on any workspace list (not only recent/config ones).
- **Per-list rates** section / command list: note the `b` key opens the same browser to add a list not yet tracked.
- Optionally a one-line "Workspace list browser" blurb: lazy drill-down, session-cached, `↑/↓` move · `Enter` open/select · `Esc` up / back.
- If there is a pending Roadmap/backlog mention of "picker across all workspace lists", drop it (now shipped).

- [ ] **Step 2: Commit**

```bash
git add README.md README.it.md
git commit -m "docs: document the workspace list browser"
```

---

## Self-Review notes (author)

- **Spec coverage:** client Spaces/SpaceContents with inline folder lists (T1) ✓; shared drill-down browser with lazy fetch + session cache (T2) ✓; selection routing by `origin` to log (task-pick flow) and rates (add/select row) (T2) ✓; log "Browse all…" entry (T3) ✓; rates `b` key (T4) ✓; demo data (T2) ✓; docs (T5) ✓.
- **Build-green / staticcheck ordering:** `screenListBrowser` + `browserScreen`/cache fields + `spacesMsg`/`spaceContentsMsg` + all browser funcs land together in T2, all referenced within T2: `updateListBrowser` via routeKey; `view` via View; the two msg handlers via `Update`; `browserEnter`/`itemCount`/`browserRow`/`selectBrowsedList` internally; `spaceContentsCmd`/`loadSpaceContentsCmd` via `browserEnter`; `spacesCmd`/`loadSpacesCmd`/`openListBrowser` via `TestOpenListBrowserCacheHitAndMiss`; demo cmds via `spacesCmd`/`spaceContentsCmd` + tests. (Without that test `openListBrowser`/`spacesCmd`/`loadSpacesCmd` would be U1000 until T3 — the test keeps T2's staticcheck gate clean.) The two entry points (T3 log, T4 rates) then call `openListBrowser`. No forward references.
- **Type consistency:** `openListBrowser(origin screen) (Model, tea.Cmd)`, `selectBrowsedList(id, name string) tea.Cmd` (pointer receiver, called on addressable local), `spaceContentsMsg{spaceID, folders, folderless}`, `browserSpaceContents{folders, folderless}` used identically across producer/consumer.
- **Watch (flag for reviewer):** `selectBrowsedList` is a pointer receiver like `applyReport`/`assignStatuses` — confirm it's called on an addressable `m` (it is: from `browserEnter`, a value receiver whose local `m` is addressable). The log return path sets `screen=screenLog` + `loading=true` + `step=logListPick` then fires `listTasksCmd`; confirm the existing `taskListMsg` handler advances to `logTaskPick` without needing a screen change. Esc at the top browser level returns to `origin` — confirm both origins (log/rates) render correctly on return.
