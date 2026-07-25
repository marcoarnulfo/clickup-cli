package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// rangePreset pairs a preset id with its menu label.
type rangePreset struct {
	id    string
	label string
}

var rangePresets = []rangePreset{
	{report.PresetThisMonth, "This month"},
	{report.PresetLastMonth, "Last month"},
	{report.PresetLast7d, "Last 7 days"},
	{report.PresetLast30d, "Last 30 days"},
	{report.PresetThisWeek, "This week"},
	{report.PresetCustom, "Custom…"},
}

type rangeModel struct {
	idx       int  // selected preset row
	editing   bool // custom from/to inputs shown
	field     int  // 0 = from, 1 = to
	fromInput textinput.Model
	toInput   textinput.Model
	msg       string // validation error
}

// newRange builds the picker with the current preset preselected.
func newRange(current string) rangeModel {
	rm := rangeModel{}
	for i, p := range rangePresets {
		if p.id == current {
			rm.idx = i
		}
	}
	return rm
}

func (m Model) updateRange(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rs := m.rangeScreen
	k := keysFor(m) // reflects rs.editing as it stood before this keypress

	if rs.editing {
		switch {
		case key.Matches(msg, k.Confirm):
			if rs.field == 0 {
				rs.field = 1
				rs.fromInput.Blur()
				rs.toInput.Focus()
				m.rangeScreen = rs
				return m, nil
			}
			from, errF := time.Parse("2006-01-02", rs.fromInput.Value())
			to, errT := time.Parse("2006-01-02", rs.toInput.Value())
			if errF != nil || errT != nil {
				rs.msg = "Invalid date: use YYYY-MM-DD"
				m.rangeScreen = rs
				return m, nil
			}
			if to.Before(from) {
				rs.msg = "'To' must not be before 'From'"
				m.rangeScreen = rs
				return m, nil
			}
			m.preset = report.PresetCustom
			m.customStart = from
			m.customEnd = to
			m.periodMode = periodModeMonth // an explicit range pick always wins over week mode (#4)
			m.screen = screenHome
			return m, nil
		case key.Matches(msg, k.Back):
			// Closes the custom-date editor and stays on screenRange (the
			// two-step "back to the preset list" behavior) — it does NOT
			// navigate away, unlike Back everywhere else on this screen.
			rs.editing = false
			rs.msg = ""
			m.rangeScreen = rs
			return m, nil
		case key.Matches(msg, k.NextField), key.Matches(msg, k.PrevField):
			// Only two fields, so Tab and Shift+Tab both just swap focus between them.
			if rs.field == 0 {
				rs.field = 1
				rs.fromInput.Blur()
				rs.toInput.Focus()
			} else {
				rs.field = 0
				rs.toInput.Blur()
				rs.fromInput.Focus()
			}
			m.rangeScreen = rs
			return m, nil
		}
		var cmd tea.Cmd
		if rs.field == 0 {
			rs.fromInput, cmd = rs.fromInput.Update(msg)
		} else {
			rs.toInput, cmd = rs.toInput.Update(msg)
		}
		m.rangeScreen = rs
		return m, cmd
	}

	switch {
	case key.Matches(msg, k.Up):
		if rs.idx > 0 {
			rs.idx--
		}
	case key.Matches(msg, k.Down):
		if rs.idx < len(rangePresets)-1 {
			rs.idx++
		}
	case key.Matches(msg, k.Confirm):
		p := rangePresets[rs.idx]
		if p.id == report.PresetCustom {
			rs.editing = true
			rs.field = 0
			rs.msg = ""
			rs.fromInput = newTextInput("From (YYYY-MM-DD)")
			rs.toInput = newTextInput("To (YYYY-MM-DD)")
			if m.preset == report.PresetCustom {
				// Reopening an already-active custom range: prefill instead of
				// losing it to a blank editor.
				rs.fromInput.SetValue(m.customStart.Format("2006-01-02"))
				rs.toInput.SetValue(m.customEnd.Format("2006-01-02"))
			}
			rs.fromInput.Focus()
			rs.toInput.Blur()
			m.rangeScreen = rs
			return m, nil
		}
		m.preset = p.id
		m.periodMode = periodModeMonth // an explicit range pick always wins over week mode (#4)
		m.screen = screenHome
		return m, nil
	case key.Matches(msg, k.Back):
		m.screen = screenHome
		return m, nil
	}
	m.rangeScreen = rs
	return m, nil
}

func (rs rangeModel) view(th theme) string {
	b := th.Title.Render("Report range") + "\n\n"
	for i, p := range rangePresets {
		cursor := "  "
		line := p.label
		if i == rs.idx {
			cursor = "▸ "
			line = th.Accent.Render(line)
		}
		b += cursor + line + "\n"
	}
	if rs.editing {
		b += "\n" + rs.fromInput.View() + "\n" + rs.toInput.View() + "\n"
	}
	if rs.msg != "" {
		b += "\n" + th.Err.Render(rs.msg)
	}
	help := "↑/↓ select · Enter: choose/next · Esc: back"
	if rs.editing {
		help = "Tab: from/to · Enter: confirm · Esc: cancel"
	}
	b += "\n" + th.Help.Render(help)
	return b
}
