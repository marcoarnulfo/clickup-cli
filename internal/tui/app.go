package tui

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
	"github.com/marcoarnulfo/clickup-cli/internal/service"
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
	screenListBrowser
	screenBudget
)

// Async messages.
type (
	entriesMsg  struct{ entries []report.TimeEntry }
	teamsMsg    struct{ teams []clickup.Team }
	membersMsg  struct{ members []clickup.Member }
	statusesMsg struct{ byTask map[string]string }
	errMsg      struct{ err error }

	// retryableErrMsg is a recoverable API error dispatched from a specific
	// origin screen. Unlike errMsg (which always dead-ends on screenError),
	// its handler routes back to origin when it's a screen that knows how to
	// show an inline error (currently only screenHome); other origins fall
	// back to screenError, matching the old errMsg behavior.
	retryableErrMsg struct {
		origin screen
		err    error
	}

	spacesMsg        struct{ spaces []clickup.Space }
	spaceContentsMsg struct {
		spaceID    string
		folders    []clickup.Folder
		folderless []clickup.List
	}
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
	// periodMode overrides preset with the current ISO week when set to
	// periodModeWeek (#4); "" (periodModeMonth) is the default month/preset
	// behavior. Toggled from Home with 'w'.
	periodMode string

	// injectable clock (default: time.Now)
	now func() time.Time

	// loc is the resolved location for range computation and report building
	// (#83): the configured timezone, falling back to time.Local. Set once at
	// New() and re-resolved (with error surfacing) by locOrErr at each report
	// build, mirroring pricingOrErr.
	loc *time.Location

	// data
	report          report.Report
	entries         []report.TimeEntry
	selectedMembers map[int]bool     // selected member ids; empty = all (no filter)
	teamMembers     []clickup.Member // workspace members (session cache)

	// client-side report filter (list/tag/status/billable); empty/nil = no filter
	filterLists    map[string]bool
	filterTags     map[string]bool
	filterStatuses map[string]bool
	filterBillable *bool             // nil = no constraint; see report.FilterCriteria.Billable
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
	budgetScreen  budgetModel

	// shared Space→Folder→List browser (log/rates entry points)
	browserScreen   listBrowserModel
	browserSpaces   []clickup.Space
	browserContents map[string]browserSpaceContents
}

// New builds the root model from the config.
func New(cfg config.Config) Model {
	demo := demoEnabled()
	if demo {
		cfg = demoConfig()
	}
	m := Model{
		cfg:    cfg,
		demo:   demo,
		scope:  "me",
		preset: report.PresetThisMonth,
		client: clickup.New(cfg.Token),
		now:    time.Now,
	}
	// Best-effort default so range/label display works before the first report
	// build; a genuinely invalid configured zone is caught and surfaced by
	// locOrErr the first time a report is actually built (see #83).
	m.loc, _ = service.LoadLocation(cfg.Timezone, time.Local)
	t := m.now()
	m.year, m.month = t.Year(), t.Month()
	if m.demo || m.cfg.Valid() {
		m.screen = screenHome
		m.home = newHome()
	} else {
		m.screen = screenSetup
		m.setup = newSetup()
	}
	return m
}

func (m Model) Init() tea.Cmd { return nil }

// currentRange returns the [start, end) period the report should cover.
// periodMode == periodModeWeek overrides everything else with the current
// ISO week, derived from the injected clock (m.now()) and the Model's single
// resolved location (m.loc) — never time.Now() and never a second location
// (see the task's binding note on the week toggle). Otherwise it follows the
// active preset (custom uses the inclusive customStart..customEnd).
func (m Model) currentRange() (start, end time.Time) {
	if m.periodMode == periodModeWeek {
		loc := m.loc
		if loc == nil { // same nil-means-UTC convention as the report package (#83)
			loc = time.UTC
		}
		isoYear, isoWeek := m.now().In(loc).ISOWeek()
		return report.WeekRange(isoYear, isoWeek, loc)
	}
	if m.preset == report.PresetCustom {
		return report.CustomRange(m.customStart, m.customEnd, m.loc)
	}
	return report.RangeForPreset(m.preset, m.year, m.month, m.now(), m.loc)
}

