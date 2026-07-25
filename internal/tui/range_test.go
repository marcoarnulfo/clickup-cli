package tui

import (
	"slices"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// TestRangeListModeKeyLabels pins the label set updateRange accepts while
// browsing the preset list (rangeScreen.editing == false), plus q — the
// screen's Quit stays off today (see TestQuitBindingPerScreen).
func TestRangeListModeKeyLabels(t *testing.T) {
	t.Parallel()
	m := Model{screen: screenRange, rangeScreen: newRange(report.PresetThisMonth), nav: []screen{screenHome}}
	want := []string{"down", "enter", "esc", "j", "k", "up"}
	if got := enabledLabels(keysFor(m)); !slices.Equal(got, want) {
		t.Errorf("range list-mode labels = %v, want %v", got, want)
	}
}

// TestRangeEditingModeKeyLabels pins the DIFFERENT label set accepted while
// the custom-date editor is open: no up/down (they'd be typed into the
// focused field instead), plus tab/shift+tab to swap fields.
func TestRangeEditingModeKeyLabels(t *testing.T) {
	t.Parallel()
	m := Model{screen: screenRange, rangeScreen: newRange(report.PresetThisMonth), nav: []screen{screenHome}}
	m.rangeScreen.editing = true
	want := []string{"enter", "esc", "shift+tab", "tab"}
	if got := enabledLabels(keysFor(m)); !slices.Equal(got, want) {
		t.Errorf("range editing-mode labels = %v, want %v", got, want)
	}
}

func TestRangeSelectPreset(t *testing.T) {
	m := Model{screen: screenRange, preset: report.PresetThisMonth, rangeScreen: newRange(report.PresetThisMonth), nav: []screen{screenHome}}
	// move to "last_7d" and confirm (order: this_month, last_month, last_7d, ...)
	m.rangeScreen.idx = 2
	u, _ := m.updateRange(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if m.preset != report.PresetLast7d {
		t.Errorf("preset = %q, want last_7d", m.preset)
	}
	if m.screen != screenHome {
		t.Errorf("screen = %v, want home", m.screen)
	}
}

// #4 (review followup): committing a preset from the Range screen must clear
// a prior week-mode toggle, or the ISO week silently keeps overriding the
// newly picked preset in currentRange.
func TestRangeSelectPresetClearsWeekMode(t *testing.T) {
	m := Model{
		screen: screenRange, preset: report.PresetThisMonth, periodMode: periodModeWeek,
		rangeScreen: newRange(report.PresetThisMonth),
		nav:         []screen{screenHome},
	}
	m.rangeScreen.idx = 2 // "last_7d"
	u, _ := m.updateRange(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if m.preset != report.PresetLast7d {
		t.Fatalf("preset = %q, want last_7d", m.preset)
	}
	if m.periodMode == periodModeWeek {
		t.Error("committing a preset must clear periodMode, not leave week mode active")
	}
}

// Same regression, for the custom-range commit path.
func TestRangeCustomValidDatesClearsWeekMode(t *testing.T) {
	m := Model{screen: screenRange, periodMode: periodModeWeek, rangeScreen: newRange(report.PresetThisMonth), nav: []screen{screenHome}}
	m.rangeScreen.idx = 5 // "custom"
	u, _ := m.updateRange(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	rs := m.rangeScreen
	rs.editing = true
	rs.fromInput.SetValue("2026-07-01")
	rs.toInput.SetValue("2026-07-15")
	rs.field = 1
	m.rangeScreen = rs
	u, _ = m.updateRange(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if m.preset != report.PresetCustom {
		t.Fatalf("preset = %q, want custom", m.preset)
	}
	if m.periodMode == periodModeWeek {
		t.Error("committing a custom range must clear periodMode, not leave week mode active")
	}
}

func TestRangeCustomValidDates(t *testing.T) {
	m := Model{screen: screenRange, rangeScreen: newRange(report.PresetThisMonth), nav: []screen{screenHome}}
	m.rangeScreen.idx = 5 // "custom"
	u, _ := m.updateRange(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	rs := m.rangeScreen
	rs.editing = true
	rs.fromInput.SetValue("2026-07-01")
	rs.toInput.SetValue("2026-07-15")
	rs.field = 1 // on the "to" field
	m.rangeScreen = rs
	u, _ = m.updateRange(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if m.preset != report.PresetCustom {
		t.Fatalf("preset = %q, want custom", m.preset)
	}
	if m.customStart.Format("2006-01-02") != "2026-07-01" || m.customEnd.Format("2006-01-02") != "2026-07-15" {
		t.Errorf("custom = %s..%s", m.customStart.Format("2006-01-02"), m.customEnd.Format("2006-01-02"))
	}
	if m.screen != screenHome {
		t.Errorf("screen = %v, want home after valid custom", m.screen)
	}
}

func TestRangeCustomInvalidStays(t *testing.T) {
	m := Model{screen: screenRange, rangeScreen: newRange(report.PresetThisMonth), nav: []screen{screenHome}}
	rs := m.rangeScreen
	rs.idx = 5
	rs.editing = true
	rs.fromInput.SetValue("nope")
	rs.toInput.SetValue("2026-07-15")
	rs.field = 1
	m.rangeScreen = rs
	u, _ := m.updateRange(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if m.screen != screenRange {
		t.Errorf("invalid custom should stay on range screen, got %v", m.screen)
	}
	if m.rangeScreen.msg == "" {
		t.Error("expected a validation message")
	}
}

func TestRangeCustomReopenPrefills(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	m := Model{
		screen:      screenRange,
		nav:         []screen{screenHome},
		preset:      report.PresetCustom,
		customStart: start,
		customEnd:   end,
		rangeScreen: newRange(report.PresetCustom),
	}
	m.rangeScreen.idx = 5 // "custom" row
	u, _ := m.updateRange(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if !m.rangeScreen.editing {
		t.Fatal("expected editing mode after Enter on Custom")
	}
	if got := m.rangeScreen.fromInput.Value(); got != "2026-06-01" {
		t.Errorf("fromInput = %q, want prefilled 2026-06-01", got)
	}
	if got := m.rangeScreen.toInput.Value(); got != "2026-06-20" {
		t.Errorf("toInput = %q, want prefilled 2026-06-20", got)
	}
}

func TestRangeCustomReopenEmptyWhenNotCustomPreset(t *testing.T) {
	m := Model{
		screen:      screenRange,
		nav:         []screen{screenHome},
		preset:      report.PresetThisMonth,
		customStart: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		customEnd:   time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC),
		rangeScreen: newRange(report.PresetThisMonth),
	}
	m.rangeScreen.idx = 5 // "custom" row
	u, _ := m.updateRange(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)
	if got := m.rangeScreen.fromInput.Value(); got != "" {
		t.Errorf("fromInput = %q, want empty when the active preset is not custom", got)
	}
	if got := m.rangeScreen.toInput.Value(); got != "" {
		t.Errorf("toInput = %q, want empty when the active preset is not custom", got)
	}
}

func TestRangeEditingTabSwitchesField(t *testing.T) {
	m := Model{screen: screenRange, rangeScreen: newRange(report.PresetThisMonth), nav: []screen{screenHome}}
	rs := m.rangeScreen
	rs.idx = 5
	m.rangeScreen = rs
	u, _ := m.updateRange(tea.KeyMsg{Type: tea.KeyEnter}) // enter editing mode
	m = u.(Model)
	if m.rangeScreen.field != 0 || !m.rangeScreen.fromInput.Focused() {
		t.Fatalf("expected field 0 (from) focused right after entering editing mode")
	}

	u, _ = m.updateRange(tea.KeyMsg{Type: tea.KeyTab})
	m = u.(Model)
	if m.rangeScreen.field != 1 {
		t.Errorf("field = %d, want 1 after Tab", m.rangeScreen.field)
	}
	if m.rangeScreen.fromInput.Focused() || !m.rangeScreen.toInput.Focused() {
		t.Error("Tab should blur 'from' and focus 'to'")
	}

	u, _ = m.updateRange(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = u.(Model)
	if m.rangeScreen.field != 0 {
		t.Errorf("field = %d, want 0 after Shift+Tab", m.rangeScreen.field)
	}
	if !m.rangeScreen.fromInput.Focused() || m.rangeScreen.toInput.Focused() {
		t.Error("Shift+Tab should focus 'from' and blur 'to'")
	}
}

// #59 Task 3 step 3: esc in list mode returns to Home.
func TestRangeEscListModeReturnsHome(t *testing.T) {
	m := Model{screen: screenRange, rangeScreen: newRange(report.PresetThisMonth), nav: []screen{screenHome}}
	next, _ := m.updateRange(tea.KeyMsg{Type: tea.KeyEsc})
	if got := next.(Model).screen; got != screenHome {
		t.Errorf("esc from range list -> %v, want screenHome", got)
	}
}

// #59 Task 3 step 3: esc in editing mode does NOT navigate — it only closes
// the custom-date editor, staying on screenRange with editing == false. This
// is the two-step "back to the preset list" behavior; asserting a screen
// change here would delete it without any golden file noticing.
func TestRangeEscEditingModeClosesEditorNotScreen(t *testing.T) {
	m := Model{screen: screenRange, rangeScreen: newRange(report.PresetThisMonth), nav: []screen{screenHome}}
	m.rangeScreen.editing = true
	next, _ := m.updateRange(tea.KeyMsg{Type: tea.KeyEsc})
	nm := next.(Model)
	if nm.screen != screenRange {
		t.Errorf("esc from range editing -> %v, want to stay on screenRange", nm.screen)
	}
	if nm.rangeScreen.editing {
		t.Error("esc from range editing should clear editing, got still true")
	}
}

func TestHomeDOpensRange(t *testing.T) {
	m := Model{screen: screenHome, preset: report.PresetThisMonth}
	u, _ := m.updateHome(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = u.(Model)
	if m.screen != screenRange {
		t.Errorf("d should open range screen, got %v", m.screen)
	}
}
