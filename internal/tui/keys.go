package tui

// bkey aliases bubbles/key: log_test.go's helper func key(s string) is a
// package-scope identifier, and importing bubbles/key under its default
// name collides with it (Go rejects an import name that matches any
// package-level declaration, even one in a different file).
import bkey "github.com/charmbracelet/bubbles/key"

// keyDefaults is the screen-independent binding table: every key the TUI
// knows, defined exactly once, with its help text. User-configurable
// keybindings (#82) will override entries here and nothing else.
type keyDefaults struct {
	Quit bkey.Binding
	Back bkey.Binding

	// Home (home.go's updateHome switch, migrated in full here since Home is
	// the one screen this task fully covers).
	PrevMonth   bkey.Binding
	NextMonth   bkey.Binding
	ToggleWeek  bkey.Binding
	Range       bkey.Binding
	ToggleScope bkey.Binding
	LogHours    bkey.Binding
	Timer       bkey.Binding
	Members     bkey.Binding
	Generate    bkey.Binding
	// … one field per distinct action, added as screens arrive in Tasks 3-4 …
}

func defaultKeys() keyDefaults {
	return keyDefaults{
		Quit: bkey.NewBinding(bkey.WithKeys("q"), bkey.WithHelp("q", "quit")),
		Back: bkey.NewBinding(bkey.WithKeys("esc"), bkey.WithHelp("esc", "back")),

		PrevMonth:   bkey.NewBinding(bkey.WithKeys("left", "h"), bkey.WithHelp("◂/h", "prev month (this_month only)")),
		NextMonth:   bkey.NewBinding(bkey.WithKeys("right", "l"), bkey.WithHelp("▸/l", "next month (this_month only)")),
		ToggleWeek:  bkey.NewBinding(bkey.WithKeys("w"), bkey.WithHelp("w", "this week/month")),
		Range:       bkey.NewBinding(bkey.WithKeys("d"), bkey.WithHelp("d", "range")),
		ToggleScope: bkey.NewBinding(bkey.WithKeys("t"), bkey.WithHelp("t", "me/team")),
		LogHours:    bkey.NewBinding(bkey.WithKeys("n"), bkey.WithHelp("n", "log hours")),
		Timer:       bkey.NewBinding(bkey.WithKeys("c"), bkey.WithHelp("c", "manage timer")),
		Members:     bkey.NewBinding(bkey.WithKeys("f"), bkey.WithHelp("f", "select members")),
		Generate:    bkey.NewBinding(bkey.WithKeys("enter"), bkey.WithHelp("enter", "generate report")),
	}
}

// keyMap is one screen's active bindings, plus the order the footer (#69)
// and the ? overlay (#69) will render them in. A zero Binding is disabled
// and matches nothing — that is how contextual keys are switched off.
type keyMap struct {
	Quit bkey.Binding
	Back bkey.Binding

	PrevMonth   bkey.Binding
	NextMonth   bkey.Binding
	ToggleWeek  bkey.Binding
	Range       bkey.Binding
	ToggleScope bkey.Binding
	LogHours    bkey.Binding
	Timer       bkey.Binding
	Members     bkey.Binding
	Generate    bkey.Binding

	short []bkey.Binding
	full  [][]bkey.Binding
}

func (k keyMap) ShortHelp() []bkey.Binding  { return k.short }
func (k keyMap) FullHelp() [][]bkey.Binding { return k.full }

// allBindings returns every binding field, so the parity tests do not have
// to know the struct's shape. Keep it in sync when a field is added.
func (k keyMap) allBindings() []bkey.Binding {
	return []bkey.Binding{
		k.Quit, k.Back,
		k.PrevMonth, k.NextMonth, k.ToggleWeek, k.Range, k.ToggleScope,
		k.LogHours, k.Timer, k.Members, k.Generate,
	}
}

