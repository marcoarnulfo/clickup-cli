package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func filtersFixture() Model {
	entries := []report.TimeEntry{
		{ListName: "Website", Tags: []string{"frontend"}, Status: "in progress", Billable: true},
		{ListName: "Mobile", Tags: []string{"backend"}, Status: "done", Billable: true},
	}
	m := Model{screen: screenFilters, entries: entries, now: time.Now}
	m.filtersScreen = newFilters(entries, nil, nil, nil)
	return m
}

func TestFiltersToggleAndApply(t *testing.T) {
	m := filtersFixture()
	// section 0 = Lists; toggle first option (row 0)
	u, _ := m.updateFilters(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = u.(Model)
	u, _ = m.updateFilters(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if m.screen != screenReport {
		t.Fatalf("apply should go to report, got %v", m.screen)
	}
	if len(m.filterLists) == 0 {
		t.Fatal("expected a list filter written to root")
	}
}

// #57: applying filters with an unparseable billing.rounding.increment must
// route to screenError instead of switching to screenReport with a stale
// (unfiltered) report — applyReport's false return must not be papered over.
func TestFiltersApplyWithBadRoundingRoutesToErrorScreen(t *testing.T) {
	m := filtersFixture()
	m.cfg = config.Config{}
	m.cfg.Billing.Rounding.Increment = "not-a-duration"
	u, _ := m.updateFilters(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = u.(Model)
	u, _ = m.updateFilters(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if m.screen != screenError {
		t.Fatalf("screen = %v, want screenError", m.screen)
	}
	if m.err == nil || !strings.Contains(m.err.Error(), "not-a-duration") {
		t.Fatalf("err = %v, want it to name the offending increment", m.err)
	}
}

func TestFiltersTabChangesSection(t *testing.T) {
	m := filtersFixture()
	u, _ := m.updateFilters(tea.KeyMsg{Type: tea.KeyTab})
	m = u.(Model)
	if m.filtersScreen.sec != 1 {
		t.Errorf("tab should move to section 1, got %d", m.filtersScreen.sec)
	}
}

func TestFiltersEscDiscards(t *testing.T) {
	m := filtersFixture()
	u, _ := m.updateFilters(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = u.(Model)
	u, _ = m.updateFilters(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(Model)
	if m.screen != screenReport {
		t.Errorf("esc should return to report, got %v", m.screen)
	}
	if len(m.filterLists) != 0 {
		t.Error("esc must not write filters to root")
	}
}

func TestReportFOpensFilters(t *testing.T) {
	m := Model{screen: screenReport, entries: []report.TimeEntry{{ListName: "A"}}, now: time.Now}
	u, _ := m.updateReport(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	m = u.(Model)
	if m.screen != screenFilters {
		t.Errorf("f should open filters, got %v", m.screen)
	}
}
