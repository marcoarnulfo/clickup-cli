package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/clickup"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
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
	default:
		b += styleHelp.Render("…")
	}
	if lg.msg != "" {
		b += "\n" + styleErr.Render(lg.msg)
	}
	b += "\n\n" + styleHelp.Render("Esc: annulla · Ctrl+C: esci")
	return b
}
