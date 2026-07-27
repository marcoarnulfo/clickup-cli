package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
)

// footerView renders one screen's advertised bindings as the bottom help line,
// or as stacked columns when showAll is set.
//
// It is a pure function rather than a sub-model on purpose. help.Model is
// configuration plus two state fields, so rebuilding it per render costs
// nothing, and it cannot fall victim to the one hazard this package keeps
// meeting: a sub-model that must be written back explicitly and silently
// zeroes itself when someone forgets.
//
// help.New() is deliberately NOT used: it derives its styles from lipgloss's
// default renderer, which would bypass the injected-renderer discipline the
// theme exists to enforce.
func footerView(th theme, width int, showAll bool, k keyMap) string {
	h := help.Model{
		Width:   width,
		ShowAll: showAll,
		// The house separator, not bubbles' " • ".
		ShortSeparator: " · ",
		FullSeparator:  "    ",
		Ellipsis:       "…",
		Styles: help.Styles{
			Ellipsis:       th.Help,
			ShortKey:       th.Help.Bold(true),
			ShortDesc:      th.Help,
			ShortSeparator: th.Help,
			FullKey:        th.Help.Bold(true),
			FullDesc:       th.Help,
			FullSeparator:  th.Help,
		},
	}
	return h.View(k)
}

// pairHelp returns a display-only binding that advertises two related bindings
// as a single footer item, so "↑/↓/j/k move" replaces "↑/k move up · ↓/j move
// down".
//
// It is never passed to key.Matches: matching only ever reads keyMap's own
// fields, while short and full are the advertisement list. That separation is
// the same one ForceQuit uses — enablement says what is accepted, short says
// what is shown.
func pairHelp(a, b key.Binding, keyLabel, desc string) key.Binding {
	p := key.NewBinding(
		key.WithKeys(append(append([]string{}, a.Keys()...), b.Keys()...)...),
		key.WithHelp(keyLabel, desc),
	)
	p.SetEnabled(a.Enabled() || b.Enabled())
	return p
}

// anyKeyHelp returns a display-only binding for a handler whose behaviour is
// triggered by the ABSENCE of a match — the delete confirmation's "any other
// key cancels". No real binding can express that, and dropping it from the
// footer would lose information the hand-written line carried.
func anyKeyHelp(desc string) key.Binding {
	return key.NewBinding(key.WithKeys("any"), key.WithHelp("any key", desc))
}
