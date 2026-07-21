package clickup

import "context"

// ListName resolves a list's name (GET /list/{id}) with an in-memory cache.
// On error it returns the list_id as a fallback (along with the error).
func (c *Client) ListName(ctx context.Context, listID string) (string, error) {
	c.mu.Lock()
	name, ok := c.listNames[listID]
	c.mu.Unlock()
	if ok {
		return name, nil
	}

	var resp struct {
		Name string `json:"name"`
	}
	if err := c.get(ctx, "/list/"+listID, nil, &resp); err != nil {
		return listID, err
	}

	c.mu.Lock()
	c.listNames[listID] = resp.Name
	c.mu.Unlock()
	return resp.Name, nil
}
