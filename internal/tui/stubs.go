package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

type exportModel struct{}

func newExport(r report.Report) exportModel { return exportModel{} }

func (e exportModel) view() string { return "export" }

func (m Model) updateExport(tea.KeyMsg) (tea.Model, tea.Cmd) { return m, nil }