// reloadEntriesCmd picks the source for time entries: demo data (no I/O)
// in demo mode, otherwise the real API call. origin identifies the screen
// that dispatched the load, so a failure can be routed back there (see
// retryableErrMsg); demoEntriesCmd never fails, so it doesn't need it.
func (m Model) reloadEntriesCmd(origin screen) tea.Cmd {
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
	return loadEntriesCmd(m.client, m.cfg.WorkspaceID, start, end, m.scope, assignees, origin)
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

// pricingOrErr builds report.Pricing from config via the shared
// service.PricingFromConfig. On error (an unparseable billing.rounding
// increment, see #57) it routes to screenError exactly like the errMsg case
// in Update, and ok is false so the caller must skip the report rebuild.
func (m *Model) pricingOrErr() (report.Pricing, bool) {
	p, err := service.PricingFromConfig(m.cfg)
	if err != nil {
		m.err = err
		m.screen = screenError
		return report.Pricing{}, false
	}
	return p, true
}

// locOrErr resolves and (re-)caches the TUI's location — the configured
// timezone, falling back to time.Local — and mirrors pricingOrErr: an
// invalid configured zone routes to screenError instead of silently falling
// back. Call it right before currentRange/report.Build at every report-build
// site (#83): a range computed in one zone and a report built in another
// would mis-assign entries at day boundaries.
func (m *Model) locOrErr() (*time.Location, bool) {
	loc, err := service.LoadLocation(m.cfg.Timezone, time.Local)
	if err != nil {
		m.err = err
		m.screen = screenError
		return nil, false
	}
	m.loc = loc
	return loc, true
}

// filterCriteria assembles the active client-side filter from session state.
func (m Model) filterCriteria() report.FilterCriteria {
	return report.FilterCriteria{
		Lists: m.filterLists, Tags: m.filterTags, Statuses: m.filterStatuses,
		Billable: m.filterBillable,
	}
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

// loadEntriesCmd calls the report I/O pipeline (internal/service) in the
// background and returns entriesMsg or retryableErrMsg{origin, err}. For scope
// "team" with an empty assignees slice it derives ALL workspace members (via
// TeamMembers) and filters on them; a non-empty assignees slice is used as-is
// (skipping the members lookup). For scope "me" no assignee filter is applied.
func loadEntriesCmd(c *clickup.Client, teamID string, start, end time.Time, scope string, assignees []int, origin screen) tea.Cmd {
	return func() tea.Msg {
		// 60s (raised from 30s): under the rate limiter a report spanning many
		// lists spends real time in ListNames enrichment waits.
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		entries, err := service.LoadEntries(ctx, c, teamID, start, end, scope, assignees)
		if err != nil {
			return retryableErrMsg{origin: origin, err: err}
		}
		return entriesMsg{entries: entries}
	}
}

// statusEnrichConcurrency bounds how many /task/{id} lookups statusEnrichCmd
// runs at once, mirroring clickup.Client.ListNames' pattern.
const statusEnrichConcurrency = 8

// statusEnrichCmd fetches the current status of each task id, in parallel
// (bounded concurrency), and returns them as a statusesMsg. A single
// non-retrievable task (deleted, no permission, rate-limited, …) must not
// brick the Filters screen for the whole session: per the spec, its status
// resolves to "" and enrichment continues with the rest. An unauthorized
// token is the one failure worth surfacing as errMsg, since it means the
// token itself needs re-entering via the setup wizard; on the first such
// error the derived context is canceled and the partial byTask is discarded.
func statusEnrichCmd(c *clickup.Client, taskIDs []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		byTask := make(map[string]string, len(taskIDs))
		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, statusEnrichConcurrency)

		var authErrOnce sync.Once
		var authErr error

		for _, id := range taskIDs {
			wg.Add(1)
			sem <- struct{}{}
			go func(id string) {
				defer wg.Done()
				defer func() { <-sem }()

				st, err := c.TaskStatus(ctx, id)
				if err != nil {
					if errors.Is(err, clickup.ErrUnauthorized) {
						authErrOnce.Do(func() {
							authErr = err
							cancel() // stop further/in-flight lookups
						})
						return
					}
					mu.Lock()
					byTask[id] = "" // non-retrievable: cache as resolved-empty, don't retry within this load
					mu.Unlock()
					return
				}
				mu.Lock()
				byTask[id] = st
				mu.Unlock()
			}(id)
		}
		wg.Wait()

		if authErr != nil {
			return errMsg{err: authErr}
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

// pruneFilters intersects each of filterLists/filterTags/filterStatuses with
// the values actually present in m.entries, dropping any selection whose
// value no longer occurs (e.g. after a range change swaps in a different set
// of entries). Without this, a stale selection silently filters the report
// down to nothing with no way to clear it from the Filters screen.
func (m *Model) pruneFilters() {
	lists := map[string]bool{}
	tags := map[string]bool{}
	statuses := map[string]bool{}
	for _, e := range m.entries {
		if e.ListName != "" {
			lists[e.ListName] = true
		}
		for _, t := range e.Tags {
			tags[t] = true
		}
		if e.Status != "" {
			statuses[e.Status] = true
		}
	}
	m.filterLists = pruneFilterSet(m.filterLists, lists)
	m.filterTags = pruneFilterSet(m.filterTags, tags)
	m.filterStatuses = pruneFilterSet(m.filterStatuses, statuses)
}

// pruneFilterSet keeps only the selected (true) entries of sel whose key is
// present in the current set, dropping stale keys and any lingering false ones.
func pruneFilterSet(sel, present map[string]bool) map[string]bool {
	if len(sel) == 0 {
		return sel
	}
	out := make(map[string]bool, len(sel))
	for k, v := range sel {
		if v && present[k] {
			out[k] = v
		}
	}
	return out
}

// loadMembersCmd fetches the workspace members in the background and returns
// membersMsg or retryableErrMsg{origin, err}. It's Home-only today, so origin
// is always screenHome at the call site.
func loadMembersCmd(c *clickup.Client, teamID string, origin screen) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		members, err := c.TeamMembers(ctx, teamID)
		if err != nil {
			return retryableErrMsg{origin: origin, err: err}
		}
		return membersMsg{members: members}
	}
}

// spacesCmd / spaceContentsCmd pick the demo or real source.
func (m Model) spacesCmd() tea.Cmd {
	if m.demo {
		return demoSpacesCmd()
	}
	return loadSpacesCmd(m.client, m.cfg.WorkspaceID)
}

func (m Model) spaceContentsCmd(spaceID string) tea.Cmd {
	if m.demo {
		return demoSpaceContentsCmd(spaceID)
	}
	return loadSpaceContentsCmd(m.client, spaceID)
}

// openListBrowser opens the shared list browser on behalf of `origin`.
func (m Model) openListBrowser(origin screen) (Model, tea.Cmd) {
	bs := listBrowserModel{origin: origin}
	m.screen = screenListBrowser
	if len(m.browserSpaces) > 0 {
		bs.spaces = m.browserSpaces
		m.browserScreen = bs
		return m, nil
	}
	bs.loading = true
	m.browserScreen = bs
	return m, m.spacesCmd()
}

// loadSpacesCmd / loadSpaceContentsCmd fetch in the background.
func loadSpacesCmd(c *clickup.Client, teamID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		spaces, err := c.Spaces(ctx, teamID)
		if err != nil {
			return errMsg{err: err}
		}
		return spacesMsg{spaces: spaces}
	}
}

