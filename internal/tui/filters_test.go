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
		{ListName: "Mobile", Tags: []string{"backend"}, Status: "done", Billable: false},
	}
	m := Model{screen: screenFilters, entries: entries, now: time.Now}
	m.filtersScreen = newFilters(entries, nil, nil, nil, nil)
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

// #51: the Billable dimension is a tri-state radio (All/Billable only/
// Non-billable only) built on top of the existing report.FilterCriteria.Billable
// field — not a private pre-filter (A3, binding).
func TestFiltersBillableDefaultsToAll(t *testing.T) {
	m := filtersFixture()
	sec := m.filtersScreen.sections[3]
	if sec.title != "Billable" {
		t.Fatalf("section 3 = %q, want Billable", sec.title)
	}
	if !sec.selected[billableOptAll] {
		t.Error("with no prior filter, 'All' should be preselected")
	}
}

func TestFiltersBillableRestrictsVisibleEntries(t *testing.T) {
	m := filtersFixture()
	// Tab to the Billable section (index 3).
	for i := 0; i < 3; i++ {
		u, _ := m.updateFilters(tea.KeyMsg{Type: tea.KeyTab})
		m = u.(Model)
	}
	// Row 1 = "Billable only".
	u, _ := m.updateFilters(tea.KeyMsg{Type: tea.KeyDown})
	m = u.(Model)
	u, _ = m.updateFilters(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = u.(Model)
	u, _ = m.updateFilters(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if m.filterBillable == nil || !*m.filterBillable {
		t.Fatalf("filterBillable = %v, want *true", m.filterBillable)
	}
	got := m.visibleEntries()
	if len(got) != 1 || !got[0].Billable {
		t.Fatalf("visibleEntries = %+v, want only the billable entry", got)
	}
}

func TestFiltersBillableRadioIsExclusive(t *testing.T) {
	m := filtersFixture()
	for i := 0; i < 3; i++ {
		u, _ := m.updateFilters(tea.KeyMsg{Type: tea.KeyTab})
		m = u.(Model)
	}
	u, _ := m.updateFilters(tea.KeyMsg{Type: tea.KeyDown}) // row 1: Billable only
	m = u.(Model)
	u, _ = m.updateFilters(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = u.(Model)
	u, _ = m.updateFilters(tea.KeyMsg{Type: tea.KeyDown}) // row 2: Non-billable only
	m = u.(Model)
	u, _ = m.updateFilters(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = u.(Model)
	sec := m.filtersScreen.sections[3]
	n := 0
	for _, v := range sec.selected {
		if v {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("radio section should keep exactly one selection, got %d", n)
	}
	if !sec.selected[billableOptNo] {
		t.Error("expected Non-billable only to be the selected option")
	}
}

// #51 (review followup): 'a' (select all/none) must be a no-op on the
// Billable radio section — applying it would select all three mutually
// exclusive options at once, breaking the exactly-one-selected invariant.
func TestFiltersBillableANoOp(t *testing.T) {
	m := filtersFixture()
	for i := 0; i < 3; i++ {
		u, _ := m.updateFilters(tea.KeyMsg{Type: tea.KeyTab})
		m = u.(Model)
	}
	before := copyBool(m.filtersScreen.sections[3].selected)

	u, _ := m.updateFilters(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = u.(Model)

	after := m.filtersScreen.sections[3].selected
	if len(before) != len(after) {
		t.Fatalf("selection map size changed: %v -> %v", before, after)
	}
	for k, v := range before {
		if after[k] != v {
			t.Errorf("a should be a no-op on the Billable section; %q changed from %v to %v", k, v, after[k])
		}
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
