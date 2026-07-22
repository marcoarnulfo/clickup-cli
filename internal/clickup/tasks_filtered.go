package clickup

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// TaskFilter narrows a FilteredTeamTasks query. Zero-value fields are omitted
// from the request (no filtering on that dimension).
type TaskFilter struct {
	Statuses      []string
	Assignees     []string
	ListIDs       []string
	Tags          []string
	DateUpdatedGt int64
	DateUpdatedLt int64
}

// query builds the url.Values for this filter (without the page number,
// which FilteredTeamTasks sets separately).
func (f TaskFilter) query() url.Values {
	q := url.Values{}
	for _, s := range f.Statuses {
		q.Add("statuses[]", s)
	}
	for _, a := range f.Assignees {
		q.Add("assignees[]", a)
	}
	for _, l := range f.ListIDs {
		q.Add("list_ids[]", l)
	}
	for _, t := range f.Tags {
		q.Add("tags[]", t)
	}
	if f.DateUpdatedGt != 0 {
		q.Set("date_updated_gt", strconv.FormatInt(f.DateUpdatedGt, 10))
	}
	if f.DateUpdatedLt != 0 {
		q.Set("date_updated_lt", strconv.FormatInt(f.DateUpdatedLt, 10))
	}
	return q
}

// FilteredTeamTasks returns one page of tasks across a team/workspace,
// narrowed by f. GET /team/{team_id}/task.
//
// ClickUp v2 exposes no page metadata on this endpoint, so hasMore is
// inferred from a full 100-task page (ClickUp's fixed page size).
func (c *Client) FilteredTeamTasks(ctx context.Context, teamID string, f TaskFilter, page int) ([]Task, bool, error) {
	q := f.query()
	q.Set("page", strconv.Itoa(page))

	var resp struct {
		Tasks []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status struct {
				Status string `json:"status"`
			} `json:"status"`
			List struct {
				ID string `json:"id"`
			} `json:"list"`
		} `json:"tasks"`
	}
	if err := c.do(ctx, http.MethodGet, "/team/"+teamID+"/task", q, nil, &resp); err != nil {
		return nil, false, err
	}

	out := make([]Task, len(resp.Tasks))
	for i, t := range resp.Tasks {
		out[i] = Task{
			ID:     t.ID,
			Name:   t.Name,
			Status: t.Status.Status,
			ListID: t.List.ID,
		}
	}
	hasMore := len(out) == 100
	return out, hasMore, nil
}

// AllFilteredTeamTasks pages through FilteredTeamTasks until exhausted and
// returns the concatenated result.
func (c *Client) AllFilteredTeamTasks(ctx context.Context, teamID string, f TaskFilter) ([]Task, error) {
	var all []Task
	for page := 0; ; page++ {
		tasks, hasMore, err := c.FilteredTeamTasks(ctx, teamID, f, page)
		if err != nil {
			return nil, err
		}
		all = append(all, tasks...)
		if !hasMore {
			return all, nil
		}
	}
}
