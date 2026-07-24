package clickup

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// TestTimeEntriesBillable checks the three cases the API can send for the
// "billable" field: explicit true, explicit false, and absent (which must
// default to true, preserving today's "bill everything" behavior).
func TestTimeEntriesBillable(t *testing.T) {
	body := `{"data":[
		{"id":"1","task":{"id":"t","name":"T"},"user":{"id":1,"username":"u"},"start":"1000","duration":"3600000","billable":true},
		{"id":"2","task":{"id":"t","name":"T"},"user":{"id":1,"username":"u"},"start":"1000","duration":"3600000","billable":false},
		{"id":"3","task":{"id":"t","name":"T"},"user":{"id":1,"username":"u"},"start":"1000","duration":"3600000"}
	]}`
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	})
	defer srv.Close()

	got, err := c.TimeEntries(context.Background(), "team", time.UnixMilli(0), time.UnixMilli(9e12), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []bool{true, false, true} // absent -> true
	for i, e := range got {
		if e.Billable != want[i] {
			t.Errorf("entry %s billable=%v want %v", e.ID, e.Billable, want[i])
		}
	}
}

// TestTimeEntriesDescription checks the description survives the decode: a
// blank prefill on edit (#94) would otherwise wipe the ClickUp description on
// every save, since rawEntry didn't carry it before.
func TestTimeEntriesDescription(t *testing.T) {
	body := `{"data":[
		{"id":"1","task":{"id":"t","name":"T"},"user":{"id":1,"username":"u"},"start":"1000","duration":"3600000","description":"wip"}
	]}`
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	})
	defer srv.Close()

	got, err := c.TimeEntries(context.Background(), "team", time.UnixMilli(0), time.UnixMilli(9e12), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Description != "wip" {
		t.Fatalf("Description = %q, want %q", got[0].Description, "wip")
	}
}

func TestTimeEntriesParsesEntryTags(t *testing.T) {
	const payload = `{"data":[
		{"id":"e1","start":"1700000000000","duration":"3600000",
		 "task":{"id":"t1","name":"Fix"},
		 "task_tags":[{"name":"backend"}],
		 "tags":[{"name":"focus"},{"name":"client-A"}]}
	]}`
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	})
	defer srv.Close()

	entries, err := c.TimeEntries(context.Background(), "team1",
		time.UnixMilli(0), time.UnixMilli(2_000_000_000_000), nil)
	if err != nil {
		t.Fatalf("TimeEntries error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	e := entries[0]
	// task tags stay in Tags (report grouping); entry tags go to EntryTags.
	if len(e.Tags) != 1 || e.Tags[0] != "backend" {
		t.Errorf("Tags = %v, want [backend] (task tags)", e.Tags)
	}
	if len(e.EntryTags) != 2 || e.EntryTags[0] != "focus" || e.EntryTags[1] != "client-A" {
		t.Errorf("EntryTags = %v, want [focus client-A]", e.EntryTags)
	}
}
