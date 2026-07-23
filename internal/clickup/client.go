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
	"net/url"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ErrUnauthorized indicates a missing/invalid/revoked token (HTTP 401).
// Callers use it with errors.Is to relaunch the setup wizard.
var ErrUnauthorized = errors.New("unauthorized token")

// retryDelay is the backoff between retries on 429 when the server gives no
// Retry-After header (overridden in tests).
var retryDelay = 2 * time.Second

// retryAfterClamp bounds how long we honor a server-supplied Retry-After,
// so a misbehaving/far-future value can't stall the caller for hours
// (overridden in tests).
var retryAfterClamp = 30 * time.Second

// maxRetries is the maximum number of retries on 429.
const maxRetries = 2

// Client queries the ClickUp API v2.
type Client struct {
	token   string
	BaseURL string
	http    *http.Client
	limiter *rate.Limiter // shared across all requests to stay under ClickUp's rate limit

	mu        sync.Mutex        // protects listNames (used by commands running in a goroutine)
	listNames map[string]string // cache list_id -> name
}

// New creates a client with the personal token.
func New(token string) *Client {
	return &Client{
		token:   token,
		BaseURL: "https://api.clickup.com/api/v2",
		http:    &http.Client{Timeout: 30 * time.Second},
		// ClickUp's documented limit is 100 req/min; stay comfortably under it.
		limiter:   rate.NewLimiter(rate.Limit(90.0/60.0), 30),
		listNames: make(map[string]string),
	}
}

// apiError represents ClickUp's standard error body.
type apiError struct {
	Err   string `json:"err"`
	ECODE string `json:"ECODE"`
}

// retryAfter parses the Retry-After header (RFC 9110): either delta-seconds
// or an HTTP-date. It returns the wait duration and whether the header was
// present and parseable.
func retryAfter(h http.Header) (time.Duration, bool) {
	v := h.Get("Retry-After")
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		return time.Until(t), true
	}
	return 0, false
}

// do performs an authenticated HTTP request through the shared rate limiter,
// retrying on 429 (honoring Retry-After, clamped) up to maxRetries times, and
// decodes the JSON response into out via finishJSON.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	for attempt := 0; ; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return err
		}
		var reader io.Reader
		if body != nil {
			buf, err := json.Marshal(body)
			if err != nil {
				return err
			}
			reader = bytes.NewReader(buf)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", c.token)
		// Preserve current behavior: post() set Content-Type on every POST, even a
		// nil body (e.g. StopTimer). Keep it for any POST or any body-carrying request.
		if body != nil || method == http.MethodPost {
			req.Header.Set("Content-Type", "application/json")
		}
		if len(query) > 0 {
			req.URL.RawQuery = query.Encode()
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt >= maxRetries {
				return fmt.Errorf("clickup API 429: rate limit exceeded after %d attempts", attempt)
			}
			wait := retryDelay
			if d, ok := retryAfter(resp.Header); ok {
				wait = d
			}
			if wait > retryAfterClamp {
				wait = retryAfterClamp
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			continue // retry re-consumes the limiter at loop top
		}
		return finishJSON(resp.StatusCode, bodyBytes, out)
	}
}

// get performs an authenticated GET and decodes the JSON into out. It keeps
// the map ergonomics of the old API (skipping empty values).
func (c *Client) get(ctx context.Context, path string, query map[string]string, out any) error {
	var q url.Values
	if len(query) > 0 {
		q = url.Values{}
		for k, v := range query {
			if v != "" {
				q.Set(k, v)
			}
		}
	}
	return c.do(ctx, http.MethodGet, path, q, nil, out)
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
	return c.do(ctx, http.MethodPost, path, nil, body, out)
}

// put performs an authenticated PUT with a JSON body and, if out != nil,
// decodes the response.
func (c *Client) put(ctx context.Context, path string, body any, out any) error {
	return c.do(ctx, http.MethodPut, path, nil, body, out)
}

// del performs an authenticated DELETE (no body).
func (c *Client) del(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil, nil)
}
