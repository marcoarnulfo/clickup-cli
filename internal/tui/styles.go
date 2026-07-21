package tui

import "github.com/charmbracelet/lipgloss"

var (
	colAccent = lipgloss.Color("205") // magenta ClickUp-ish
	colDim    = lipgloss.Color("240")
	colErr    = lipgloss.Color("196")
	colOK     = lipgloss.Color("42")

	styleTitle  = lipgloss.NewStyle().Bold(true).Foreground(colAccent).MarginBottom(1)
	styleHelp   = lipgloss.NewStyle().Foreground(colDim)
	styleErr    = lipgloss.NewStyle().Foreground(colErr).Bold(true)
	styleAccent = lipgloss.NewStyle().Foreground(colAccent)
	styleOK     = lipgloss.NewStyle().Foreground(colOK)
	styleBox    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)
