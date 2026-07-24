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
// The request shape is confirmed against ClickUp's official OpenAPI v2 spec for
// "Update a time Entry": the body accepts a "tags" array of {name, tag_bg?,
// tag_fg?} objects and a "tag_action" string of "replace"|"add"|"remove" (one
// action per request). "replace" sets the tag set atomically, so no
// desired-vs-current diff (and no current-tags parameter) is needed. It has not
// yet been exercised against a live workspace (see #129); if a live call ever
// contradicts the spec, the documented fallback is the dedicated collection
// endpoints — POST /team/{id}/time_entries/tags (add) + DELETE (remove), body
// {"time_entry_ids":["<id>"],"tags":[{"name":...}]}, computed as
// desired-vs-current, which would widen this signature to take the current set.
func (c *Client) SetTimeEntryTags(ctx context.Context, teamID, entryID string, desired []string) error {
	tags := make([]map[string]any, 0, len(desired))
	for _, name := range desired {
		tags = append(tags, map[string]any{"name": name})
	}
	body := map[string]any{"tags": tags, "tag_action": "replace"}
	return c.put(ctx, "/team/"+teamID+"/time_entries/"+entryID, body, nil)
}
