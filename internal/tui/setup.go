package tui

import (
	"context"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
)

type setupStep int

const (
	stepToken setupStep = iota
	stepWorkspace
	stepRate
	stepCurrency
)

type setupModel struct {
	step    setupStep
	input   textinput.Model
	teams   []clickup.Team
	teamIdx int
	tmpCfg  config.Config
	loading bool
	msg     string
}

func newSetup() setupModel {
	ti := textinput.New()
	ti.Placeholder = "pk_xxx… (ClickUp → Settings → Apps → API Token)"
	ti.Focus()
	ti.CharLimit = 128
	ti.Width = 50
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	return setupModel{step: stepToken, input: ti}
}

func (s setupModel) token() string { return s.tmpCfg.Token }

func (s setupModel) withTeams(teams []clickup.Team) (setupModel, tea.Cmd) {
	s.teams = teams
	s.loading = false
	s.step = stepWorkspace
	return s, nil
}

func (m Model) updateSetup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := m.setup
	switch s.step {
	case stepToken:
		if s.loading {
			return m, nil // validazione in corso: ignora ulteriori input
		}
		if msg.Type == tea.KeyEnter && s.input.Value() != "" {
			s.tmpCfg.Token = s.input.Value()
			s.loading = true
			s.msg = "Validazione token…"
			m.setup = s
			m.client = clickup.New(s.tmpCfg.Token)
			return m, validateAndLoadTeamsCmd(m.client)
		}
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		s.tmpCfg.Token = s.input.Value()
		m.setup = s
		return m, cmd

	case stepWorkspace:
		switch msg.String() {
		case "up", "k":
			if s.teamIdx > 0 {
				s.teamIdx--
			}
		case "down", "j":
			if s.teamIdx < len(s.teams)-1 {
				s.teamIdx++
			}
		case "enter":
			if len(s.teams) > 0 {
				s.tmpCfg.WorkspaceID = s.teams[s.teamIdx].ID
				s.step = stepRate
				s.input = newNumberInput("Tariffa oraria (es. 45) — vuoto per saltare")
			}
		}
		m.setup = s
		return m, nil

	case stepRate:
		if msg.Type == tea.KeyEnter {
			if v := s.input.Value(); v != "" {
				rate, err := strconv.ParseFloat(v, 64)
				if err != nil {
					s.msg = "Tariffa non valida: inserisci un numero (es. 45)"
					m.setup = s
					return m, nil
				}
				s.tmpCfg.Rate = rate
			}
			s.msg = ""
			s.step = stepCurrency
			s.input = newTextInput("Valuta (es. EUR) — vuoto per EUR")
			m.setup = s
			return m, nil
		}
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		m.setup = s
		return m, cmd

	case stepCurrency:
		if msg.Type == tea.KeyEnter {
			s.tmpCfg.Currency = s.input.Value()
			if s.tmpCfg.Currency == "" {
				s.tmpCfg.Currency = "EUR"
			}
			m.cfg = s.tmpCfg
			_ = config.Save(m.cfg)
			m.client = clickup.New(m.cfg.Token)
			m.home = newHome(m.year, m.month)
			m.screen = screenHome
			return m, nil
		}
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		m.setup = s
		return m, cmd
	}
	return m, nil
}

func (s setupModel) view() string {
	b := styleTitle.Render("Setup ClickUp Hours CLI") + "\n\n"
	switch s.step {
	case stepToken:
		b += "Incolla il tuo token API personale:\n\n" + s.input.View()
		if s.msg != "" {
			b += "\n\n" + styleHelp.Render(s.msg)
		}
	case stepWorkspace:
		b += "Scegli il workspace:\n\n"
		for i, t := range s.teams {
			cursor := "  "
			line := t.Name + " (" + t.ID + ")"
			if i == s.teamIdx {
				cursor = "▸ "
				line = styleAccent.Render(line)
			}
			b += cursor + line + "\n"
		}
	case stepRate:
		b += s.input.View()
		if s.msg != "" {
			b += "\n\n" + styleErr.Render(s.msg)
		}
	case stepCurrency:
		b += s.input.View()
	}
	b += "\n\n" + styleHelp.Render("Enter: conferma · Ctrl+C: esci")
	return b
}

// validateAndLoadTeamsCmd valida il token (CurrentUser) e carica i teams.
func validateAndLoadTeamsCmd(c *clickup.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if _, err := c.CurrentUser(ctx); err != nil {
			return errMsg{err: err}
		}
		teams, err := c.Teams(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return teamsMsg{teams: teams}
	}
}

func newTextInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.Width = 40
	return ti
}

func newNumberInput(placeholder string) textinput.Model {
	ti := newTextInput(placeholder)
	ti.CharLimit = 10
	return ti
}