func loadSpaceContentsCmd(c *clickup.Client, spaceID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		folders, folderless, err := c.SpaceContents(ctx, spaceID)
		if err != nil {
			return errMsg{err: err}
		}
		return spaceContentsMsg{spaceID: spaceID, folders: folders, folderless: folderless}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "q" && m.screen != screenSetup && m.screen != screenRates && m.screen != screenRange && m.screen != screenListBrowser {
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

	case retryableErrMsg:
		m.err = msg.err
		if errors.Is(msg.err, clickup.ErrUnauthorized) {
			m.screen = screenSetup
			m.setup = newSetup()
			return m, nil
		}
		switch msg.origin {
		case screenHome:
			m.home.errText = "Error: " + msg.err.Error()
			m.screen = screenHome
		default:
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
		m.assignStatuses() // re-stamp session-cached statuses onto the freshly loaded entries
		m.pruneFilters()   // drop filter selections whose value no longer occurs in the new entries
		groupBy := m.report.GroupBy
		if groupBy == "" {
			groupBy = report.GroupByTotal // first load: summary of the month
		}
		if groupBy == report.GroupByMember && m.scope != "team" {
			// member grouping is team-only: never let it leak into a "me" report.
			groupBy = report.GroupByTotal
		}
		if _, ok := m.locOrErr(); !ok {
			return m, nil
		}
		start, end := m.currentRange()
		p, ok := m.pricingOrErr()
		if !ok {
			return m, nil
		}
		m.report = report.Build(m.visibleEntries(), groupBy, p, start, end, m.loc)
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
		m.filtersScreen = newFilters(m.entries, m.filterLists, m.filterTags, m.filterStatuses, m.filterBillable)
		m.screen = screenFilters
		return m, nil

	case spacesMsg:
		m.browserSpaces = msg.spaces
		if m.screen != screenListBrowser {
			return m, nil // stale: the user navigated away while spaces loaded
		}
		bs := m.browserScreen
		bs.spaces = msg.spaces
		bs.loading = false
		bs.level = browseSpaces
		bs.idx = 0
		m.browserScreen = bs
		return m, nil

	case spaceContentsMsg:
		if m.browserContents == nil {
			m.browserContents = map[string]browserSpaceContents{}
		}
		m.browserContents[msg.spaceID] = browserSpaceContents{folders: msg.folders, folderless: msg.folderless}
		bs := m.browserScreen
		if bs.spaceID == msg.spaceID {
			bs.folders = msg.folders
			bs.folderless = msg.folderless
			bs.loading = false
			bs.level = browseSpaceContents
			bs.idx = 0
			m.browserScreen = bs
		}
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
	case screenListBrowser:
		return m.updateListBrowser(msg)
	case screenBudget:
		return m.updateBudget(msg)
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
	case screenListBrowser:
		return m.browserScreen.view()
	case screenBudget:
		return m.budgetScreen.view()
	case screenError:
		return styleErr.Render("Error: ") + m.err.Error() + "\n\n" + styleHelp.Render("press a key to return home")
	}
	return ""
}
