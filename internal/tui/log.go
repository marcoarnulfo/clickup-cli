package tui

import (
	"context"
	"fmt"
	"sort"
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
	// liste presenti solo in config: ordine deterministico (id crescente)
	var cfgIDs []string
	for id := range cfg.Rates {
		if _, ok := names[id]; !ok {
			cfgIDs = append(cfgIDs, id)
		}
	}
	sort.Strings(cfgIDs)
	for _, id := range cfgIDs {
		remember(id, "")
	}
	lists := make([]taskListChoice, len(order))
	for i, id := range order {
		lists[i] = taskListChoice{id: id, name: names[id]}
	}
	return logModel{lists: lists}
}

type logDoneMsg struct{ summary string }

type taskListMsg struct{ tasks []clickup.Task }

type timerMsg struct{ timer *clickup.RunningTimer }

// startTimerCmd avvia il timer e poi legge lo stato corrente per avere lo start
// autoritativo restituito da ClickUp.
func startTimerCmd(c *clickup.Client, teamID, tid, desc string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := c.StartTimer(ctx, teamID, tid, desc); err != nil {
			return errMsg{err: err}
		}
		rt, err := c.CurrentTimer(ctx, teamID)
		if err != nil {
			return errMsg{err: err}
		}
		if rt == nil {
			rt = &clickup.RunningTimer{TaskID: tid, TaskName: tid}
		}
		return timerMsg{timer: rt}
	}
}

// stopTimerCmd ferma il timer; l'entry è creata da ClickUp.
func stopTimerCmd(c *clickup.Client, teamID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		e, err := c.StopTimer(ctx, teamID)
		if err != nil {
			return errMsg{err: err}
		}
		return logDoneMsg{summary: fmt.Sprintf("timer fermato: %s registrate", e.Duration)}
	}
}

// currentTimerCmd legge il timer in corso (nil se nessuno).
func currentTimerCmd(c *clickup.Client, teamID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rt, err := c.CurrentTimer(ctx, teamID)
		if err != nil {
			return errMsg{err: err}
		}
		return timerMsg{timer: rt}
	}
}

