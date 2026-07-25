package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
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
	k := keysFor(m)
	switch {
	case key.Matches(msg, k.Up):
		if e.idx > 0 {
			e.idx--
		}
	case key.Matches(msg, k.Down):
		if e.idx < len(exportFormats)-1 {
			e.idx++
		}
	case key.Matches(msg, k.Back):
		m.screen = screenReport
		m.export = e
		return m, nil
	case key.Matches(msg, k.Confirm):
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

func (e exportModel) view(th theme) string {
	b := th.Title.Render("Export report") + "\n\n"
	for i, f := range exportFormats {
		cursor := "  "
		line := f.label
		if i == e.idx {
			cursor = "▸ "
			line = th.Accent.Render(line)
		}
		b += cursor + line + "\n"
	}
	if e.done != "" {
		b += "\n" + th.OK.Render("Saved: "+e.done)
	}
	if e.err != nil {
		b += "\n" + th.Err.Render("Error: "+e.err.Error())
	}
	b += "\n\n" + th.Help.Render("↑/↓ select · Enter: export · Esc: back to report")
	return b
}
