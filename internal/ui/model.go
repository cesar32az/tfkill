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

// scanFinishedMsg is sent by runScan when the scanner completes.
type scanFinishedMsg struct {
	results  []scanner.Result
	duration time.Duration
	err      error // non-nil when some directories could not be accessed
}

type model struct {
	dir             string
	dryRun          bool
	results         []scanner.Result
	deleted         []bool  // parallel to results; tracks UI-level deletion state
	deleteErr       string  // last delete failure message, shown in footer
	scanErr         error   // non-fatal walk errors surfaced to the user
	cursor          int
	totalSaved      int64
	totalReleasable int64
	confirmMode     bool

	isScanning   bool
	scanDuration time.Duration
	spinner      spinner.Model
}

func InitialModel(dir string, dryRun bool) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = warningStyle

	return model{
		dir:        dir,
		dryRun:     dryRun,
		isScanning: true,
		spinner:    s,
	}
}

// runScan runs the scanner in a goroutine and returns the result as a tea.Msg.
func runScan(dir string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		res, err := scanner.Scan(dir)
		return scanFinishedMsg{
			results:  res,
			duration: time.Since(start),
			err:      err,
		}
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, runScan(m.dir))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case scanFinishedMsg:
		m.results = msg.results
		m.deleted = make([]bool, len(msg.results))
		m.scanDuration = msg.duration
		m.scanErr = msg.err
		m.isScanning = false
		for _, r := range m.results {
			m.totalReleasable += r.Size
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if m.isScanning {
			if msg.String() == "ctrl+c" || msg.String() == "q" {
				return m, tea.Quit
			}
			return m, nil
		}

		// Clear any previous delete error on the next keypress.
		m.deleteErr = ""

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
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.results)-1 {
				m.cursor++
			}
		case " ":
			i := m.cursor
			if m.deleted[i] {
				return m, nil
			}
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
	if m.dryRun {
		m.deleted[i] = true
		m.totalSaved += m.results[i].Size
		return
	}
	if err := os.RemoveAll(m.results[i].Path); err != nil {
		m.deleteErr = fmt.Sprintf("Error deleting %s: %v", m.results[i].Path, err)
		return
	}
	m.deleted[i] = true
	m.totalSaved += m.results[i].Size
}

func (m model) View() string {
	// 1. LOADING SCREEN
	if m.isScanning {
		return fmt.Sprintf("\n  %s Scanning directories in: %s...\n\n", m.spinner.View(), m.dir)
	}

	// 2. HEADER
	logo := selectedStyle.Render(`
  _    __ _    _  _  _
 | |_ / _| | _(_)| || |
 | __| |_| |/ / || || |
 | |_|  _|   <| || || |
  \__|_| |_|\_\_||_||_|
`)

	releasableMB := float64(m.totalReleasable) / 1024 / 1024
	savedMB := float64(m.totalSaved) / 1024 / 1024

	savedLabel := "Space saved:      "
	if m.dryRun {
		savedLabel = "Would free:       "
	}

	dryRunBadge := ""
	if m.dryRun {
		dryRunBadge = "\n" + warningStyle.Bold(true).Render("[ DRY RUN — no files will be deleted ]")
	}

	scanWarn := ""
	if m.scanErr != nil {
		scanWarn = "\n" + warningStyle.Render(fmt.Sprintf("⚠ Some directories could not be read: %v", m.scanErr))
	}

	stats := fmt.Sprintf(
		"Releasable space: %s\n%s%s\nSearch completed  %s%s%s",
		secondaryStyle.Render(fmt.Sprintf("%.2f MB", releasableMB)),
		savedLabel,
		selectedStyle.Render(fmt.Sprintf("%.2f MB", savedMB)),
		grayStyle.Render(fmt.Sprintf("%.2fs", m.scanDuration.Seconds())),
		dryRunBadge,
		scanWarn,
	)

	header := lipgloss.JoinHorizontal(lipgloss.Center, logo, "    ", stats)

	// 3. RESULTS
	s := "\n" + header + "\n\n"

	if len(m.results) == 0 {
		return s + "No unnecessary .terraform folders found.\n"
	}

	for i, res := range m.results {
		statusIcon := "  "
		lineText := res.Path

		if m.deleted[i] {
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
		if res.HasState && !m.deleted[i] {
			warning = warningStyle.Render(" ⚠️ local state")
		}

		sizeStr := secondaryStyle.Render(fmt.Sprintf("%.2f MB", float64(res.Size)/1024/1024))
		s += fmt.Sprintf("%s %-50s %s %s\n", statusIcon, lineText, sizeStr, warning)
	}

	// 4. FOOTER
	if m.deleteErr != "" {
		s += "\n" + warningStyle.Bold(true).Render(m.deleteErr)
	} else if m.confirmMode {
		action := "Delete"
		if m.dryRun {
			action = "Mark as would-delete"
		}
		s += "\n" + warningStyle.Bold(true).Render(fmt.Sprintf("WARNING! This folder has a local state. %s anyway? (y/n)", action))
	} else if m.dryRun {
		s += "\n" + helpStyle.Render("UP/DOWN to select - SPACE to simulate delete - Q to quit")
	} else {
		s += "\n" + helpStyle.Render("UP/DOWN to select - SPACE to delete - Q to quit")
	}

	return s
}
