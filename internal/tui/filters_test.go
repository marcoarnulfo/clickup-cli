package tui

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// filtersScreenFixture is a Model parked on screenFilters with two entries
// (one billable, one not) so the screen's toggle/apply/discard behavior has
// something concrete to act on.
func filtersScreenFixture() Model {
	entries := []report.TimeEntry{
		{ListName: "Website", Tags: []string{"frontend"}, Status: "in progress", Billable: true},
		{ListName: "Mobile", Tags: []string{"backend"}, Status: "done", Billable: false},
	}
	m := Model{screen: screenFilters, entries: entries, now: time.Now, nav: []screen{screenHome, screenReport}}
	m.filtersScreen = newFilters(entries, nil, nil, nil, nil)
	return m
}

// filtersFixture builds entries whose list names produce n Lists options, plus
// one tag and one status, so the Filters screen has something to scroll.
func filtersFixture(n int) []report.TimeEntry {
	entries := make([]report.TimeEntry, 0, n)
	for i := range n {
		entries = append(entries, report.TimeEntry{
			ListName: fmt.Sprintf("List %02d", i),
			Tags:     []string{"backend"},
			Status:   "done",
		})
	}
	return entries
}

func TestFiltersToggleAndApply(t *testing.T) {
	m := filtersScreenFixture()
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
	m := filtersScreenFixture()
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
	m := filtersScreenFixture()
	u, _ := m.updateFilters(tea.KeyMsg{Type: tea.KeyTab})
	m = u.(Model)
	if m.filtersScreen.sec != 1 {
		t.Errorf("tab should move to section 1, got %d", m.filtersScreen.sec)
	}
}

func TestFiltersEscDiscards(t *testing.T) {
	m := filtersScreenFixture()
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
	m := filtersScreenFixture()
	sec := m.filtersScreen.sections[3]
	if sec.title != "Billable" {
		t.Fatalf("section 3 = %q, want Billable", sec.title)
	}
	if !sec.selected[billableOptAll] {
		t.Error("with no prior filter, 'All' should be preselected")
	}
}

func TestFiltersBillableRestrictsVisibleEntries(t *testing.T) {
	m := filtersScreenFixture()
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
	m := filtersScreenFixture()
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
	m := filtersScreenFixture()
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

// TestFiltersKeyLabels pins the exact label set filters.go's updateFilters
// accepts today, plus q. The old switch also had a "space" case arm, but
// bubbletea maps the space rune to KeySpace whose String() is " ", so that
// arm never fired — dropped here deliberately, not a regression (#59 Task 3).
func TestFiltersKeyLabels(t *testing.T) {
	t.Parallel()
	m := filtersScreenFixture()
	want := []string{" ", "?", "a", "ctrl+c", "ctrl+p", "down", "enter", "esc", "j", "k", "q", "shift+tab", "tab", "up"}
	if got := enabledLabels(keysFor(m)); !slices.Equal(got, want) {
		t.Errorf("filters labels = %v, want %v", got, want)
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

// The Filters screen had no window at all: many lists or tags simply ran off the
// bottom of a short terminal, with no way to reach them (#28).
func TestFiltersViewNeverExceedsItsRowBudget(t *testing.T) {
	th := testTheme(true)
	fs := newFilters(filtersFixture(40), nil, nil, nil, nil)
	for _, rows := range []int{6, 10, 20} {
		if got := strings.Count(fs.view(th, rows), "\n") + 1; got > rows {
			t.Errorf("row budget %d: view rendered %d lines", rows, got)
		}
	}
}

// The window must follow the cursor. Asserting on the rendered cursor is not an
// option: under termenv.Ascii the active row is byte-identical to the others
// (th.Accent renders nothing), and "▸" marks the section header, which scrolls
// out. So assert on the window itself, which is what the feature actually is.
func TestFiltersWindowFollowsTheCursor(t *testing.T) {
	m := newTestModel()
	m.filtersScreen = newFilters(filtersFixture(40), nil, nil, nil, nil)
	m.screen = screenFilters
	m.height = 12
	for range 30 { // walk down well past the first window
		mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = mm.(Model)
	}
	fs := m.filtersScreen
	if fs.row == 0 && fs.sec == 0 {
		t.Fatal("the cursor never moved: the fixture has no options to walk")
	}
	row, rows := filtersCursorRow(fs), filtersRows(m.height)-2
	if row < fs.top || row >= fs.top+rows {
		t.Errorf("cursor at visual row %d is outside the window [%d, %d)", row, fs.top, fs.top+rows)
	}
}

// scrollWindow is the palette's own scrolling, extracted so both screens share
// one idiom. Same behavior, now with a name.
func TestScrollWindow(t *testing.T) {
	tests := []struct {
		name                 string
		idx, top, rows, want int
	}{
		{"inside stays", 3, 2, 5, 2},
		{"above pulls up", 1, 4, 5, 1},
		{"below pushes down", 9, 2, 5, 5},
		{"first row", 0, 0, 5, 0},
		{"zero rows", 7, 3, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := scrollWindow(tc.idx, tc.top, tc.rows); got != tc.want {
				t.Errorf("scrollWindow(%d, %d, %d) = %d, want %d", tc.idx, tc.top, tc.rows, got, tc.want)
			}
		})
	}
}

// The cursor's visual row must account for section headers and the "(none)"
// placeholder an empty section still renders, or the window scrolls to the wrong
// place. With two Lists options, Lists row 1 is visual row 2 (header + first
// option); the Tags section that follows starts after header + 2 options.
func TestFiltersCursorRowCountsHeadersAndOptions(t *testing.T) {
	fs := newFilters(filtersFixture(2), nil, nil, nil, nil)
	fs.sec, fs.row = 0, 1
	if got := filtersCursorRow(fs); got != 2 {
		t.Errorf("filtersCursorRow at Lists row 1 = %d, want 2", got)
	}
	fs.sec, fs.row = 1, 0
	if got := filtersCursorRow(fs); got != 4 {
		t.Errorf("filtersCursorRow at Tags row 0 = %d, want 4", got)
	}
}
