package tui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// TestHistoryOpensFromAnyEntry: 'h' dispatches historyCmd and moves to
// screenLoading, even on an entry the user does NOT own — history is
// read-only and explicitly NOT ownership-gated (unlike 'e'/'x').
func TestHistoryOpensFromAnyEntry(t *testing.T) {
	m := newTestModel()
	other := report.TimeEntry{ID: "e2", TaskName: "Deploy", UserID: 999, Start: time.Now()}
	m = browserWithEntries(m, other)

	m2, cmd := m.Update(key("h"))
	mm := m2.(Model)
	if cmd == nil {
		t.Fatal("h did not dispatch a command")
	}
	if mm.screen != screenLoading {
		t.Errorf("screen = %v, want screenLoading", mm.screen)
	}
}

// TestHistoryMsgStoresChangesAndSwitchesMode covers the top-level Update
// handler for historyMsg: it must store the changes on entriesScreen, flip
// mode to entriesHistory, and land back on screenEntries (not screenLoading).
func TestHistoryMsgStoresChangesAndSwitchesMode(t *testing.T) {
	m := newTestModel()
	m.screen = screenLoading
	changes := []clickup.HistoryChange{
		{Field: "duration", Before: "3600000", After: "5400000", Date: time.Unix(1700000000, 0).UTC(), User: "alice"},
	}
	m2, _ := m.Update(historyMsg{changes: changes})
	mm := m2.(Model)
	if mm.screen != screenEntries {
		t.Errorf("screen = %v, want screenEntries", mm.screen)
	}
	if mm.entriesScreen.mode != entriesHistory {
		t.Errorf("mode = %v, want entriesHistory", mm.entriesScreen.mode)
	}
	if len(mm.entriesScreen.historyChanges) != 1 || mm.entriesScreen.historyChanges[0].Field != "duration" {
		t.Errorf("historyChanges not stored: %+v", mm.entriesScreen.historyChanges)
	}
}

// TestHistoryEscReturnsToList: Esc from the history mode goes back to the
// entries list, never a dead end.
func TestHistoryEscReturnsToList(t *testing.T) {
	m := newTestModel()
	m.screen = screenEntries
	m.entriesScreen = entriesModel{mode: entriesHistory, historyChanges: []clickup.HistoryChange{{Field: "duration"}}}

	m2, _ := m.Update(key("esc"))
	mm := m2.(Model)
	if mm.entriesScreen.mode != entriesList {
		t.Errorf("esc from history did not return to entriesList: mode=%v", mm.entriesScreen.mode)
	}
}

// TestHistoryFetchErrorRoutesThroughEntriesErr: a failing history request must
// surface inline in the browser (entriesErrMsg -> screenEntries), never a
// dead-end screenError.
func TestHistoryFetchErrorRoutesThroughEntriesErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"err":"boom"}`))
	}))
	defer srv.Close()
	c := clickup.New("tok")
	c.BaseURL = srv.URL

	m := newTestModel()
	m.client = c
	m.cfg.WorkspaceID = "team1"
	own := report.TimeEntry{ID: "e1", TaskName: "Fix", UserID: 1, Start: time.Now()}
	m = browserWithEntries(m, own)

	cmd := m.historyCmd("e1")
	if cmd == nil {
		t.Fatal("historyCmd returned nil")
	}
	msg := cmd()
	errMsg, ok := msg.(entriesErrMsg)
	if !ok {
		t.Fatalf("got %T, want entriesErrMsg", msg)
	}
	if errMsg.err == nil {
		t.Error("entriesErrMsg.err is nil")
	}

	m2, _ := m.Update(errMsg)
	mm := m2.(Model)
	if mm.screen != screenEntries {
		t.Errorf("error did not stay on screenEntries: screen=%v", mm.screen)
	}
	if !mm.entriesScreen.msgErr || mm.entriesScreen.msg == "" {
		t.Errorf("error not shown inline: msg=%q msgErr=%v", mm.entriesScreen.msg, mm.entriesScreen.msgErr)
	}
}

// TestHistoryCmdRequestsExpectedPath confirms historyCmd's real-mode branch
// hits GET /team/{team}/time_entries/{id}/history and decodes the payload.
func TestHistoryCmdRequestsExpectedPath(t *testing.T) {
	const payload = `{"data":[{"field":"duration","before":3600000,"after":5400000,"date":"1700000000000","user":{"username":"alice"}}]}`
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(payload))
	}))
	defer srv.Close()
	c := clickup.New("tok")
	c.BaseURL = srv.URL

	m := newTestModel()
	m.client = c
	m.cfg.WorkspaceID = "team1"

	msg := m.historyCmd("e1")()
	hm, ok := msg.(historyMsg)
	if !ok {
		t.Fatalf("got %T, want historyMsg", msg)
	}
	if len(hm.changes) != 1 || hm.changes[0].After != "5400000" {
		t.Errorf("changes = %+v", hm.changes)
	}
	if !strings.HasSuffix(gotPath, "/team/team1/time_entries/e1/history") {
		t.Errorf("path = %q", gotPath)
	}
}

// TestDemoHistoryCmdReturnsFixedChanges: demo mode never calls the API and
// still produces a non-empty, demonstrable history.
func TestDemoHistoryCmdReturnsFixedChanges(t *testing.T) {
	msg := demoHistoryCmd()()
	hm, ok := msg.(historyMsg)
	if !ok {
		t.Fatalf("got %T, want historyMsg", msg)
	}
	if len(hm.changes) == 0 {
		t.Error("demo history is empty, want a small fixed sample")
	}
}

// TestEntriesHistoryViewRendersChanges is a light smoke test on the view
// itself: field/user/before/after must show up in the rendered string.
func TestEntriesHistoryViewRendersChanges(t *testing.T) {
	es := entriesModel{mode: entriesHistory, historyChanges: []clickup.HistoryChange{
		{Field: "duration", Before: "3600000", After: "5400000", Date: time.Date(2026, time.July, 3, 9, 5, 0, 0, time.UTC), User: "alice"},
	}}
	out := entriesHistoryView(es, time.UTC)
	for _, want := range []string{"duration", "3600000", "5400000", "alice"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q:\n%s", want, out)
		}
	}
}

// TestEntriesHistoryViewEmpty covers the no-history case (no changes yet
// recorded for the entry).
func TestEntriesHistoryViewEmpty(t *testing.T) {
	out := entriesHistoryView(entriesModel{mode: entriesHistory}, time.UTC)
	if !strings.Contains(out, "No recorded changes") {
		t.Errorf("empty-history view missing the empty-state message:\n%s", out)
	}
}
