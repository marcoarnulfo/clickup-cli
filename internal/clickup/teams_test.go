package clickup

import (
	"context"
	"net/http"
	"testing"
)

func TestTeamMembers(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"teams":[
			{"id":"900","name":"Acme","members":[{"user":{"id":1,"username":"alice"}},{"user":{"id":2,"username":"bob"}}]},
			{"id":"901","name":"Other","members":[{"user":{"id":9,"username":"zoe"}}]}
		]}`))
	})
	defer srv.Close()

	members, err := c.TeamMembers(context.Background(), "900")
	if err != nil {
		t.Fatalf("TeamMembers error: %v", err)
	}
	if len(members) != 2 || members[0].Username != "alice" || members[1].ID != 2 {
		t.Errorf("members = %+v", members)
	}

	if _, err := c.TeamMembers(context.Background(), "404"); err == nil {
		t.Error("expected error for unknown workspace")
	}
}
