package clickup

import (
	"context"
	"net/http"
	"testing"
)

func TestTaskStatus(t *testing.T) {
	var gotPath string
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"id":"t1","status":{"status":"in progress","type":"custom"}}`))
	})
	defer srv.Close()
	st, err := c.TaskStatus(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/task/t1" {
		t.Errorf("path = %q", gotPath)
	}
	if st != "in progress" {
		t.Errorf("status = %q", st)
	}
}
