package clickup

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestTimeEntryTags(t *testing.T) {
	const payload = `{"data":[{"name":"focus"},{"name":"client-A"},{"name":"review"}]}`
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/team/team1/time_entries/tags" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(payload))
	})
	defer srv.Close()

	tags, err := c.TimeEntryTags(context.Background(), "team1")
	if err != nil {
		t.Fatalf("TimeEntryTags error: %v", err)
	}
	if len(tags) != 3 || tags[0] != "focus" || tags[2] != "review" {
		t.Errorf("tags = %v, want [focus client-A review]", tags)
	}
}

// TestSetTimeEntryTags asserts the request(s) SetTimeEntryTags makes to move the
// entry to the desired set. Adapt the assertions to the mechanism the spike
// selects (replace-PUT, or add/remove POST/DELETE).
func TestSetTimeEntryTags(t *testing.T) {
	var reqs []string // "METHOD PATH BODY"
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		reqs = append(reqs, r.Method+" "+r.URL.Path+" "+string(raw))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	defer srv.Close()

	if err := c.SetTimeEntryTags(context.Background(), "team1", "e1", []string{"focus", "new-tag"}); err != nil {
		t.Fatalf("SetTimeEntryTags error: %v", err)
	}
	if len(reqs) == 0 {
		t.Fatalf("no request made")
	}
	// At least one request must carry the desired tag names and target the entry.
	all := strings.Join(reqs, "\n")
	for _, want := range []string{"focus", "new-tag", "e1"} {
		if !strings.Contains(all, want) {
			t.Errorf("requests %q missing %q", all, want)
		}
	}
	// sanity: each request body (the 3rd space-separated field) is valid JSON.
	for _, s := range reqs {
		if parts := strings.SplitN(s, " ", 3); len(parts) == 3 && parts[2] != "" {
			var v any
			if json.Unmarshal([]byte(parts[2]), &v) != nil {
				t.Errorf("request body not JSON: %q", parts[2])
			}
		}
	}
}