// keysFor returns the bindings the given Model state accepts right now. It
// is pure: nothing is cached, so there is no second copy of the truth to
// keep in sync. Contextual gating lives here AND in the handlers' guards —
// once a binding is disabled key.Matches fails and the guard is
// unreachable, so enablement is load-bearing for behavior, not just for
// display.
func keysFor(m Model) keyMap {
	d := defaultKeys()
	switch m.screen {
	case screenHome:
		return homeKeys(m, d)
	case screenLoading, screenError:
		// No key handler, but both accept q today; a zero keyMap would
		// silently disable it and leave Loading with no exit at all.
		return keyMap{Quit: d.Quit, short: []bkey.Binding{d.Quit}}
	case screenSetup:
		return setupKeys()
	case screenReport:
		return reportKeys(d)
	case screenExport:
		return exportKeys(d)
	case screenRates:
		return ratesKeys()
	case screenLog:
		return logKeys()
	case screenMembers:
		return membersKeys(d)
	case screenRange:
		return rangeKeys()
	case screenFilters:
		return filtersKeys(d)
	case screenListBrowser:
		return listBrowserKeys()
	case screenBudget:
		return budgetKeys(d)
	case screenEntries:
		return entriesKeys()
	}
	return keyMap{}
}

// homeKeys is the full binding set for screenHome (home.go's updateHome):
// every case label of that switch, plus the global q (added by keysFor's
// caller, not here — see app.go). Members is the one contextually gated
// binding home.go's own view already hides/shows conditionally (the "f:
// select members" help fragment is team-scope-only); the other keys have no
// such conditional display today; their gating stays inline in home.go
// until Task 3 migrates the handler and moves the guard here too (per the
// plan's "Home's c" / "Home's left/right" enablement⇔guard tests).
func homeKeys(m Model, d keyDefaults) keyMap {
	k := keyMap{
		Quit:        d.Quit,
		PrevMonth:   d.PrevMonth,
		NextMonth:   d.NextMonth,
		ToggleWeek:  d.ToggleWeek,
		Range:       d.Range,
		ToggleScope: d.ToggleScope,
		LogHours:    d.LogHours,
		Timer:       d.Timer,
		Members:     d.Members,
		Generate:    d.Generate,
	}
	// SetEnabled mutates the copy in k, so it must run BEFORE short/full are
	// built from those fields — the slices hold value copies, not references.
	k.Members.SetEnabled(m.scope == "team")
	k.short = []bkey.Binding{k.Generate, k.Range, k.Members, k.Quit}
	k.full = [][]bkey.Binding{
		{k.PrevMonth, k.NextMonth, k.ToggleWeek},
		{k.Range, k.ToggleScope, k.Members},
		{k.Generate, k.LogHours, k.Timer},
		{k.Quit},
	}
	return k
}

// setupKeys, ratesKeys, logKeys, rangeKeys, listBrowserKeys and entriesKeys
// are minimal Quit-off placeholders for screens not yet migrated (Tasks
// 3-4): q does not quit these screens today (see app.go's old exclusion
// list, replaced in this task by keysFor(m).Quit), so keysFor must not
// enable it here either. Each grows its real bindings when its handler
// migrates.
func setupKeys() keyMap       { return keyMap{} }
func ratesKeys() keyMap       { return keyMap{} }
func logKeys() keyMap         { return keyMap{} }
func rangeKeys() keyMap       { return keyMap{} }
func listBrowserKeys() keyMap { return keyMap{} }
func entriesKeys() keyMap     { return keyMap{} }

// reportKeys, exportKeys, membersKeys, filtersKeys and budgetKeys are
// minimal placeholders too, but q DOES quit these screens today, so Quit is
// the one binding each carries until its handler migrates in Task 3.
func reportKeys(d keyDefaults) keyMap {
	return keyMap{Quit: d.Quit, short: []bkey.Binding{d.Quit}}
}

func exportKeys(d keyDefaults) keyMap {
	return keyMap{Quit: d.Quit, short: []bkey.Binding{d.Quit}}
}

func membersKeys(d keyDefaults) keyMap {
	return keyMap{Quit: d.Quit, short: []bkey.Binding{d.Quit}}
}

func filtersKeys(d keyDefaults) keyMap {
	return keyMap{Quit: d.Quit, short: []bkey.Binding{d.Quit}}
}

func budgetKeys(d keyDefaults) keyMap {
	return keyMap{Quit: d.Quit, short: []bkey.Binding{d.Quit}}
}
