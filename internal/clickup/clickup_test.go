package clickup

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(h http.HandlerFunc) (*Client, *httptest.Server) {
	srv := httptest.NewServer(h)
	c := New("tok_test")
	c.BaseURL = srv.URL
	return c, srv
}

func TestCurrentUser(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "tok_test" {
			t.Errorf("missing/incorrect auth header: %q", r.Header.Get("Authorization"))
		}
		w.Write([]byte(`{"user":{"id":42,"username":"Marco"}}`))
	})
	defer srv.Close()
	u, err := c.CurrentUser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != 42 || u.Username != "Marco" {
		t.Fatalf("got %+v", u)
	}
}

func TestTeams(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"teams":[{"id":"900","name":"Workspace","members":[{"user":{"id":1,"username":"a"}}]}]}`))
	})
	defer srv.Close()
	teams, err := c.Teams(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(teams) != 1 || teams[0].ID != "900" || len(teams[0].Members) != 1 {
		t.Fatalf("got %+v", teams)
	}
}

func TestTimeEntriesParsesDurationAndTask(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("start_date"); got == "" {
			t.Errorf("missing start_date")
		}
		w.Write([]byte(`{"data":[{
			"id":"e1",
			"task":{"id":"t1","name":"Bug fix"},
			"task_location":{"list_id":"l1"},
			"user":{"id":7,"username":"Marco"},
			"start":"1751360400000",
			"duration":"7200000"
		}]}`))
	})
	defer srv.Close()
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	entries, err := c.TimeEntries(context.Background(), "900", start, end, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.TaskName != "Bug fix" || e.UserID != 7 || e.Duration != 2*time.Hour {
		t.Fatalf("bad entry: %+v", e)
	}
}

func TestUnauthorizedIsTyped(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"err":"Token invalid","ECODE":"OAUTH_017"}`))
	})
	defer srv.Close()
	_, err := c.CurrentUser(context.Background())
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("401 should wrap ErrUnauthorized, got %v", err)
	}
}

func TestRetryOn429ThenSuccess(t *testing.T) {
	old := retryDelay
	retryDelay = 5 * time.Millisecond // test veloce
	defer func() { retryDelay = old }()

	var calls atomic.Int32
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte(`{"user":{"id":1,"username":"x"}}`))
	})
	defer srv.Close()

	u, err := c.CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("should succeed after one retry, got %v", err)
	}
	if u.ID != 1 {
		t.Fatalf("got %+v", u)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 calls (429 then 200), got %d", calls.Load())
	}
}

func TestTimeEntriesSkipsRunningTimer(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		// una entry consuntivata + un timer in corso (duration negativa)
		w.Write([]byte(`{"data":[
			{"id":"e1","task":{"id":"t1","name":"Done"},"task_location":{"list_id":"l1"},"user":{"id":1,"username":"x"},"start":"1751360400000","duration":"3600000"},
			{"id":"e2","task":{"id":"t2","name":"Running"},"task_location":{"list_id":"l1"},"user":{"id":1,"username":"x"},"start":"1751360400000","duration":"-1751360400000"}
		]}`))
	})
	defer srv.Close()
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	entries, err := c.TimeEntries(context.Background(), "900", start, end, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].TaskName != "Done" {
		t.Fatalf("running timer should be skipped, got %+v", entries)
	}
}

func TestTimeEntriesNumericListID(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		// list_id come NUMERO (non stringa): flexString deve gestirlo
		w.Write([]byte(`{"data":[{"id":"e1","task":{"id":"t1","name":"X"},"task_location":{"list_id":901},"user":{"id":1,"username":"x"},"start":"1751360400000","duration":"3600000"}]}`))
	})
	defer srv.Close()
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	entries, err := c.TimeEntries(context.Background(), "900", start, end, nil)
	if err != nil {
		t.Fatalf("numeric list_id should parse, got %v", err)
	}
	if len(entries) != 1 || entries[0].ListID != "901" {
		t.Fatalf("expected list_id 901, got %+v", entries)
	}
}

func TestTimeEntriesNullListID(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		// list_id null (tempo tracciato senza task/lista) -> stringa vuota, non "null"
		w.Write([]byte(`{"data":[{"id":"e1","task":{"id":"t1","name":"X"},"task_location":{"list_id":null},"user":{"id":1,"username":"x"},"start":"1751360400000","duration":"3600000"}]}`))
	})
	defer srv.Close()
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	entries, err := c.TimeEntries(context.Background(), "900", start, end, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ListID != "" {
		t.Fatalf("null list_id should become empty string, got %q", entries[0].ListID)
	}
}

func TestTimeEntriesEscapedStringListID(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		// list_id stringa con carattere escaped: deve essere de-escaped correttamente
		w.Write([]byte(`{"data":[{"id":"e1","task":{"id":"t1","name":"X"},"task_location":{"list_id":"a\"b"},"user":{"id":1,"username":"x"},"start":"1751360400000","duration":"3600000"}]}`))
	})
	defer srv.Close()
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	entries, err := c.TimeEntries(context.Background(), "900", start, end, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ListID != `a"b` {
		t.Fatalf("escaped string list_id mis-parsed, got %q", entries[0].ListID)
	}
}

func TestTimeEntriesMalformedDurationErrors(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		// durata non numerica: deve produrre un errore, non una entry a zero
		w.Write([]byte(`{"data":[{"id":"e1","task":{"id":"t1","name":"X"},"task_location":{"list_id":"l1"},"user":{"id":1,"username":"x"},"start":"1751360400000","duration":"abc"}]}`))
	})
	defer srv.Close()
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if _, err := c.TimeEntries(context.Background(), "900", start, end, nil); err == nil {
		t.Fatal("expected error on malformed duration, got nil")
	}
}
