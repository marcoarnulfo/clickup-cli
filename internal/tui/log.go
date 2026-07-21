package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/duration"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

type logStep int

const (
	logModeSelect logStep = iota
	logTimerPick
	logListPick
	logTaskPick
	logIDInput
	logForm
	logTimerRunning
	logDone
)

type logMode int

const (
	modeGuided logMode = iota
	modeID
	modeTimer
)

// taskListChoice è una lista nota (report ∪ config) mostrata nel picker guidato.
type taskListChoice struct {
	id   string
	name string
}

type logModel struct {
	step logStep
	mode logMode

	lists   []taskListChoice
	listIdx int

	tasks   []clickup.Task
	taskIdx int
	loading bool

	taskID   string
	taskName string

	input textinput.Model

	// form (campi compilati in sequenza)
	formField int // 0=durata 1=data 2=nota
	durStr    string
	dateStr   string

	// timer
	timer *clickup.RunningTimer

	msg string
}

// newLog costruisce la schermata dalle liste note (entries ∪ config.Rates),
// in ordine deterministico per una vista stabile.
func newLog(entries []report.TimeEntry, cfg config.Config) logModel {
	names := map[string]string{}
	var order []string
	remember := func(id, name string) {
		if id == "" {
			return
		}
		if _, ok := names[id]; !ok {
			order = append(order, id)
			names[id] = id
		}
		if name != "" {
			names[id] = name
		}
	}
	for _, e := range entries {
		remember(e.ListID, e.ListName)
	}
	lists := make([]taskListChoice, len(order))
	for i, id := range order {
		lists[i] = taskListChoice{id: id, name: names[id]}
	}
	return logModel{lists: lists}
}

type logDoneMsg struct{ summary string }

// createEntryCmd crea la time entry in background.
func createEntryCmd(c *clickup.Client, teamID, tid string, start time.Time, dur time.Duration, desc string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := c.CreateTimeEntry(ctx, teamID, tid, start, dur, desc); err != nil {
			return errMsg{err: err}
		}
		return logDoneMsg{summary: fmt.Sprintf("%s registrate su %s", dur, tid)}
	}
}

// enterForm inizializza il form: campo durata, data default = oggi.
func enterForm(lg logModel) logModel {
	lg.step = logForm
	lg.formField = 0
	lg.durStr = ""
	lg.dateStr = time.Now().Format("2006-01-02")
	lg.msg = ""
	lg.input = newTextInput("Durata (es. 2h30, 1.5h, 90m)")
	return lg
}

func (m Model) updateLog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lg := m.logScreen

	// Esc torna al report da qualunque passo non-input (gli input gestiscono
	// Esc localmente nei task successivi).
	if msg.Type == tea.KeyEsc && lg.step != logIDInput && lg.step != logForm {
		m.screen = screenReport
		return m, nil
	}

	switch lg.step {
	case logModeSelect:
		switch msg.String() {
		case "1":
			lg.mode = modeGuided
			lg.step = logListPick
		case "2":
			lg.mode = modeID
			lg.step = logIDInput
			lg.input = newTextInput("ID o URL del task")
		case "3":
			lg.mode = modeTimer
			lg.step = logTimerPick
		}

	case logIDInput:
		if msg.Type == tea.KeyEsc {
			lg.step = logModeSelect
			lg.msg = ""
			m.logScreen = lg
			return m, nil
		}
		if msg.Type == tea.KeyEnter {
			lg.taskID = lg.input.Value()
			lg = enterForm(lg)
			m.logScreen = lg
			return m, nil
		}
		var cmd tea.Cmd
		lg.input, cmd = lg.input.Update(msg)
		m.logScreen = lg
		return m, cmd

	case logForm:
		if msg.Type == tea.KeyEsc {
			m.screen = screenReport
			return m, nil
		}
		if msg.Type == tea.KeyEnter {
			val := lg.input.Value()
			switch lg.formField {
			case 0: // durata
				if _, err := duration.Parse(val); err != nil {
					lg.msg = "Durata non valida (es. 2h30, 1.5h, 90m)"
					m.logScreen = lg
					return m, nil
				}
				lg.durStr = val
				lg.formField = 1
				lg.msg = ""
				lg.input = newTextInput("Data (YYYY-MM-DD)")
				lg.input.SetValue(lg.dateStr)
				m.logScreen = lg
				return m, nil
			case 1: // data
				if val == "" {
					val = lg.dateStr
				}
				if _, err := time.Parse("2006-01-02", val); err != nil {
					lg.msg = "Data non valida (formato YYYY-MM-DD)"
					m.logScreen = lg
					return m, nil
				}
				lg.dateStr = val
				lg.formField = 2
				lg.msg = ""
				lg.input = newTextInput("Nota (opzionale)")
				m.logScreen = lg
				return m, nil
			case 2: // nota -> submit
				dur, _ := duration.Parse(lg.durStr)
				day, _ := time.Parse("2006-01-02", lg.dateStr)
				start := time.Date(day.Year(), day.Month(), day.Day(), 9, 0, 0, 0, time.Local)
				note := lg.input.Value()
				m.logScreen = lg
				m.screen = screenLoading
				return m, createEntryCmd(m.client, m.cfg.WorkspaceID, lg.taskID, start, dur, note)
			}
		}
		var cmd tea.Cmd
		lg.input, cmd = lg.input.Update(msg)
		m.logScreen = lg
		return m, cmd

	case logDone:
		switch msg.String() {
		case "r":
			m.screen = screenLoading
			return m, loadEntriesCmd(m.client, m.cfg.WorkspaceID, m.year, m.month, m.scope)
		case "esc", "enter":
			m.screen = screenReport
			return m, nil
		}
	}

	m.logScreen = lg
	return m, nil
}

func (lg logModel) view() string {
	b := styleTitle.Render("Log ore") + "\n\n"
	switch lg.step {
	case logModeSelect:
		b += "Scegli la modalità:\n\n"
		b += "  " + styleAccent.Render("1") + ") Guidato — scegli lista e task\n"
		b += "  " + styleAccent.Render("2") + ") Task ID/URL — vai diretto al form\n"
		b += "  " + styleAccent.Render("3") + ") Timer — start/stop cronometro\n"
	case logIDInput:
		b += "ID o URL del task:\n\n" + lg.input.View()
	case logForm:
		labels := []string{"Durata", "Data", "Nota (opzionale)"}
		b += "Task: " + styleAccent.Render(lg.taskID) + "\n\n"
		b += labels[lg.formField] + ":\n\n" + lg.input.View()
	case logDone:
		b += styleOK.Render("✓ Ore registrate.") + "\n\n"
		b += styleHelp.Render("r: ricarica il report · Esc: torna al report")
	default:
		b += styleHelp.Render("…")
	}
	if lg.msg != "" {
		b += "\n" + styleErr.Render(lg.msg)
	}
	b += "\n\n" + styleHelp.Render("Esc: annulla · Ctrl+C: esci")
	return b
}
