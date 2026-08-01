package tui

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
	"github.com/marcoarnulfo/clickup-cli/internal/service"
	"github.com/marcoarnulfo/clickup-cli/internal/themes"
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
	screenEntries
)

// overlayKind is the floating layer drawn over the current screen, orthogonal
// to m.screen: opening one does not touch m.nav and closing one is not a pop().
// An overlay is not a place you navigated to.
//
// There are two values because there is one client. The third value arrives
// with the third client, not before.
type overlayKind int

const (
	overlayNone overlayKind = iota
	overlayPalette
)

// Async messages.
type (
	entriesMsg struct {
		entries    []report.TimeEntry
		start, end time.Time
	}
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

	// historyMsg carries a time entry's change history (Task 8), delivered by
	// historyCmd and rendered by the read-only entriesHistory browser mode.
	historyMsg struct{ changes []clickup.HistoryChange }

	spacesMsg        struct{ spaces []clickup.Space }
	spaceContentsMsg struct {
		spaceID    string
		folders    []clickup.Folder
		folderless []clickup.List
	}

	// updateAvailableMsg carries a newer published release. It is only ever
	// sent when one exists: unlike every other command in this program, the
	// update check never reports its failures — it emits no errMsg and never
	// routes to screenError, because a failed update check is not the user's
	// problem.
	updateAvailableMsg struct{ latest string }
)

// Model is the root model of the TUI.
type Model struct {
	cfg    config.Config
	client *clickup.Client
	demo   bool // demo mode (fake data, no API)
	screen screen
	// nav is the parent chain: the screen to return to on pop(), and the one
	// above it, and so on. The current screen is m.screen, not the top of
	// nav. An empty nav means "nowhere to go back to" (Home lives here).
	nav []screen
	err error

	// theme carries every style the views render through (#54). It is passed
	// explicitly to each view rather than read from package state, so a view
	// can never render with an unset theme.
	theme theme
	// keys is the resolved binding table. The zero value means the built-in
	// defaults, so a Model built by hand in a test routes normally.
	keys KeyTable

	// latestVersion is the newer published release, "" when up to date or
	// unknown (the check hasn't returned yet, is disabled, or failed silently).
	latestVersion string

	width, height int

	// helpAll toggles the footer between short and full help (#69 Task 4).
	// Flipped by '?' wherever keysFor(m).Help is enabled for the current
	// screen; nothing renders it yet — Task 5 wires the footer into View().
	helpAll bool

	// overlay is the floating layer over m.screen, and palette is its state
	// when overlay == overlayPalette (#71). While an overlay is open it owns
	// the keyboard: see Update's tea.KeyMsg branch.
	overlay overlayKind
	palette paletteModel

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

	// loadedStart/loadedEnd are the range the currently loaded entries were
	// fetched for, pinned from the pair the load itself resolved (#28). Zero
	// means nothing has been loaded yet.
	loadedStart, loadedEnd time.Time

	// injectable clock (default: time.Now)
	now func() time.Time

	// live timer (#91): the running timer surfaced globally on Home. ticking
	// guards against arming a second 1s tick chain; tickCount paces the periodic
	// re-poll. userID is the authenticated user (ownership gating, #94/#98).
	runningTimer *clickup.RunningTimer
	ticking      bool
	tickCount    int
	userID       int

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

	// demo-only session state for the entries browser (#98/#99): real mode
	// never allocates these, so a nil-map read is always false/absent (safe).
	demoDeleted   map[string]bool             // ids deleted this session, hidden from every demo reload
	demoOverrides map[string]report.TimeEntry // ids edited this session (Task 7), replacing the fixture value

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
	entriesScreen entriesModel

	// shared Space→Folder→List browser (log/rates entry points)
	browserScreen   listBrowserModel
	browserSpaces   []clickup.Space
	browserContents map[string]browserSpaceContents
}

