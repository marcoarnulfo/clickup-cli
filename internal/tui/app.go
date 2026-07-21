package tui

import (
	"context"
	"errors"
	"slices"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

type screen int

const (
	screenSetup screen = iota
	screenHome
	screenLoading
	screenReport
	screenExport
	screenRates
	screenLog
	screenError
	screenMembers
	screenRange
	screenFilters
)

// Async messages.
type (
	entriesMsg  struct{ entries []report.TimeEntry }
	teamsMsg    struct{ teams []clickup.Team }
	membersMsg  struct{ members []clickup.Member }
	statusesMsg struct{ byTask map[string]string }
	errMsg      struct{ err error }
)

// Model is the root model of the TUI.
type Model struct {
	cfg    config.Config
	client *clickup.Client
	demo   bool // demo mode (fake data, no API)
	screen screen
	err    error

	width, height int

	// current selection
	year        int
	month       time.Month
	scope       string    // "me" | "team"
	preset      string    // report.Preset* ; default report.PresetThisMonth
	customStart time.Time // used when preset == report.PresetCustom
	customEnd   time.Time

	// data
	report          report.Report
	entries         []report.TimeEntry
	selectedMembers map[int]bool     // selected member ids; empty = all (no filter)
	teamMembers     []clickup.Member // workspace members (session cache)

	// client-side report filter (list/tag/status); empty = no filter
	filterLists    map[string]bool
	filterTags     map[string]bool
	filterStatuses map[string]bool
	taskStatus     map[string]string // task id -> current status (session cache)

	// sub-models
	setup         setupModel
	home          homeModel
	rep           reportModel
	export        exportModel
	ratesScreen   ratesModel
	logScreen     logModel
	membersScreen membersModel
	rangeScreen   rangeModel
	filtersScreen filtersModel
}

// New builds the root model from the config.
func New(cfg config.Config) Model {
	now := time.Now()
	demo := demoEnabled()
	if demo {
		cfg = demoConfig()
	}
	m := Model{
		cfg:    cfg,
		demo:   demo,
		year:   now.Year(),
		month:  now.Month(),
		scope:  "me",
		preset: report.PresetThisMonth,
		client: clickup.New(cfg.Token),
	}
	if demo || cfg.Valid() {
		m.screen = screenHome
		m.home = newHome()
	} else {
		m.screen = screenSetup
		m.setup = newSetup()
	}
	return m
}

func (m Model) Init() tea.Cmd { return nil }

// currentRange returns the [start, end) period the report should cover, from the
// active preset (custom uses the inclusive customStart..customEnd).
func (m Model) currentRange() (start, end time.Time) {
	if m.preset == report.PresetCustom {
		return m.customStart, m.customEnd.AddDate(0, 0, 1)
	}
	return report.RangeForPreset(m.preset, m.year, m.month, time.Now())
}

// reloadEntriesCmd picks the source for time entries: demo data (no I/O)
// in demo mode, otherwise the real API call.
func (m Model) reloadEntriesCmd() tea.Cmd {
	// The member filter is a team-scope concept; never carry a stale
	// selection into a "me" load.
	var assignees []int
	if m.scope == "team" {
		assignees = m.selectedAssignees()
	}
	start, end := m.currentRange()
	if m.demo {
		if m.scope != "team" {
			// The real API filters "me" scope server-side to the authenticated
			// caller; mirror that here instead of summing all demo users.
			assignees = []int{demoSelfID}
		}
		return demoEntriesCmd(start, end, assignees)
	}
	return loadEntriesCmd(m.client, m.cfg.WorkspaceID, start, end, m.scope, assignees)
}

// selectedAssignees returns the ids of the currently selected members, sorted.
// An empty result means "no member filter" (all members).
func (m Model) selectedAssignees() []int {
	var ids []int
	for id, on := range m.selectedMembers {
		if on {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// ratesFromConfig builds the report rates from config (default + overrides).
func ratesFromConfig(cfg config.Config) report.Rates {
	return report.Rates{Default: cfg.Rate, ByList: cfg.Rates}
}

// filterCriteria assembles the active client-side filter from session state.
func (m Model) filterCriteria() report.FilterCriteria {
	return report.FilterCriteria{Lists: m.filterLists, Tags: m.filterTags, Statuses: m.filterStatuses}
}

// visibleEntries applies the active filter to the loaded entries.
func (m Model) visibleEntries() []report.TimeEntry {
	return report.Filter(m.entries, m.filterCriteria())
}

// filteredNote returns " · filtered" when any client-side filter is active.
func (m Model) filteredNote() string {
	if m.filterCriteria().Empty() {
		return ""
	}
	return " · filtered"
}

// loadEntriesCmd calls the API in the background and returns entriesMsg or errMsg.
// For scope "team" with an empty assignees slice it derives ALL workspace members
// (via TeamMembers) and filters on them; a non-empty assignees slice is used as-is
// (skipping the members lookup). For scope "me" no assignee filter is applied.
func loadEntriesCmd(c *clickup.Client, teamID string, start, end time.Time, scope string, assignees []int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if scope == "team" && len(assignees) == 0 {
			members, err := c.TeamMembers(ctx, teamID)
			if err != nil {
				return errMsg{err: err}
			}
			for _, mem := range members {
				assignees = append(assignees, mem.ID)
			}
		}

		entries, err := c.TimeEntries(ctx, teamID, start, end, assignees)
		if err != nil {
			return errMsg{err: err}
		}
		// Resolve human-readable list names ONCE per unique list_id
		// (avoids repeated calls, including failed ones, for the same list).
		resolved := map[string]string{}
		for _, e := range entries {
			if e.ListID == "" {
				continue
			}
			if _, done := resolved[e.ListID]; done {
				continue
			}
			if name, err := c.ListName(ctx, e.ListID); err == nil {
				resolved[e.ListID] = name
			} else {
				resolved[e.ListID] = "" // attempted: don't retry within this load
			}
		}
		for i := range entries {
			if name := resolved[entries[i].ListID]; name != "" {
				entries[i].ListName = name
			}
		}
		return entriesMsg{entries: entries}
	}
}

// statusEnrichCmd fetches the current status of each task id and returns them
// as a statusesMsg (or errMsg on the first failure).
func statusEnrichCmd(c *clickup.Client, taskIDs []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		byTask := make(map[string]string, len(taskIDs))
		for _, id := range taskIDs {
			st, err := c.TaskStatus(ctx, id)
			if err != nil {
				return errMsg{err: err}
			}
			byTask[id] = st
		}
		return statusesMsg{byTask: byTask}
	}
}

// tasksMissingStatus returns the distinct task ids of loaded entries whose status
// is not yet cached.
func (m Model) tasksMissingStatus() []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range m.entries {
		if e.TaskID == "" || seen[e.TaskID] {
			continue
		}
		seen[e.TaskID] = true
		if _, ok := m.taskStatus[e.TaskID]; !ok {
			out = append(out, e.TaskID)
		}
	}
	return out
}

// assignStatuses copies cached statuses onto the loaded entries.
func (m *Model) assignStatuses() {
	for i := range m.entries {
		if st, ok := m.taskStatus[m.entries[i].TaskID]; ok {
			m.entries[i].Status = st
		}
	}
}

// loadMembersCmd fetches the workspace members in the background and returns
// membersMsg or errMsg.
func loadMembersCmd(c *clickup.Client, teamID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		members, err := c.TeamMembers(ctx, teamID)
		if err != nil {
			return errMsg{err: err}
		}
		return membersMsg{members: members}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "q" && m.screen != screenSetup && m.screen != screenRates {
			return m, tea.Quit
		}
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		return m.routeKey(msg)

	case errMsg:
		m.err = msg.err
		// Invalid/revoked token: relaunch the setup wizard (spec §8).
		if errors.Is(msg.err, clickup.ErrUnauthorized) {
			m.screen = screenSetup
			m.setup = newSetup()
		} else {
			m.screen = screenError
		}
		return m, nil

	case logErrMsg:
		// Log-flow error: stay on the log screen with the message, so the
		// filled form / selected task is not lost and the user can retry.
		m.logScreen.loading = false
		m.logScreen.msg = "Error: " + msg.err.Error()
		m.screen = screenLog
		return m, nil

	case entriesMsg:
		m.entries = msg.entries
		groupBy := m.report.GroupBy
		if groupBy == "" {
			groupBy = report.GroupByTotal // first load: summary of the month
		}
		if groupBy == report.GroupByMember && m.scope != "team" {
			// member grouping is team-only: never let it leak into a "me" report.
			groupBy = report.GroupByTotal
		}
		start, end := m.currentRange()
		m.report = report.Build(m.visibleEntries(), groupBy, ratesFromConfig(m.cfg), m.cfg.Currency, start, end)
		m.report.Scope = m.scope
		m.rep = newReport(m.report, m.memberFilterNote()+m.filteredNote())
		m.screen = screenReport
		return m, nil

	case teamsMsg:
		// delivered to setup for workspace selection
		var cmd tea.Cmd
		m.setup, cmd = m.setup.withTeams(msg.teams)
		return m, cmd

	case logDoneMsg:
		m.logScreen.step = logDone
		m.logScreen.msg = msg.summary
		m.screen = screenLog
		return m, nil

	case taskListMsg:
		m.logScreen.tasks = msg.tasks
		m.logScreen.taskIdx = 0
		m.logScreen.loading = false
		m.logScreen.step = logTaskPick
		return m, nil

	case timerMsg:
		if m.screen != screenLog && m.screen != screenLoading {
			return m, nil // stale timer message: the user left the screen
		}
		m.logScreen.timer = msg.timer
		if msg.timer != nil {
			m.logScreen.step = logTimerRunning
		}
		m.screen = screenLog
		return m, nil

	case membersMsg:
		m.teamMembers = msg.members
		if len(m.selectedMembers) == 0 {
			m.selectedMembers = make(map[int]bool, len(msg.members))
			for _, mem := range msg.members {
				m.selectedMembers[mem.ID] = true // default: all selected
			}
		}
		m.membersScreen = newMembers(msg.members, m.selectedMembers)
		m.screen = screenMembers
		return m, nil

	case statusesMsg:
		if m.taskStatus == nil {
			m.taskStatus = map[string]string{}
		}
		for id, st := range msg.byTask {
			m.taskStatus[id] = st
		}
		m.assignStatuses()
		m.filtersScreen = newFilters(m.entries, m.filterLists, m.filterTags, m.filterStatuses)
		m.screen = screenFilters
		return m, nil
	}
	return m, nil
}