// listTasksCmd carica i task di una lista in background.
func listTasksCmd(c *clickup.Client, listID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		tasks, err := c.ListTasks(ctx, listID)
		if err != nil {
			return errMsg{err: err}
		}
		return taskListMsg{tasks: tasks}
	}
}

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
			m.logScreen = lg
			return m, currentTimerCmd(m.client, m.cfg.WorkspaceID)
		}

	case logTimerPick:
		switch msg.String() {
		case "1":
			lg.step = logListPick
		case "2":
			lg.step = logIDInput
			lg.input = newTextInput("ID o URL del task")
		}
		m.logScreen = lg
		return m, nil

	case logTimerRunning:
		switch msg.String() {
		case "s":
			m.logScreen = lg
			m.screen = screenLoading
			return m, stopTimerCmd(m.client, m.cfg.WorkspaceID)
		case "esc":
			m.screen = screenReport
			return m, nil
		}
		m.logScreen = lg
		return m, nil

	case logIDInput:
		if msg.Type == tea.KeyEsc {
			lg.step = logModeSelect
			lg.msg = ""
			m.logScreen = lg
			return m, nil
		}
		if msg.Type == tea.KeyEnter {
			id := clickup.ExtractTaskID(lg.input.Value())
			if id == "" {
				lg.msg = "Inserisci un id o un URL valido"
				m.logScreen = lg
				return m, nil
			}
			lg.taskID = id
			lg.taskName = id
			lg.msg = ""
			if lg.mode == modeTimer {
				m.logScreen = lg
				m.screen = screenLoading
				return m, startTimerCmd(m.client, m.cfg.WorkspaceID, id, "")
			}
			lg = enterForm(lg)
			lg.taskID = id
			lg.taskName = id
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

	case logListPick:
		switch msg.String() {
		case "up", "k":
			if lg.listIdx > 0 {
				lg.listIdx--
			}
		case "down", "j":
			if lg.listIdx < len(lg.lists)-1 {
				lg.listIdx++
			}
		case "enter":
			if len(lg.lists) > 0 {
				lg.loading = true
				m.logScreen = lg
				return m, listTasksCmd(m.client, lg.lists[lg.listIdx].id)
			}
		}
		m.logScreen = lg
		return m, nil

	case logTaskPick:
		switch msg.String() {
		case "up", "k":
			if lg.taskIdx > 0 {
				lg.taskIdx--
			}
		case "down", "j":
			if lg.taskIdx < len(lg.tasks)-1 {
				lg.taskIdx++
			}
		case "enter":
			if len(lg.tasks) > 0 {
				t := lg.tasks[lg.taskIdx]
				lg.taskID = t.ID
				lg.taskName = t.Name
				if lg.mode == modeTimer {
					m.logScreen = lg
					m.screen = screenLoading
					return m, startTimerCmd(m.client, m.cfg.WorkspaceID, t.ID, "")
				}
				id, name := t.ID, t.Name
				lg = enterForm(lg)
				lg.taskID, lg.taskName = id, name
				m.logScreen = lg
				return m, nil
			}
		}
		m.logScreen = lg
		return m, nil

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
	case logTimerPick:
		b += "Timer — come scegli il task?\n\n"
		b += "  " + styleAccent.Render("1") + ") Guidato (lista → task)\n"
		b += "  " + styleAccent.Render("2") + ") Task ID/URL\n"
	case logTimerRunning:
		if lg.timer == nil {
			b += styleHelp.Render("Nessun timer in corso.") + "\n"
			b += "\n" + styleHelp.Render("Esc: torna al report")
			break
		}
		b += "⏱  Timer in corso su: " + styleAccent.Render(lg.timer.TaskName) + "\n"
		if !lg.timer.Start.IsZero() {
			b += "Avviato: " + lg.timer.Start.Local().Format("15:04:05") + "\n"
		}
		b += "\n" + styleHelp.Render("s: ferma e registra · Esc: annulla")
	case logListPick:
		if lg.loading {
			b += styleHelp.Render("Caricamento task…") + "\n\n"
		}
		b += "Scegli la lista:\n\n"
		for i, l := range lg.lists {
			cursor := "  "
			line := l.name
			if i == lg.listIdx {
				cursor = "▸ "
				line = styleAccent.Render(line)
			}
			b += cursor + line + "\n"
		}
		if len(lg.lists) == 0 {
			b += styleHelp.Render("Nessuna lista nota: usa la modalità ID.") + "\n"
		}
		b += "\n" + styleHelp.Render("↑/↓ scegli · Enter: apri i task")
	case logTaskPick:
		b += "Scegli il task:\n\n"
		for i, tk := range lg.tasks {
			cursor := "  "
			line := truncate(tk.Name, 40)
			if i == lg.taskIdx {
				cursor = "▸ "
				line = styleAccent.Render(line)
			}
			b += cursor + line + "\n"
		}
		if len(lg.tasks) == 0 {
			b += styleHelp.Render("Nessun task nella lista.") + "\n"
		}
		b += "\n" + styleHelp.Render("↑/↓ scegli · Enter: continua")
	case logIDInput:
		b += "ID o URL del task:\n\n" + lg.input.View()
	case logForm:
		labels := []string{"Durata", "Data", "Nota (opzionale)"}
		b += "Task: " + styleAccent.Render(lg.taskID) + "\n\n"
		b += labels[lg.formField] + ":\n\n" + lg.input.View()
	case logDone:
		b += styleOK.Render("✓ Ore registrate.") + "\n\n"
		if lg.msg != "" {
			b += styleOK.Render(lg.msg) + "\n\n"
		}
		b += styleHelp.Render("r: ricarica il report · Esc: torna al report")
	default:
		b += styleHelp.Render("…")
	}
	if lg.msg != "" && lg.step != logDone {
		b += "\n" + styleErr.Render(lg.msg)
	}
	b += "\n\n" + styleHelp.Render("Esc: annulla · Ctrl+C: esci")
	return b
}
