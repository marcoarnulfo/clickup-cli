package tui

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// rateRow is a list shown in the rates screen.
type rateRow struct {
	listID string
	name   string
}

type ratesModel struct {
	rows    []rateRow
	idx     int
	editing bool
	input   textinput.Model
	rates   map[string]float64 // current overrides (list_id -> rate)
	def     float64            // default rate
	cur     string             // currency
	msg     string             // error message (invalid rate)
}

// newRates builds the screen from the lists in the current report merged with
// those already present in config (cfg.Rates). Lists "only in config" are added
// in deterministic order (ascending id) for a stable view.
func newRates(entries []report.TimeEntry, cfg config.Config) ratesModel {
	names := map[string]string{}
	var order []string
	remember := func(id, name string) {
		if id == "" {
			return
		}
		if _, ok := names[id]; !ok {
			order = append(order, id)
			names[id] = id // default label = id
		}
		if name != "" {
			names[id] = name
		}
	}
	for _, e := range entries {
		remember(e.ListID, e.ListName)
	}
	// lists present only in config: deterministic order
	var cfgIDs []string
	for id := range cfg.Rates {
		if _, ok := names[id]; !ok {
			cfgIDs = append(cfgIDs, id)
		}
	}
	slices.Sort(cfgIDs)
	for _, id := range cfgIDs {
		remember(id, "")
	}

	rows := make([]rateRow, len(order))
	for i, id := range order {
		rows[i] = rateRow{listID: id, name: names[id]}
	}
	rates := map[string]float64{}
	for k, v := range cfg.Rates {
		rates[k] = v
	}
	return ratesModel{rows: rows, rates: rates, def: cfg.Rate, cur: cfg.Currency}
}

// validRate accepts only a finite number ≥ 0. The decimal comma is accepted
// as well as the dot (handy for the Italian keyboard).
func validRate(s string) (float64, bool) {
	s = strings.ReplaceAll(s, ",", ".")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// numericRune reports whether a rune is allowed in the rate field (digits and separator).
func numericRune(r rune) bool {
	return (r >= '0' && r <= '9') || r == '.' || r == ','
}

func (m Model) updateRates(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rt := m.ratesScreen

	if rt.editing {
		switch msg.Type {
		case tea.KeyEnter:
			v := rt.input.Value()
			if v == "" {
				rt.editing = false // empty = no change (to clear an override, use 'd')
				rt.msg = ""
			} else if f, ok := validRate(v); ok {
				rt.rates[rt.rows[rt.idx].listID] = f
				rt.editing = false
				rt.msg = ""
			} else {
				rt.msg = "Invalid rate: enter a number ≥ 0"
			}
			m.ratesScreen = rt
			return m, nil
		case tea.KeyEsc:
			rt.editing = false
			rt.msg = ""
			m.ratesScreen = rt
			return m, nil
		}
		// Numeric-only field: ignore characters that aren't allowed (digits and separator).
		if msg.Type == tea.KeyRunes {
			for _, r := range msg.Runes {
				if !numericRune(r) {
					m.ratesScreen = rt
					return m, nil
				}
			}
		}
		var cmd tea.Cmd
		rt.input, cmd = rt.input.Update(msg)
		m.ratesScreen = rt
		return m, cmd
	}

	switch msg.String() {
	case "up", "k":
		if rt.idx > 0 {
			rt.idx--
		}
	case "down", "j":
		if rt.idx < len(rt.rows)-1 {
			rt.idx++
		}
	case "enter":
		if len(rt.rows) > 0 {
			rt.editing = true
			rt.msg = ""
			rt.input = newNumberInput("new rate (Esc to cancel)")
		}
	case "d":
		if len(rt.rows) > 0 {
			delete(rt.rates, rt.rows[rt.idx].listID) // revert to the default rate
		}
	case "s":
		// Build the map to save, excluding redundant overrides
		// (equal to the default). Use a copy: if saving fails the
		// working copy stays intact.
		toSave := map[string]float64{}
		for id, v := range rt.rates {
			if v != rt.def {
				toSave[id] = v
			}
		}
		m.cfg.Rates = toSave
		if err := config.Save(m.cfg); err != nil {
			rt.msg = "Error saving config: " + err.Error()
			m.ratesScreen = rt
			return m, nil
		}
		rt.rates = toSave // update the working copy only after a successful save
		g := m.report.GroupBy
		if g == "" {
			g = report.GroupByTotal
		}
		start, end := m.currentRange()
		m.report = report.Build(m.visibleEntries(), g, ratesFromConfig(m.cfg), m.cfg.Currency, start, end)
		m.report.Scope = m.scope
		m.rep = newReport(m.report, m.memberFilterNote()+m.filteredNote())
		m.screen = screenReport
		m.ratesScreen = rt
		return m, nil
	case "esc":
		// Discard unsaved changes and return to the report.
		m.screen = screenReport
		return m, nil
	}
	m.ratesScreen = rt
	return m, nil
}

func (rt ratesModel) view() string {
	b := styleTitle.Render("Per-list rates") + "\n\n"
	if len(rt.rows) == 0 {
		b += styleHelp.Render("No lists in the current report.") + "\n"
	}
	for i, row := range rt.rows {
		rate := rt.def
		tag := "(default)"
		if v, ok := rt.rates[row.listID]; ok {
			rate = v
			tag = "(override)"
		}
		cursor := "  "
		line := fmt.Sprintf("%-28s %8.2f %s %s", truncate(row.name, 28), rate, rt.cur, tag)
		if i == rt.idx {
			cursor = "▸ "
			line = styleAccent.Render(line)
		}
		b += cursor + line + "\n"
	}
	if rt.editing {
		b += "\n" + rt.input.View()
	}
	if rt.msg != "" {
		b += "\n" + styleErr.Render(rt.msg)
	}
	b += "\n\n" + styleHelp.Render("↑/↓ select · Enter: edit · d: use default · s: save · Esc: cancel")
	return b
}
