package clickup

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestUpdateTimeEntry(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{}}`))
	})
	defer srv.Close()

	start := time.UnixMilli(1_700_000_000_000).UTC()
	err := c.UpdateTimeEntry(context.Background(), "team1", "e42", start, 90*time.Minute, "wip", false)
	if err != nil {
		t.Fatalf("UpdateTimeEntry error: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if gotPath != "/team/team1/time_entries/e42" {
		t.Errorf("path = %s", gotPath)
	}
	if gotBody["start"] != float64(1_700_000_000_000) {
		t.Errorf("start = %v, want 1700000000000", gotBody["start"])
	}
	if gotBody["duration"] != float64((90 * time.Minute).Milliseconds()) {
		t.Errorf("duration = %v", gotBody["duration"])
	}
	if gotBody["description"] != "wip" {
		t.Errorf("description = %v", gotBody["description"])
	}
	if gotBody["billable"] != false {
		t.Errorf("billable = %v, want false", gotBody["billable"])
	}
}

func TestDeleteTimeEntry(t *testing.T) {
	var gotMethod, gotPath string
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	if err := c.DeleteTimeEntry(context.Background(), "team1", "e42"); err != nil {
		t.Fatalf("DeleteTimeEntry error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", gotMethod)
	}
	if gotPath != "/team/team1/time_entries/e42" {
		t.Errorf("path = %s", gotPath)
	}
}
