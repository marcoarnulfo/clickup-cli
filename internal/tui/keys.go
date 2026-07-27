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

	// ForceQuit is the unconditional ctrl+c kill switch (app.go's Update,
	// migrated in Task 4 step 4): unlike Quit, its enablement never varies
	// per screen, so it is checked directly against defaultKeys(), bypassing
	// keysFor(m) entirely, both before and after that migration. As of #69
	// Task 2 it ALSO lives in keyMap (assigned by every constructor, listed
	// in allBindings) purely so the footer can advertise it on the six
	// screens where q cannot quit — that copy's Enabled() is never what
	// key.Matches consults for the actual kill switch.
	ForceQuit key.Binding

	// Help toggles the full-help overlay (#69 Task 4). Unlike ForceQuit, its
	// enablement genuinely varies per screen: keysFor leaves it unassigned
	// wherever a '?' keypress already means something else (a focused
	// textinput, an any-key confirmation, Error's own catch-all).
	Help key.Binding

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

	// Rates' section switch and screen-specific actions (rates.go's
	// updateRates switch, migrated in Task 4). NextSection/PrevSection cycle
	// the Lists/Members/Overrides/Rules tabs — a distinct action from
	// NextField/PrevField above even though tab/shift+tab overlap, because
	// this screen also accepts right/l (next) and left/h (prev) as
	// synonyms (rates.go:414,417).
	NextSection  key.Binding
	PrevSection  key.Binding
	ListCurrency key.Binding
	ListBudget   key.Binding
	NewOverride  key.Binding
	ClearValue   key.Binding
	BrowseList   key.Binding
	Save         key.Binding

	// Entries' browser actions (entries.go's updateEntries switch, migrated
	// in Task 4). Delete/Edit/Tags are ownership-gated (canEdit); History is
	// not (it's read-only, allowed on any entry). ConfirmDelete (y/Y, no
	// enter) is entriesConfirmDelete's own yes, distinct from Yes below
	// because that step has no enter-means-yes shortcut (entries.go:260).
	Delete        key.Binding
	Edit          key.Binding
	History       key.Binding
	Tags          key.Binding
	ConfirmDelete key.Binding
	NewTag        key.Binding

	// Yes/No is the billable-toggle keypress shared verbatim by log.go's
	// logForm (formField 3) and entries.go's updateEntriesEdit (editStep 4):
	// both switches accept exactly "n"/"N" for no and "y"/"Y"/"enter" for
	// yes (log.go:359-362, entries.go:293-296).
	Yes key.Binding
	No  key.Binding

	// Log's mode/step-specific actions (log.go's updateLog switch, migrated
	// in Task 4). PickGuided/PickByID/PickTimer are logModeSelect's three
	// initial choices; logTimerPick reuses PickGuided/PickByID for the
	// timer sub-flow's two options, which have no third (log.go:283-291,
	// 298-304).
	PickGuided key.Binding
	PickByID   key.Binding
	PickTimer  key.Binding
	StopTimer  key.Binding
	// … one field per distinct action, added as screens arrive in Tasks 3-4 …
}

func defaultKeys() keyDefaults {
	return keyDefaults{
		Quit:      key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		Back:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		ForceQuit: key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "force quit")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),

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

		NextSection:  key.NewBinding(key.WithKeys("tab", "right", "l"), key.WithHelp("tab/▸", "next section")),
		PrevSection:  key.NewBinding(key.WithKeys("shift+tab", "left", "h"), key.WithHelp("shift+tab/◂", "prev section")),
		ListCurrency: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "list currency")),
		ListBudget:   key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "list budget")),
		NewOverride:  key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new override")),
		ClearValue:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "clear/revert")),
		BrowseList:   key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "browse lists")),
		Save:         key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "save")),

		Delete:        key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "delete")),
		Edit:          key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		History:       key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "history")),
		Tags:          key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "tags")),
		ConfirmDelete: key.NewBinding(key.WithKeys("y", "Y"), key.WithHelp("y", "confirm delete")),
		NewTag:        key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new tag")),

		Yes: key.NewBinding(key.WithKeys("y", "Y", "enter"), key.WithHelp("y/enter", "yes")),
		No:  key.NewBinding(key.WithKeys("n", "N"), key.WithHelp("n", "no")),

		PickGuided: key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "guided")),
		PickByID:   key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "task ID/URL")),
		PickTimer:  key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "timer")),
		StopTimer:  key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "stop timer")),
	}
}

