package clickup

import (
	"context"
	"net/http"
	"testing"
)

// TestTimeEntryHistory is the Task 8 SPIKE: it confirms the response shape of
// GET /team/{id}/time_entries/{id}/history against the documented ClickUp v2
// payload (before/after as either scalars or, potentially, objects; date as an
// epoch-ms string; user as a nested object).
// TODO(spike): verified against docs, not a live call.
func TestTimeEntryHistory(t *testing.T) {
	const payload = `{"data":[
		{"field":"duration","before":3600000,"after":5400000,"date":"1700000000000","user":{"username":"alice"}},
		{"field":"description","before":"","after":"wip","date":"1700000100000","user":{"username":"alice"}}
	]}`
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/team/team1/time_entries/e1/history" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(payload))
	})
	defer srv.Close()

	h, err := c.TimeEntryHistory(context.Background(), "team1", "e1")
	if err != nil {
		t.Fatalf("TimeEntryHistory error: %v", err)
	}
	if len(h) != 2 {
		t.Fatalf("got %d changes, want 2", len(h))
	}
	if h[0].Field != "duration" || h[0].Before != "3600000" || h[0].After != "5400000" || h[0].User != "alice" {
		t.Errorf("change[0] = %+v", h[0])
	}
	if h[1].After != "wip" {
		t.Errorf("change[1].After = %q", h[1].After)
	}
	if h[0].Date.IsZero() {
		t.Errorf("change[0].Date is zero, want a decoded epoch-ms timestamp")
	}
}

// TestJSONString covers the tolerant scalar-or-object normalization used for
// Before/After: strings verbatim, numbers as their literal, everything else
// (object/array) as compact JSON so the view never breaks on an unexpected type.
func TestJSONString(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"string", `"wip"`, "wip"},
		{"number", `3600000`, "3600000"},
		{"empty string", `""`, ""},
		{"null", `null`, ""},
		{"empty raw", ``, ""},
		{"object fallback", `{"a":1}`, `{"a":1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := jsonString([]byte(tc.raw))
			if got != tc.want {
				t.Errorf("jsonString(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}
