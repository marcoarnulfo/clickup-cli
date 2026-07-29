package tui

import (
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
)

// action is one row of the command palette (#71).
//
// run returns tea.Model rather than Model on purpose: that is routeKey's own
// signature, so a screen action is a direct forward with no type assertion in
// between.
type action struct {
	label string // "Export", "Go to rates"
	hint  string // the key that also does this, "" for a navigation row
	run   func(Model) (tea.Model, tea.Cmd)
}

// paletteActions is everything ctrl+p offers right now: the current screen's
// commands first, then the cross-screen navigation. With an empty query every
// score is 0, so this order survives into the rendered list — "what can I do
// here" above "take me elsewhere".
func paletteActions(m Model) []action {
	screen := screenActions(m)
	global := globalActions(m)
	out := make([]action, 0, len(screen)+len(global))
	out = append(out, screen...)
	return append(out, global...)
}

// screenActions derives the current screen's commands from its keymap rather
// than from a second list that would have to be kept in step with it.
//
// Three things fall out of that for free: the gating (a disabled binding simply
// is not here), the wording (the label is the footer's own description, so the
// two surfaces cannot drift), and the behaviour (run replays the key, so there
// is no duplicated handler to diverge).
//
// screenKeys, not keysFor: with the palette open keysFor answers for the
// palette, and this list would empty itself on the first keystroke.
func screenActions(m Model) []action {
	var out []action
	for _, b := range screenKeys(m).paletteBindings() {
		if !b.Enabled() {
			continue
		}
		keys := b.Keys()
		if len(keys) == 0 {
			continue
		}
		msg, ok := keyMsgFor(keys[0])
		if !ok {
			// Unreachable today, pinned by TestEveryPaletteBindingIsReplayable.
			// Dropping beats firing a KeyMsg that matches nothing.
			continue
		}
		out = append(out, action{
			label: capitalize(b.Help().Desc),
			hint:  b.Help().Key,
			run:   func(m Model) (tea.Model, tea.Cmd) { return m.routeKey(msg) },
		})
	}
	return out
}

// globalActions is the cross-screen navigation ctrl+p offers from anywhere.
//
// The rule this list obeys, and the reason it is short: global actions are
// navigation only. Anything that changes state stays a screen action, where the
// footer teaches it and the palette repeats it.
//
// Each row opens its screen through the same openX method the key handler uses
// (see open.go), so the two can never build a screen differently.
func globalActions(m Model) []action {
	hasReport := len(m.entries) > 0
	rows := []struct {
		label   string
		target  screen
		enabled bool
		run     func(Model) (tea.Model, tea.Cmd)
	}{
		{"Go to home", screenHome, true, func(m Model) (tea.Model, tea.Cmd) {
			return m.resetTo(screenHome), nil
		}},
		{"Go to report", screenReport, hasReport, func(m Model) (tea.Model, tea.Cmd) {
			return m.goTo(screenReport), nil
		}},
		{"Go to rates", screenRates, true, func(m Model) (tea.Model, tea.Cmd) {
			return m.openRates(), nil
		}},
		{"Go to budgets", screenBudget, hasReport, func(m Model) (tea.Model, tea.Cmd) {
			// A false return means the config's pricing or timezone failed to
			// parse, and openBudgetView has already routed to screenError —
			// which is the right landing either way, so there is nothing to add.
			m.openBudgetView()
			return m, nil
		}},
		{"Go to filters", screenFilters, hasReport, func(m Model) (tea.Model, tea.Cmd) {
			mm, cmd := m.openFilters()
			return mm, cmd
		}},
		{"Go to entries", screenEntries, hasReport, func(m Model) (tea.Model, tea.Cmd) {
			mm, cmd := m.openEntries()
			return mm, cmd
		}},
		{"Go to export", screenExport, hasReport, func(m Model) (tea.Model, tea.Cmd) {
			return m.openExport(), nil
		}},
		{"Go to range", screenRange, true, func(m Model) (tea.Model, tea.Cmd) {
			return m.openRange(), nil
		}},
		{"Go to members", screenMembers, m.scope == "team", func(m Model) (tea.Model, tea.Cmd) {
			mm, cmd := m.openMembers(m.screen)
			return mm, cmd
		}},
		// "Go to log hours", not "Log hours": on Home and Report the screen
		// keymap already yields an action labelled exactly "Log hours", and two
		// identical rows in one list read as a bug rather than as the legible
		// do-it-here / take-me-there pair the other duplicates form.
		{"Go to log hours", screenLog, true, func(m Model) (tea.Model, tea.Cmd) {
			return m.openLog(), nil
		}},
	}

	var out []action
	for _, r := range rows {
		if !r.enabled || r.target == m.screen {
			continue
		}
		out = append(out, action{label: r.label, run: r.run})
	}
	// Quit is the one row that is not navigation, so it has no target screen to
	// compare against and is appended rather than given a fake one.
	return append(out, action{label: "Quit", run: func(m Model) (tea.Model, tea.Cmd) {
		return m, tea.Quit
	}})
}

// keyMsgFor rebuilds the tea.KeyMsg a binding's first key would produce.
//
// The set is closed on purpose. key.Matches compares msg.String() against the
// binding's key strings, and these four round-trip exactly. A KeyRunes message
// carrying "tab" would round-trip too, but every handler that reads msg.Type
// rather than key.Matches would then see the wrong thing — so the palette
// leaves those bindings out instead of forging their messages. ok is false for
// anything else, so an action that cannot be executed faithfully is dropped
// rather than mis-fired.
func keyMsgFor(s string) (tea.KeyMsg, bool) {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}, true
	// PrevMonth and NextMonth lead with "left" and "right" (keys.go:124-125),
	// and they are the only two palette bindings whose first key is not a rune.
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}, true
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}, true
	}
	if r := []rune(s); len(r) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: r}, true
	}
	return tea.KeyMsg{}, false
}

// capitalize upper-cases the first rune. strings.ToUpper on s[:1] would corrupt
// any label whose first character is multibyte.
func capitalize(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