// New builds the root model from the config and the already-resolved palette
// and bindings it uses. Resolving them is the caller's job (internal/cli's
// resolveTheme and resolveKeys): by the time New runs, invalid configuration
// has already been turned into a startup error, so New itself never fails on it.
func New(cfg config.Config, pal themes.Palette, kt KeyTable) Model {
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
		theme:  newTheme(lipgloss.DefaultRenderer(), pal),
		keys:   kt,
	}
	// Best-effort default so range/label display works before the first report
	// build; a genuinely invalid configured zone is caught and surfaced by
	// locOrErr the first time a report is actually built (see #83).
	m.loc, _ = service.LoadLocation(cfg.Timezone, time.Local)
	m.year, m.month = defaultYearMonth(m.now(), m.loc)
	if m.demo || m.cfg.Valid() {
		m = m.resetTo(screenHome)
		m.home = newHome()
	} else {
		m = m.resetTo(screenSetup)
		m.setup = newSetup()
	}
	return m
}

// defaultYearMonth picks the year/month a newly-constructed Model should
// default to, deriving it from now resolved into loc rather than now's own
// location. Without this, a configured non-local timezone (m.loc) would be
// ignored for exactly this one calendar pick: a user in Rome with
// timezone: Pacific/Auckland configured would get the wrong default month
// for a few hours a day, even though m.loc is resolved one line above in New.
// loc == nil (an invalid configured zone, surfaced later by locOrErr) is
// treated as UTC, the same nil-means-UTC convention used throughout
// internal/report and by currentRange's week branch.
func defaultYearMonth(now time.Time, loc *time.Location) (int, time.Month) {
	if loc == nil {
		loc = time.UTC
	}
	t := now.In(loc)
	return t.Year(), t.Month()
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.updateCheckCmd(), m.runningTimerProbeCmd(), m.currentUserCmd())
}

// updateCheckCmd checks GitHub for a newer release in the background and
// returns updateAvailableMsg when one exists. It returns nil (issuing no
// command at all) when the check is disabled or, per the demo's zero-I/O
// rule, whenever m.demo is set. See updateAvailableMsg for why this command
// never reports failure.
func (m Model) updateCheckCmd() tea.Cmd {
	if !service.UpdateCheckEnabled(m.cfg, m.demo) {
		return nil
	}
	return func() tea.Msg {
		latest, newer := service.CheckForUpdate(context.Background(), service.UpdateOptions{
			Current: service.CurrentVersion(),
		})
		if !newer {
			return nil
		}
		return updateAvailableMsg{latest: latest}
	}
}

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

// activeRange is the range the LOADED entries actually cover: the pinned pair
// when a load has happened, else what the next load would use.
//
// The split matters because a relative preset moves under you. Every rebuild
// over already-loaded entries goes through here, so it describes its own data.
// Every surface that describes the NEXT load — Home's label, and the loads
// themselves — uses currentRange() instead: Home changes month, week mode and
// preset WITHOUT reloading, so a pinned label would freeze while the user
// navigates.
func (m Model) activeRange() (start, end time.Time) {
	if !m.loadedStart.IsZero() {
		return m.loadedStart, m.loadedEnd
	}
	return m.currentRange()
}

// reloadEntriesCmd picks the source for time entries: demo data (no I/O)
// in demo mode, otherwise the real API call. origin identifies the screen
// that dispatched the load, so a failure can be routed back there (see
// retryableErrMsg); demoEntriesCmd never fails, so it doesn't need it.
func (m Model) reloadEntriesCmd(origin screen) tea.Cmd {
	assignees := m.reloadAssignees()
	start, end := m.currentRange()
	if m.demo {
		return m.demoEntriesCmd(start, end, assignees)
	}
	return loadEntriesCmd(m.client, m.cfg.WorkspaceID, start, end, m.scope, assignees, origin)
}

