package clickup

import "context"

// ListName risolve il nome di una lista (GET /list/{id}) con cache in-memory.
// In caso d'errore ritorna il list_id come fallback (insieme all'errore).
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
