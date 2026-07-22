package clickup

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

// taskJSON builds one ClickUp task JSON object with the given id/name.
func taskJSON(id, name string) string {
	return fmt.Sprintf(`{"id":%q,"name":%q,"status":{"status":"in progress"},"list":{"id":"l1"}}`, id, name)
}

func tasksPageJSON(n int, startID int) string {
	out := "["
	for i := 0; i < n; i++ {
		if i > 0 {
			out += ","
		}
		out += taskJSON(fmt.Sprintf("t%d", startID+i), fmt.Sprintf("Task %d", startID+i))
	}
	return out + "]"
}

func TestFilteredTeamTasksFullPageHasMore(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tasks":` + tasksPageJSON(100, 0) + `}`))
	})
	defer srv.Close()

	tasks, hasMore, err := c.FilteredTeamTasks(context.Background(), "900", TaskFilter{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 100 {
		t.Fatalf("want 100 tasks, got %d", len(tasks))
	}
	if !hasMore {
		t.Fatal("want hasMore=true for a full 100-task page")
	}
}

func TestFilteredTeamTasksPartialPageNoMore(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tasks":` + tasksPageJSON(30, 0) + `}`))
	})
	defer srv.Close()

	tasks, hasMore, err := c.FilteredTeamTasks(context.Background(), "900", TaskFilter{}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 30 {
		t.Fatalf("want 30 tasks, got %d", len(tasks))
	}
	if hasMore {
		t.Fatal("want hasMore=false for a partial (< 100) page")
	}
}

func TestFilteredTeamTasksParsesStatusAndListID(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tasks":[{"id":"t1","name":"Bug","status":{"status":"open"},"list":{"id":"l42"}}]}`))
	})
	defer srv.Close()

	tasks, _, err := c.FilteredTeamTasks(context.Background(), "900", TaskFilter{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(tasks))
	}
	tk := tasks[0]
	if tk.ID != "t1" || tk.Name != "Bug" || tk.Status != "open" || tk.ListID != "l42" {
		t.Fatalf("bad task: %+v", tk)
	}
}

func TestFilteredTeamTasksMapsFilterToQueryParams(t *testing.T) {
	var gotQuery url.Values
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Write([]byte(`{"tasks":[]}`))
	})
	defer srv.Close()

	filter := TaskFilter{
		Statuses:  []string{"open", "in progress"},
		Assignees: []string{"1", "2"},
		ListIDs:   []string{"l1", "l2"},
		Tags:      []string{"urgent"},
	}
	_, _, err := c.FilteredTeamTasks(context.Background(), "900", filter, 2)
	if err != nil {
		t.Fatal(err)
	}

	if got := gotQuery["statuses[]"]; len(got) != 2 || got[0] != "open" || got[1] != "in progress" {
		t.Errorf("statuses[] = %v", got)
	}
	if got := gotQuery["assignees[]"]; len(got) != 2 || got[0] != "1" || got[1] != "2" {
		t.Errorf("assignees[] = %v", got)
	}
	if got := gotQuery["list_ids[]"]; len(got) != 2 || got[0] != "l1" || got[1] != "l2" {
		t.Errorf("list_ids[] = %v", got)
	}
	if got := gotQuery["tags[]"]; len(got) != 1 || got[0] != "urgent" {
		t.Errorf("tags[] = %v", got)
	}
	if got := gotQuery.Get("page"); got != "2" {
		t.Errorf("page = %q, want 2", got)
	}
}

func TestAllFilteredTeamTasksConcatenatesPages(t *testing.T) {
	var calls int
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		calls++
		if page == "0" {
			w.Write([]byte(`{"tasks":` + tasksPageJSON(100, 0) + `}`))
			return
		}
		w.Write([]byte(`{"tasks":` + tasksPageJSON(30, 100) + `}`))
	})
	defer srv.Close()

	tasks, err := c.AllFilteredTeamTasks(context.Background(), "900", TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 130 {
		t.Fatalf("want 130 tasks total, got %d", len(tasks))
	}
	if calls != 2 {
		t.Fatalf("want 2 page requests, got %d", calls)
	}
	if tasks[0].ID != "t0" || tasks[129].ID != "t129" {
		t.Fatalf("unexpected concatenation order: first=%q last=%q", tasks[0].ID, tasks[129].ID)
	}
}
