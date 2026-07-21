package clickup

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStartTimer(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()
	c := New("tok")
	c.BaseURL = srv.URL
	if err := c.StartTimer(context.Background(), "team1", "task5", "note"); err != nil {
		t.Fatalf("StartTimer error: %v", err)
	}
	if gotPath != "/team/team1/time_entries/start" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["tid"] != "task5" {
		t.Errorf("tid = %v", gotBody["tid"])
	}
}

func TestStopTimer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/team/team1/time_entries/stop" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"id":"e1","task":{"id":"task5","name":"T5"},"start":"1700000000000","duration":"600000"}}`))
	}))
	defer srv.Close()
	c := New("tok")
	c.BaseURL = srv.URL
	e, err := c.StopTimer(context.Background(), "team1")
	if err != nil {
		t.Fatalf("StopTimer error: %v", err)
	}
	if e.TaskID != "task5" || e.Duration.Minutes() != 10 {
		t.Errorf("entry = %+v", e)
	}
}

func TestCurrentTimerRunning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"task":{"id":"task5","name":"T5"},"start":"1700000000000"}}`))
	}))
	defer srv.Close()
	c := New("tok")
	c.BaseURL = srv.URL
	rt, err := c.CurrentTimer(context.Background(), "team1")
	if err != nil {
		t.Fatalf("CurrentTimer error: %v", err)
	}
	if rt == nil || rt.TaskID != "task5" || rt.TaskName != "T5" {
		t.Errorf("timer = %+v", rt)
	}
}

func TestCurrentTimerNone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()
	c := New("tok")
	c.BaseURL = srv.URL
	rt, err := c.CurrentTimer(context.Background(), "team1")
	if err != nil {
		t.Fatalf("CurrentTimer error: %v", err)
	}
	if rt != nil {
		t.Errorf("expected nil (no timer), got %+v", rt)
	}
}
