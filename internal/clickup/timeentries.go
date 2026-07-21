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

// flexString decodifica un campo JSON che può arrivare come stringa, come numero
// o come null (gli id ClickUp variano tra endpoint). Normalizza sempre a stringa;
// null diventa stringa vuota. Le stringhe sono de-escaped correttamente.
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

// toTimeEntry converte una rawEntry nel tipo dominio. Errore su start/duration
// non parsabili. Nota: NON filtra le durate negative (compito del chiamante).
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
	}, nil
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
		e, err := r.toTimeEntry()
		if err != nil {
			return nil, err
		}
		if e.Duration < 0 {
			continue // timer in corso: durata negativa, non è tempo consuntivato
		}
		out = append(out, e)
	}
	return out, nil
}