// keyMap is one screen's active bindings, plus the order the footer (#69)
// and the ? overlay (#69) will render them in. A zero Binding is disabled
// and matches nothing — that is how contextual keys are switched off.
type keyMap struct {
	Quit      key.Binding
	Back      key.Binding
	ForceQuit key.Binding
	Help      key.Binding

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

	NextSection  key.Binding
	PrevSection  key.Binding
	ListCurrency key.Binding
	ListBudget   key.Binding
	NewOverride  key.Binding
	ClearValue   key.Binding
	BrowseList   key.Binding
	Save         key.Binding

	Delete        key.Binding
	Edit          key.Binding
	History       key.Binding
	Tags          key.Binding
	ConfirmDelete key.Binding
	NewTag        key.Binding

	Yes key.Binding
	No  key.Binding

	PickGuided key.Binding
	PickByID   key.Binding
	PickTimer  key.Binding
	StopTimer  key.Binding

	short []key.Binding
	full  [][]key.Binding
}

func (k keyMap) ShortHelp() []key.Binding  { return k.short }
func (k keyMap) FullHelp() [][]key.Binding { return k.full }

// allBindings returns every binding field, so the parity tests do not have to
// know the struct's shape. A field missing here drops silently out of every
// parity test, so TestAllBindingsCoversEveryField pins the count by reflection
// rather than trusting anyone to remember.
//
// full is populated for every screen except Error, which has no key worth a
// full-help column (any key returns Home) and keeps its inline sentence
// instead. Nothing renders FullHelp() yet — Task 4's ? overlay is what will.
func (k keyMap) allBindings() []key.Binding {
	return []key.Binding{
		k.Quit, k.Back, k.ForceQuit, k.Help,
		k.PrevMonth, k.NextMonth, k.ToggleWeek, k.Range, k.ToggleScope,
		k.LogHours, k.Timer, k.Members, k.Generate,
		k.Up, k.Down, k.Confirm, k.ToggleItem, k.SelectAll, k.NextField, k.PrevField,
		k.GroupBy, k.ChangeRange, k.Reload, k.Export, k.Rates, k.Filters, k.Budget, k.OpenEntries,
		k.NextSection, k.PrevSection, k.ListCurrency, k.ListBudget, k.NewOverride, k.ClearValue, k.BrowseList, k.Save,
		k.Delete, k.Edit, k.History, k.Tags, k.ConfirmDelete, k.NewTag, k.Yes, k.No,
		k.PickGuided, k.PickByID, k.PickTimer, k.StopTimer,
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
	case screenLoading:
		k := keyMap{Quit: d.Quit, ForceQuit: d.ForceQuit, Help: d.Help}
		k.short = []key.Binding{k.Help, k.Quit}
		k.full = [][]key.Binding{{k.Help, k.Quit}}
		return k
	case screenError:
		// Every key returns Home here, so ? must not claim one. ForceQuit is
		// assigned because ctrl+c is accepted — it is simply not advertised.
		k := keyMap{Quit: d.Quit, ForceQuit: d.ForceQuit}
		k.short = []key.Binding{k.Quit}
		return k
	case screenSetup:
		return setupKeys(m, d)
	case screenReport:
		return reportKeys(d)
	case screenExport:
		return exportKeys(d)
	case screenRates:
		return ratesKeys(m, d)
	case screenLog:
		return logKeys(m, d)
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
		return entriesKeys(m, d)
	}
	return keyMap{}
}

