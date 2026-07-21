package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

func TestRangeSelectPreset(t *testing.T) {
	m := Model{screen: screenRange, preset: report.PresetThisMonth, rangeScreen: newRange(report.PresetThisMonth)}
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

func TestRangeCustomValidDates(t *testing.T) {
	m := Model{screen: screenRange, rangeScreen: newRange(report.PresetThisMonth)}
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
	m := Model{screen: screenRange, rangeScreen: newRange(report.PresetThisMonth)}
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

func TestHomeDOpensRange(t *testing.T) {
	m := Model{screen: screenHome, preset: report.PresetThisMonth}
	u, _ := m.updateHome(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = u.(Model)
	if m.screen != screenRange {
		t.Errorf("d should open range screen, got %v", m.screen)
	}
}
