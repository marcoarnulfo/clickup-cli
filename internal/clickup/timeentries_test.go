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
