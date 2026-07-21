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
	screenError
)

// Messaggi async.
type (
	entriesMsg struct{ entries []report.TimeEntry }
	teamsMsg   struct{ teams []clickup.Team }
	errMsg     struct{ err error }
)

// Model è il modello radice della TUI.
type Model struct {
	cfg    config.Config
	client *clickup.Client
	screen screen
	err    error

	width, height int

	// selezione corrente
	year  int
	month time.Month
	scope string // "me" | "team"

	// dati
	report  report.Report
	entries []report.TimeEntry

	// sotto-modelli
	setup       setupModel
	home        homeModel
	rep         reportModel
	export      exportModel
	ratesScreen ratesModel
}

// New costruisce il modello radice a partire dalla config.
func New(cfg config.Config) Model {
	now := time.Now()
	m := Model{
		cfg:    cfg,
		year:   now.Year(),
		month:  now.Month(),
		scope:  "me",
		client: clickup.New(cfg.Token),
	}
	if cfg.Valid() {
		m.screen = screenHome
		m.home = newHome(m.year, m.month)
	} else {
		m.screen = screenSetup
		m.setup = newSetup()
	}
	return m
}

func (m Model) Init() tea.Cmd { return nil }

// ratesFromConfig costruisce le tariffe per il report dalla config (default + override).
func ratesFromConfig(cfg config.Config) report.Rates {
	return report.Rates{Default: cfg.Rate, ByList: cfg.Rates}
}

// loadEntriesCmd chiama l'API in background e ritorna entriesMsg o errMsg.
// Per lo scope "team" ricava gli id di tutti i membri del workspace (via Teams)
// e li passa come assignees, così il report copre l'intero team; per "me" nessun
// assignee (l'API torna le voci dell'utente autenticato).
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
				return errMsg{err: fmt.Errorf("workspace %s non trovato o non accessibile con questo token", teamID)}
			}
		}

		start, end := report.MonthRange(year, month)
		entries, err := c.TimeEntries(ctx, teamID, start, end, assignees)
		if err != nil {
			return errMsg{err: err}
		}
		// Risolvi i nomi leggibili delle liste UNA sola volta per list_id unico
		// (evita chiamate ripetute, incluse quelle fallite, per la stessa lista).
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
				resolved[e.ListID] = "" // tentato: non ritentare in questo caricamento
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
		// Token invalido/revocato: rilancia il setup wizard (spec §8).
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
			groupBy = report.GroupByTotal // primo caricamento: sintesi del mese
		}
		m.report = report.Build(msg.entries, groupBy, ratesFromConfig(m.cfg), m.cfg.Currency, m.year, m.month)
		m.report.Scope = m.scope
		m.rep = newReport(m.report)
		m.screen = screenReport
		return m, nil

	case teamsMsg:
		// consegnato al setup per la scelta workspace
		var cmd tea.Cmd
		m.setup, cmd = m.setup.withTeams(msg.teams)
		return m, cmd
	}
	return m, nil
}

// routeKey inoltra i tasti allo screen attivo.
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
		return m.home.view()
	case screenLoading:
		return styleTitle.Render("Caricamento ore…")
	case screenReport:
		return m.rep.view()
	case screenExport:
		return m.export.view()
	case screenRates:
		return m.ratesScreen.view()
	case screenError:
		return styleErr.Render("Errore: ") + m.err.Error() + "\n\n" + styleHelp.Render("premi un tasto per tornare alla home")
	}
	return ""
}
