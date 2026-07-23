package clickup

import (
	"context"
	"encoding/json"
	"strconv"
	"time"
)

// HistoryChange is one recorded change to a time entry. Before/After are
// normalized to strings (the API may return scalars or objects) so the view
// never breaks on an unexpected type.
type HistoryChange struct {
	Field  string
	Before string
	After  string
	Date   time.Time
	User   string
}

// jsonString normalizes any JSON value to a display string: strings verbatim,
// numbers as their literal, everything else as compact JSON.
func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String()
	}
	return string(raw) // object/array: raw JSON fallback
}

// TimeEntryHistory returns the change history of a time entry, oldest first.
// GET /team/{team_id}/time_entries/{timer_id}/history.
// TODO(spike): verified against the documented ClickUp v2 payload shape, not a live call.
func (c *Client) TimeEntryHistory(ctx context.Context, teamID, entryID string) ([]HistoryChange, error) {
	var resp struct {
		Data []struct {
			Field  string          `json:"field"`
			Before json.RawMessage `json:"before"`
			After  json.RawMessage `json:"after"`
			Date   flexString      `json:"date"`
			User   struct {
				Username string `json:"username"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/team/"+teamID+"/time_entries/"+entryID+"/history", nil, &resp); err != nil {
		return nil, err
	}
	out := make([]HistoryChange, 0, len(resp.Data))
	for _, d := range resp.Data {
		var when time.Time
		if ms, err := strconv.ParseInt(string(d.Date), 10, 64); err == nil {
			when = time.UnixMilli(ms).UTC()
		}
		out = append(out, HistoryChange{
			Field:  d.Field,
			Before: jsonString(d.Before),
			After:  jsonString(d.After),
			Date:   when,
			User:   d.User.Username,
		})
	}
	return out, nil
}
