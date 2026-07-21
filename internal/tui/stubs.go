package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcoarnulfo/clickup-cli/internal/report"
)

type homeModel struct{}
type reportModel struct{}
type exportModel struct{}

func newHome(year int, month time.Month) homeModel { return homeModel{} }
func newReport(r report.Report) reportModel        { return reportModel{} }
func newExport(r report.Report) exportModel        { return exportModel{} }

func (h homeModel) view() string    { return "home" }
func (rm reportModel) view() string { return "report" }
func (e exportModel) view() string  { return "export" }

func (m Model) updateHome(tea.KeyMsg) (tea.Model, tea.Cmd)   { return m, nil }
func (m Model) updateReport(tea.KeyMsg) (tea.Model, tea.Cmd) { return m, nil }
func (m Model) updateExport(tea.KeyMsg) (tea.Model, tea.Cmd) { return m, nil }
