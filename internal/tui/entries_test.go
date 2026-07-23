package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func TestCanEdit(t *testing.T) {
	e := report.TimeEntry{UserID: 5}
	if canEdit(e, 0) {
		t.Error("userID 0 must gate everything read-only")
	}
	if canEdit(report.TimeEntry{UserID: 0}, 0) {
		t.Error("zero-owner vs zero-self must not match")
	}
	if !canEdit(e, 5) {
		t.Error("owner should be editable")
	}
	if canEdit(e, 6) {
		t.Error("non-owner should be read-only")
	}
}

func TestSortEntriesByStartDesc(t *testing.T) {
	a := report.TimeEntry{ID: "a", Start: time.Unix(100, 0)}
	b := report.TimeEntry{ID: "b", Start: time.Unix(200, 0)}
	got := sortEntriesByStartDesc([]report.TimeEntry{a, b})
	if got[0].ID != "b" || got[1].ID != "a" {
		t.Errorf("order = %v, want b,a", []string{got[0].ID, got[1].ID})
	}
}

func TestVOpensEntriesBrowser(t *testing.T) {
	m := newTestModel()
	m.screen = screenReport
	m.userID = 1
	m.entries = []report.TimeEntry{{ID: "e1", TaskName: "Fix", Start: time.Now(), Duration: time.Hour, UserID: 1, Billable: true}}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	mm := m2.(Model)
	if mm.screen != screenEntries {
		t.Fatalf("v did not open entries browser: screen=%v", mm.screen)
	}
	if !strings.Contains(mm.View(), "Fix") {
		t.Errorf("browser missing the entry:\n%s", mm.View())
	}
}
