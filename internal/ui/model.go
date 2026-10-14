package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cesar32az/tfkill/internal/scanner"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	// Fixed column widths (chars).
	sizeColWidth    = 10 // "1023.45 MB"
	warningColWidth = 16 // " ⚠️ local state"
	iconWidth       = 2  // "▶ " / "✔ "
	rowPadding      = 4  // gaps between columns
	minPathWidth    = 20
	defaultWidth    = 100

	// Below this terminal height the ASCII logo is hidden.
	compactHeaderBelow = 24
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
	offset          int // index of the first result row shown in the list window
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
		m.clampOffset()
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
		m.clampOffset()
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
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "pgup", "ctrl+u", "ctrl+b":
			m.moveCursor(-m.listHeight())
		case "pgdown", "ctrl+d", "ctrl+f":
			m.moveCursor(m.listHeight())
		case "home", "g":
			m.moveCursor(-len(m.results))
		case "end", "G":
			m.moveCursor(len(m.results))
		case " ":
			if len(m.results) == 0 {
				return m, nil
			}
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

// moveCursor shifts the cursor by delta rows (clamped to the result range)
// and scrolls the list window so the cursor stays visible.
func (m *model) moveCursor(delta int) {
	m.cursor += delta
	if m.cursor > len(m.results)-1 {
		m.cursor = len(m.results) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.clampOffset()
}

// clampOffset keeps the cursor inside the visible list window and the
// window inside the result range.
func (m *model) clampOffset() {
	h := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	if maxOff := len(m.results) - h; m.offset > maxOff {
		m.offset = maxOff
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// listHeight returns how many result rows fit between the header and the
// footer. Before the first WindowSizeMsg the height is unknown, so every
// row is shown.
func (m model) listHeight() int {
	if m.height <= 0 {
		return len(m.results)
	}
	// top chrome + scroll indicator line + blank line + footer
	h := m.height - strings.Count(m.headerView(), "\n") - 2 - lipgloss.Height(m.footerView())
	if h < 1 {
		return 1
	}
	return h
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

	s := m.headerView()

	// 2. RESULTS
	if len(m.results) == 0 {
		return s + secondaryStyle.Render("✓  No .terraform or .terragrunt-cache directories found.\n")
	}

	h := m.listHeight()
	end := m.offset + h
	if end > len(m.results) {
		end = len(m.results)
	}
	for i := m.offset; i < end; i++ {
		s += m.fitWidth(m.rowView(i)) + "\n"
	}
	s += m.scrollIndicator(end) + "\n"

	// 3. FOOTER
	return s + "\n" + m.footerView()
}

// headerView renders the logo + stats block, including the blank lines
// around it. The logo is dropped on short terminals so the list keeps room.
func (m model) headerView() string {
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
	if m.height > 0 && m.height < compactHeaderBelow {
		header = lipgloss.NewStyle().PaddingLeft(2).Render(stats)
	}
	return "\n" + m.fitWidth(header) + "\n\n"
}

// rowView renders result i as a single line.
func (m model) rowView(i int) string {
	res := m.results[i]
	pw := m.pathWidth()

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
	case m.deleted[i] && m.cursor == i:
		icon = grayStyle.Render("▶ ")
		pathCol = grayStyle.Render(rawPath)
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

	return icon + pathCol + " " + dots + " " + sizeCol + warnCol
}

// scrollIndicator shows the visible range when the list does not fit.
func (m model) scrollIndicator(end int) string {
	total := len(m.results)
	if m.offset == 0 && end == total {
		return grayStyle.Render(fmt.Sprintf("  %d folders", total))
	}
	more := ""
	if m.offset > 0 {
		more += "  ↑ more above"
	}
	if end < total {
		more += "  ↓ more below"
	}
	return grayStyle.Render(fmt.Sprintf("  %d–%d of %d%s", m.offset+1, end, total, more))
}

func (m model) footerView() string {
	switch {
	case m.deleteErr != "":
		return warningStyle.Bold(true).Width(m.width).Render("✗  " + m.deleteErr)
	case m.confirmMode:
		action := "delete"
		if m.dryRun {
			action = "mark as would-delete"
		}
		return m.fitWidth(warningStyle.Bold(true).Render(
			fmt.Sprintf("⚠  Local state detected. %s anyway? ", action),
		) + renderHelp([][2]string{{"y", "yes"}, {"n / esc", "no"}}))
	}
	action := "delete"
	if m.dryRun {
		action = "simulate delete"
	}
	return m.fitWidth(renderHelp([][2]string{
		{"↑↓ / jk", "navigate"}, {"pgup/pgdn · g/G", "jump"}, {"space", action}, {"q", "quit"},
	}))
}

// fitWidth truncates every line of s to the terminal width so nothing wraps
// and throws off the line count used for scrolling.
func (m model) fitWidth(s string) string {
	if m.width <= 0 {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(m.width).Render(s)
}

// Summary is printed after the program exits, since the alt screen is
// cleared on quit.
func (m model) Summary() string {
	if m.isScanning {
		return ""
	}
	label := "Space saved"
	if m.dryRun {
		label = "Would free"
	}
	deleted := 0
	for _, d := range m.deleted {
		if d {
			deleted++
		}
	}
	return fmt.Sprintf("tfkill: %d of %d folders removed · %s: %s · releasable: %s",
		deleted, len(m.results), label, formatSize(m.totalSaved), formatSize(m.totalReleasable))
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
