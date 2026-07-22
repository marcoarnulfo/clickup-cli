package clickup

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestDoRetriesOn429WithRetryAfterSeconds(t *testing.T) {
	var calls int
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte(`{"ok":true}`))
	})
	defer srv.Close()
	var out struct {
		OK bool `json:"ok"`
	}
	if err := c.do(context.Background(), http.MethodGet, "/x", nil, nil, &out); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || !out.OK {
		t.Fatalf("calls=%d out=%+v", calls, out)
	}
}

func TestDoRetryAfterClampBoundsTheWait(t *testing.T) {
	// A far-future Retry-After (HTTP-date) must be CLAMPED, not obeyed. Force the
	// clamp tiny and assert the retried request succeeds quickly (not in an hour).
	old := retryAfterClamp
	retryAfterClamp = 10 * time.Millisecond
	defer func() { retryAfterClamp = old }()
	var calls int
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat))
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte(`{"ok":true}`))
	})
	defer srv.Close()
	start := time.Now()
	var out struct {
		OK bool `json:"ok"`
	}
	if err := c.do(context.Background(), http.MethodGet, "/x", nil, nil, &out); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || !out.OK || time.Since(start) > time.Second {
		t.Fatalf("calls=%d ok=%v elapsed=%v (clamp not applied?)", calls, out.OK, time.Since(start))
	}
}

func TestDoUnauthorizedMapsToErrUnauthorized(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"err":"Token invalid","ECODE":"OAUTH_017"}`))
	})
	defer srv.Close()
	var out struct{}
	err := c.do(context.Background(), http.MethodGet, "/x", nil, nil, &out)
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("401 should wrap ErrUnauthorized, got %v", err)
	}
}

func TestPostStillWorksThroughDo(t *testing.T) {
	type reqBody struct {
		Name string `json:"name"`
	}
	var gotContentType string
	var gotBody reqBody
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Write([]byte(`{"ok":true}`))
	})
	defer srv.Close()
	var out struct {
		OK bool `json:"ok"`
	}
	if err := c.post(context.Background(), "/x", reqBody{Name: "hi"}, &out); err != nil {
		t.Fatal(err)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotBody.Name != "hi" || !out.OK {
		t.Fatalf("body=%+v out=%+v", gotBody, out)
	}
}
