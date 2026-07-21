package clickup

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// flexString decodifica un campo JSON che può arrivare come stringa OPPURE
// come numero (gli id ClickUp variano tra endpoint). Normalizza sempre a stringa.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	*f = flexString(strings.Trim(string(b), `"`))
	return nil
}

// rawEntry rispecchia una voce dell'array "data" di /team/{id}/time_entries.
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
	Start    string `json:"start"`    // epoch ms come stringa
	Duration string `json:"duration"` // ms come stringa (negativa se timer in corso)
}

// TimeEntries ritorna le voci di tempo del workspace nel range [start, end).
// Se assignees è non vuoto, filtra su quegli utenti (scope team).
// Le voci con durata negativa (timer in esecuzione) vengono scartate.
func (c *Client) TimeEntries(ctx context.Context, teamID string, start, end time.Time, assignees []int) ([]report.TimeEntry, error) {
	q := map[string]string{
		"start_date": strconv.FormatInt(start.UnixMilli(), 10),
		"end_date":   strconv.FormatInt(end.UnixMilli(), 10),
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
		ms, _ := strconv.ParseInt(r.Duration, 10, 64)
		if ms < 0 {
			continue // timer in corso: durata negativa, non è tempo consuntivato
		}
		startMs, _ := strconv.ParseInt(r.Start, 10, 64)
		listID := string(r.TaskLocation.ListID)
		out = append(out, report.TimeEntry{
			ID:       r.ID,
			TaskID:   r.Task.ID,
			TaskName: r.Task.Name,
			ListID:   listID,
			ListName: listID, // nome lista risolto in v1.1; per ora l'ID
			UserID:   r.User.ID,
			UserName: r.User.Username,
			Start:    time.UnixMilli(startMs).UTC(),
			Duration: time.Duration(ms) * time.Millisecond,
		})
	}
	return out, nil
}
