# Edit time-entry tags (#125) — Design

> Spec for #125, the follow-up deferred from #94 (v1.8). Adds editing of a time
> entry's own **time-tracking tags** from the entry browser. Branch:
> `feat/edit-entry-tags`.

## 1. Goal

Let a user **view and edit the tags of a single time entry** — ClickUp's
per-entry *time-tracking tags*, not the task's tags — from the v1.8 entry
browser, via a dedicated multi-select picker that can also **create new tags**,
with full demo parity and the pure packages kept pure.

## 2. Key distinction (decided)

ClickUp has two separate tag concepts:
- **Task tags** (`task_tags` in the entry payload) — tags on the *task*. Today
  `report.TimeEntry.Tags` is mapped from these, and the **report groups and
  filters by them**. This feature does **not** touch them.
- **Time-entry tags** (time-tracking tags) — tags on the *entry itself*, a
  workspace-level set. This is what #125 edits.

## 3. Scope

**In:** a `t` action on the browser (own entries) → a multi-select tag picker
seeded with the entry's current time-entry tags; toggle existing workspace tags;
create a new tag on the fly; save (reconciles to the desired set); demo parity.

**Out:** editing task tags; renaming/deleting workspace tags; bulk-tagging
multiple entries; tag colors in the UI (colors are sent as defaults if the API
requires them, but not surfaced/edited).

## 4. Data model (report stays pure)

- `report.TimeEntry` gains **`EntryTags []string`** — the entry's own
  time-tracking tags, **distinct from `Tags`** (task tags). One plain field, no
  new imports; `internal/report` stays pure. The report's tag grouping/filtering
  continues to use `Tags` and is unchanged.
- `rawEntry` (`internal/clickup/timeentries.go`) gains `Tags []struct{ Name string } \`json:"tags"\`` (the entry's own tags, currently ignored — only `task_tags` is parsed) and `toTimeEntry` sets `EntryTags` from it (order preserved).

## 5. Client — SPIKE-FIRST (the exact API is uncertain)

The time-entry tag API is the least-documented part. **The first task is a spike**
(`httptest` fixtures from the documented v2 shape) to confirm, before building:
1. `GET /team/{team_id}/time_entries/tags` — the workspace's time-entry tags
   (each has at least `name`, plus colors we ignore).
2. How to **set** an entry's tags. Two candidate mechanisms; the spike decides:
   - the single-entry `PUT /team/{id}/time_entries/{id}` with `tags` + `tag_action`, or
   - the collection endpoints `POST` / `DELETE /team/{id}/time_entries/tags`
     (body `{time_entry_ids: [id], tags: [...]}`) applied as an add/remove diff.
3. Whether adding a tag name that doesn't exist **auto-creates** it (expected for
   time-entry tags), or needs a separate create call.

Client surface (stable regardless of which mechanism the spike selects):
- `TimeEntryTags(ctx, teamID string) ([]string, error)` → the workspace tag names.
- `SetTimeEntryTags(ctx, teamID, entryID string, desired []string) error` — sets
  the entry's tags to exactly `desired`. Internally it either sends a `replace`
  (if the PUT supports it) or reconciles via add/remove diff against the entry's
  current tags. New (unknown) names are created by the same call if the API
  auto-creates; otherwise the method creates them first. The **picker never sees
  this branching** — it hands over a desired set.
- Tag payloads that require color fields use ClickUp's defaults (the spike pins
  the exact object shape; a plausible `tag_bg`/`tag_fg` default is used).

If the spike shows the endpoint genuinely cannot set per-entry tags, STOP and
report — but unlike history this feature is **not** sacrificial: the user asked
for it. A blocked spike escalates to the human (the API shape may need a real
call to confirm), it does not silently drop the feature.

## 6. Picker UI (`entriesTags` mode)

- New `entriesMode` value `entriesTags` (appended to the existing iota:
  `entriesList`, `entriesConfirmDelete`, `entriesEdit`, `entriesHistory`, +`entriesTags`).
- `t` in `entriesList` opens it, **ownership-gated** (`canEdit`, like `e`/`x`):
  sets `entriesTags` + `tagLoading = true` and dispatches the tag fetch. `t` on a
  non-owned entry is a no-op (status: read-only).
- State on `entriesModel`:
  - `tagAll []string` — workspace tags (fetched), **unioned** with the entry's
    current `EntryTags` (defensive: a current tag missing from the fetched list
    still shows), sorted, deduped.
  - `tagSel map[string]bool` — selected set, seeded from the entry's `EntryTags`.
  - `tagIdx int` — cursor.
  - `tagNewMode bool` + `input textinput.Model` — new-tag entry.
  - `tagLoading bool`, `tagEntryID string`.
- Keys (list mode): `↑/↓`/`k/j` move · `space` toggle the tag under the cursor ·
  `n` → new-tag input · `enter` save · `esc` cancel (back to `entriesList`).
- Keys (new-tag input mode): type the name; `enter` adds it to `tagAll`
  (if absent) and selects it, returns to list mode with the cursor on it;
  `esc` cancels the input (back to list). A blank or duplicate name is a no-op.
- View: `Tags: <task name>`, then the tag list with `[x]`/`[ ]` and a cursor,
  then (in new-tag mode) the input line, then the help line. `tagLoading` shows
  "Loading tags…".
- Reuse: the `membersModel` multi-select idiom (`selected map`, space toggle) and
  `newTextInput`.

## 7. Commands & demo parity

- `tagsFetchCmd(entryID)` — real: `TimeEntryTags`; demo: a fixed workspace tag
  set — returns `tagsMsg{tags []string}`. A fetch failure routes via `entriesErr`
  (stay in the browser, `msgErr = true`), never `screenError`.
- `setTagsCmd(entryID string, desired []string)` — real: `SetTimeEntryTags` then
  `reloadForBrowser(mm, "Tags saved.")`; demo: record a `demoOverrides` entry with
  updated `EntryTags` **before** building the cmd (same pattern as edit/delete),
  then reload. Errors via `entriesErr`.
- Demo reuses `demoOverrides` (a full `report.TimeEntry`): `demoEntriesSnapshot`
  already applies overrides on every reload path, so demo tag edits persist.
- Demo entries carry some `EntryTags` in the fixture so the picker is
  demonstrable, and the demo GIF is regenerated to show `t` + the picker.

## 8. Browser row

The entry row shows its time-entry tags compactly at the end, truncated
(e.g. ` #focus #client-A`), so the user sees what they're editing. Distinct from
the task-tag data the report groups by (which the row never showed).

## 9. Ownership, testing, constraints

- Ownership: tag editing is limited to your **own** entries (`canEdit`),
  consistent with edit/delete. History-style read-only-on-any does not apply
  (this is a mutation).
- `internal/report` / `internal/duration` stay pure. bubbletea value-receiver;
  write-back before return; new msgs (`tagsMsg`) in the top-level `Update` switch.
- TDD (RED → GREEN). Client verbs via `httptest` + the `newTestClient` helper.
  TUI via `Update()` + simulated msgs.
- **Pre-commit gate: `gofmt -l .`, `go vet ./...`, `go run honnef.co/go/tools/cmd/staticcheck@latest ./...`, `go build ./...`, `go test ./... -race`** — all clean/green. (staticcheck is in CI and MUST be in the local gate — it broke v1.8's first CI run.)
- Everything ENGLISH; Conventional Commits; **no `Co-Authored-By` trailer**.

## 10. Out of scope / follow-ups

- Creating/renaming/deleting workspace tags as a management screen.
- Tag colors in the UI.
- Bulk tagging across entries.
