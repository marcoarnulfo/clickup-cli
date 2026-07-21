package clickup

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// flexString decodes a JSON field that can arrive as a string, a number,
// or null (ClickUp ids vary between endpoints). It always normalizes to a string;
// null becomes an empty string. Strings are properly de-escaped.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*f = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = flexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err == nil {
		*f = flexString(n.String())
		return nil
	}
	return fmt.Errorf("flexString: unhandled value: %s", b)
}

// rawEntry mirrors an entry of the "data" array from /team/{id}/time_entries.
type rawEntry struct {
	ID   string `json:"id"`
	Task struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"task"`
	TaskLocation struct {
		ListID flexString `json:"list_id"`
	} `json:"task_location"`
	User struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
	} `json:"user"`
	Start    string `json:"start"`    // epoch ms as a string
	Duration string `json:"duration"` // ms as a string (negative if a timer is running)
	TaskTags []struct {
		Name string `json:"name"`
	} `json:"task_tags"`
}

// toTimeEntry converts a rawEntry into the domain type. Errors if start/duration
// can't be parsed. Note: it does NOT filter out negative durations (caller's job).
func (r rawEntry) toTimeEntry() (report.TimeEntry, error) {
	ms, err := strconv.ParseInt(r.Duration, 10, 64)
	if err != nil {
		return report.TimeEntry{}, fmt.Errorf("invalid duration for entry %s: %q", r.ID, r.Duration)
	}
	startMs, err := strconv.ParseInt(r.Start, 10, 64)
	if err != nil {
		return report.TimeEntry{}, fmt.Errorf("invalid start for entry %s: %q", r.ID, r.Start)
	}
	listID := string(r.TaskLocation.ListID)
	tags := make([]string, 0, len(r.TaskTags))
	for _, t := range r.TaskTags {
		tags = append(tags, t.Name)
	}
	return report.TimeEntry{
		ID:       r.ID,
		TaskID:   r.Task.ID,
		TaskName: r.Task.Name,
		ListID:   listID,
		ListName: listID,
		UserID:   r.User.ID,
		UserName: r.User.Username,
		Start:    time.UnixMilli(startMs).UTC(),
		Duration: time.Duration(ms) * time.Millisecond,
		Tags:     tags,
	}, nil
}

// TimeEntries returns the workspace's time entries in the range [start, end).
// If assignees is non-empty, it filters on those users (team scope).
// Entries with negative duration (a running timer) are discarded.
func (c *Client) TimeEntries(ctx context.Context, teamID string, start, end time.Time, assignees []int) ([]report.TimeEntry, error) {
	q := map[string]string{
		"start_date":        strconv.FormatInt(start.UnixMilli(), 10),
		"end_date":          strconv.FormatInt(end.UnixMilli(), 10),
		"include_task_tags": "true",
	}
	if len(assignees) > 0 {
		ids := make([]string, len(assignees))
		for i, a := range assignees {
			ids[i] = strconv.Itoa(a)
		}
		q["assignee"] = strings.Join(ids, ",")
	}

	var resp struct {
		Data []rawEntry `json:"data"`
	}
	if err := c.get(ctx, "/team/"+teamID+"/time_entries", q, &resp); err != nil {
		return nil, err
	}

	out := make([]report.TimeEntry, 0, len(resp.Data))
	for _, r := range resp.Data {
		e, err := r.toTimeEntry()
		if err != nil {
			return nil, err
		}
		if e.Duration < 0 {
			continue // timer running: negative duration, not booked time
		}
		out = append(out, e)
	}
	return out, nil
}
