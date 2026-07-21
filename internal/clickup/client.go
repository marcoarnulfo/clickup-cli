// Package clickup è un client minimale per la ClickUp API v2 (time tracking).
package clickup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ErrUnauthorized indica un token mancante/invalido/revocato (HTTP 401).
// I chiamanti la usano con errors.Is per rilanciare il setup wizard.
var ErrUnauthorized = errors.New("token non autorizzato")

// retryDelay è il backoff tra i tentativi in caso di 429 (override nei test).
var retryDelay = 2 * time.Second

// maxRetries è il numero massimo di ritentativi su 429.
const maxRetries = 2

// Client interroga la ClickUp API v2.
type Client struct {
	token   string
	BaseURL string
	http    *http.Client

	mu        sync.Mutex        // protegge listNames (usata da comandi in goroutine)
	listNames map[string]string // cache list_id -> nome
}

// New crea un client con il token personale.
func New(token string) *Client {
	return &Client{
		token:     token,
		BaseURL:   "https://api.clickup.com/api/v2",
		http:      &http.Client{Timeout: 30 * time.Second},
		listNames: make(map[string]string),
	}
}

// apiError rappresenta il corpo d'errore standard di ClickUp.
type apiError struct {
	Err   string `json:"err"`
	ECODE string `json:"ECODE"`
}

// get esegue una GET autenticata e decodifica il JSON in out.
func (c *Client) get(ctx context.Context, path string, query map[string]string, out any) error {
	return c.getRetry(ctx, path, query, out, 0)
}

// getRetry implementa la GET con backoff limitato sul 429 (attempt = tentativi già fatti).
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
			return fmt.Errorf("clickup API 429: rate limit superato dopo %d tentativi", attempt)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
		}
		return c.getRetry(ctx, path, query, out, attempt+1)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		var ae apiError
		_ = json.Unmarshal(body, &ae)
		return fmt.Errorf("%w: %s", ErrUnauthorized, ae.Err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var ae apiError
		_ = json.Unmarshal(body, &ae)
		if ae.Err != "" {
			return fmt.Errorf("clickup API %d: %s (%s)", resp.StatusCode, ae.Err, ae.ECODE)
		}
		return fmt.Errorf("clickup API %d: %s", resp.StatusCode, string(body))
	}
	return json.Unmarshal(body, out)
}
