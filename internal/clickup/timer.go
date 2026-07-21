package clickup

import (
	"context"
	"strconv"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// RunningTimer describes a timer currently running.
type RunningTimer struct {
	TaskID   string
	TaskName string
	Start    time.Time
}

// StartTimer starts a timer on task tid. POST /team/{id}/time_entries/start.
func (c *Client) StartTimer(ctx context.Context, teamID, tid, description string) error {
	body := map[string]any{"tid": tid, "description": description}
	return c.post(ctx, "/team/"+teamID+"/time_entries/start", body, nil)
}

// StopTimer stops the running timer and returns the time entry created by ClickUp.
// POST /team/{id}/time_entries/stop.
func (c *Client) StopTimer(ctx context.Context, teamID string) (report.TimeEntry, error) {
	var resp struct {
		Data rawEntry `json:"data"`
	}
	if err := c.post(ctx, "/team/"+teamID+"/time_entries/stop", nil, &resp); err != nil {
		return report.TimeEntry{}, err
	}
	return resp.Data.toTimeEntry()
}

// CurrentTimer returns the running timer, or nil if there isn't one.
// GET /team/{id}/time_entries/current.
func (c *Client) CurrentTimer(ctx context.Context, teamID string) (*RunningTimer, error) {
	var resp struct {
		Data struct {
			Task struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"task"`
			Start flexString `json:"start"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/team/"+teamID+"/time_entries/current", nil, &resp); err != nil {
		return nil, err
	}
	if resp.Data.Task.ID == "" {
		return nil, nil // no timer running
	}
	var start time.Time
	if ms, err := strconv.ParseInt(string(resp.Data.Start), 10, 64); err == nil {
		start = time.UnixMilli(ms).UTC()
	}
	return &RunningTimer{
		TaskID:   resp.Data.Task.ID,
		TaskName: resp.Data.Task.Name,
		Start:    start,
	}, nil
}
