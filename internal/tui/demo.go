package tui

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/config"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

// demoEnabled indica se la modalità demo è attiva (env CLICKUP_DEMO non vuota).
// In demo la TUI salta il setup e usa dati fittizi: nessuna chiamata all'API,
// utile per provare l'app senza account e per generare il GIF del README.
func demoEnabled() bool { return os.Getenv("CLICKUP_DEMO") != "" }

// demoConfig è una config fittizia per la modalità demo (nessun token reale).
func demoConfig() config.Config {
	return config.Config{
		Token:       "DEMO",
		WorkspaceID: "demo",
		Currency:    "EUR",
		Rate:        50,
		Rates:       map[string]float64{"web": 65, "mobile": 45},
	}
}

// demoEntries ritorna time entry fittizie per il mese dato, così il report
// mostra ore e importi realistici senza chiamare l'API.
func demoEntries(year int, month time.Month) []report.TimeEntry {
	at := func(d, h, m int) time.Time { return time.Date(year, month, d, h, m, 0, 0, time.UTC) }
	mk := func(id, taskID, task, listID, list string, start time.Time, dur time.Duration) report.TimeEntry {
		return report.TimeEntry{
			ID: id, TaskID: taskID, TaskName: task,
			ListID: listID, ListName: list,
			UserID: 1, UserName: "demo",
			Start: start, Duration: dur,
		}
	}
	return []report.TimeEntry{
		mk("1", "t1", "Landing page redesign", "web", "Website", at(3, 9, 0), 3*time.Hour+30*time.Minute),
		mk("2", "t2", "API integration", "web", "Website", at(3, 14, 0), 2*time.Hour),
		mk("3", "t3", "Bugfix checkout", "web", "Website", at(5, 10, 0), 1*time.Hour+15*time.Minute),
		mk("4", "t4", "Onboarding screens", "mobile", "Mobile app", at(6, 9, 30), 4*time.Hour),
		mk("5", "t5", "Push notifications", "mobile", "Mobile app", at(7, 11, 0), 2*time.Hour+45*time.Minute),
		mk("6", "t6", "Release QA", "mobile", "Mobile app", at(10, 15, 0), 1*time.Hour+30*time.Minute),
	}
}

// demoEntriesCmd consegna le entry fittizie come entriesMsg (nessuna I/O).
func demoEntriesCmd(year int, month time.Month) tea.Cmd {
	return func() tea.Msg {
		return entriesMsg{entries: demoEntries(year, month)}
	}
}
