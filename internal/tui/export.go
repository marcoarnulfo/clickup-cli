package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/export"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

type exportFormat struct {
	label string
	key   string
	ext   string
}

var exportFormats = []exportFormat{
	{"CSV", "csv", "csv"},
	{"JSON", "json", "json"},
	{"Markdown", "markdown", "md"},
}

type exportModel struct {
	r    report.Report
	idx  int
	done string
	err  error
}

func newExport(r report.Report) exportModel { return exportModel{r: r} }

func (m Model) updateExport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	e := m.export
	switch msg.String() {
	case "up", "k":
		if e.idx > 0 {
			e.idx--
		}
	case "down", "j":
		if e.idx < len(exportFormats)-1 {
			e.idx++
		}
	case "esc":
		m.screen = screenReport
		m.export = e
		return m, nil
	case "enter":
		f := exportFormats[e.idx]
		path := fmt.Sprintf("clickup-report-%04d-%02d.%s", e.r.Year, int(e.r.Month), f.ext)
		if err := export.ToFile(f.key, e.r, path); err != nil {
			e.err = err
		} else {
			e.err = nil
			e.done = path
		}
	}
	m.export = e
	return m, nil
}

func (e exportModel) view() string {
	b := styleTitle.Render("Esporta report") + "\n\n"
	for i, f := range exportFormats {
		cursor := "  "
		line := f.label
		if i == e.idx {
			cursor = "▸ "
			line = styleAccent.Render(line)
		}
		b += cursor + line + "\n"
	}
	if e.done != "" {
		b += "\n" + styleOK.Render("Salvato: "+e.done)
	}
	if e.err != nil {
		b += "\n" + styleErr.Render("Errore: "+e.err.Error())
	}
	b += "\n\n" + styleHelp.Render("↑/↓ scegli · Enter: esporta · Esc: torna al report")
	return b
}
