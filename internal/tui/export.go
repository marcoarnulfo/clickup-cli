package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/export"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

type exportFormat struct {
	label string
	key   string // export.ToFile format key
	ext   string
	// prefix is the output filename stem. The CSV invoice needs its own:
	// it shares the .csv extension with the bucket CSV and would otherwise
	// overwrite it.
	prefix string
}

// exportFormats must cover every format export.ToFile supports, so the TUI and
// `clup report --format` offer the same set.
var exportFormats = []exportFormat{
	{"CSV", "csv", "csv", "clickup-report"},
	{"JSON", "json", "json", "clickup-report"},
	{"Markdown", "markdown", "md", "clickup-report"},
	{"HTML", "html", "html", "clickup-report"},
	{"CSV invoice", "csv-invoice", "csv", "clickup-invoice"},
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
		path := fmt.Sprintf("%s-%s.%s", f.prefix, report.PeriodFileSlug(e.r.Start, e.r.End), f.ext)
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
	b := styleTitle.Render("Export report") + "\n\n"
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
		b += "\n" + styleOK.Render("Saved: "+e.done)
	}
	if e.err != nil {
		b += "\n" + styleErr.Render("Error: "+e.err.Error())
	}
	b += "\n\n" + styleHelp.Render("↑/↓ select · Enter: export · Esc: back to report")
	return b
}