// routeKey forwards keys to the active screen.
func (m Model) routeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenSetup:
		return m.updateSetup(msg)
	case screenHome:
		return m.updateHome(msg)
	case screenReport:
		return m.updateReport(msg)
	case screenExport:
		return m.updateExport(msg)
	case screenRates:
		return m.updateRates(msg)
	case screenLog:
		return m.updateLog(msg)
	case screenMembers:
		return m.updateMembers(msg)
	case screenRange:
		return m.updateRange(msg)
	case screenFilters:
		return m.updateFilters(msg)
	case screenError:
		if !m.cfg.Valid() {
			m.screen = screenSetup
			m.setup = newSetup()
		} else {
			m.screen = screenHome
		}
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	switch m.screen {
	case screenSetup:
		return m.setup.view()
	case screenHome:
		return m.home.view(m.rangeLabel(), m.scope, m.homeMembersNote())
	case screenLoading:
		return styleTitle.Render("Loading hours…")
	case screenReport:
		return m.rep.view()
	case screenExport:
		return m.export.view()
	case screenRates:
		return m.ratesScreen.view()
	case screenLog:
		return m.logScreen.view()
	case screenMembers:
		return m.membersScreen.view()
	case screenRange:
		return m.rangeScreen.view()
	case screenFilters:
		return m.filtersScreen.view()
	case screenError:
		return styleErr.Render("Error: ") + m.err.Error() + "\n\n" + styleHelp.Render("press a key to return home")
	}
	return ""
}
