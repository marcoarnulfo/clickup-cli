package tui

// An import name shares the package block with package-level declarations, so
// log_test.go's tea.KeyMsg helper is named keyMsg rather than key: that is what
// lets bubbles/key keep its own name here and in every handler.
import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// keyDefaults is the screen-independent binding table: every key the TUI
// knows, defined exactly once, with its help text. User-configurable
// keybindings (#82) will override entries here and nothing else.
type keyDefaults struct {
	Quit key.Binding
	Back key.Binding

	// Home (home.go's updateHome switch, migrated in full here since Home is
	// the one screen this task fully covers).
	PrevMonth   key.Binding
	NextMonth   key.Binding
	ToggleWeek  key.Binding
	Range       key.Binding
	ToggleScope key.Binding
	LogHours    key.Binding
	Timer       key.Binding
	Members     key.Binding
	Generate    key.Binding

	// Generic list-navigation and small-form actions (Task 3): the physical
	// key and the meaning behind it are identical across every screen that
	// uses them (move the cursor, confirm, toggle the current item, select
	// all/none, switch field/section), so each is defined once here rather
	// than duplicated per screen.
	Up         key.Binding
	Down       key.Binding
	Confirm    key.Binding
	ToggleItem key.Binding
	SelectAll  key.Binding
	NextField  key.Binding
	PrevField  key.Binding

	// Report's screen-opening actions (report.go's updateReport switch).
	// Budget is shared with budget.go: 'b' opens the budget view from
	// Report and the same binding closes it from Budget (see budgetKeys).
	GroupBy     key.Binding
	ChangeRange key.Binding
	Reload      key.Binding
	Export      key.Binding
	Rates       key.Binding
	Filters     key.Binding
	Budget      key.Binding
	OpenEntries key.Binding
	// … one field per distinct action, added as screens arrive in Tasks 3-4 …
}

func defaultKeys() keyDefaults {
	return keyDefaults{
		Quit: key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		Back: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),

		PrevMonth:   key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("◂/h", "prev month (this_month only)")),
		NextMonth:   key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("▸/l", "next month (this_month only)")),
		ToggleWeek:  key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "this week/month")),
		Range:       key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "range")),
		ToggleScope: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "me/team")),
		LogHours:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "log hours")),
		Timer:       key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "manage timer")),
		Members:     key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "select members")),
		Generate:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "generate report")),

		Up:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "move up")),
		Down:       key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "move down")),
		Confirm:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		ToggleItem: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
		SelectAll:  key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "all/none")),
		NextField:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field/section")),
		PrevField:  key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev field/section")),

		GroupBy:     key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "grouping")),
		ChangeRange: key.NewBinding(key.WithKeys("m", "s"), key.WithHelp("m/s", "change range/scope")),
		Reload:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reload")),
		Export:      key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "export")),
		Rates:       key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "rates")),
		Filters:     key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filters")),
		Budget:      key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "budgets")),
		OpenEntries: key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "entries")),
	}
}

// keyMap is one screen's active bindings, plus the order the footer (#69)
// and the ? overlay (#69) will render them in. A zero Binding is disabled
// and matches nothing — that is how contextual keys are switched off.
type keyMap struct {
	Quit key.Binding
	Back key.Binding

	PrevMonth   key.Binding
	NextMonth   key.Binding
	ToggleWeek  key.Binding
	Range       key.Binding
	ToggleScope key.Binding
	LogHours    key.Binding
	Timer       key.Binding
	Members     key.Binding
	Generate    key.Binding

	Up         key.Binding
	Down       key.Binding
	Confirm    key.Binding
	ToggleItem key.Binding
	SelectAll  key.Binding
	NextField  key.Binding
	PrevField  key.Binding

	GroupBy     key.Binding
	ChangeRange key.Binding
	Reload      key.Binding
	Export      key.Binding
	Rates       key.Binding
	Filters     key.Binding
	Budget      key.Binding
	OpenEntries key.Binding

	short []key.Binding
	full  [][]key.Binding
}

func (k keyMap) ShortHelp() []key.Binding  { return k.short }
func (k keyMap) FullHelp() [][]key.Binding { return k.full }

// allBindings returns every binding field, so the parity tests do not have
// to know the struct's shape. Keep it in sync when a field is added.
func (k keyMap) allBindings() []key.Binding {
	return []key.Binding{
		k.Quit, k.Back,
		k.PrevMonth, k.NextMonth, k.ToggleWeek, k.Range, k.ToggleScope,
		k.LogHours, k.Timer, k.Members, k.Generate,
		k.Up, k.Down, k.Confirm, k.ToggleItem, k.SelectAll, k.NextField, k.PrevField,
		k.GroupBy, k.ChangeRange, k.Reload, k.Export, k.Rates, k.Filters, k.Budget, k.OpenEntries,
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
		return keyMap{Quit: d.Quit, short: []key.Binding{d.Quit}}
	case screenSetup:
		return setupKeys(m, d)
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
		return rangeKeys(m, d)
	case screenFilters:
		return filtersKeys(d)
	case screenListBrowser:
		return listBrowserKeys(d)
	case screenBudget:
		return budgetKeys(d)
	case screenEntries:
		return entriesKeys()
	}
	return keyMap{}
}

