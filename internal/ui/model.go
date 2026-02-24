package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cesar32az/tfkill/internal/scanner"
)

const (
	// Fixed column widths (chars).
	sizeColWidth    = 10 // "1023.45 MB"
	warningColWidth = 16 // " ⚠️ local state"
	iconWidth       = 2  // "▶ " / "✔ "
	rowPadding      = 4  // gaps between columns
	minPathWidth    = 20
	defaultWidth    = 100
)

// scanFinishedMsg is sent by runScan when the scanner completes.
type scanFinishedMsg struct {
	results  []scanner.Result
	duration time.Duration
	err      error
}

type model struct {
	dir        string
	dryRun     bool
	ctx        context.Context
	cancelScan context.CancelFunc

	width  int
	height int

	results         []scanner.Result
	deleted         []bool
	deleteErr       string
	scanErr         error
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

	ctx, cancel := context.WithCancel(context.Background())

	return model{
		dir:        dir,
		dryRun:     dryRun,
		ctx:        ctx,
		cancelScan: cancel,
		isScanning: true,
		spinner:    s,
		width:      defaultWidth,
	}
}

func runScan(ctx context.Context, dir string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		res, err := scanner.Scan(ctx, dir)
		return scanFinishedMsg{results: res, duration: time.Since(start), err: err}
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, runScan(m.ctx, m.dir))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case scanFinishedMsg:
		m.cancelScan()
		m.results = make([]scanner.Result, len(msg.results))
		copy(m.results, msg.results)
		m.deleted = make([]bool, len(msg.results))
		m.scanDuration = msg.duration
		if msg.err != nil && msg.err != context.Canceled {
			m.scanErr = msg.err
		}
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
				m.cancelScan()
				return m, tea.Quit
			}
			return m, nil
		}

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
		m.deleteErr = fmt.Sprintf("cannot delete %s: %v", m.results[i].Path, err)
		return
	}
	m.deleted[i] = true
	m.totalSaved += m.results[i].Size
}

// pathWidth returns the available width for the path column given the
// current terminal width.
func (m model) pathWidth() int {
	fixed := iconWidth + sizeColWidth + warningColWidth + rowPadding
	w := m.width - fixed
	if w < minPathWidth {
		return minPathWidth
	}
	return w
}

func (m model) View() string {
	// 1. LOADING SCREEN
	if m.isScanning {
		return fmt.Sprintf("\n  %s Scanning %s…\n\n", m.spinner.View(), grayStyle.Render(m.dir))
	}

	// 2. HEADER
	logo := selectedStyle.Render(`
  _    __ _    _  _  _
 | |_ / _| | _(_)| || |
 | __| |_| |/ / || || |
 | |_|  _|   <| || || |
  \__|_| |_|\_\_||_||_|
`)

	savedLabel := "Space saved:  "
	if m.dryRun {
		savedLabel = "Would free:   "
	}

	dryRunBadge := ""
	if m.dryRun {
		dryRunBadge = "\n" + warningStyle.Bold(true).Render("[ DRY RUN — no files will be deleted ]")
	}

	scanWarn := ""
	if m.scanErr != nil {
		scanWarn = "\n" + warningStyle.Render(fmt.Sprintf("⚠  Some directories could not be read: %v", m.scanErr))
	}

	stats := fmt.Sprintf(
		"Releasable:   %s\n%s%s\nCompleted in  %s%s%s",
		secondaryStyle.Render(formatSize(m.totalReleasable)),
		savedLabel,
		selectedStyle.Render(formatSize(m.totalSaved)),
		grayStyle.Render(fmt.Sprintf("%.2fs", m.scanDuration.Seconds())),
		dryRunBadge,
		scanWarn,
	)

	header := lipgloss.JoinHorizontal(lipgloss.Center, logo, "    ", stats)
	s := "\n" + header + "\n\n"

	// 3. RESULTS
	if len(m.results) == 0 {
		return s + secondaryStyle.Render("✓  No .terraform or .terragrunt-cache directories found.\n")
	}

	pw := m.pathWidth()

	for i, res := range m.results {
		// Icon column
		icon := "  "

		// Truncate path to pw runes before styling so lipgloss.Width()
		// returns the true visible length we need for dot-leader math.
		rawPath := res.Path
		pathRunes := []rune(rawPath)
		if len(pathRunes) > pw {
			rawPath = string(pathRunes[:pw-1]) + "…"
		}
		visLen := len([]rune(rawPath))

		// Style the (already-truncated) path.
		var pathCol string
		switch {
		case m.deleted[i]:
			icon = secondaryStyle.Render("✔ ")
			pathCol = grayStyle.Render(rawPath)
		case m.cursor == i && m.confirmMode:
			icon = selectedStyle.Render("▶ ")
			pathCol = warningStyle.Bold(true).Render(rawPath)
		case m.cursor == i:
			icon = selectedStyle.Render("▶ ")
			pathCol = selectedStyle.Render(rawPath)
		default:
			pathCol = rawPath
		}

		// Dot leader: fills the gap between path and size column.
		dotsCount := pw - visLen - 2 // -2 for the spaces flanking the dots
		if dotsCount < 1 {
			dotsCount = 1
		}
		dots := grayStyle.Render(strings.Repeat("·", dotsCount))

		// Size column — right-aligned, fixed width
		sizeCol := lipgloss.NewStyle().
			Width(sizeColWidth).
			Align(lipgloss.Right).
			Foreground(green).
			Render(formatSize(res.Size))

		// Warning column
		warnCol := ""
		if res.HasState && !m.deleted[i] {
			warnCol = warningStyle.Render(" ⚠  local state")
		}

		s += icon + pathCol + " " + dots + " " + sizeCol + warnCol + "\n"
	}

	// 4. FOOTER
	s += "\n"
	if m.deleteErr != "" {
		s += warningStyle.Bold(true).Render("✗  " + m.deleteErr)
	} else if m.confirmMode {
		action := "delete"
		if m.dryRun {
			action = "mark as would-delete"
		}
		s += warningStyle.Bold(true).Render(
			fmt.Sprintf("⚠  Local state detected. %s anyway? ", action),
		)
		s += renderHelp([][2]string{{"y", "yes"}, {"n / esc", "no"}})
	} else if m.dryRun {
		s += renderHelp([][2]string{{"↑↓ / jk", "navigate"}, {"space", "simulate delete"}, {"q", "quit"}})
	} else {
		s += renderHelp([][2]string{{"↑↓ / jk", "navigate"}, {"space", "delete"}, {"q", "quit"}})
	}

	return s
}

// formatSize returns a human-readable size string that adapts its unit
// to the magnitude of bytes: KB below 1 MB, MB below 1 GB, GB otherwise.
func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.2f GB", float64(bytes)/gb)
	case bytes >= mb:
		return fmt.Sprintf("%.2f MB", float64(bytes)/mb)
	default:
		return fmt.Sprintf("%.1f KB", float64(bytes)/kb)
	}
}
