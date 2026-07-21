package clickup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSpaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/team/900/space" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"spaces":[{"id":"s1","name":"Engineering"},{"id":"s2","name":"Marketing"}]}`))
	}))
	defer srv.Close()
	c := New("tok")
	c.BaseURL = srv.URL
	spaces, err := c.Spaces(context.Background(), "900")
	if err != nil {
		t.Fatal(err)
	}
	if len(spaces) != 2 || spaces[0].ID != "s1" || spaces[1].Name != "Marketing" {
		t.Errorf("spaces = %+v", spaces)
	}
}

func TestSpaceContents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/space/s1/folder":
			_, _ = w.Write([]byte(`{"folders":[{"id":"f1","name":"Backend","lists":[{"id":"l1","name":"API"},{"id":"l2","name":"Auth"}]}]}`))
		case "/space/s1/list":
			_, _ = w.Write([]byte(`{"lists":[{"id":"l9","name":"Roadmap"}]}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()
	c := New("tok")
	c.BaseURL = srv.URL
	folders, folderless, err := c.SpaceContents(context.Background(), "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 1 || folders[0].Name != "Backend" || len(folders[0].Lists) != 2 || folders[0].Lists[0].Name != "API" {
		t.Errorf("folders = %+v", folders)
	}
	if len(folderless) != 1 || folderless[0].ID != "l9" {
		t.Errorf("folderless = %+v", folderless)
	}
}
