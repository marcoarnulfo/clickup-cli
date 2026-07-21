package clickup

import (
	"context"
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