// homeKeys is the full binding set for screenHome (home.go's updateHome):
// every case label of that switch, plus q — which is declared here like any
// other binding but handled globally in app.go rather than in home.go's own
// switch. Members, Timer and the month-nav pair are
// the contextually gated bindings: Members to the team scope, Timer to a
// running timer, PrevMonth/NextMonth to the this_month preset outside week
// mode. home.go's view used to hide and show the first two through
// hand-written help fragments, which the generated footer replaced; the guard
// for all three used to live inline in the handler and now lives here
// instead (see TestHomeMembersKeyIsTeamScopeOnly, TestHomeTimerKeyMatchesGuard
// and TestHomeMonthNavKeysMatchGuard).
func homeKeys(m Model, d keyDefaults) keyMap {
	k := keyMap{
		Quit:        d.Quit,
		ForceQuit:   d.ForceQuit,
		Help:        d.Help,
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
	monthPair := pairHelp(k.PrevMonth, k.NextMonth, "◂/▸/h/l", "change month")
	k.short = []key.Binding{k.Generate, k.Range, k.Members, k.Help, k.Quit}
	k.full = [][]key.Binding{
		{monthPair, k.ToggleWeek},
		{k.Range, k.ToggleScope, k.Members},
		{k.Generate, k.LogHours, k.Timer},
		{k.Help, k.Quit},
	}
	return k
}

// setupKeys is the binding set for screenSetup (setup.go's updateSetup): each
// wizard step accepts a different label set, so the step decides which
// generic bindings are live. stepToken and stepRate/stepCurrency only ever
// react to Confirm (everything else is forwarded to the focused textinput);
// stepWorkspace also has a list to move through.
func setupKeys(m Model, d keyDefaults) keyMap {
	k := keyMap{Confirm: d.Confirm, ForceQuit: d.ForceQuit}
	if m.setup.step == stepWorkspace {
		k.Up = d.Up
		k.Down = d.Down
		k.Help = d.Help
		pair := pairHelp(k.Up, k.Down, "↑/↓/j/k", "move")
		k.short = []key.Binding{pair, k.Confirm, k.Help, k.ForceQuit}
		k.full = [][]key.Binding{{pair, k.Confirm}, {k.Help, k.ForceQuit}}
		return k
	}
	// stepToken, stepRate and stepCurrency forward everything but Enter to a
	// focused textinput, so ? must not claim a keystroke there — Help stays
	// unassigned (zero Binding), which help.FullHelpView silently drops.
	k.short = []key.Binding{k.Confirm, k.ForceQuit}
	k.full = [][]key.Binding{{k.Confirm}, {k.ForceQuit}}
	return k
}

// logKeys is the binding set for screenLog (log.go's updateLog): each step of
// the flow has its own label set. Quit stays off in every step (q does not
// quit this screen today).
//
// Back is present in almost every step: it mirrors updateLog's own outer
// guard (originally an Esc-vs-tea.KeyEsc comparison gated on lg.step !=
// logIDInput/logForm, now key.Matches(msg, k.Back) with the same step
// exclusion), which pops back to whoever pushed the log flow (see
// Model.nav) for every step except logIDInput and logForm — those two
// handle Back internally with a DIFFERENT destination (one step back within
// the flow, not out of it), but the accepted LABEL is identical either way,
// which is all this function models.
func logKeys(m Model, d keyDefaults) keyMap {
	lg := m.logScreen
	switch lg.step {
	case logModeSelect:
		k := keyMap{Back: d.Back, ForceQuit: d.ForceQuit, Help: d.Help, PickGuided: d.PickGuided, PickByID: d.PickByID, PickTimer: d.PickTimer}
		k.short = []key.Binding{k.PickGuided, k.PickByID, k.PickTimer, k.Back, k.Help, k.ForceQuit}
		k.full = [][]key.Binding{{k.PickGuided, k.PickByID, k.PickTimer}, {k.Help, k.Back, k.ForceQuit}}
		return k

	case logTimerPick:
		k := keyMap{Back: d.Back, ForceQuit: d.ForceQuit, Help: d.Help, PickGuided: d.PickGuided, PickByID: d.PickByID}
		k.short = []key.Binding{k.PickGuided, k.PickByID, k.Back, k.Help, k.ForceQuit}
		k.full = [][]key.Binding{{k.PickGuided, k.PickByID}, {k.Help, k.Back, k.ForceQuit}}
		return k

	case logListPick, logTaskPick:
		k := keyMap{Back: d.Back, ForceQuit: d.ForceQuit, Help: d.Help, Up: d.Up, Down: d.Down, Confirm: d.Confirm}
		pair := pairHelp(k.Up, k.Down, "↑/↓/j/k", "move")
		k.short = []key.Binding{pair, k.Confirm, k.Back, k.Help, k.ForceQuit}
		k.full = [][]key.Binding{{pair, k.Confirm}, {k.Help, k.Back, k.ForceQuit}}
		return k

	case logIDInput:
		// A focused textinput takes '?' as a character, so Help stays
		// unassigned here.
		k := keyMap{Confirm: d.Confirm, Back: d.Back, ForceQuit: d.ForceQuit}
		k.short = []key.Binding{k.Confirm, k.Back, k.ForceQuit}
		k.full = [][]key.Binding{{k.Confirm}, {k.Back, k.ForceQuit}}
		return k

	case logForm:
		if lg.formField == 3 { // billable toggle (keypress, not a text field)
			k := keyMap{Yes: d.Yes, No: d.No, Back: d.Back, ForceQuit: d.ForceQuit, Help: d.Help}
			k.short = []key.Binding{k.Yes, k.No, k.Back, k.Help, k.ForceQuit}
			k.full = [][]key.Binding{{k.Yes, k.No}, {k.Help, k.Back, k.ForceQuit}}
			return k
		}
		// Every other form field is a focused textinput, so Help stays
		// unassigned here too.
		k := keyMap{Confirm: d.Confirm, Back: d.Back, ForceQuit: d.ForceQuit}
		k.short = []key.Binding{k.Confirm, k.Back, k.ForceQuit}
		k.full = [][]key.Binding{{k.Confirm}, {k.Back, k.ForceQuit}}
		return k

	case logTimerRunning:
		k := keyMap{StopTimer: d.StopTimer, Back: d.Back, ForceQuit: d.ForceQuit, Help: d.Help}
		k.short = []key.Binding{k.StopTimer, k.Back, k.Help, k.ForceQuit}
		k.full = [][]key.Binding{{k.StopTimer}, {k.Help, k.Back, k.ForceQuit}}
		return k

	case logDone:
		k := keyMap{Reload: d.Reload, Confirm: d.Confirm, Back: d.Back, ForceQuit: d.ForceQuit, Help: d.Help}
		k.short = []key.Binding{k.Reload, k.Confirm, k.Back, k.Help, k.ForceQuit}
		k.full = [][]key.Binding{{k.Reload, k.Confirm}, {k.Help, k.Back, k.ForceQuit}}
		return k
	}
	return keyMap{}
}

// entriesKeys is the binding set for screenEntries (entries.go's
// updateEntries): the browser mode decides the label set, mirroring the
// priority order of updateEntries' own switch on es.mode. Quit stays off in
// every mode (q does not quit this screen today).
//
// entriesList gates Delete/Edit/Tags to ownership (canEdit) AND a non-empty
// list; History is gated to a non-empty list only — it is read-only and
// deliberately NOT ownership-gated (entries.go:162-169,
// TestEntriesHistoryKeyIsNotOwnershipGated). entriesConfirmDelete's "any
// other key cancels" default clause has no binding (it is the absence of a
// match); entriesEdit's editStep==4 (billable) is the one sub-step with a
// different label set (Yes/No instead of Confirm); entriesTags forwards to
// a textinput while tagNewMode is set, same shape as rates'/setup's
// free-text steps.
func entriesKeys(m Model, d keyDefaults) keyMap {
	es := m.entriesScreen

	switch es.mode {
	case entriesConfirmDelete:
		// The default clause ("any other key cancels") has no real binding —
		// anyKeyHelp stands in for it — and that catch-all is also why Help
		// stays unassigned: a '?' here cancels like any other key would.
		k := keyMap{ConfirmDelete: d.ConfirmDelete, ForceQuit: d.ForceQuit}
		cancel := anyKeyHelp("cancel")
		k.short = []key.Binding{k.ConfirmDelete, cancel, k.ForceQuit}
		k.full = [][]key.Binding{{k.ConfirmDelete, cancel}, {k.ForceQuit}}
		return k

	case entriesEdit:
		if es.editStep == 4 {
			k := keyMap{Yes: d.Yes, No: d.No, Back: d.Back, ForceQuit: d.ForceQuit, Help: d.Help}
			k.short = []key.Binding{k.Yes, k.No, k.Back, k.Help, k.ForceQuit}
			k.full = [][]key.Binding{{k.Yes, k.No}, {k.Help, k.Back, k.ForceQuit}}
			return k
		}
		// Steps 0-3 forward everything but Enter/Esc to a focused textinput,
		// so Help stays unassigned.
		k := keyMap{Confirm: d.Confirm, Back: d.Back, ForceQuit: d.ForceQuit}
		k.short = []key.Binding{k.Confirm, k.Back, k.ForceQuit}
		k.full = [][]key.Binding{{k.Confirm}, {k.Back, k.ForceQuit}}
		return k

	case entriesHistory:
		k := keyMap{Back: d.Back, ForceQuit: d.ForceQuit, Help: d.Help}
		k.short = []key.Binding{k.Back, k.Help, k.ForceQuit}
		k.full = [][]key.Binding{{k.Help, k.Back, k.ForceQuit}}
		return k

	case entriesTags:
		if es.tagNewMode {
			// A focused textinput takes '?' as a character, so Help stays
			// unassigned here.
			k := keyMap{Confirm: d.Confirm, Back: d.Back, ForceQuit: d.ForceQuit}
			k.short = []key.Binding{k.Confirm, k.Back, k.ForceQuit}
			k.full = [][]key.Binding{{k.Confirm}, {k.Back, k.ForceQuit}}
			return k
		}
		k := keyMap{
			Up: d.Up, Down: d.Down, ToggleItem: d.ToggleItem,
			NewTag: d.NewTag, Confirm: d.Confirm, Back: d.Back,
			ForceQuit: d.ForceQuit, Help: d.Help,
		}
		pair := pairHelp(k.Up, k.Down, "↑/↓/j/k", "move")
		k.short = []key.Binding{k.ToggleItem, k.NewTag, k.Confirm, k.Back, k.Help, k.ForceQuit}
		k.full = [][]key.Binding{
			{pair, k.ToggleItem},
			{k.NewTag, k.Confirm},
			{k.Help, k.Back, k.ForceQuit},
		}
		return k

	default: // entriesList
		k := keyMap{
			Up: d.Up, Down: d.Down, Back: d.Back,
			Delete: d.Delete, Edit: d.Edit, History: d.History, Tags: d.Tags,
			ForceQuit: d.ForceQuit, Help: d.Help,
		}
		hasEntries := len(es.entries) > 0
		editable := hasEntries && canEdit(es.entries[es.idx], m.userID)
		k.Delete.SetEnabled(editable)
		k.Edit.SetEnabled(editable)
		k.Tags.SetEnabled(editable)
		k.History.SetEnabled(hasEntries)
		pair := pairHelp(k.Up, k.Down, "↑/↓/j/k", "move")
		k.short = []key.Binding{k.Edit, k.Delete, k.Tags, k.History, k.Back, k.Help, k.ForceQuit}
		k.full = [][]key.Binding{
			{pair},
			{k.Edit, k.Delete, k.Tags, k.History},
			{k.Help, k.Back, k.ForceQuit},
		}
		return k
	}
}

// ratesKeys is the binding set for screenRates (rates.go's updateRates): the
// same three-way priority order as the handler itself decides the mode —
// editing (a textinput is open, for ANY field, including the new-override
// draft's rate step), draft-picking (the new-override wizard's list/member
// steps, same label set for both), and the normal section switch. Quit stays
// off in every mode (q does not quit this screen today).
//
// In the normal mode, ListCurrency/ListBudget are gated to the Lists section
// with at least one row (rates.go:427,432); NewOverride to the Overrides
// section (rates.go:437); BrowseList to the Lists section only — unlike
// ListCurrency/ListBudget it has no row-count requirement (rates.go:443,
// TestRatesBIsGatedToListsSection already pins this asymmetry at the handler
// level). ClearValue and Save are unconditional.
func ratesKeys(m Model, d keyDefaults) keyMap {
	rt := m.ratesScreen

	if rt.editing {
		// A focused textinput takes '?' as a character, so Help stays
		// unassigned here — this is also reached via the override draft's
		// rate step (rt.draft.active && rt.editing).
		k := keyMap{Confirm: d.Confirm, Back: d.Back, ForceQuit: d.ForceQuit}
		k.short = []key.Binding{k.Confirm, k.Back, k.ForceQuit}
		k.full = [][]key.Binding{{k.Confirm}, {k.Back, k.ForceQuit}}
		return k
	}
	if rt.draft.active {
		k := keyMap{Up: d.Up, Down: d.Down, Confirm: d.Confirm, Back: d.Back, ForceQuit: d.ForceQuit, Help: d.Help}
		pair := pairHelp(k.Up, k.Down, "↑/↓/j/k", "move")
		k.short = []key.Binding{pair, k.Confirm, k.Back, k.Help, k.ForceQuit}
		k.full = [][]key.Binding{{pair, k.Confirm}, {k.Help, k.Back, k.ForceQuit}}
		return k
	}

	k := keyMap{
		NextSection: d.NextSection, PrevSection: d.PrevSection,
		Up: d.Up, Down: d.Down, Confirm: d.Confirm,
		ListCurrency: d.ListCurrency, ListBudget: d.ListBudget,
		NewOverride: d.NewOverride, ClearValue: d.ClearValue,
		BrowseList: d.BrowseList, Save: d.Save, Back: d.Back,
		ForceQuit: d.ForceQuit, Help: d.Help,
	}
	listsWithRows := rt.sec == secLists && len(rt.rows) > 0
	k.ListCurrency.SetEnabled(listsWithRows)
	k.ListBudget.SetEnabled(listsWithRows)
	k.NewOverride.SetEnabled(rt.sec == secOverrides)
	k.BrowseList.SetEnabled(rt.sec == secLists)
	sectionPair := pairHelp(k.NextSection, k.PrevSection, "tab/⇧tab", "section")
	movePair := pairHelp(k.Up, k.Down, "↑/↓/j/k", "move")
	k.short = []key.Binding{movePair, k.Confirm, k.Save, k.Back, k.Help, k.ForceQuit}
	k.full = [][]key.Binding{
		{sectionPair, movePair, k.Confirm},
		{k.ListCurrency, k.ListBudget, k.NewOverride, k.ClearValue, k.BrowseList},
		{k.Save, k.Help, k.Back, k.ForceQuit},
	}
	return k
}

// rangeKeys is the binding set for screenRange (range.go's updateRange): the
// list of presets and the custom-date editor accept different label sets, so
// the active mode (rangeScreen.editing) decides which are live. Quit stays
// off in both modes (q does not quit this screen today).
func rangeKeys(m Model, d keyDefaults) keyMap {
	k := keyMap{Confirm: d.Confirm, Back: d.Back, ForceQuit: d.ForceQuit}
	if m.rangeScreen.editing {
		// Both fromInput and toInput are focused textinputs here, so Help
		// stays unassigned — one keysFor branch covers both fields.
		k.NextField = d.NextField
		k.PrevField = d.PrevField
		pair := pairHelp(k.NextField, k.PrevField, "tab/⇧tab", "next/prev field")
		k.short = []key.Binding{pair, k.Confirm, k.Back, k.ForceQuit}
		k.full = [][]key.Binding{{pair, k.Confirm}, {k.Back, k.ForceQuit}}
		return k
	}
	k.Up = d.Up
	k.Down = d.Down
	k.Help = d.Help
	pair := pairHelp(k.Up, k.Down, "↑/↓/j/k", "move")
	k.short = []key.Binding{pair, k.Confirm, k.Back, k.Help, k.ForceQuit}
	k.full = [][]key.Binding{{pair, k.Confirm}, {k.Help, k.Back, k.ForceQuit}}
	return k
}

// listBrowserKeys is the binding set for screenListBrowser
// (listbrowser.go's updateListBrowser): the same four labels are accepted at
// every drill-down level (only the action taken differs), and while the
// screen is loading the handler's own guard ignores all of them anyway — so
// there is no separate loading state to model here. Quit stays off (q does
// not quit this screen today).
func listBrowserKeys(d keyDefaults) keyMap {
	k := keyMap{Up: d.Up, Down: d.Down, Confirm: d.Confirm, Back: d.Back, ForceQuit: d.ForceQuit, Help: d.Help}
	pair := pairHelp(k.Up, k.Down, "↑/↓/j/k", "move")
	k.short = []key.Binding{pair, k.Confirm, k.Back, k.Help, k.ForceQuit}
	k.full = [][]key.Binding{{pair, k.Confirm}, {k.Help, k.Back, k.ForceQuit}}
	return k
}

// reportKeys, exportKeys, membersKeys, filtersKeys and budgetKeys are the
// binding sets for their screens (report.go, export.go, members.go,
// filters.go, budget.go); q DOES quit these screens today.
func reportKeys(d keyDefaults) keyMap {
	k := keyMap{
		Quit: d.Quit, ForceQuit: d.ForceQuit, Help: d.Help, Back: d.Back, GroupBy: d.GroupBy, ChangeRange: d.ChangeRange, Reload: d.Reload,
		Export: d.Export, Rates: d.Rates, LogHours: d.LogHours, Filters: d.Filters,
		Budget: d.Budget, OpenEntries: d.OpenEntries,
	}
	k.short = []key.Binding{k.GroupBy, k.Export, k.Filters, k.Budget, k.Back, k.Help, k.Quit}
	k.full = [][]key.Binding{
		{k.GroupBy, k.Export, k.Rates, k.LogHours},
		{k.Filters, k.Budget, k.OpenEntries},
		{k.ChangeRange, k.Reload, k.Help, k.Back, k.Quit},
	}
	return k
}

func exportKeys(d keyDefaults) keyMap {
	k := keyMap{Quit: d.Quit, ForceQuit: d.ForceQuit, Help: d.Help, Up: d.Up, Down: d.Down, Confirm: d.Confirm, Back: d.Back}
	pair := pairHelp(k.Up, k.Down, "↑/↓/j/k", "move")
	k.short = []key.Binding{pair, k.Confirm, k.Back, k.Help, k.Quit}
	k.full = [][]key.Binding{{pair, k.Confirm}, {k.Help, k.Back, k.Quit}}
	return k
}

func membersKeys(d keyDefaults) keyMap {
	k := keyMap{
		Quit: d.Quit, ForceQuit: d.ForceQuit, Help: d.Help, Up: d.Up, Down: d.Down, ToggleItem: d.ToggleItem,
		SelectAll: d.SelectAll, Confirm: d.Confirm, Back: d.Back,
	}
	pair := pairHelp(k.Up, k.Down, "↑/↓/j/k", "move")
	k.short = []key.Binding{k.ToggleItem, k.SelectAll, k.Confirm, k.Back, k.Help, k.Quit}
	k.full = [][]key.Binding{
		{pair, k.ToggleItem, k.SelectAll},
		{k.Confirm},
		{k.Help, k.Back, k.Quit},
	}
	return k
}

func filtersKeys(d keyDefaults) keyMap {
	k := keyMap{
		Quit: d.Quit, ForceQuit: d.ForceQuit, Help: d.Help, NextField: d.NextField, PrevField: d.PrevField, Up: d.Up, Down: d.Down,
		ToggleItem: d.ToggleItem, SelectAll: d.SelectAll, Confirm: d.Confirm, Back: d.Back,
	}
	// "section", not "field": on this screen tab cycles the four filter
	// sections (filters.go's NextField/PrevField arms move fs.sec), which is
	// what the hand-written line it replaced said too.
	sectionPair := pairHelp(k.NextField, k.PrevField, "tab/⇧tab", "section")
	movePair := pairHelp(k.Up, k.Down, "↑/↓/j/k", "move")
	k.short = []key.Binding{sectionPair, k.ToggleItem, k.Confirm, k.Back, k.Help, k.Quit}
	k.full = [][]key.Binding{
		{sectionPair, movePair, k.ToggleItem, k.SelectAll},
		{k.Confirm},
		{k.Help, k.Back, k.Quit},
	}
	return k
}

func budgetKeys(d keyDefaults) keyMap {
	k := keyMap{Quit: d.Quit, ForceQuit: d.ForceQuit, Help: d.Help, Back: d.Back, Budget: d.Budget}
	// The default help text ("budgets") describes b from the report, where it
	// opens this screen. Here the same key closes it.
	k.Budget.SetHelp("b", "close budgets")
	k.short = []key.Binding{k.Back, k.Budget, k.Help, k.Quit}
	k.full = [][]key.Binding{{k.Budget}, {k.Help, k.Back, k.Quit}}
	return k
}
