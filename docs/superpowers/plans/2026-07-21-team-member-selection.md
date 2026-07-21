# Team Member Selection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** In the `team` scope, let the user multi-select which workspace members to include in the report, and add a per-member grouping.

**Architecture:** A new dedicated `screenMembers` reached from Home with `f` (team scope only). Member selection lives on the root `Model` for the session (`teamMembers` cache + `selectedMembers` set, empty = all). Entry loading passes the selected member ids as `assignee` filters. The pure `report` package gains a `GroupByMember` mode; the TUI's `nextGroupBy` becomes scope-aware.

**Tech Stack:** Go 1.26, bubbletea/lipgloss (Charm), `net/http`+`httptest` for the client, standard `testing` (table-driven).

## Global Constraints

- Go **1.26**; no new third-party dependencies.
- Everything in the repo is written in **English**: identifiers, comments, test messages, UI strings, commit messages. (Design docs under `docs/superpowers/` stay in Italian.)
- `internal/report` and `internal/duration` stay **pure**: no I/O, no imports of `config`/`clickup`.
- **Conventional Commits**. **NEVER** add `Co-Authored-By` to commit messages.
- Before each commit: `gofmt -l .` (empty), `go vet ./...`, `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`, `go test ./... -race` — all clean/green.
- TUI convention: `Model` is a value receiver; write-back sub-models explicitly (`m.sub = x`) before `return`. Screens are `updateX(msg)`/`view()`. Async work runs as `tea.Cmd` returning typed msgs handled in `Update`.
- Semantics **empty selection = all members** (no filter) must hold consistently across Home, `reloadEntriesCmd`, and `loadEntriesCmd`.

---

## File Structure

- `internal/report/aggregate.go` — add `GroupByMember` const + `keyFor` case (pure).
- `internal/clickup/teams.go` — add `TeamMembers(ctx, teamID)`.
- `internal/tui/app.go` — root state (`teamMembers`, `selectedMembers`, `membersScreen`), `screenMembers` const, `membersMsg` type, `selectedAssignees`, `loadEntriesCmd(assignees)`, `reloadEntriesCmd`, `loadMembersCmd`, `membersMsg` handler, routeKey/View wiring.
- `internal/tui/members.go` — new: `membersModel`, `newMembers`, `updateMembers`, `view`.
- `internal/tui/home.go` — `f` key + member count in the Home view + `homeMembersNote`.
- `internal/tui/report.go` — scope-aware `nextGroupBy`, `newReport(note)`, `reportModel.note`, `memberFilterNote`.
- `internal/tui/demo.go` — multi-user demo entries + `demoMembers`/`demoMembersCmd`.
- Tests alongside each file; `README.md`/`README.it.md` docs update in the last task.

---

### Task 1: report — GroupByMember grouping (pure)

**Files:**
- Modify: `internal/report/aggregate.go`
- Test: `internal/report/aggregate_test.go`

**Interfaces:**
- Consumes: existing `Build`, `TimeEntry.UserName`.
- Produces: `report.GroupByMember = "member"`; `Build(entries, "member", ...)` groups by `UserName`.

- [ ] **Step 1: Write the failing test**

Add to `internal/report/aggregate_test.go`:

```go
func TestBuildGroupByMember(t *testing.T) {
	entries := []TimeEntry{
		{UserName: "alice", ListID: "l1", Duration: 2 * time.Hour},
		{UserName: "bob", ListID: "l1", Duration: 1 * time.Hour},
		{UserName: "alice", ListID: "l1", Duration: 30 * time.Minute},
	}
	r := Build(entries, GroupByMember, Rates{Default: 10}, "EUR", 2026, time.July)
	if len(r.Buckets) != 2 {
		t.Fatalf("buckets = %d, want 2", len(r.Buckets))
	}
	if r.Buckets[0].Label != "alice" || r.Buckets[0].Hours != 2.5 {
		t.Errorf("bucket[0] = %+v, want alice 2.5", r.Buckets[0])
	}
	if r.Buckets[1].Label != "bob" || r.Buckets[1].Hours != 1 {
		t.Errorf("bucket[1] = %+v, want bob 1", r.Buckets[1])
	}
}
```