// reloadAssignees is the assignee set for a reload: team scope uses the member
// selection; demo me-scope mirrors the server-side "me" filter with
// demoSelfID; real me-scope returns nil (the API filters server-side). This is
// the single derivation shared by reloadEntriesCmd and the browser's
// reloadForBrowser (entries.go), so a browser reload never disagrees with an
// ordinary report reload about which entries are in scope.
func (m Model) reloadAssignees() []int {
	if m.scope == "team" {
		return m.selectedAssignees()
	}
	if m.demo {
		return []int{demoSelfID}
	}
	return nil
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
		*m = m.replace(screenError)
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
		*m = m.replace(screenError)
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
		return entriesMsg{entries: entries, start: start, end: end}
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
// membersMsg or retryableErrMsg{origin, err}. origin is the screen to return
// a failure to — Home for the key binding, the caller's own screen when the
// command palette opens it.
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

// openListBrowser opens the shared list browser on top of the current screen
// (Rates or Log); pop() returns to whichever one pushed it.
func (m Model) openListBrowser() (Model, tea.Cmd) {
	bs := listBrowserModel{}
	m = m.goTo(screenListBrowser)
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

	case runningTimerMsg:
		if msg.failed {
			// A transient probe failure is not evidence the timer stopped: keep
			// the current indicator/tick chain untouched and let the next
			// scheduled re-poll (or the boot probe, where there is nothing to
			// keep) try again.
			return m, nil
		}
		m.runningTimer = msg.timer
		if msg.timer != nil && !m.ticking {
			m.ticking = true
			return m, tickCmd() // arm exactly one chain on nil -> non-nil
		}
		if msg.timer == nil {
			m.ticking = false // let any in-flight tick chain die on its next fire
		}
		return m, nil

	case userMsg:
		m.userID = msg.id
		return m, nil

	case tickMsg:
		if m.runningTimer == nil {
			m.ticking = false
			return m, nil // no timer: stop the chain
		}
		m.tickCount++
		if m.tickCount%repollTickInterval == 0 && !m.demo {
			// periodic re-poll (real mode only: re-issuing the demo probe would
			// reset the fake Start and make the demo stopwatch sawtooth).
			return m, tea.Batch(tickCmd(), m.runningTimerProbeCmd())
		}
		return m, tickCmd()

	case tea.KeyMsg:
		// ForceQuit first and unconditionally: with an overlay open it is the
		// only way out that nothing else can intercept.
		if key.Matches(msg, defaultKeys().ForceQuit) {
			return m, tea.Quit
		}
		// An open overlay owns the keyboard, and this check MUST stay above
		// Palette: below it, ctrl+p would call openPalette a second time and
		// wipe the query instead of closing.
		// TestPaletteCtrlPClosesRatherThanReopening pins the ordering.
		if m.overlay != overlayNone {
			return m.updateOverlay(msg)
		}
		if key.Matches(msg, keysFor(m).Quit) {
			return m, tea.Quit
		}
		// Checked here, beside Quit/ForceQuit, rather than inside routeKey: that
		// makes '?' behave identically on every screen keysFor enables it for,
		// including screenLoading, which routeKey has no case for at all.
		// keysFor(m).Help is already unassigned (a no-op key.Binding, so
		// key.Matches never fires) on every screen where '?' must mean
		// something else — the ten textinput-forwarding contexts, screenError
		// (any key -> Home), and entriesConfirmDelete (any key but y cancels).
		if key.Matches(msg, keysFor(m).Help) {
			m.helpAll = !m.helpAll
			return m, nil
		}
		if key.Matches(msg, keysFor(m).Palette) {
			return m.openPalette(), nil
		}
		return m.routeKey(msg)

	case tea.MouseMsg:
		k, ok := wheelKey(msg)
		if !ok {
			return m, nil
		}
		// The global bindings the KeyMsg case checks first — ForceQuit, Quit,
		// Help, Palette — are all keystrokes, so a wheel notch cannot match
		// any of them and they are skipped. The overlay check is not optional:
		// with the palette open the wheel must move its selection, not scroll
		// the screen behind it. It comes before the gate below because an
		// overlay owns the wheel the way it owns the keyboard, whatever screen
		// is underneath.
		if m.overlay != overlayNone {
			return m.updateOverlay(k)
		}
		if m.anyKeyIsAnAnswer() {
			return m, nil
		}
		return m.routeKey(k)

	case errMsg:
		m.err = msg.err
		// Invalid/revoked token: relaunch the setup wizard (spec §8).
		if errors.Is(msg.err, clickup.ErrUnauthorized) {
			m = m.resetTo(screenSetup)
			m.setup = newSetup()
		} else {
			m = m.replace(screenError)
		}
		return m, nil

	case retryableErrMsg:
		m.err = msg.err
		if errors.Is(msg.err, clickup.ErrUnauthorized) {
			m = m.resetTo(screenSetup)
			m.setup = newSetup()
			return m, nil
		}
		switch msg.origin {
		case screenHome:
			m.home.errText = "Error: " + msg.err.Error()
			m = m.resetTo(screenHome)
		default:
			m = m.replace(screenError)
		}
		return m, nil

	case logErrMsg:
		// Log-flow error: stay on the log screen with the message, so the
		// filled form / selected task is not lost and the user can retry.
		m.logScreen.loading = false
		m.logScreen.msg = "Error: " + msg.err.Error()
		m = m.replace(screenLog)
		return m, nil

	case entriesMsg:
		m.entries = msg.entries
		// Pin BEFORE rebuilding: activeRange (and every rebuild reached through
		// it, including rebuildReport below) must see the range this load
		// actually resolved, not one recomputed after the fact (#28).
		m.loadedStart, m.loadedEnd = msg.start, msg.end
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
		if !m.rebuildReport(groupBy) {
			return m, nil
		}
		// Report is reached three ways (Home enter, Report's own reload, and
		// the logDone reload) and all three arrive here: re-rooting rather
		// than replacing makes every arrival converge on nav == [Home], which
		// is Report's back target regardless of which path it came from.
		m = m.resetTo(screenHome).goTo(screenReport)
		return m, nil

	case entriesReloadedMsg:
		m.entries = msg.entries
		// Pin BEFORE rebuilding, same as entriesMsg above: a browser reload is
		// a load, so it must refresh the pin rather than leave a stale one (#28).
		m.loadedStart, m.loadedEnd = msg.start, msg.end
		m.assignStatuses()
		m.pruneFilters()
		if !m.applyReport() { // rebuilds m.report + m.rep; returns false on loc/pricing error
			return m, nil
		}
		es := m.entriesScreen
		es.entries = sortEntriesByStartDesc(m.visibleEntries())
		if es.idx >= len(es.entries) {
			es.idx = len(es.entries) - 1
		}
		if es.idx < 0 {
			es.idx = 0
		}
		es.mode = entriesList
		es.msg = msg.status
		es.msgErr = false
		m.entriesScreen = es
		m = m.replace(screenEntries)
		return m, nil

	case entriesErrMsg:
		es := m.entriesScreen
		es.mode = entriesList
		es.msg = msg.err.Error()
		es.msgErr = true
		m.entriesScreen = es
		m = m.replace(screenEntries)
		return m, nil

	case historyMsg:
		// Unlike tagsMsg/membersMsg/statusesMsg below, this fetch is dispatched
		// FROM screenLoading (entries.go's 'h'), not from the screen it lands
		// on — so the guard must accept screenLoading, not screenEntries.
		// screenLoading swallows every key but quit, so nothing can navigate
		// away before this reply arrives; the guard is still worth having so a
		// future change to Loading's key handling doesn't silently resurrect a
		// stale history view.
		if m.screen != screenLoading {
			return m, nil // stale: the user is no longer waiting on this fetch
		}
		es := m.entriesScreen
		es.historyChanges = msg.changes
		es.mode = entriesHistory
		m.entriesScreen = es
		m = m.replace(screenEntries)
		return m, nil

	case tagsMsg:
		if m.screen != screenEntries {
			return m, nil // stale: the user left the browser before the fetch landed
		}
		es := m.entriesScreen
		es.tagLoading = false
		es.tagAll = unionSortedTags(msg.tags, es.tagAll) // fetched ∪ current, deduped+sorted
		m.entriesScreen = es
		return m, nil

	case teamsMsg:
		// delivered to setup for workspace selection
		var cmd tea.Cmd
		m.setup, cmd = m.setup.withTeams(msg.teams)
		return m, cmd

	case logDoneMsg:
		m.logScreen.step = logDone
		m.logScreen.msg = msg.summary
		m = m.replace(screenLog)
		return m, nil

	case timerStoppedMsg:
		m.runningTimer = nil
		m.ticking = false
		m.logScreen.timer = nil
		m.logScreen.step = logDone
		m.logScreen.msg = msg.summary
		m = m.replace(screenLog)
		return m, nil

	case taskListMsg:
		m.logScreen.tasks = msg.tasks
		m.logScreen.taskIdx = 0
		m.logScreen.loading = false
		m.logScreen.step = logTaskPick
		return m, nil

	case timerMsg:
		// Update global state first, unconditionally: a timer started from the
		// log flow must surface on Home's live indicator and start ticking even
		// though (unlike runningTimerMsg) this msg is otherwise scoped to the
		// log screen.
		m.runningTimer = msg.timer
		var tick tea.Cmd
		if msg.timer != nil && !m.ticking {
			m.ticking = true
			tick = tickCmd()
		}
		if m.screen != screenLog && m.screen != screenLoading {
			return m, tick // stale for the log screen, but global state is updated
		}
		m.logScreen.timer = msg.timer
		if msg.timer != nil {
			m.logScreen.step = logTimerRunning
		}
		m = m.replace(screenLog)
		return m, tick

	case membersMsg:
		if m.screen != screenMembers {
			return m, nil // stale: the user left the members screen before the fetch landed
		}
		m.teamMembers = msg.members
		if len(m.selectedMembers) == 0 {
			m.selectedMembers = make(map[int]bool, len(msg.members))
			for _, mem := range msg.members {
				m.selectedMembers[mem.ID] = true // default: all selected
			}
		}
		m.membersScreen = newMembers(msg.members, m.selectedMembers)
		m = m.replace(screenMembers)
		return m, nil

	case statusesMsg:
		if m.screen != screenFilters {
			return m, nil // stale: the user left the filters screen before the fetch landed
		}
		if m.taskStatus == nil {
			m.taskStatus = map[string]string{}
		}
		for id, st := range msg.byTask {
			m.taskStatus[id] = st
		}
		m.assignStatuses()
		m.filtersScreen = newFilters(m.entries, m.filterLists, m.filterTags, m.filterStatuses, m.filterBillable)
		m = m.replace(screenFilters)
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

	case updateAvailableMsg:
		m.latestVersion = msg.latest
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
	case screenEntries:
		return m.updateEntries(msg)
	case screenError:
		if !m.cfg.Valid() {
			m = m.resetTo(screenSetup)
			m.setup = newSetup()
		} else {
			m = m.resetTo(screenHome)
		}
		return m, nil
	}
	return m, nil
}

