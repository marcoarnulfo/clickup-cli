# Edit time-entry tags (#125) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `t` action on the v1.8 entry browser to view and edit a single time entry's own **time-tracking tags** (distinct from task tags), via a multi-select picker that can create new tags, with demo parity.

**Architecture:** A new pure field `EntryTags` on `report.TimeEntry`; two new client verbs (`TimeEntryTags` GET, `SetTimeEntryTags` — spike-gated on the exact ClickUp mechanism); a new `entriesTags` mode on the existing `entriesModel` reusing the v1.8 browser infra (`canEdit`, `entriesErr`, `reloadForBrowser`, `demoOverrides`, `demoEntriesSnapshot`).

**Tech Stack:** Go 1.26, bubbletea/bubbles, stdlib `net/http`/`encoding/json`/`httptest`.

**Spec:** `docs/superpowers/specs/2026-07-23-edit-entry-tags-design.md`

## Global Constraints

- Edit **time-entry tags** (the entry's own time-tracking tags), NOT task tags. `report.TimeEntry.Tags` (task tags, used by report grouping/filtering) is untouched; the new tags live in a separate field `EntryTags`.
- `internal/report` and `internal/duration` stay **pure**: no I/O, no `time.Now`, no import of config/clickup/tui. Adding a plain `EntryTags []string` field is allowed.
- bubbletea (Elm): root `Model` value-receiver; per-screen `updateX`/`view`; explicit write-back `m.sub = x` before return; new msgs handled in the top-level `Update` type-switch in `app.go`.
- Client: `Authorization: <token>` (NO Bearer); `do(ctx, method, path, query, body, out)` is method-agnostic (retries 429 on all verbs); tests use the house helper `newTestClient(h http.HandlerFunc) (*Client, *httptest.Server)`.
- Reuse v1.8 browser infra — do NOT reinvent: `canEdit(e, userID)`, `entriesErr`/`entriesErrMsg`, `reloadForBrowser(m, status) tea.Msg`, `demoOverrides map[string]report.TimeEntry`, `demoEntriesSnapshot`, `locOr`, `newTextInput`, shared styles. Mirror the `membersModel` multi-select idiom for the picker.
- Ownership: tag editing is limited to the user's **own** entries (`canEdit`), like edit/delete.
- Demo: `CLICKUP_DEMO=1` never touches `m.client`; every command has a demo branch.
- **Pre-commit gate (run ALL before every commit):** `gofmt -l .`, `go vet ./...`, `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`, `go test ./... -race` — all clean/green. **staticcheck is mandatory** (it runs in CI and broke v1.8's first CI run on an SA4006 dead assignment).
- Everything ENGLISH (code, comments, tests, UI strings, commit messages). Conventional Commits. **NEVER** a `Co-Authored-By` trailer.

---

## File Structure

- `internal/report/model.go` — `EntryTags []string` on `TimeEntry`.
- `internal/clickup/timeentries.go` — parse the entry's own `tags` → `EntryTags`.
- `internal/clickup/entrytags.go` (NEW) — `TimeEntryTags`, `SetTimeEntryTags`.
- `internal/clickup/entrytags_test.go` (NEW) — spike + verb tests.
- `internal/tui/entries.go` — `entriesTags` mode, `t` action, picker state/update/view, `tagsFetchCmd`/`setTagsCmd`, row tag display.
- `internal/tui/app.go` — `tagsMsg` handler; picker state fields on `entriesModel` (or a sub-struct).
- `internal/tui/demo.go` — demo tag fetch + demo `EntryTags` in the fixture.
- `internal/tui/*_test.go` — picker transition tests.
- `CHANGELOG.md`, `README.md`, `README.it.md`, `docs/demo.tape` — docs.

---

## Task 1: Data model — `EntryTags` on `report.TimeEntry` + parse the entry's own tags

**Files:**
- Modify: `internal/report/model.go`
- Modify: `internal/clickup/timeentries.go`
- Test: `internal/clickup/timeentries_test.go`

**Interfaces:**
- Produces: `report.TimeEntry.EntryTags []string` (the entry's own time-tracking tags, distinct from `Tags`).

- [ ] **Step 1: Write the failing test** — add to `internal/clickup/timeentries_test.go` (adapt to the existing test's construction style there):

```go
func TestTimeEntriesParsesEntryTags(t *testing.T) {
	const payload = `{"data":[
		{"id":"e1","start":"1700000000000","duration":"3600000",
		 "task":{"id":"t1","name":"Fix"},
		 "task_tags":[{"name":"backend"}],
		 "tags":[{"name":"focus"},{"name":"client-A"}]}
	]}`
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	})
	defer srv.Close()

	entries, err := c.TimeEntries(context.Background(), "team1",
		time.UnixMilli(0), time.UnixMilli(2_000_000_000_000), nil)
	if err != nil {
		t.Fatalf("TimeEntries error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	e := entries[0]
	// task tags stay in Tags (report grouping); entry tags go to EntryTags.
	if len(e.Tags) != 1 || e.Tags[0] != "backend" {
		t.Errorf("Tags = %v, want [backend] (task tags)", e.Tags)
	}
	if len(e.EntryTags) != 2 || e.EntryTags[0] != "focus" || e.EntryTags[1] != "client-A" {
		t.Errorf("EntryTags = %v, want [focus client-A]", e.EntryTags)
	}
}
```

> **Implementer note:** match the existing `timeentries_test.go` imports/construction (it already tests `TimeEntries`; reuse its `newTestClient` usage and imports — `context`, `net/http`, `time`).

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/clickup/ -run TestTimeEntriesParsesEntryTags -v` → FAIL (`EntryTags` undefined).

- [ ] **Step 3a: Add the field** — in `internal/report/model.go`, in the `TimeEntry` struct, after `Tags []string`:

```go
	EntryTags []string // the entry's own time-tracking tags (distinct from Tags = task tags)
```

- [ ] **Step 3b: Parse it** — in `internal/clickup/timeentries.go`, add to `rawEntry` (after the `TaskTags` field):

```go
	Tags []struct {
		Name string `json:"name"`
	} `json:"tags"` // the entry's own time-tracking tags
```

and in `toTimeEntry`, build and set it (mirror the `task_tags` loop):

```go
	entryTags := make([]string, 0, len(r.Tags))
	for _, t := range r.Tags {
		entryTags = append(entryTags, t.Name)
	}
```

then add `EntryTags: entryTags,` to the returned `report.TimeEntry{...}` literal.

- [ ] **Step 4: Run test to verify it passes** — `go test ./internal/clickup/ -run TestTimeEntriesParsesEntryTags -v` → PASS. Then `go build ./...`.

- [ ] **Step 5: Commit**

```bash
git add internal/report/model.go internal/clickup/timeentries.go internal/clickup/timeentries_test.go
git commit -m "feat(report): carry a time entry's own tags in EntryTags"
```

---

## Task 2: Client — SPIKE + `TimeEntryTags` (GET) + `SetTimeEntryTags`

**Files:**
- Create: `internal/clickup/entrytags.go`, `internal/clickup/entrytags_test.go`

**Interfaces:**
- Produces:
  - `TimeEntryTags(ctx, teamID string) ([]string, error)` → `GET /team/{id}/time_entries/tags`.
  - `SetTimeEntryTags(ctx, teamID, entryID string, desired []string) error` — sets the entry's tags to exactly `desired`.

- [ ] **Step 1: SPIKE — confirm the API shape.** ClickUp's time-entry tag API is the least-documented part. Write `entrytags_test.go` with `httptest` fixtures built from the documented v2 shape and confirm the decode/requests. **Determine and encode which set mechanism ClickUp uses**, in this order of preference:
  - (a) `PUT /team/{id}/time_entries/{id}` accepting `{"tags":[...],"tag_action":"replace"}` — if the docs/a real call confirm it, use replace (simplest).
  - (b) otherwise `POST /team/{id}/time_entries/tags` (add) + `DELETE /team/{id}/time_entries/tags` (remove), body `{"time_entry_ids":["<id>"],"tags":[{"name":"..."}]}`, applied as a diff of desired-vs-current.

  If you cannot determine the shape from the documented API with confidence, STOP and report BLOCKED with what you found — this feature is **not** sacrificial (the user asked for it); the controller will get the shape confirmed rather than guessing. Add a `// TODO(spike): verified against docs, not a live call` marker where the shape is assumed.

  The tests (written against your chosen mechanism):

```go
package clickup

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestTimeEntryTags(t *testing.T) {
	const payload = `{"data":[{"name":"focus"},{"name":"client-A"},{"name":"review"}]}`
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/team/team1/time_entries/tags" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(payload))
	})
	defer srv.Close()

	tags, err := c.TimeEntryTags(context.Background(), "team1")
	if err != nil {
		t.Fatalf("TimeEntryTags error: %v", err)
	}
	if len(tags) != 3 || tags[0] != "focus" || tags[2] != "review" {
		t.Errorf("tags = %v, want [focus client-A review]", tags)
	}
}

// TestSetTimeEntryTags asserts the request(s) SetTimeEntryTags makes to move the
// entry to the desired set. Adapt the assertions to the mechanism the spike
// selects (replace-PUT, or add/remove POST/DELETE).
func TestSetTimeEntryTags(t *testing.T) {
	var reqs []string // "METHOD PATH BODY"
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		reqs = append(reqs, r.Method+" "+r.URL.Path+" "+string(raw))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	defer srv.Close()

	if err := c.SetTimeEntryTags(context.Background(), "team1", "e1", []string{"focus", "new-tag"}); err != nil {
		t.Fatalf("SetTimeEntryTags error: %v", err)
	}
	if len(reqs) == 0 {
		t.Fatalf("no request made")
	}
	// At least one request must carry the desired tag names and target the entry.
	all := ""
	for _, s := range reqs {
		all += s + "\n"
	}
	for _, want := range []string{"focus", "new-tag", "e1"} {
		if !contains(all, want) {
			t.Errorf("requests %q missing %q", all, want)
		}
	}
	// sanity: the body is valid JSON on each request
	for _, s := range reqs {
		if i := indexSpaceSpace(s); i >= 0 {
			var v any
			if body := s[i:]; body != "" && json.Unmarshal([]byte(body), &v) != nil {
				t.Errorf("request body not JSON: %q", body)
			}
		}
	}
}
```

> **Implementer note:** `contains`/`indexSpaceSpace` are throwaway test helpers — use `strings.Contains` and `strings.Index(s, "  ")`-style logic inline instead of inventing helpers, or simplify the assertions to what your chosen mechanism actually sends. The point the test pins: the desired names and the entry id reach the server as valid JSON. If the spike picks the add/remove diff, also assert that `SetTimeEntryTags` fetches or is told the current set — since `SetTimeEntryTags(ctx, teamID, entryID, desired)` has no current set, the diff mechanism needs the current tags: pass them in as a parameter `SetTimeEntryTags(ctx, teamID, entryID string, current, desired []string)` and update the interface + callers accordingly. Prefer replace-PUT if the API supports it precisely to avoid needing `current`.

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/clickup/ -run 'TimeEntryTags|SetTimeEntryTags' -v` → FAIL (undefined).

- [ ] **Step 3: Implement** — `internal/clickup/entrytags.go`. Example for the **replace-PUT** mechanism (adapt if the spike selects add/remove):

```go
package clickup

import "context"

// TimeEntryTags returns the workspace's time-tracking tag names.
// GET /team/{team_id}/time_entries/tags.
func (c *Client) TimeEntryTags(ctx context.Context, teamID string) ([]string, error) {
	var resp struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/team/"+teamID+"/time_entries/tags", nil, &resp); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(resp.Data))
	for _, t := range resp.Data {
		out = append(out, t.Name)
	}
	return out, nil
}

// SetTimeEntryTags sets the entry's time-tracking tags to exactly `desired`.
// Unknown names are created by the same call (time-entry tags auto-create).
// PUT /team/{team_id}/time_entries/{timer_id} with tags + tag_action=replace.
//
// TODO(spike): if the single-entry PUT does not accept tags+tag_action, switch
// to POST/DELETE /team/{id}/time_entries/tags applied as a desired-vs-current
// diff (see the plan's Task 2 note).
func (c *Client) SetTimeEntryTags(ctx context.Context, teamID, entryID string, desired []string) error {
	tags := make([]map[string]any, 0, len(desired))
	for _, name := range desired {
		tags = append(tags, map[string]any{"name": name})
	}
	body := map[string]any{"tags": tags, "tag_action": "replace"}
	return c.put(ctx, "/team/"+teamID+"/time_entries/"+entryID, body, nil)
}
```

> `c.put` and `c.get` already exist (v1.8 / pre-existing). Adapt the body/paths to the spike's verified shape; keep the `SetTimeEntryTags` signature stable for callers (Task 4) unless the diff mechanism forces the `current` parameter — in which case update this plan's Task 4 calls too.

- [ ] **Step 4: Run tests to verify they pass** — `go test ./internal/clickup/ -run 'TimeEntryTags|SetTimeEntryTags' -v` → PASS. `go build ./...`.

- [ ] **Step 5: Commit**

```bash
git add internal/clickup/entrytags.go internal/clickup/entrytags_test.go
git commit -m "feat(clickup): add TimeEntryTags and SetTimeEntryTags"
```

---

## Task 3: Picker — `t` action, fetch, multi-select toggle, row display

**Files:**
- Modify: `internal/tui/entries.go` (picker state, `t`, `entriesTags` update/view, fetch cmd, row tags)
- Modify: `internal/tui/app.go` (`tagsMsg` handler)
- Modify: `internal/tui/demo.go` (demo tag fetch + demo `EntryTags` fixture)
- Test: `internal/tui/entries_tags_test.go`

**Interfaces:**
- Consumes: `canEdit`, `entriesErr`/`entriesErrMsg`, `entriesModel`, `locOr`, styles, `report.TimeEntry.EntryTags`.
- Produces (Task 4 reuses): picker state on `entriesModel`; `entriesTags` mode; `tagsMsg{tags []string}`; `(m Model) tagsFetchCmd(entryID string) tea.Cmd`.

- [ ] **Step 1: Write the failing test** — `internal/tui/entries_tags_test.go`:

```go
package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func TestTOpensTagPickerGatedOnOwnership(t *testing.T) {
	m := newTestModel()
	own := report.TimeEntry{ID: "e1", TaskName: "Fix", UserID: 1, Start: time.Now(), EntryTags: []string{"focus"}}
	m = browserWithEntries(m, own)
	m2, cmd := m.Update(keyRunes("t"))
	mm := m2.(Model)
	if mm.entriesScreen.mode != entriesTags {
		t.Fatalf("t did not open the tag picker: mode=%v", mm.entriesScreen.mode)
	}
	if !mm.entriesScreen.tagLoading || cmd == nil {
		t.Errorf("t should start the tag fetch (loading + a cmd)")
	}
	// current tags seed the selection
	if !mm.entriesScreen.tagSel["focus"] {
		t.Errorf("current EntryTags did not seed tagSel")
	}
}

func TestTGatedOnOwnership(t *testing.T) {
	m := newTestModel()
	other := report.TimeEntry{ID: "e2", UserID: 2, Start: time.Now()}
	m = browserWithEntries(m, other)
	m2, _ := m.Update(keyRunes("t"))
	if m2.(Model).entriesScreen.mode != entriesList {
		t.Errorf("t on a non-owned entry must be a no-op")
	}
}

func TestTagsMsgPopulatesAndUnionsCurrent(t *testing.T) {
	m := newTestModel()
	own := report.TimeEntry{ID: "e1", TaskName: "Fix", UserID: 1, Start: time.Now(), EntryTags: []string{"focus", "legacy"}}
	m = browserWithEntries(m, own)
	m = m.Update2Tags() // helper: open picker (see note)
	m2, _ := m.Update(tagsMsg{tags: []string{"focus", "client-A"}})
	mm := m2.(Model)
	es := mm.entriesScreen
	if es.tagLoading {
		t.Errorf("tagsMsg should clear loading")
	}
	// tagAll = fetched ∪ current, deduped: focus, client-A, legacy
	joined := strings.Join(es.tagAll, ",")
	for _, want := range []string{"focus", "client-A", "legacy"} {
		if !strings.Contains(joined, want) {
			t.Errorf("tagAll %q missing %q (should union current tags)", joined, want)
		}
	}
}

func TestSpaceTogglesTag(t *testing.T) {
	m := newTestModel()
	own := report.TimeEntry{ID: "e1", TaskName: "Fix", UserID: 1, Start: time.Now(), EntryTags: []string{"focus"}}
	m = browserWithEntries(m, own)
	m = m.Update2Tags()
	m2, _ := m.Update(tagsMsg{tags: []string{"focus", "client-A"}})
	mm := m2.(Model)
	// cursor on the first tag; space toggles it off
	before := mm.entriesScreen.tagSel[mm.entriesScreen.tagAll[0]]
	m3, _ := mm.Update(key("space"))
	after := m3.(Model).entriesScreen.tagSel[mm.entriesScreen.tagAll[0]]
	if before == after {
		t.Errorf("space did not toggle the tag under the cursor")
	}
}
```

> **Implementer note:** there is no `Update2Tags` helper — that's shorthand for "open the picker": `next, _ := m.Update(keyRunes("t")); m = next.(Model)`. Inline it (or add a small local test helper). `key("space")` — confirm the space key string the framework delivers (`" "` via `tea.KeySpace`/`tea.KeyRunes{' '}`); use whatever `key()`/`keyRunes()` produce for space, matching how `membersModel` reads it. Reuse existing `key`/`keyRunes`/`newTestModel`/`browserWithEntries`.

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/tui/ -run 'TOpensTag|TGatedOn|TagsMsg|SpaceToggles' -v` → FAIL.

- [ ] **Step 3a: Add picker state + `entriesTags` mode** — in `internal/tui/entries.go`, append `entriesTags` to the `entriesMode` iota (after `entriesHistory`), and add to `entriesModel`:

```go
	// tag picker (#125)
	tagAll     []string        // workspace tags ∪ the entry's current tags, sorted+deduped
	tagSel     map[string]bool // selected set (seeded from EntryTags)
	tagIdx     int
	tagNewMode bool            // typing a new tag name
	tagLoading bool
	tagEntryID string
```

- [ ] **Step 3b: `t` opens the picker** — in `updateEntries`, in the `entriesList` switch, add:

```go
		case "t":
			if len(es.entries) > 0 && canEdit(es.entries[es.idx], m.userID) {
				e := es.entries[es.idx]
				es.mode = entriesTags
				es.tagLoading = true
				es.tagEntryID = e.ID
				es.tagIdx = 0
				es.tagNewMode = false
				es.tagSel = map[string]bool{}
				for _, tg := range e.EntryTags {
					es.tagSel[tg] = true
				}
				es.tagAll = append([]string(nil), e.EntryTags...) // shown until the fetch lands
				m.entriesScreen = es
				return m, m.tagsFetchCmd(e.ID)
			}
```

- [ ] **Step 3c: Fetch cmd + `tagsMsg`** — in `entries.go`:

```go
// tagsMsg carries the workspace's time-entry tags for the picker.
type tagsMsg struct{ tags []string }

func (m Model) tagsFetchCmd(entryID string) tea.Cmd {
	if m.demo {
		return demoTagsFetchCmd()
	}
	c := m.client
	teamID := m.cfg.WorkspaceID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		tags, err := c.TimeEntryTags(ctx, teamID)
		if err != nil {
			return entriesErr(err)
		}
		return tagsMsg{tags: tags}
	}
}
```

In `app.go` `Update` (top-level switch):

```go
	case tagsMsg:
		es := m.entriesScreen
		es.tagLoading = false
		es.tagAll = unionSortedTags(msg.tags, es.tagAll) // fetched ∪ current, deduped+sorted
		m.entriesScreen = es
		return m, nil
```

Add `unionSortedTags` to `entries.go`:

```go
// unionSortedTags merges two tag lists, deduped and sorted, so a current tag
// missing from the workspace fetch still appears in the picker.
func unionSortedTags(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range append(append([]string(nil), a...), b...) {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	slices.Sort(out)
	return out
}
```

(ensure `slices` is imported in `entries.go` — it already is, from `sortEntriesByStartDesc`.)

- [ ] **Step 3d: `entriesTags` update (list mode: nav + space toggle; esc)** — add a `case entriesTags:` to `updateEntries` (new-tag input and Enter-save arrive in Task 4; for now handle nav/toggle/esc):

```go
	case entriesTags:
		switch msg.String() {
		case "esc":
			es.mode = entriesList
		case "up", "k":
			if es.tagIdx > 0 {
				es.tagIdx--
			}
		case "down", "j":
			if es.tagIdx < len(es.tagAll)-1 {
				es.tagIdx++
			}
		case " ": // space toggles the tag under the cursor
			if len(es.tagAll) > 0 {
				name := es.tagAll[es.tagIdx]
				es.tagSel[name] = !es.tagSel[name]
			}
		}
		// n (new tag) and enter (save) wired in Task 4.
```

> **Implementer:** confirm the key string for space in this codebase's `updateEntries` (match how other screens read space; `msg.String()` for `tea.KeySpace` is `" "`). Keep the write-back `m.entriesScreen = es` at the end of `updateEntries` (it already exists).

- [ ] **Step 3e: Picker view** — add to `entriesView` a branch for `es.mode == entriesTags` (render before the list branch, like `entriesConfirmDelete`):

```go
	if es.mode == entriesTags {
		b := styleTitle.Render("Tags") + "  " + styleAccent.Render(truncate(tagPickerTaskName(es), 40)) + "\n\n"
		if es.tagLoading {
			b += styleHelp.Render("Loading tags…") + "\n"
			return b + "\n" + styleHelp.Render("Esc: cancel")
		}
		for i, name := range es.tagAll {
			cursor := "  "
			box := "[ ]"
			if es.tagSel[name] {
				box = "[x]"
			}
			line := box + " " + name
			if i == es.tagIdx {
				cursor = "▸ "
				line = styleAccent.Render(line)
			}
			b += cursor + line + "\n"
		}
		if len(es.tagAll) == 0 {
			b += styleHelp.Render("No tags yet.") + "\n"
		}
		b += "\n" + styleHelp.Render("↑/↓ select · space: toggle · n: new tag · Enter: save · Esc: cancel")
		if es.msg != "" {
			b += "\n" + styleErr.Render(es.msg)
		}
		return b
	}
```

Add the small helper `tagPickerTaskName(es entriesModel) string` returning the task name of the entry being edited (find it in `es.entries` by `es.tagEntryID`, else `""`).

- [ ] **Step 3f: Row shows entry tags** — in `entriesView`'s row builder, append the entry's tags compactly. After the existing `line := fmt.Sprintf(...)` and the read-only suffix, before styling, add:

```go
		if len(e.EntryTags) > 0 {
			line += "  " + truncate(tagBadges(e.EntryTags), 20)
		}
```

with a helper `tagBadges(tags []string) string` → `"#focus #client-A"` (space-joined, each prefixed `#`).

- [ ] **Step 3g: Demo tag fetch + fixture** — in `demo.go`:

```go
func demoTagsFetchCmd() tea.Cmd {
	return func() tea.Msg {
		return tagsMsg{tags: []string{"focus", "client-A", "review", "urgent"}}
	}
}
```

And give at least one demo self-owned entry some `EntryTags` in `demoEntries` (e.g. `EntryTags: []string{"focus"}`) so the picker seeds a selection and the row shows tags.

- [ ] **Step 4: Run tests to verify they pass** — `go test ./internal/tui/ -run 'TOpensTag|TGatedOn|TagsMsg|SpaceToggles' -v` → PASS. `go build ./...`; `go test ./internal/tui/ -race`.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/entries.go internal/tui/app.go internal/tui/demo.go internal/tui/entries_tags_test.go
git commit -m "feat(tui): time-entry tag picker (fetch, multi-select, row display)"
```

---

## Task 4: Save + create new tags + demo persistence

**Files:**
- Modify: `internal/tui/entries.go` (`n` new-tag input, `enter` save, `setTagsCmd`, demo override)
- Test: `internal/tui/entries_tags_save_test.go`

**Interfaces:**
- Produces: `(m Model) setTagsCmd(entryID string, desired []string) tea.Cmd`.

- [ ] **Step 1: Write the failing test** — `internal/tui/entries_tags_save_test.go`:

```go
package tui

import (
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func openTagPicker(m Model, fetched []string) Model {
	next, _ := m.Update(keyRunes("t"))
	m = next.(Model)
	next, _ = m.Update(tagsMsg{tags: fetched})
	return next.(Model)
}

func TestNewTagAddsAndSelects(t *testing.T) {
	m := newTestModel()
	m.demo = true
	own := report.TimeEntry{ID: "e1", TaskName: "Fix", UserID: 1, Start: time.Now()}
	m = browserWithEntries(m, own)
	m = openTagPicker(m, []string{"focus"})

	// n enters new-tag mode; type "sprint-7"; enter adds+selects it.
	next, _ := m.Update(keyRunes("n"))
	m = next.(Model)
	if !m.entriesScreen.tagNewMode {
		t.Fatalf("n did not enter new-tag mode")
	}
	for _, r := range "sprint-7" {
		next, _ = m.Update(keyRunes(string(r)))
		m = next.(Model)
	}
	next, _ = m.Update(key("enter"))
	m = next.(Model)
	if m.entriesScreen.tagNewMode {
		t.Errorf("enter did not leave new-tag mode")
	}
	if !m.entriesScreen.tagSel["sprint-7"] {
		t.Errorf("new tag not selected")
	}
	found := false
	for _, s := range m.entriesScreen.tagAll {
		if s == "sprint-7" {
			found = true
		}
	}
	if !found {
		t.Errorf("new tag not added to tagAll")
	}
}

func TestSaveRecordsDemoOverrideAndReloads(t *testing.T) {
	m := newTestModel()
	m.demo = true
	m.loc = time.UTC
	own := report.TimeEntry{ID: "e1", TaskName: "Fix", UserID: 1, Start: time.Now(), EntryTags: []string{"focus"}}
	m = browserWithEntries(m, own)
	m = openTagPicker(m, []string{"focus", "client-A"})

	// toggle client-A on (cursor may be anywhere; set selection directly then save)
	m.entriesScreen.tagSel["client-A"] = true
	next, cmd := m.Update(key("enter")) // save
	mm := next.(Model)
	if cmd == nil {
		t.Fatalf("save did not dispatch a cmd")
	}
	ov, ok := mm.demoOverrides["e1"]
	if !ok {
		t.Fatalf("save did not record a demo override")
	}
	if !contains2(ov.EntryTags, "client-A") || !contains2(ov.EntryTags, "focus") {
		t.Errorf("override EntryTags = %v, want focus+client-A", ov.EntryTags)
	}
}

func contains2(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/tui/ -run 'NewTagAdds|SaveRecords' -v` → FAIL.

- [ ] **Step 3a: New-tag input mode** — extend the `entriesTags` case in `updateEntries`. When `es.tagNewMode`, route typing to the input; otherwise handle `n`/`enter` in list mode:

```go
	case entriesTags:
		if es.tagNewMode {
			switch msg.Type {
			case tea.KeyEnter:
				name := strings.TrimSpace(es.input.Value())
				es.tagNewMode = false
				if name != "" {
					if !es.tagSel[name] {
						es.tagSel[name] = true
					}
					if !slices.Contains(es.tagAll, name) {
						es.tagAll = unionSortedTags(es.tagAll, []string{name})
					}
					// move cursor to the new tag
					if i := slices.Index(es.tagAll, name); i >= 0 {
						es.tagIdx = i
					}
				}
				m.entriesScreen = es
				return m, nil
			case tea.KeyEsc:
				es.tagNewMode = false
				m.entriesScreen = es
				return m, nil
			}
			var cmd tea.Cmd
			es.input, cmd = es.input.Update(msg)
			m.entriesScreen = es
			return m, cmd
		}
		switch msg.String() {
		case "esc":
			es.mode = entriesList
		case "up", "k":
			if es.tagIdx > 0 {
				es.tagIdx--
			}
		case "down", "j":
			if es.tagIdx < len(es.tagAll)-1 {
				es.tagIdx++
			}
		case " ":
			if len(es.tagAll) > 0 {
				name := es.tagAll[es.tagIdx]
				es.tagSel[name] = !es.tagSel[name]
			}
		case "n":
			es.tagNewMode = true
			es.input = newTextInput("New tag name")
		case "enter":
			desired := selectedTags(es.tagSel)
			id := es.tagEntryID
			es.mode = entriesList
			m.entriesScreen = es
			if m.demo { // record the demo override BEFORE building the cmd
				if m.demoOverrides == nil {
					m.demoOverrides = map[string]report.TimeEntry{}
				}
				base := entryByID(es.entries, id)
				base.EntryTags = desired
				m.demoOverrides[id] = base
			}
			m.screen = screenLoading
			return m, m.setTagsCmd(id, desired)
		}
```

Add helpers to `entries.go`:

```go
// selectedTags returns the selected tag names, sorted (stable request bodies).
func selectedTags(sel map[string]bool) []string {
	var out []string
	for name, on := range sel {
		if on {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// entryByID returns a copy of the entry with the given id (zero value if absent).
func entryByID(entries []report.TimeEntry, id string) report.TimeEntry {
	for _, e := range entries {
		if e.ID == id {
			return e
		}
	}
	return report.TimeEntry{}
}
```

Ensure `strings` is imported in `entries.go`.

- [ ] **Step 3b: `setTagsCmd`** — in `entries.go`:

```go
func (m Model) setTagsCmd(entryID string, desired []string) tea.Cmd {
	mm := m
	if m.demo { // override already recorded in updateEntries
		return func() tea.Msg { return reloadForBrowser(mm, "Tags saved.") }
	}
	c := m.client
	teamID := m.cfg.WorkspaceID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := c.SetTimeEntryTags(ctx, teamID, entryID, desired); err != nil {
			return entriesErr(err)
		}
		return reloadForBrowser(mm, "Tags saved.")
	}
}
```

> If Task 2's spike forced `SetTimeEntryTags(ctx, teamID, entryID, current, desired)`, pass the entry's current tags here (`entryByID(m.entriesScreen.entries, entryID).EntryTags`).

- [ ] **Step 4: Run tests to verify they pass** — `go test ./internal/tui/ -run 'NewTagAdds|SaveRecords' -v` → PASS. Also add the new-tag input line to the picker view (Task 3's view branch): when `es.tagNewMode`, render `es.input.View()` under the list. `go build ./...`; `go test ./internal/tui/ -race`.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/entries.go internal/tui/entries_tags_save_test.go
git commit -m "feat(tui): save time-entry tags and create new tags from the picker"
```

---

## Task 5: Docs & demo

**Files:**
- Modify: `CHANGELOG.md`, `README.md`, `README.it.md`, `docs/demo.tape`
- Regenerate: `docs/demo.gif`

- [ ] **Step 1: CHANGELOG** — under `## [Unreleased]`, add an `### Added` bullet: from the entry browser, `t` on your own entry opens a tag picker to edit that entry's time-tracking tags (toggle existing, create new); note it's the entry's own tags, distinct from task tags. (Do not invent a version number — it stays under `[Unreleased]`.)

- [ ] **Step 2: README (both languages)** — add `t: edit tags` to the entry-browser command list in `README.md` and mirror in `README.it.md`, with one line clarifying these are the entry's time-tracking tags.

- [ ] **Step 3: demo tape** — update `docs/demo.tape` to show `v` → `t` → toggle a tag / add a new one → save, then regenerate: `vhs docs/demo.tape`. If `vhs` is unavailable, leave `docs/demo.gif` unchanged and flag it in the commit body (do not fake the GIF).

- [ ] **Step 4: Gate** — `gofmt -l .`, `go vet ./...`, `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`, `go test ./... -race` → all clean/green.

- [ ] **Step 5: Commit**

```bash
git add CHANGELOG.md README.md README.it.md docs/demo.tape docs/demo.gif
git commit -m "docs: document editing a time entry's tags (#125)"
```

---

## Self-Review (author checklist)

**Spec coverage:** EntryTags data model → Task 1; client GET/SET (spike) → Task 2; picker + `t` + row → Task 3; save + new tags + demo → Task 4; docs → Task 5. ✅

**Placeholder scan:** the exact tag-set API is explicitly spike-gated (Task 2 Step 1) — the one genuinely unknown shape, flagged not hidden, with a concrete BLOCKED escalation (not sacrificial). The `SetTimeEntryTags` signature may gain a `current` param if the diff mechanism wins — called out at both the definition (Task 2) and the caller (Task 4). No silent TBDs. ✅

**Type consistency:** `EntryTags` (Task 1) consumed in 3/4; `tagsMsg`/`tagsFetchCmd`/picker state (Task 3) consumed in 4; `setTagsCmd` + `demoOverrides` reuse (Task 4). Reused v1.8 infra (`canEdit`, `entriesErr`, `reloadForBrowser`, `demoEntriesSnapshot`) verified present. ✅

**Known risks handed to the reviewer:** (a) the tag-set mechanism is spike-gated and may change `SetTimeEntryTags`'s signature — Task 2; (b) the space-key string in `updateEntries` must match the framework — Task 3 note; (c) staticcheck is in the gate now (v1.8 lesson).
