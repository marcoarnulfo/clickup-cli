package tui

import (
	"fmt"
	"strings"
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
// two surfaces cannot drift), and the behavior (run replays the key, so there
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
		configured := b.Keys()[0]
		msg, err := parseKeyName(configured)
		if err != nil {
			// Overrides are parsed by ResolveKeys before Model construction, and
			// defaults are covered by TestEveryPaletteBindingIsReplayable.
			panic(fmt.Sprintf("invalid key %q reached the command palette: %v", configured, err))
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
		// keymap already yields an action labeled exactly "Log hours", and two
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

// canonicalKeyTypes is the exported Bubble Tea v1.3.10 key set whose String
// forms represent one terminal key. KeyRunes is handled separately so a config
// can contain any single rune without treating a multi-rune sequence as a key.
var canonicalKeyTypes = []tea.KeyType{
	// Canonical control-code names. Several values also have aliases; only the
	// spelling returned by KeyMsg.String is listed here.
	tea.KeyCtrlAt,
	tea.KeyCtrlA, tea.KeyCtrlB, tea.KeyCtrlC, tea.KeyCtrlD, tea.KeyCtrlE,
	tea.KeyCtrlF, tea.KeyCtrlG, tea.KeyCtrlH, tea.KeyTab, tea.KeyCtrlJ,
	tea.KeyCtrlK, tea.KeyCtrlL, tea.KeyEnter, tea.KeyCtrlN, tea.KeyCtrlO,
	tea.KeyCtrlP, tea.KeyCtrlQ, tea.KeyCtrlR, tea.KeyCtrlS, tea.KeyCtrlT,
	tea.KeyCtrlU, tea.KeyCtrlV, tea.KeyCtrlW, tea.KeyCtrlX, tea.KeyCtrlY,
	tea.KeyCtrlZ, tea.KeyEsc, tea.KeyCtrlBackslash, tea.KeyCtrlCloseBracket,
	tea.KeyCtrlCaret, tea.KeyCtrlUnderscore, tea.KeyBackspace,

	// Other keys recognized by Bubble Tea's terminal input parser.
	tea.KeyUp, tea.KeyDown, tea.KeyRight, tea.KeyLeft, tea.KeyShiftTab,
	tea.KeyHome, tea.KeyEnd, tea.KeyPgUp, tea.KeyPgDown,
	tea.KeyCtrlPgUp, tea.KeyCtrlPgDown, tea.KeyDelete, tea.KeyInsert, tea.KeySpace,
	tea.KeyCtrlUp, tea.KeyCtrlDown, tea.KeyCtrlRight, tea.KeyCtrlLeft,
	tea.KeyCtrlHome, tea.KeyCtrlEnd,
	tea.KeyShiftUp, tea.KeyShiftDown, tea.KeyShiftRight, tea.KeyShiftLeft,
	tea.KeyShiftHome, tea.KeyShiftEnd,
	tea.KeyCtrlShiftUp, tea.KeyCtrlShiftDown, tea.KeyCtrlShiftRight, tea.KeyCtrlShiftLeft,
	tea.KeyCtrlShiftHome, tea.KeyCtrlShiftEnd,
	tea.KeyF1, tea.KeyF2, tea.KeyF3, tea.KeyF4, tea.KeyF5,
	tea.KeyF6, tea.KeyF7, tea.KeyF8, tea.KeyF9, tea.KeyF10,
	tea.KeyF11, tea.KeyF12, tea.KeyF13, tea.KeyF14, tea.KeyF15,
	tea.KeyF16, tea.KeyF17, tea.KeyF18, tea.KeyF19, tea.KeyF20,
}

var keyNameAliases = map[string]string{
	"space":  " ",
	"ctrl+`": "ctrl+@",
	"ctrl+i": "tab",
	"ctrl+m": "enter",
	"ctrl+[": "esc",
	"ctrl+?": "backspace",
}

// parseKeyName validates a canonical key name and rebuilds the exact tea.KeyMsg
// a terminal sends. ResolveKeys and command-palette replay both use it, so a
// binding accepted at startup cannot later disappear from the palette.
//
// The exact String round trip is load-bearing: key.Matches compares strings,
// while handlers may also inspect Type. Reconstructing a special name as runes
// would pass the former and violate the latter.
func parseKeyName(name string) (tea.KeyMsg, error) {
	base := name
	alt := strings.HasPrefix(base, "alt+")
	if alt {
		base = strings.TrimPrefix(base, "alt+")
	}
	if canonical, ok := keyNameAliases[base]; ok {
		if alt {
			canonical = "alt+" + canonical
		}
		return tea.KeyMsg{}, fmt.Errorf("noncanonical key %q; use %q instead", name, canonical)
	}

	for _, typ := range canonicalKeyTypes {
		msg := tea.KeyMsg{Type: typ, Alt: alt}
		if typ == tea.KeySpace {
			msg.Runes = []rune{' '}
		}
		if msg.String() == name {
			return msg, nil
		}
	}

	runes := []rune(base)
	if len(runes) == 1 && (runes[0] > 0x1f && runes[0] != 0x7f) {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: runes, Alt: alt}
		if msg.String() == name {
			return msg, nil
		}
	}
	return tea.KeyMsg{}, fmt.Errorf("key %q is not a canonical single Bubble Tea key", name)
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
