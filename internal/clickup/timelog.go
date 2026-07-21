package clickup

import (
	"context"
	"strings"
	"time"
)

// CreateTimeEntry creates a time entry for the authenticated user.
// POST /team/{team_id}/time_entries with {tid, start(ms), duration(ms), description}.
func (c *Client) CreateTimeEntry(ctx context.Context, teamID, tid string, start time.Time, dur time.Duration, description string) error {
	body := map[string]any{
		"tid":         tid,
		"start":       start.UnixMilli(),
		"duration":    dur.Milliseconds(),
		"description": description,
	}
	return c.post(ctx, "/team/"+teamID+"/time_entries", body, nil)
}

// Task is a minimal ClickUp task (id + name) for the TUI picker.
type Task struct {
	ID   string
	Name string
}

// ListTasks returns the tasks of a list. GET /list/{list_id}/task.
func (c *Client) ListTasks(ctx context.Context, listID string) ([]Task, error) {
	var resp struct {
		Tasks []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"tasks"`
	}
	if err := c.get(ctx, "/list/"+listID+"/task", nil, &resp); err != nil {
		return nil, err
	}
	out := make([]Task, len(resp.Tasks))
	for i, t := range resp.Tasks {
		out[i] = Task{ID: t.ID, Name: t.Name}
	}
	return out, nil
}

// ExtractTaskID extracts the task id from a bare id or a ClickUp URL
// (.../t/<id> or .../<id>). Returns "" if it finds nothing plausible.
func ExtractTaskID(input string) string {
	s := strings.TrimSpace(input)
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "/") {
		return s // bare id
	}
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i] // remove query/fragment
	}
	s = strings.TrimRight(s, "/")
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		s = s[idx+1:]
	}
	return s
}
