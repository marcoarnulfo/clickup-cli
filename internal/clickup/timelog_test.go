package clickup

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCreateTimeEntry(t *testing.T) {
	var gotPath, gotAuth, gotCT string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"id":"1"}}`))
	}))
	defer srv.Close()

	c := New("tok_x")
	c.BaseURL = srv.URL
	start := time.UnixMilli(1_700_000_000_000).UTC()
	err := c.CreateTimeEntry(context.Background(), "team1", "task9", start, 90*time.Minute, "note")
	if err != nil {
		t.Fatalf("CreateTimeEntry errore: %v", err)
	}
	if gotPath != "/team/team1/time_entries" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuth != "tok_x" {
		t.Errorf("auth = %q (deve essere senza Bearer)", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("content-type = %q", gotCT)
	}
	if gotBody["tid"] != "task9" {
		t.Errorf("tid = %v", gotBody["tid"])
	}
	if gotBody["duration"] != float64(90*60*1000) {
		t.Errorf("duration(ms) = %v", gotBody["duration"])
	}
	if gotBody["start"] != float64(1_700_000_000_000) {
		t.Errorf("start(ms) = %v", gotBody["start"])
	}
	if gotBody["description"] != "note" {
		t.Errorf("description = %v", gotBody["description"])
	}
}

func TestCreateTimeEntryUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"err":"Token invalid","ECODE":"OAUTH_017"}`))
	}))
	defer srv.Close()
	c := New("bad")
	c.BaseURL = srv.URL
	err := c.CreateTimeEntry(context.Background(), "t", "x", time.Now(), time.Hour, "")
	if err == nil {
		t.Fatal("atteso errore 401")
	}
}

func TestListTasks(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"tasks":[{"id":"a1","name":"Task A"},{"id":"b2","name":"Task B"}]}`))
	}))
	defer srv.Close()
	c := New("tok")
	c.BaseURL = srv.URL
	tasks, err := c.ListTasks(context.Background(), "list7")
	if err != nil {
		t.Fatalf("ListTasks errore: %v", err)
	}
	if gotPath != "/list/list7/task" {
		t.Errorf("path = %q", gotPath)
	}
	if len(tasks) != 2 || tasks[0].ID != "a1" || tasks[1].Name != "Task B" {
		t.Errorf("tasks = %+v", tasks)
	}
}