(If `time` is not yet imported in the test file, add it.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/report/ -run TestBuildGroupByMember -v`
Expected: FAIL — `undefined: GroupByMember`.

- [ ] **Step 3: Write minimal implementation**

In `internal/report/aggregate.go`, add the constant to the grouping block:

```go
const (
	GroupByTask   = "task"
	GroupByList   = "list"
	GroupByDay    = "day"
	GroupByMember = "member"
	GroupByTotal  = "total"
)
```

And add the case to `keyFor`:

```go
func keyFor(e TimeEntry, groupBy string) string {
	switch groupBy {
	case GroupByTask:
		return e.TaskName
	case GroupByList:
		return e.ListName
	case GroupByDay:
		return e.Start.Format("2006-01-02")
	case GroupByMember:
		return e.UserName
	default:
		return "Total"
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/report/ -race` → PASS (all report tests).

- [ ] **Step 5: Commit**

```bash
git add internal/report/aggregate.go internal/report/aggregate_test.go
git commit -m "feat(report): add per-member grouping"
```

---

### Task 2: clickup — TeamMembers

**Files:**
- Modify: `internal/clickup/teams.go`
- Test: `internal/clickup/teams_test.go` (new)

**Interfaces:**
- Consumes: existing `Client.Teams`, `Member`.
- Produces: `func (c *Client) TeamMembers(ctx context.Context, teamID string) ([]Member, error)` — members of the given workspace; error if not found/accessible.

- [ ] **Step 1: Write the failing test**

Create `internal/clickup/teams_test.go`:

```go
package clickup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTeamMembers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"teams":[
			{"id":"900","name":"Acme","members":[{"user":{"id":1,"username":"alice"}},{"user":{"id":2,"username":"bob"}}]},
			{"id":"901","name":"Other","members":[{"user":{"id":9,"username":"zoe"}}]}
		]}`))
	}))
	defer srv.Close()
	c := New("tok")
	c.BaseURL = srv.URL

	members, err := c.TeamMembers(context.Background(), "900")
	if err != nil {
		t.Fatalf("TeamMembers error: %v", err)
	}
	if len(members) != 2 || members[0].Username != "alice" || members[1].ID != 2 {
		t.Errorf("members = %+v", members)
	}

	if _, err := c.TeamMembers(context.Background(), "404"); err == nil {
		t.Error("expected error for unknown workspace")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/clickup/ -run TestTeamMembers -v`
Expected: FAIL — `undefined: (*Client).TeamMembers`.

- [ ] **Step 3: Write minimal implementation**

In `internal/clickup/teams.go`, add `"fmt"` to the imports and append:

```go
// TeamMembers returns the members of the given workspace (team) id.
// It errors if the workspace is not accessible with the token.
func (c *Client) TeamMembers(ctx context.Context, teamID string) ([]Member, error) {
	teams, err := c.Teams(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range teams {
		if t.ID == teamID {
			return t.Members, nil
		}
	}
	return nil, fmt.Errorf("workspace %s not found or not accessible with this token", teamID)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/clickup/ -race` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/clickup/teams.go internal/clickup/teams_test.go
git commit -m "feat(clickup): add TeamMembers for a single workspace"
```

---

### Task 3: root state + assignee-aware entry loading

**Files:**
- Modify: `internal/tui/app.go`
- Test: `internal/tui/app_test.go`

**Interfaces:**
- Consumes: `clickup.TeamMembers` (Task 2), `clickup.Member`.
- Produces:
  - `screenMembers` (new `screen` const, appended to the iota).
  - `membersMsg struct{ members []clickup.Member }`.
  - `Model.teamMembers []clickup.Member`, `Model.selectedMembers map[int]bool`.
  - `func (m Model) selectedAssignees() []int` (sorted; empty = all).
  - `loadEntriesCmd(c, teamID, year, month, scope string, assignees []int) tea.Cmd`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/tui/app_test.go`:

```go
func TestSelectedAssignees(t *testing.T) {
	m := Model{selectedMembers: map[int]bool{3: true, 1: false, 2: true}}
	got := m.selectedAssignees()
	if len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Errorf("selectedAssignees = %v, want [2 3]", got)
	}
}

func TestLoadEntriesTeamExplicitAssignees(t *testing.T) {
	teamCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/time_entries"):
			if got := r.URL.Query().Get("assignee"); got != "7,9" {
				t.Errorf("assignee = %q, want 7,9", got)
			}
			w.Write([]byte(`{"data":[]}`))
		case strings.HasSuffix(r.URL.Path, "/team"):
			teamCalled = true
			w.Write([]byte(`{"teams":[]}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	c := clickup.New("tok")
	c.BaseURL = srv.URL

	msg := loadEntriesCmd(c, "900", 2026, time.July, "team", []int{7, 9})()
	if _, ok := msg.(entriesMsg); !ok {
		t.Fatalf("expected entriesMsg, got %T", msg)
	}
	if teamCalled {
		t.Error("explicit assignees: /team must not be called")
	}
}
```

Also update the three existing calls in `app_test.go` to pass the new argument:
- line ~63: `loadEntriesCmd(c, "900", 2026, time.July, "team", nil)()`
- line ~91: `loadEntriesCmd(c, "900", 2026, time.July, "team", nil)()`
- line ~116: `loadEntriesCmd(c, "900", 2026, time.July, "me", nil)()`

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./... 2>&1 | head` — Expected: build errors (signature mismatch / undefined `selectedMembers`).

- [ ] **Step 3: Write minimal implementation**

In `internal/tui/app.go`:

(a) Append `screenMembers` to the `screen` iota (keep existing order — add at the end):

```go
const (
	screenSetup screen = iota
	screenHome
	screenLoading
	screenReport
	screenExport
	screenRates
	screenLog
	screenError
	screenMembers
)
```

(b) Add the message type to the `type (...)` block:

```go
	membersMsg struct{ members []clickup.Member }
```

(c) Add root fields to `Model` (near `entries`):

```go
	teamMembers     []clickup.Member // workspace members (session cache)
	selectedMembers map[int]bool     // selected member ids; empty = all (no filter)
```

(d) Replace the imports: remove `"fmt"` (now unused here — the not-found error moved to `clickup.TeamMembers`) and add `"slices"`.

(e) Add the helper:

```go
// selectedAssignees returns the ids of the currently selected members, sorted.
// An empty result means "no member filter" (all members).
func (m Model) selectedAssignees() []int {
	var ids []int
	for id, on := range m.selectedMembers {
		if on {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}
```

(f) Update `reloadEntriesCmd` to pass the assignees:

```go
func (m Model) reloadEntriesCmd() tea.Cmd {
	if m.demo {
		return demoEntriesCmd(m.year, m.month)
	}
	return loadEntriesCmd(m.client, m.cfg.WorkspaceID, m.year, m.month, m.scope, m.selectedAssignees())
}
```

(g) Replace `loadEntriesCmd` with the assignee-aware version (list-name resolution unchanged):

```go
// loadEntriesCmd calls the API in the background and returns entriesMsg or errMsg.
// For scope "team" with an empty assignees slice it derives ALL workspace members
// (via TeamMembers) and filters on them; a non-empty assignees slice is used as-is
// (skipping the members lookup). For scope "me" no assignee filter is applied.
func loadEntriesCmd(c *clickup.Client, teamID string, year int, month time.Month, scope string, assignees []int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if scope == "team" && len(assignees) == 0 {
			members, err := c.TeamMembers(ctx, teamID)
			if err != nil {
				return errMsg{err: err}
			}
			for _, mem := range members {
				assignees = append(assignees, mem.ID)
			}
		}

		start, end := report.MonthRange(year, month)
		entries, err := c.TimeEntries(ctx, teamID, start, end, assignees)
		if err != nil {
			return errMsg{err: err}
		}
		// Resolve human-readable list names ONCE per unique list_id
		// (avoids repeated calls, including failed ones, for the same list).
		resolved := map[string]string{}
		for _, e := range entries {
			if e.ListID == "" {
				continue
			}
			if _, done := resolved[e.ListID]; done {
				continue
			}
			if name, err := c.ListName(ctx, e.ListID); err == nil {
				resolved[e.ListID] = name
			} else {
				resolved[e.ListID] = "" // attempted: don't retry within this load
			}
		}
		for i := range entries {
			if name := resolved[entries[i].ListID]; name != "" {
				entries[i].ListName = name
			}
		}
		return entriesMsg{entries: entries}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -race` → PASS (existing + new).
Run: `go build ./...` → clean.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat(tui): thread selected member assignees through entry loading"
```

---

### Task 4: demo mode — multiple users + demo members

**Files:**
- Modify: `internal/tui/demo.go`
- Test: `internal/tui/demo_test.go`

**Interfaces:**
- Consumes: `membersMsg` (Task 3), `clickup.Member`.
- Produces: multi-user `demoEntries`; `demoMembers() []clickup.Member`; `demoMembersCmd() tea.Cmd`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/tui/demo_test.go`:

```go
func TestDemoMembers(t *testing.T) {
	if len(demoMembers()) != 3 {
		t.Fatalf("demoMembers = %d, want 3", len(demoMembers()))
	}
	if _, ok := demoMembersCmd()().(membersMsg); !ok {
		t.Fatalf("demoMembersCmd should produce membersMsg")
	}
}

func TestDemoEntriesMultipleUsers(t *testing.T) {
	users := map[string]bool{}
	for _, e := range demoEntries(2026, time.July) {
		users[e.UserName] = true
	}
	if len(users) < 2 {
		t.Errorf("expected multiple demo users, got %v", users)
	}
}
```

(Add `"github.com/marcoarnulfo/clickup-cli/internal/clickup"` to the test imports only if a test references it — these two don't, so no change.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run 'TestDemoMembers|TestDemoEntriesMultipleUsers' -v`
Expected: FAIL — `undefined: demoMembers` / `undefined: demoMembersCmd`.

- [ ] **Step 3: Write minimal implementation**

In `internal/tui/demo.go`, add `"github.com/marcoarnulfo/clickup-cli/internal/clickup"` to imports, then replace `demoEntries` and append the two helpers:

```go
// demoEntries returns fake time entries for the given month, spread across a few
// users so the member selection and per-member grouping are meaningful in demo.
func demoEntries(year int, month time.Month) []report.TimeEntry {
	at := func(d, h, m int) time.Time { return time.Date(year, month, d, h, m, 0, 0, time.UTC) }
	mk := func(id, taskID, task, listID, list string, uid int, user string, start time.Time, dur time.Duration) report.TimeEntry {
		return report.TimeEntry{
			ID: id, TaskID: taskID, TaskName: task,
			ListID: listID, ListName: list,
			UserID: uid, UserName: user,
			Start: start, Duration: dur,
		}
	}
	return []report.TimeEntry{
		mk("1", "t1", "Landing page redesign", "web", "Website", 1, "alice", at(3, 9, 0), 3*time.Hour+30*time.Minute),
		mk("2", "t2", "API integration", "web", "Website", 2, "bob", at(3, 14, 0), 2*time.Hour),
		mk("3", "t3", "Bugfix checkout", "web", "Website", 1, "alice", at(5, 10, 0), 1*time.Hour+15*time.Minute),
		mk("4", "t4", "Onboarding screens", "mobile", "Mobile app", 3, "carol", at(6, 9, 30), 4*time.Hour),
		mk("5", "t5", "Push notifications", "mobile", "Mobile app", 2, "bob", at(7, 11, 0), 2*time.Hour+45*time.Minute),
		mk("6", "t6", "Release QA", "mobile", "Mobile app", 3, "carol", at(10, 15, 0), 1*time.Hour+30*time.Minute),
	}
}

// demoMembers returns the fake workspace members for demo mode.
func demoMembers() []clickup.Member {
	return []clickup.Member{
		{ID: 1, Username: "alice"},
		{ID: 2, Username: "bob"},
		{ID: 3, Username: "carol"},
	}
}

// demoMembersCmd delivers the fake members as membersMsg (no I/O).
func demoMembersCmd() tea.Cmd {
	return func() tea.Msg { return membersMsg{members: demoMembers()} }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -race` → PASS. (`TestDemoEntriesBuildReport` still sees 2 lists → 2 buckets.)

- [ ] **Step 5: Commit**

```bash
git add internal/tui/demo.go internal/tui/demo_test.go
git commit -m "feat(tui): multi-user demo data and demo members"
```

---

### Task 5: member selection screen

**Files:**
- Create: `internal/tui/members.go`
- Modify: `internal/tui/app.go` (root field, routeKey, View, membersMsg handler)
- Test: `internal/tui/members_test.go` (new)

**Interfaces:**
- Consumes: `clickup.Member`, `Model.teamMembers`, `Model.selectedMembers`, `screenMembers`, `membersMsg`.
- Produces:
  - `membersModel struct{ members []clickup.Member; selected map[int]bool; idx int; loading bool }`.
  - `newMembers(members []clickup.Member, selected map[int]bool) membersModel`.
  - `func (m Model) updateMembers(msg tea.KeyMsg) (tea.Model, tea.Cmd)`.
  - `Model.membersScreen membersModel`.

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/members_test.go`:

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
)

func membersFixture() Model {
	mems := []clickup.Member{{ID: 1, Username: "alice"}, {ID: 2, Username: "bob"}}
	sel := map[int]bool{1: true, 2: true}
	return Model{
		screen:          screenMembers,
		teamMembers:     mems,
		selectedMembers: sel,
		membersScreen:   newMembers(mems, sel),
	}
}

func spaceKey() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}} }
func runeKey(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestMembersToggleAndConfirm(t *testing.T) {
	m := membersFixture()
	u, _ := m.updateMembers(spaceKey()) // toggle alice (idx 0) off
	m = u.(Model)
	if m.membersScreen.selected[1] {
		t.Error("alice should be deselected after space")
	}
	u, _ = m.updateMembers(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if m.screen != screenHome {
		t.Errorf("enter should return home, got %v", m.screen)
	}
	if m.selectedMembers[1] {
		t.Error("root selection should reflect alice deselected")
	}
}

func TestMembersAllNone(t *testing.T) {
	m := membersFixture()
	u, _ := m.updateMembers(runeKey("a")) // all selected -> clear
	m = u.(Model)
	if m.membersScreen.selected[1] || m.membersScreen.selected[2] {
		t.Error("'a' with all selected should clear all")
	}
	u, _ = m.updateMembers(runeKey("a")) // none -> select all
	m = u.(Model)
	if !m.membersScreen.selected[1] || !m.membersScreen.selected[2] {
		t.Error("'a' with none selected should select all")
	}
}

func TestMembersEscDiscards(t *testing.T) {
	m := membersFixture()
	u, _ := m.updateMembers(spaceKey()) // toggle alice off (on the copy)
	m = u.(Model)
	u, _ = m.updateMembers(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(Model)
	if m.screen != screenHome {
		t.Errorf("esc should return home, got %v", m.screen)
	}
	if !m.selectedMembers[1] {
		t.Error("esc must discard: root alice still selected")
	}
}

func TestMembersMsgDefaultsAll(t *testing.T) {
	m := Model{}
	u, _ := m.Update(membersMsg{members: []clickup.Member{{ID: 1, Username: "a"}, {ID: 2, Username: "b"}}})
	m = u.(Model)
	if m.screen != screenMembers {
		t.Errorf("screen = %v, want screenMembers", m.screen)
	}
	if !m.selectedMembers[1] || !m.selectedMembers[2] {
		t.Error("default selection should be all members")
	}
	if len(m.teamMembers) != 2 {
		t.Error("teamMembers should be cached")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./... 2>&1 | head` — Expected: `undefined: newMembers` / `undefined: membersScreen`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/tui/members.go`:

```go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
)

// membersModel is the team-member selection screen. Its `selected` set is a
// defensive copy of the root's, so Esc can discard changes without touching root.
type membersModel struct {
	members  []clickup.Member
	selected map[int]bool
	idx      int
	loading  bool
}

// newMembers builds the screen from the workspace members and the current
// selection (copied defensively).
func newMembers(members []clickup.Member, selected map[int]bool) membersModel {
	sel := make(map[int]bool, len(selected))
	for id, on := range selected {
		sel[id] = on
	}
	return membersModel{members: members, selected: sel}
}

// allSelected reports whether every member is currently selected.
func (mm membersModel) allSelected() bool {
	if len(mm.members) == 0 {
		return false
	}
	for _, mem := range mm.members {
		if !mm.selected[mem.ID] {
			return false
		}
	}
	return true
}

func (m Model) updateMembers(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	mm := m.membersScreen
	if mm.loading {
		return m, nil
	}
	switch msg.String() {
	case "up", "k":
		if mm.idx > 0 {
			mm.idx--
		}
	case "down", "j":
		if mm.idx < len(mm.members)-1 {
			mm.idx++
		}
	case " ", "space":
		if len(mm.members) > 0 {
			id := mm.members[mm.idx].ID
			mm.selected[id] = !mm.selected[id]
		}
	case "a":
		on := !mm.allSelected() // all selected -> clear; else select all
		for _, mem := range mm.members {
			mm.selected[mem.ID] = on
		}
	case "enter":
		m.selectedMembers = mm.selected
		m.membersScreen = mm
		m.screen = screenHome
		return m, nil
	case "esc":
		m.screen = screenHome // discard: don't write mm back to root
		return m, nil
	}
	m.membersScreen = mm
	return m, nil
}

func (mm membersModel) view() string {
	if mm.loading {
		return styleTitle.Render("Loading members…")
	}
	b := styleTitle.Render("Team members") + "\n\n"
	if len(mm.members) == 0 {
		b += styleHelp.Render("No members in this workspace.") + "\n"
	}
	for i, mem := range mm.members {
		box := "[ ]"
		if mm.selected[mem.ID] {
			box = "[x]"
		}
		cursor := "  "
		line := box + " " + mem.Username
		if i == mm.idx {
			cursor = "▸ "
			line = styleAccent.Render(line)
		}
		b += cursor + line + "\n"
	}
	b += "\n" + styleHelp.Render("↑/↓ move · Space toggle · a: all/none · Enter: confirm · Esc: cancel")
	return b
}
```

In `internal/tui/app.go`:

(a) Add the root field next to the other sub-models:

```go
	membersScreen membersModel
```

(b) Add the `membersMsg` handler in the `Update` type-switch:

```go
	case membersMsg:
		m.teamMembers = msg.members
		if len(m.selectedMembers) == 0 {
			m.selectedMembers = make(map[int]bool, len(msg.members))
			for _, mem := range msg.members {
				m.selectedMembers[mem.ID] = true // default: all selected
			}
		}
		m.membersScreen = newMembers(msg.members, m.selectedMembers)
		m.screen = screenMembers
		return m, nil
```

(c) Add to `routeKey`:

```go
	case screenMembers:
		return m.updateMembers(msg)
```

(d) Add to `View`:

```go
	case screenMembers:
		return m.membersScreen.view()
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -race` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/members.go internal/tui/members_test.go internal/tui/app.go
git commit -m "feat(tui): team member selection screen"
```

---

### Task 6: Home entry point (`f`) + member count

**Files:**
- Modify: `internal/tui/home.go`
- Modify: `internal/tui/app.go` (`loadMembersCmd`, View call passes the note)
- Test: `internal/tui/home_test.go` (new)

**Interfaces:**
- Consumes: `newMembers` (Task 5), `demoMembersCmd` (Task 4), `clickup.TeamMembers` (Task 2), `Model.teamMembers`/`selectedMembers`/`selectedAssignees`.
- Produces: `f` opens `screenMembers` (team scope only); `loadMembersCmd(c, teamID) tea.Cmd`; `func (m Model) homeMembersNote() string`; `homeModel.view(year, month, scope, membersNote string)`.

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/home_test.go`:

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./... 2>&1 | head` — Expected: `undefined: homeMembersNote` and view-arity errors.

- [ ] **Step 3: Write minimal implementation**

In `internal/tui/app.go`, add the loader near `loadEntriesCmd`:

```go
// loadMembersCmd fetches the workspace members in the background and returns
// membersMsg or errMsg.
func loadMembersCmd(c *clickup.Client, teamID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		members, err := c.TeamMembers(ctx, teamID)
		if err != nil {
			return errMsg{err: err}
		}
		return membersMsg{members: members}
	}
}
```

And update the Home render call in `View`:

```go
	case screenHome:
		return m.home.view(m.year, m.month, m.scope, m.homeMembersNote())
```

In `internal/tui/home.go`, add the `f` case to `updateHome` (before `case "enter"`):

```go
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
```

Add the note helper and update the view in `home.go`:

```go
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
	help := styleHelp.Render("◂/▸ change month · t: me/team · f: select members · Enter: generate report · n: log hours · q: quit")
	return title + "\n\n" + sel + "\n\n" + help
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -race` → PASS.
Run: `go build ./...` → clean.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/home.go internal/tui/home_test.go internal/tui/app.go
git commit -m "feat(tui): open member selection from Home and show member count"
```

---

### Task 7: scope-aware grouping cycle + report title + docs

**Files:**
- Modify: `internal/tui/report.go`
- Modify: `internal/tui/app.go`, `internal/tui/rates.go`, `internal/tui/log_test.go` (newReport call sites)
- Modify: `README.md`, `README.it.md`
- Test: `internal/tui/report.go` tests in `internal/tui/report_test.go` (new)

**Interfaces:**
- Consumes: `report.GroupByMember` (Task 1), `Model.selectedAssignees`/`teamMembers` (Tasks 3/5).
- Produces: `nextGroupBy(g, scope string) string`; `newReport(r report.Report, note string) reportModel`; `reportModel.note`; `func (m Model) memberFilterNote() string`.

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/report_test.go`:

```go
package tui

import (
	"testing"

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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go build ./... 2>&1 | head` — Expected: `nextGroupBy` arity error / `undefined: memberFilterNote`.

- [ ] **Step 3: Write minimal implementation**

In `internal/tui/report.go`:

(a) Add `note` to `reportModel` and update `newReport`:

```go
type reportModel struct {
	r    report.Report
	note string
}

func newReport(r report.Report, note string) reportModel { return reportModel{r: r, note: note} }
```

(b) Scope-aware `nextGroupBy`:

```go
// nextGroupBy cycles total -> task -> list -> day -> [member] -> total.
// The member grouping is only offered for the team scope.
func nextGroupBy(g, scope string) string {
	switch g {
	case report.GroupByTotal:
		return report.GroupByTask
	case report.GroupByTask:
		return report.GroupByList
	case report.GroupByList:
		return report.GroupByDay
	case report.GroupByDay:
		if scope == "team" {
			return report.GroupByMember
		}
		return report.GroupByTotal
	default: // includes GroupByMember
		return report.GroupByTotal
	}
}
```

(c) Update the `g` case in `updateReport`:

```go
	case "g":
		g := nextGroupBy(m.report.GroupBy, m.scope)
		m.report = report.Build(m.entries, g, ratesFromConfig(m.cfg), m.cfg.Currency, m.year, m.month)
		m.report.Scope = m.scope
		m.rep = newReport(m.report, m.memberFilterNote())
```

(d) Title uses the note in `view`:

```go
	title := styleTitle.Render(fmt.Sprintf("Report %04d-%02d — scope %s%s — grouped by %s",
		r.Year, int(r.Month), r.Scope, rm.note, r.GroupBy))
```

(e) Add the note helper (report.go imports `fmt`):

```go
// memberFilterNote returns " (k/n members)" when the team scope has a partial
// member selection, else "".
func (m Model) memberFilterNote() string {
	if m.scope != "team" || len(m.teamMembers) == 0 {
		return ""
	}
	k := len(m.selectedAssignees())
	n := len(m.teamMembers)
	if k == 0 || k == n {
		return ""
	}
	return fmt.Sprintf(" (%d/%d members)", k, n)
}
```

(f) Update the other `newReport` call sites to pass the note:
- `internal/tui/app.go` (in the `entriesMsg` handler): `m.rep = newReport(m.report, m.memberFilterNote())`
- `internal/tui/rates.go` (in the `s` save case): `m.rep = newReport(m.report, m.memberFilterNote())`
- `internal/tui/log_test.go` (test setup): `m.rep = newReport(m.report, "")`

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./... -race` → PASS.
Run: `gofmt -l .` (empty), `go vet ./...`, `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...` → all clean.

- [ ] **Step 5: Update the READMEs**

In **`README.md`** and **`README.it.md`**:

- Add a row to the TUI commands table (after the `t` row):
  - EN: `| \`f\` | Home | Open **member selection** (team scope): multi-select which members the report covers |`
  - IT: `| \`f\` | Home | Apre la **selezione membri** (scope team): multiselezione dei membri inclusi nel report |`
- In the `g` grouping row, mention the member grouping for team scope:
  - EN: `Cycle grouping: total → task → list → day → member (team) → total`
  - IT: `Cicla il raggruppamento: totale → task → lista → giorno → membro (team) → totale`
- Update the **Team scope** section: note that members can now be picked individually with `f` (default: all), replacing the "no per-member selection" caveat.
- In the **Roadmap** section, drop "per-member selection (v1.3/v1.4)" from the pending highlights (now shipped).

- [ ] **Step 6: Commit**

```bash
git add internal/tui/report.go internal/tui/report_test.go internal/tui/app.go internal/tui/rates.go internal/tui/log_test.go README.md README.it.md
git commit -m "feat(tui): scope-aware grouping with per-member view and docs"
```

---

## Self-Review notes (author)

- **Spec coverage:** dedicated screen from Home (Tasks 5/6) ✓; group-by member (Tasks 1/7) ✓; default all + empty=all (Tasks 3/5/6) ✓; session-only state (Task 3) ✓; demo multi-user (Task 4) ✓; `TeamMembers` extraction (Task 2) ✓; report title note (Task 7) ✓.
- **Empty=all consistency:** `selectedAssignees` empty → `loadEntriesCmd` team fallback derives all; `homeMembersNote` shows k=n; `memberFilterNote` returns "" (no partial note). Consistent.
- **Type consistency:** `nextGroupBy(g, scope)`, `newReport(r, note)`, `membersMsg{members}`, `TeamMembers(ctx, teamID)` used identically across producers/consumers.
- **Build-green ordering:** `screenMembers`/`membersMsg` declared in Task 3 (unused is legal); `demoMembersCmd` (Task 4) and `membersScreen` field/handler (Task 5) fill in before Home wires `f` (Task 6). `fmt` removed from app.go in Task 3, not reintroduced there (helpers using `fmt` live in home.go/report.go).
