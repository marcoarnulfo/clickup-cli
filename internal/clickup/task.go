package clickup

import "context"

// TaskStatus returns the current status name of a task (GET /task/{id}).
func (c *Client) TaskStatus(ctx context.Context, taskID string) (string, error) {
	var resp struct {
		Status struct {
			Status string `json:"status"`
		} `json:"status"`
	}
	if err := c.get(ctx, "/task/"+taskID, nil, &resp); err != nil {
		return "", err
	}
	return resp.Status.Status, nil
}
