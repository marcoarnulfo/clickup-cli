package tui

import tea "github.com/charmbracelet/bubbletea"

// The screen-opening surface: every method here builds a screen's sub-model and
// navigates to it, so a key handler and the command palette (#71) open a screen
// the same way instead of each knowing how.
//
// Three older members of this family live elsewhere and stay there — moving
// them would inflate a diff for no behavioural gain: openListBrowser in app.go,
// openEntries in entries.go, openBudgetView in report.go.

// openExport builds the export screen for the current report.
func (m Model) openExport() Model {
	m.export = newExport(m.report)
	return m.goTo(screenExport)
}

// openRates builds the per-list/per-member rates editor.
func (m Model) openRates() Model {
	m.ratesScreen = newRates(m.entries, m.cfg)
	return m.goTo(screenRates)
}

// openLog builds the hour-logging flow at its first step.
func (m Model) openLog() Model {
	m.logScreen = newLog(m.entries, m.cfg)
	return m.goTo(screenLog)
}

// openRange builds the range/preset picker.
func (m Model) openRange() Model {
	m.rangeScreen = newRange(m.preset)
	return m.goTo(screenRange)
}

// openFilters opens the client-side filter screen, enriching task statuses
// first when any are missing. The demo branch keeps the zero-I/O rule.
func (m Model) openFilters() (Model, tea.Cmd) {
	missing := m.tasksMissingStatus()
	if len(missing) == 0 {
		m.assignStatuses()
		m.filtersScreen = newFilters(m.entries, m.filterLists, m.filterTags, m.filterStatuses, m.filterBillable)
		return m.goTo(screenFilters), nil
	}
	m.filtersScreen = filtersModel{loadingStatuses: true}
	m = m.goTo(screenFilters)
	if m.demo {
		return m, demoStatusEnrichCmd(m.entries)
	}
	return m, statusEnrichCmd(m.client, missing)
}

// openMembers opens the member selector, fetching the workspace roster when it
// is not cached yet.
//
// origin is where a load failure returns to. It is a parameter rather than a
// constant screenHome because the command palette can open this screen from
// anywhere: attributing a failure to Home while the user was on Rates would
// both lose their place and misname the culprit.
func (m Model) openMembers(origin screen) (Model, tea.Cmd) {
	if len(m.teamMembers) > 0 {
		m.membersScreen = newMembers(m.teamMembers, m.selectedMembers)
		return m.goTo(screenMembers), nil
	}
	m.membersScreen = membersModel{loading: true}
	m = m.goTo(screenMembers)
	if m.demo {
		return m, demoMembersCmd()
	}
	return m, loadMembersCmd(m.client, m.cfg.WorkspaceID, origin)
}
