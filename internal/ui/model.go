package ui

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cesar32az/tfkill/internal/scanner"
)

// Custom message sent when the scanner finishes
type scanFinishedMsg struct {
	results  []scanner.Result
	duration time.Duration
}

type model struct {
	dir             string
	results         []scanner.Result
	cursor          int
	totalSaved      int64
	totalReleasable int64
	confirmMode     bool

	// New fields for UX
	isScanning   bool
	scanDuration time.Duration
	spinner      spinner.Model
}

func InitialModel(dir string) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = warningStyle

	return model{
		dir:        dir,
		isScanning: true,
		spinner:    s,
	}
}

// Command that runs the scanner in the background
func runScan(dir string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		res := scanner.Scan(dir)
		return scanFinishedMsg{
			results:  res,
			duration: time.Since(start),
		}
	}
}

// Init starts the spinner and scanner at the same time
func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, runScan(m.dir))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// If we receive the scanner results
	case scanFinishedMsg:
		m.results = msg.results
		m.scanDuration = msg.duration
		m.isScanning = false
		// Calculate the total space that can be freed
		for _, r := range m.results {
			m.totalReleasable += r.Size
		}
		return m, nil

	// Spinner animation
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	// Keyboard events
	case tea.KeyMsg:
		if m.isScanning {
			if msg.String() == "ctrl+c" || msg.String() == "q" {
				return m, tea.Quit
			}
			return m, nil // Block other keys while scanning
		}

		if m.confirmMode {
			switch msg.String() {
			case "y", "Y":
				m.deleteCurrent()
				m.confirmMode = false
			case "n", "N", "esc":
				m.confirmMode = false
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 { m.cursor-- }
		case "down", "j":
			if m.cursor < len(m.results)-1 { m.cursor++ }
		case " ":
			i := m.cursor
			if m.results[i].Deleted { return m, nil }

			if m.results[i].HasState {
				m.confirmMode = true
			} else {
				m.deleteCurrent()
			}
		}
	}
	return m, nil
}

func (m *model) deleteCurrent() {
	i := m.cursor
	err := os.RemoveAll(m.results[i].Path)
	if err == nil {
		m.results[i].Deleted = true
		m.totalSaved += m.results[i].Size
	}
}

func (m model) View() string {
	// 1. LOADING SCREEN
	if m.isScanning {
		return fmt.Sprintf("\n  %s Scanning directories in: %s...\n\n", m.spinner.View(), m.dir)
	}

	// 2. NPKILL STYLE HEADER
	logo := selectedStyle.Render(`
  _    __ _    _  _  _
 | |_ / _| | _(_)| || |
 | __| |_| |/ / || || |
 | |_|  _|   <| || || |
  \__|_| |_|\_\_||_||_|
`)

	releasableMB := float64(m.totalReleasable) / 1024 / 1024
	savedMB := float64(m.totalSaved) / 1024 / 1024

	stats := fmt.Sprintf(
		"Releasable space: %s\nSpace saved:      %s\nSearch completed  %s",
		secondaryStyle.Render(fmt.Sprintf("%.2f MB", releasableMB)),
		selectedStyle.Render(fmt.Sprintf("%.2f MB", savedMB)),
		grayStyle.Render(fmt.Sprintf("%.2fs", m.scanDuration.Seconds())),
	)

	// Join the logo and statistics in two columns
	header := lipgloss.JoinHorizontal(lipgloss.Center, logo, "    ", stats)

	// 3. RESULTS
	s := "\n" + header + "\n\n"

	if len(m.results) == 0 {
		return s + "No unnecessary .terraform folders found.\n"
	}

	for i, res := range m.results {
		statusIcon := "  "
		lineText := res.Path

		if res.Deleted {
			statusIcon = secondaryStyle.Render("✔ ")
			lineText = grayStyle.Render(res.Path)
		} else if m.cursor == i {
			statusIcon = selectedStyle.Render("▶ ")
			if m.confirmMode {
				lineText = warningStyle.Bold(true).Render(res.Path)
			} else {
				lineText = selectedStyle.Render(res.Path)
			}
		}

		warning := ""
		if res.HasState && !res.Deleted {
			warning = warningStyle.Render(" ⚠️ local state")
		}

		sizeStr := secondaryStyle.Render(fmt.Sprintf("%.2f MB", float64(res.Size)/1024/1024))
		s += fmt.Sprintf("%s %-50s %s %s\n", statusIcon, lineText, sizeStr, warning)
	}

	// 4. FOOTER
	if m.confirmMode {
		s += "\n" + warningStyle.Bold(true).Render("WARNING! This folder has a local state. Delete anyway? (y/n)")
	} else {
		s += "\n" + helpStyle.Render("UP/DOWN to select - SPACE to delete - Q to quit")
	}

	return s
}