func (m Model) screenBody() string {
	switch m.screen {
	case screenSetup:
		return m.setup.view(m.theme)
	case screenHome:
		timerLine := ""
		if m.runningTimer != nil {
			if label := elapsedLabel(m.runningTimer.Start, m.now()); label != "" {
				timerLine = "⏱  running on " + m.runningTimer.TaskName + " — " + label
			}
		}
		return m.home.view(m.theme, m.rangeLabel(), m.scope, m.homeMembersNote(), m.latestVersion, timerLine)
	case screenLoading:
		return m.theme.Title.Render("Loading hours…")
	case screenReport:
		return m.rep.view(m.theme, m.width)
	case screenExport:
		return m.export.view(m.theme)
	case screenRates:
		return m.ratesScreen.view(m.theme, m.keys)
	case screenLog:
		m.logScreen.now = m.now()
		return m.logScreen.view(m.theme, m.keys)
	case screenMembers:
		return m.membersScreen.view(m.theme)
	case screenRange:
		return m.rangeScreen.view(m.theme)
	case screenFilters:
		return m.filtersScreen.view(m.theme, filtersRows(m.height))
	case screenListBrowser:
		return m.browserScreen.view(m.theme)
	case screenBudget:
		return m.budgetScreen.view(m.theme, m.width)
	case screenEntries:
		return m.entriesView(m.theme)
	case screenError:
		return m.theme.Err.Render("Error: ") + m.err.Error() + "\n\n" +
			m.theme.Help.Render("press a key to return home")
	}
	return ""
}

func (m Model) View() string {
	body := m.screenBody()
	if m.screen == screenError {
		// Every key returns Home here, which is not a binding — the screen
		// says so in its own sentence instead.
		return body
	}
	if m.overlay == overlayPalette {
		// Composed over the BODY, not over the finished view: the footer stays
		// below and visible, and it advertises the palette's own keys because
		// keysFor follows the overlay.
		box, x, y := m.palette.layout(m.theme, m.width, m.height, strings.Count(body, "\n")+1)
		body = composite(body, box, x, y)
	}
	// Screens differ on whether their body ends with a newline; trimming here
	// is what puts the footer the same distance below every one of them.
	return strings.TrimRight(body, "\n") + "\n\n" + footerView(m.theme, m.width, m.helpAll, keysFor(m))
}
