package clickup

import (
	"context"
	"strings"
	"time"
)

// CreateTimeEntry crea una time entry per l'utente autenticato.
// POST /team/{team_id}/time_entries con {tid, start(ms), duration(ms), description}.
func (c *Client) CreateTimeEntry(ctx context.Context, teamID, tid string, start time.Time, dur time.Duration, description string) error {
	body := map[string]any{
		"tid":         tid,
		"start":       start.UnixMilli(),
		"duration":    dur.Milliseconds(),
		"description": description,
	}
	return c.post(ctx, "/team/"+teamID+"/time_entries", body, nil)
}

// Task è un task ClickUp minimale (id + nome) per il picker della TUI.
type Task struct {
	ID   string
	Name string
}

// ListTasks ritorna i task di una lista. GET /list/{list_id}/task.
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

// ExtractTaskID estrae l'id task da un id nudo o da un URL ClickUp
// (.../t/<id> o .../<id>). Ritorna "" se non trova nulla di plausibile.
func ExtractTaskID(input string) string {
	s := strings.TrimSpace(input)
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "/") {
		return s // id nudo
	}
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i] // rimuovi query/fragment
	}
	s = strings.TrimRight(s, "/")
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		s = s[idx+1:]
	}
	return s
}
