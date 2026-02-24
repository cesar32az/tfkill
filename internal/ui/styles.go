package ui

import "github.com/charmbracelet/lipgloss"

var (
	subtle = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#626262"}
	purple = lipgloss.Color("#7D56F4")
	green  = lipgloss.Color("#04B575")
	orange = lipgloss.Color("#FF8700")

	selectedStyle = lipgloss.NewStyle().Foreground(purple).Bold(true)
	grayStyle     = lipgloss.NewStyle().Foreground(subtle)
	secondaryStyle = lipgloss.NewStyle().Foreground(green)
	warningStyle  = lipgloss.NewStyle().Foreground(orange)
	helpStyle     = lipgloss.NewStyle().Foreground(subtle).Italic(true)
)
