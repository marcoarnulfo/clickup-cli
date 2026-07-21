package clickup

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestListNamesResolvesDedupsAndSkipsEmpty(t *testing.T) {
	var calls atomic.Int32
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		id := strings.TrimPrefix(r.URL.Path, "/list/")
		w.Write([]byte(`{"name":"List ` + id + `"}`))
	})
	defer srv.Close()

	got := c.ListNames(context.Background(), []string{"1", "2", "2", "", "3"})

	want := map[string]string{"1": "List 1", "2": "List 2", "3": "List 3"}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(want), got)
	}
	for id, name := range want {
		if got[id] != name {
			t.Errorf("id %q: got %q, want %q", id, got[id], name)
		}
	}
	if calls.Load() != 3 {
		t.Fatalf("expected 3 upstream calls (dedup unique ids), got %d", calls.Load())
	}
}

func TestListNamesOmitsFailedIDs(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/list/bad" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"err":"not found","ECODE":"X"}`))
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/list/")
		w.Write([]byte(`{"name":"List ` + id + `"}`))
	})
	defer srv.Close()

	got := c.ListNames(context.Background(), []string{"good", "bad"})

	if got["good"] != "List good" {
		t.Errorf("good: got %q, want %q", got["good"], "List good")
	}
	if _, ok := got["bad"]; ok {
		t.Errorf("failed id %q should be omitted, got %+v", "bad", got)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 resolved id, got %+v", got)
	}
}
