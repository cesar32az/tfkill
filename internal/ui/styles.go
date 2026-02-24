package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	subtle = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#626262"}
	purple = lipgloss.Color("#7D56F4")
	green  = lipgloss.Color("#04B575")
	orange = lipgloss.Color("#FF8700")

	selectedStyle  = lipgloss.NewStyle().Foreground(purple).Bold(true)
	grayStyle      = lipgloss.NewStyle().Foreground(subtle)
	secondaryStyle = lipgloss.NewStyle().Foreground(green)
	warningStyle   = lipgloss.NewStyle().Foreground(orange)
	helpStyle      = lipgloss.NewStyle().Foreground(subtle)

	// Key chip: dark pill used in the footer help bar.
	keyChipStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#383838")).
			Padding(0, 1)

	separatorStyle = lipgloss.NewStyle().Foreground(subtle)
)

// renderHelp builds a footer help bar from key→description pairs.
// Example output:  ↑↓  navigate  ·  space  delete  ·  q  quit
func renderHelp(bindings [][2]string) string {
	parts := make([]string, len(bindings))
	for i, b := range bindings {
		parts[i] = keyChipStyle.Render(b[0]) + " " + helpStyle.Render(b[1])
	}
	return "  " + strings.Join(parts, separatorStyle.Render("  ·  "))
}
