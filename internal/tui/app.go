package tui

import (
	"context"
	"errors"
	"fmt"
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
)

// Async messages.
type (
	entriesMsg struct{ entries []report.TimeEntry }
	teamsMsg   struct{ teams []clickup.Team }
	errMsg     struct{ err error }
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
	year  int
	month time.Month
	scope string // "me" | "team"

	// data
	report  report.Report
	entries []report.TimeEntry

	// sub-models
	setup       setupModel
	home        homeModel
	rep         reportModel
	export      exportModel
	ratesScreen ratesModel
	logScreen   logModel
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

// reloadEntriesCmd picks the source for time entries: demo data (no I/O)
// in demo mode, otherwise the real API call.
func (m Model) reloadEntriesCmd() tea.Cmd {
	if m.demo {
		return demoEntriesCmd(m.year, m.month)
	}
	return loadEntriesCmd(m.client, m.cfg.WorkspaceID, m.year, m.month, m.scope)
}

// ratesFromConfig builds the report rates from config (default + overrides).
func ratesFromConfig(cfg config.Config) report.Rates {
	return report.Rates{Default: cfg.Rate, ByList: cfg.Rates}
}

// loadEntriesCmd calls the API in the background and returns entriesMsg or errMsg.
// For scope "team" it derives the ids of all workspace members (via Teams)
// and passes them as assignees, so the report covers the whole team; for "me" no
// assignee is set (the API returns the entries of the authenticated user).
func loadEntriesCmd(c *clickup.Client, teamID string, year int, month time.Month, scope string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var assignees []int
		if scope == "team" {
			teams, err := c.Teams(ctx)
			if err != nil {
				return errMsg{err: err}
			}
			found := false
			for _, t := range teams {
				if t.ID == teamID {
					found = true
					for _, mem := range t.Members {
						assignees = append(assignees, mem.ID)
					}
				}
			}
			if !found {
				return errMsg{err: fmt.Errorf("workspace %s not found or not accessible with this token", teamID)}
			}
		}

		start, end := report.MonthRange(year, month)
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

	case entriesMsg:
		m.entries = msg.entries
		groupBy := m.report.GroupBy
		if groupBy == "" {
			groupBy = report.GroupByTotal // first load: summary of the month
		}
		m.report = report.Build(msg.entries, groupBy, ratesFromConfig(m.cfg), m.cfg.Currency, m.year, m.month)
		m.report.Scope = m.scope
		m.rep = newReport(m.report)
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
		return m.home.view(m.year, m.month, m.scope)
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
	case screenError:
		return styleErr.Render("Error: ") + m.err.Error() + "\n\n" + styleHelp.Render("press a key to return home")
	}
	return ""
}
