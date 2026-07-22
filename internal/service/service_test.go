package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
)

func TestLoadEntriesMeScopeNoAssigneeExpansion(t *testing.T) {
	teamCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/time_entries"):
			if got := r.URL.Query().Get("assignee"); got != "" {
				t.Errorf("me scope: expected no assignee filter, got %q", got)
			}
			w.Write([]byte(`{"data":[{"id":"e1","task":{"id":"t","name":"T"},"task_location":{"list_id":"55"},"user":{"id":1,"username":"x"},"start":"1751360400000","duration":"3600000"}]}`))
		case strings.HasSuffix(r.URL.Path, "/team"):
			teamCalled = true
			w.Write([]byte(`{"teams":[]}`))
		case strings.Contains(r.URL.Path, "/list/"):
			w.Write([]byte(`{"id":"55","name":"Client Z"}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	c := clickup.New("tok")
	c.BaseURL = srv.URL

	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	entries, err := LoadEntries(context.Background(), c, "900", start, end, "me", nil)
	if err != nil {
		t.Fatalf("LoadEntries: %v", err)
	}
	if teamCalled {
		t.Error("me scope: /team must not be called")
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %v, want 1", entries)
	}
	if entries[0].ListName != "Client Z" {
		t.Errorf("ListName = %q, want %q (list-name enrichment)", entries[0].ListName, "Client Z")
	}
}

func TestLoadEntriesTeamScopeExpandsViaTeamMembers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/team") && strings.Contains(r.URL.Path, "/time_entries"):
			if got := r.URL.Query().Get("assignee"); got == "" {
				t.Error("team scope: expected assignee parameter to be set from expanded TeamMembers")
			}
			w.Write([]byte(`{"data":[{"id":"e1","task":{"id":"t","name":"T"},"task_location":{"list_id":"55"},"user":{"id":7,"username":"a"},"start":"1751360400000","duration":"3600000"}]}`))
		case strings.HasSuffix(r.URL.Path, "/team"):
			w.Write([]byte(`{"teams":[{"id":"900","name":"WS","members":[{"user":{"id":7,"username":"a"}},{"user":{"id":8,"username":"b"}}]}]}`))
		case strings.Contains(r.URL.Path, "/list/"):
			w.Write([]byte(`{"id":"55","name":"Client Z"}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	c := clickup.New("tok")
	c.BaseURL = srv.URL

	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	entries, err := LoadEntries(context.Background(), c, "900", start, end, "team", nil)
	if err != nil {
		t.Fatalf("LoadEntries: %v", err)
	}
	if len(entries) != 1 || entries[0].ListName != "Client Z" {
		t.Fatalf("wrong team entries: %+v", entries)
	}
}

func TestLoadEntriesTeamScopeExplicitAssigneesSkipsExpansion(t *testing.T) {
	teamCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/time_entries"):
			if got := r.URL.Query().Get("assignee"); got != "7,9" {
				t.Errorf("assignee = %q, want 7,9", got)
			}
			w.Write([]byte(`{"data":[]}`))
		case strings.HasSuffix(r.URL.Path, "/team"):
			teamCalled = true
			w.Write([]byte(`{"teams":[]}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	c := clickup.New("tok")
	c.BaseURL = srv.URL

	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	_, err := LoadEntries(context.Background(), c, "900", start, end, "team", []int{7, 9})
	if err != nil {
		t.Fatalf("LoadEntries: %v", err)
	}
	if teamCalled {
		t.Error("explicit assignees: /team must not be called")
	}
}

func TestLoadEntriesTeamScopeWorkspaceNotFoundErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /team returns a workspace with an id DIFFERENT from the one requested
		w.Write([]byte(`{"teams":[{"id":"OTHER","name":"X","members":[{"user":{"id":1,"username":"a"}}]}]}`))
	}))
	defer srv.Close()
	c := clickup.New("tok")
	c.BaseURL = srv.URL

	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	_, err := LoadEntries(context.Background(), c, "900", start, end, "team", nil)
	if err == nil {
		t.Fatal("team scope with workspace not found should error")
	}
}
