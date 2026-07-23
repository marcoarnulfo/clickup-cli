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
// TODO(spike): verified against docs, not a live call. ClickUp's single-entry
// update endpoint documents "tags" (array of {name}) plus "tag_action"
// ("add"|"remove"|"replace") as accepted fields; this uses "replace" to set
// the tag set atomically in one request, avoiding the need for the entry's
// current tags (no add/remove diff required). If a live call shows the
// single-entry PUT does not honor tag_action, fall back to the add/remove
// diff mechanism: POST /team/{id}/time_entries/tags (add) + DELETE
// /team/{id}/time_entries/tags (remove), body {"time_entry_ids":["<id>"],
// "tags":[{"name":...}]}, computed as desired-vs-current — which would also
// require widening this signature to accept the entry's current tags.
func (c *Client) SetTimeEntryTags(ctx context.Context, teamID, entryID string, desired []string) error {
	tags := make([]map[string]any, 0, len(desired))
	for _, name := range desired {
		tags = append(tags, map[string]any{"name": name})
	}
	body := map[string]any{"tags": tags, "tag_action": "replace"}
	return c.put(ctx, "/team/"+teamID+"/time_entries/"+entryID, body, nil)
}
