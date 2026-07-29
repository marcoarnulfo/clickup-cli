package tui

import (
	"slices"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// TestExportKeyLabels pins the exact label set export.go's updateExport
// accepts today (every case label, verbatim), plus q — handled globally in
// app.go, in no case clause of export.go itself.
func TestExportKeyLabels(t *testing.T) {
	t.Parallel()
	m := Model{screen: screenExport}
	want := []string{"?", "ctrl+c", "ctrl+p", "down", "enter", "esc", "j", "k", "q", "up"}
	if got := enabledLabels(keysFor(m)); !slices.Equal(got, want) {
		t.Errorf("export labels = %v, want %v", got, want)
	}
}

// #59 Task 3 step 3: esc has no test that would fail if it went mute.
func TestExportEscReturnsReport(t *testing.T) {
	m := Model{screen: screenExport, export: newExport(report.Report{}), nav: []screen{screenReport}}
	next, _ := m.updateExport(tea.KeyMsg{Type: tea.KeyEsc})
	if got := next.(Model).screen; got != screenReport {
		t.Errorf("esc from export -> %v, want screenReport", got)
	}
}
