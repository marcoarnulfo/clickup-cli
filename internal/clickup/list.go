package clickup

import (
	"context"
	"sync"
)

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

// listNameConcurrency bounds how many /list/{id} lookups run at once in ListNames.
const listNameConcurrency = 8

// ListNames resolves multiple list names concurrently (bounded concurrency),
// caching each via ListName. It returns list_id -> name only for the lists it
// could resolve; unresolved ids are omitted. Duplicate and empty ids are ignored.
func (c *Client) ListNames(ctx context.Context, ids []string) map[string]string {
	unique := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		unique[id] = struct{}{}
	}

	result := make(map[string]string, len(unique))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, listNameConcurrency)

	for id := range unique {
		wg.Add(1)
		sem <- struct{}{}
		go func(id string) {
			defer wg.Done()
			defer func() { <-sem }()
			if name, err := c.ListName(ctx, id); err == nil {
				mu.Lock()
				result[id] = name
				mu.Unlock()
			}
		}(id)
	}
	wg.Wait()
	return result
}
