// Package clickup is a minimal client for the ClickUp API v2 (time tracking).
package clickup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ErrUnauthorized indicates a missing/invalid/revoked token (HTTP 401).
// Callers use it with errors.Is to relaunch the setup wizard.
var ErrUnauthorized = errors.New("unauthorized token")

// retryDelay is the backoff between retries on 429 (overridden in tests).
var retryDelay = 2 * time.Second

// maxRetries is the maximum number of retries on 429.
const maxRetries = 2

// Client queries the ClickUp API v2.
type Client struct {
	token   string
	BaseURL string
	http    *http.Client

	mu        sync.Mutex        // protects listNames (used by commands running in a goroutine)
	listNames map[string]string // cache list_id -> name
}

// New creates a client with the personal token.
func New(token string) *Client {
	return &Client{
		token:     token,
		BaseURL:   "https://api.clickup.com/api/v2",
		http:      &http.Client{Timeout: 30 * time.Second},
		listNames: make(map[string]string),
	}
}

// apiError represents ClickUp's standard error body.
type apiError struct {
	Err   string `json:"err"`
	ECODE string `json:"ECODE"`
}

// get performs an authenticated GET and decodes the JSON into out.
func (c *Client) get(ctx context.Context, path string, query map[string]string, out any) error {
	return c.getRetry(ctx, path, query, out, 0)
}

// getRetry implements the GET with limited backoff on 429 (attempt = attempts already made).
func (c *Client) getRetry(ctx context.Context, path string, query map[string]string, out any, attempt int) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.token)
	if query != nil {
		q := req.URL.Query()
		for k, v := range query {
			if v != "" {
				q.Set(k, v)
			}
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusTooManyRequests {
		if attempt >= maxRetries {
			return fmt.Errorf("clickup API 429: rate limit exceeded after %d attempts", attempt)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
		}
		return c.getRetry(ctx, path, query, out, attempt+1)
	}
	return finishJSON(resp.StatusCode, body, out)
}

// finishJSON handles the status code and decoding shared by GET and POST.
func finishJSON(status int, body []byte, out any) error {
	if status == http.StatusUnauthorized {
		var ae apiError
		_ = json.Unmarshal(body, &ae)
		return fmt.Errorf("%w: %s", ErrUnauthorized, ae.Err)
	}
	if status < 200 || status >= 300 {
		var ae apiError
		_ = json.Unmarshal(body, &ae)
		if ae.Err != "" {
			return fmt.Errorf("clickup API %d: %s (%s)", status, ae.Err, ae.ECODE)
		}
		return fmt.Errorf("clickup API %d: %s", status, string(body))
	}
	if out != nil {
		return json.Unmarshal(body, out)
	}
	return nil
}

// post performs an authenticated POST with a JSON body (body nil = no body) and,
// if out != nil, decodes the response.
func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return finishJSON(resp.StatusCode, respBody, out)
}
