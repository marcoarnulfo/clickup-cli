package clickup

import (
	"context"
	"time"
)

// UpdateTimeEntry updates the core fields of a time entry.
// PUT /team/{team_id}/time_entries/{timer_id}. start/duration are milliseconds,
// mirroring CreateTimeEntry's wire format. The full core set is always sent
// (not a partial diff) so the server never keeps a stale field.
func (c *Client) UpdateTimeEntry(ctx context.Context, teamID, entryID string, start time.Time, dur time.Duration, description string, billable bool) error {
	body := map[string]any{
		"start":       start.UnixMilli(),
		"duration":    dur.Milliseconds(),
		"description": description,
		"billable":    billable,
	}
	return c.put(ctx, "/team/"+teamID+"/time_entries/"+entryID, body, nil)
}

// DeleteTimeEntry removes a time entry.
// DELETE /team/{team_id}/time_entries/{timer_id}.
func (c *Client) DeleteTimeEntry(ctx context.Context, teamID, entryID string) error {
	return c.del(ctx, "/team/"+teamID+"/time_entries/"+entryID)
}