// homeKeys is the full binding set for screenHome (home.go's updateHome):
// every case label of that switch, plus the global q (added by keysFor's
// caller, not here — see app.go). Members, Timer and the month-nav pair are
// the contextually gated bindings: Members to the team scope, Timer to a
// running timer, PrevMonth/NextMonth to the this_month preset outside week
// mode — home.go's own view already hides/shows the first two conditionally
// (the "f: select members" and "c: manage timer" help fragments); the guard
// for all three used to live inline in the handler and now lives here
// instead (see TestHomeMembersKeyIsTeamScopeOnly, TestHomeTimerKeyMatchesGuard
// and TestHomeMonthNavKeysMatchGuard).
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
	k.Timer.SetEnabled(m.runningTimer != nil)
	monthNav := m.preset == report.PresetThisMonth && m.periodMode != periodModeWeek
	k.PrevMonth.SetEnabled(monthNav)
	k.NextMonth.SetEnabled(monthNav)
	k.short = []key.Binding{k.Generate, k.Range, k.Members, k.Quit}
	k.full = [][]key.Binding{
		{k.PrevMonth, k.NextMonth, k.ToggleWeek},
		{k.Range, k.ToggleScope, k.Members},
		{k.Generate, k.LogHours, k.Timer},
		{k.Quit},
	}
	return k
}

// setupKeys is the binding set for screenSetup (setup.go's updateSetup): each
// wizard step accepts a different label set, so the step decides which
// generic bindings are live. stepToken and stepRate/stepCurrency only ever
// react to Confirm (everything else is forwarded to the focused textinput);
// stepWorkspace also has a list to move through.
func setupKeys(m Model, d keyDefaults) keyMap {
	k := keyMap{Confirm: d.Confirm}
	if m.setup.step == stepWorkspace {
		k.Up = d.Up
		k.Down = d.Down
		k.short = []key.Binding{k.Up, k.Down, k.Confirm}
		return k
	}
	k.short = []key.Binding{k.Confirm}
	return k
}

// ratesKeys, logKeys and entriesKeys are minimal Quit-off placeholders for
// screens not yet migrated (Task 4): q does not quit these screens today
// (see app.go's old exclusion list, replaced by keysFor(m).Quit), so keysFor
// must not enable it here either. Each grows its real bindings when its
// handler migrates.
func ratesKeys() keyMap   { return keyMap{} }
func logKeys() keyMap     { return keyMap{} }
func entriesKeys() keyMap { return keyMap{} }

// rangeKeys is the binding set for screenRange (range.go's updateRange): the
// list of presets and the custom-date editor accept different label sets, so
// the active mode (rangeScreen.editing) decides which are live. Quit stays
// off in both modes (q does not quit this screen today).
func rangeKeys(m Model, d keyDefaults) keyMap {
	k := keyMap{Confirm: d.Confirm, Back: d.Back}
	if m.rangeScreen.editing {
		k.NextField = d.NextField
		k.PrevField = d.PrevField
		k.short = []key.Binding{k.Confirm, k.NextField, k.PrevField, k.Back}
		return k
	}
	k.Up = d.Up
	k.Down = d.Down
	k.short = []key.Binding{k.Up, k.Down, k.Confirm, k.Back}
	return k
}

// listBrowserKeys is the binding set for screenListBrowser
// (listbrowser.go's updateListBrowser): the same four labels are accepted at
// every drill-down level (only the action taken differs), and while the
// screen is loading the handler's own guard ignores all of them anyway — so
// there is no separate loading state to model here. Quit stays off (q does
// not quit this screen today).
func listBrowserKeys(d keyDefaults) keyMap {
	k := keyMap{Up: d.Up, Down: d.Down, Confirm: d.Confirm, Back: d.Back}
	k.short = []key.Binding{k.Up, k.Down, k.Confirm, k.Back}
	return k
}

// reportKeys, exportKeys, membersKeys, filtersKeys and budgetKeys are the
// binding sets for their screens (report.go, export.go, members.go,
// filters.go, budget.go); q DOES quit these screens today.
func reportKeys(d keyDefaults) keyMap {
	k := keyMap{
		Quit: d.Quit, GroupBy: d.GroupBy, ChangeRange: d.ChangeRange, Reload: d.Reload,
		Export: d.Export, Rates: d.Rates, LogHours: d.LogHours, Filters: d.Filters,
		Budget: d.Budget, OpenEntries: d.OpenEntries,
	}
	k.short = []key.Binding{k.GroupBy, k.Export, k.Filters, k.Budget, k.Quit}
	k.full = [][]key.Binding{
		{k.GroupBy, k.Export, k.Rates, k.LogHours},
		{k.Filters, k.Budget, k.OpenEntries},
		{k.ChangeRange, k.Reload, k.Quit},
	}
	return k
}

func exportKeys(d keyDefaults) keyMap {
	k := keyMap{Quit: d.Quit, Up: d.Up, Down: d.Down, Confirm: d.Confirm, Back: d.Back}
	k.short = []key.Binding{k.Up, k.Down, k.Confirm, k.Back, k.Quit}
	return k
}

func membersKeys(d keyDefaults) keyMap {
	k := keyMap{
		Quit: d.Quit, Up: d.Up, Down: d.Down, ToggleItem: d.ToggleItem,
		SelectAll: d.SelectAll, Confirm: d.Confirm, Back: d.Back,
	}
	k.short = []key.Binding{k.Up, k.Down, k.ToggleItem, k.SelectAll, k.Confirm, k.Back, k.Quit}
	return k
}

func filtersKeys(d keyDefaults) keyMap {
	k := keyMap{
		Quit: d.Quit, NextField: d.NextField, PrevField: d.PrevField, Up: d.Up, Down: d.Down,
		ToggleItem: d.ToggleItem, SelectAll: d.SelectAll, Confirm: d.Confirm, Back: d.Back,
	}
	k.short = []key.Binding{k.NextField, k.Up, k.Down, k.ToggleItem, k.SelectAll, k.Confirm, k.Back, k.Quit}
	return k
}

func budgetKeys(d keyDefaults) keyMap {
	k := keyMap{Quit: d.Quit, Back: d.Back, Budget: d.Budget}
	k.short = []key.Binding{k.Back, k.Budget, k.Quit}
	return k
}
