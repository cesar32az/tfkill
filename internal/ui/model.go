package ui

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cesar32az/tfkill/internal/scanner"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	// Fixed column widths (chars).
	sizeColWidth    = 10 // "1023.45 MB"
	ageColWidth     = 5  // "12mo"
	warningColWidth = 16 // " ⚠️ local state"
	iconWidth       = 2  // "▶ " / "✔ "
	markWidth       = 2  // "● "
	rowPadding      = 4  // gaps between columns
	minPathWidth    = 20
	defaultWidth    = 100

	// Below this terminal height the ASCII logo is hidden.
	compactHeaderBelow = 24

	// Maximum number of folders removed concurrently.
	maxConcurrentDeletes = 4

	progressInterval = 100 * time.Millisecond
)

// Options configures the TUI.
type Options struct {
	DryRun    bool
	OlderThan time.Duration // hide results modified more recently than this (0 = off)
}

// scanFinishedMsg is sent by runScan when the scanner completes.
type scanFinishedMsg struct {
	results  []scanner.Result
	duration time.Duration
	err      error
}

// deleteFinishedMsg is sent when an asynchronous removal completes.
// index refers to the model's items slice.
type deleteFinishedMsg struct {
	index int
	err   error
}

// progressTickMsg triggers a refresh of the live scan progress.
type progressTickMsg struct{}

// progressHolder stores the latest scanner progress. The scanner calls
// Set from its worker goroutines; the UI reads it on a ticker.
type progressHolder struct {
	mu sync.Mutex
	p  scanner.Progress
}

func (h *progressHolder) Set(p scanner.Progress) {
	h.mu.Lock()
	h.p = p
	h.mu.Unlock()
}

func (h *progressHolder) Get() scanner.Progress {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.p
}

type rowState int

const (
	stateIdle rowState = iota
	stateDeleting
	stateDeleted
)

type sortKey int

const (
	sortSize sortKey = iota
	sortPath
	sortAge
)

func (k sortKey) String() string {
	switch k {
	case sortPath:
		return "path"
	case sortAge:
		return "age"
	default:
		return "size"
	}
}

// item is one scan result plus its UI state.
type item struct {
	res    scanner.Result
	rel    string // path relative to the scanned dir (display)
	state  rowState
	marked bool
}

type model struct {
	dir        string
	dryRun     bool
	olderThan  time.Duration
	ctx        context.Context
	cancelScan context.CancelFunc
	progress   *progressHolder

	width  int
	height int

	items           []item
	view            []int // indices into items: filtered + sorted
	deleteErr       string
	scanErr         error
	cursor          int // index into view
	offset          int // index of the first view row shown in the list window
	totalSaved      int64
	totalReleasable int64

	confirmMode  bool // single-row confirmation (local state)
	batchConfirm bool // batch deletion confirmation

	sortBy      sortKey
	sortDesc    bool
	filter      string
	filterInput bool

	queue    []int // items waiting for a delete slot
	inflight int
	quitting bool // q pressed while removals were running; exit when they finish

	isScanning   bool
	scanDuration time.Duration
	scanStats    scanner.Progress
	spinner      spinner.Model
}

func InitialModel(dir string, opts Options) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = warningStyle

	ctx, cancel := context.WithCancel(context.Background())

	return model{
		dir:        dir,
		dryRun:     opts.DryRun,
		olderThan:  opts.OlderThan,
		ctx:        ctx,
		cancelScan: cancel,
		progress:   &progressHolder{},
		isScanning: true,
		spinner:    s,
		width:      defaultWidth,
		sortBy:     sortSize,
		sortDesc:   true,
	}
}

// ParseAge parses a duration accepting Go syntax plus "d" (days) and
// "w" (weeks) suffixes, e.g. 30d, 2w, 12h.
func ParseAge(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	var unit time.Duration
	switch s[len(s)-1] {
	case 'd':
		unit = 24 * time.Hour
	case 'w':
		unit = 7 * 24 * time.Hour
	}
	if unit != 0 {
		n, err := strconv.ParseFloat(s[:len(s)-1], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		if n < 0 {
			return 0, fmt.Errorf("negative duration %q", s)
		}
		return time.Duration(n * float64(unit)), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	if d < 0 {
		return 0, fmt.Errorf("negative duration %q", s)
	}
	return d, nil
}

func runScan(ctx context.Context, dir string, h *progressHolder) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		res, err := scanner.Scan(ctx, dir, scanner.Options{OnProgress: h.Set})
		return scanFinishedMsg{results: res, duration: time.Since(start), err: err}
	}
}

func progressTick() tea.Cmd {
	return tea.Tick(progressInterval, func(time.Time) tea.Msg { return progressTickMsg{} })
}

func removeCmd(index int, path string) tea.Cmd {
	return func() tea.Msg {
		return deleteFinishedMsg{index: index, err: os.RemoveAll(path)}
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, runScan(m.ctx, m.dir, m.progress), progressTick())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampOffset()
		return m, nil

	case progressTickMsg:
		if !m.isScanning {
			return m, nil
		}
		m.scanStats = m.progress.Get()
		return m, progressTick()

	case scanFinishedMsg:
		m.cancelScan()
		now := time.Now()
		m.items = m.items[:0]
		for _, r := range msg.results {
			if m.olderThan > 0 && !r.ModTime.IsZero() && now.Sub(r.ModTime) < m.olderThan {
				continue
			}
			m.items = append(m.items, item{res: r, rel: relPath(m.dir, r.Path)})
			m.totalReleasable += r.Size
		}
		m.scanDuration = msg.duration
		if msg.err != nil && msg.err != context.Canceled {
			m.scanErr = msg.err
		}
		m.isScanning = false
		m.rebuild()
		return m, nil

	case deleteFinishedMsg:
		m.inflight--
		it := &m.items[msg.index]
		if msg.err != nil {
			it.state = stateIdle
			m.deleteErr = fmt.Sprintf("cannot delete %s: %v", it.res.Path, msg.err)
		} else {
			it.state = stateDeleted
			m.totalSaved += it.res.Size
		}
		if m.quitting {
			if m.inflight == 0 {
				return m, tea.Quit
			}
			return m, nil
		}
		return m, m.pump()

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

		if m.quitting {
			// Second ctrl+c forces exit even if removals are still running.
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}

		if m.filterInput {
			return m.updateFilterInput(msg)
		}

		if m.batchConfirm {
			switch msg.String() {
			case "y", "Y":
				m.batchConfirm = false
				return m, m.enqueue(m.markedIndices())
			case "n", "N", "esc":
				m.batchConfirm = false
			}
			return m, nil
		}

		if m.confirmMode {
			switch msg.String() {
			case "y", "Y":
				m.confirmMode = false
				if i, ok := m.currentItem(); ok {
					return m, m.enqueue([]int{i})
				}
			case "n", "N", "esc":
				m.confirmMode = false
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m.quit()
		case "up", "k":
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "pgup", "ctrl+u", "ctrl+b":
			m.moveCursor(-m.listHeight())
		case "pgdown", "ctrl+d", "ctrl+f":
			m.moveCursor(m.listHeight())
		case "home", "g":
			m.moveCursor(-len(m.view))
		case "end", "G":
			m.moveCursor(len(m.view))
		case "s":
			m.sortBy = (m.sortBy + 1) % 3
			m.sortDesc = m.sortBy != sortPath
			m.rebuild()
		case "S":
			m.sortDesc = !m.sortDesc
			m.rebuild()
		case "/":
			m.filterInput = true
		case "esc":
			if m.filter != "" {
				m.filter = ""
				m.rebuild()
			}
		case "v":
			if i, ok := m.currentItem(); ok && m.items[i].state == stateIdle {
				m.items[i].marked = !m.items[i].marked
			}
		case "a":
			m.toggleMarkAll()
		case "d":
			if n, _ := m.markedStats(); n > 0 {
				m.batchConfirm = true
				return m, nil
			}
			return m.deleteCurrent()
		case " ":
			if n, _ := m.markedStats(); n > 0 {
				m.batchConfirm = true
				return m, nil
			}
			return m.deleteCurrent()
		}
	}
	return m, nil
}

// updateFilterInput handles keys while the filter prompt is active.
func (m model) updateFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "enter":
		m.filterInput = false
	case "esc":
		m.filterInput = false
		m.filter = ""
		m.rebuild()
	case "backspace":
		if r := []rune(m.filter); len(r) > 0 {
			m.filter = string(r[:len(r)-1])
			m.rebuild()
		}
	case " ":
		m.filter += " "
		m.rebuild()
	default:
		if msg.Type == tea.KeyRunes {
			m.filter += string(msg.Runes)
			m.rebuild()
		}
	}
	return m, nil
}

// quit exits right away, or — when removals are running — drops the
// pending queue and waits for in-flight ones so no folder is left
// half-deleted.
func (m model) quit() (tea.Model, tea.Cmd) {
	if m.inflight == 0 {
		return m, tea.Quit
	}
	for _, i := range m.queue {
		m.items[i].state = stateIdle
	}
	m.queue = nil
	m.quitting = true
	return m, nil
}

// deleteCurrent handles space/d on the cursor row when nothing is marked.
func (m model) deleteCurrent() (tea.Model, tea.Cmd) {
	i, ok := m.currentItem()
	if !ok || m.items[i].state != stateIdle {
		return m, nil
	}
	if m.items[i].res.HasState {
		m.confirmMode = true
		return m, nil
	}
	return m, m.enqueue([]int{i})
}

// currentItem returns the items index under the cursor.
func (m model) currentItem() (int, bool) {
	if m.cursor < 0 || m.cursor >= len(m.view) {
		return 0, false
	}
	return m.view[m.cursor], true
}

// markedIndices lists marked, still-idle items (including ones hidden by
// the current filter).
func (m model) markedIndices() []int {
	var out []int
	for i, it := range m.items {
		if it.marked && it.state == stateIdle {
			out = append(out, i)
		}
	}
	return out
}

// markedStats returns how many items are marked, and their total size.
func (m model) markedStats() (n int, size int64) {
	for _, it := range m.items {
		if it.marked && it.state == stateIdle {
			n++
			size += it.res.Size
		}
	}
	return n, size
}

// markedHidden counts marked, idle items not in the current filtered view.
func (m model) markedHidden() int {
	visible := make(map[int]bool, len(m.view))
	for _, i := range m.view {
		visible[i] = true
	}
	n := 0
	for i, it := range m.items {
		if it.marked && it.state == stateIdle && !visible[i] {
			n++
		}
	}
	return n
}

func (m model) markedWithState() int {
	n := 0
	for _, it := range m.items {
		if it.marked && it.state == stateIdle && it.res.HasState {
			n++
		}
	}
	return n
}

// toggleMarkAll marks every visible non-deleted row, or unmarks them all
// if they were already all marked.
func (m *model) toggleMarkAll() {
	all := true
	found := false
	for _, i := range m.view {
		if m.items[i].state != stateIdle {
			continue
		}
		found = true
		if !m.items[i].marked {
			all = false
		}
	}
	if !found {
		return
	}
	for _, i := range m.view {
		if m.items[i].state == stateIdle {
			m.items[i].marked = !all
		}
	}
}

// enqueue schedules the given items for deletion and starts as many
// removals as the concurrency limit allows. In dry-run mode rows are
// marked deleted immediately.
func (m *model) enqueue(indices []int) tea.Cmd {
	for _, i := range indices {
		it := &m.items[i]
		if it.state != stateIdle {
			continue
		}
		it.marked = false
		if m.dryRun {
			it.state = stateDeleted
			m.totalSaved += it.res.Size
			continue
		}
		it.state = stateDeleting
		m.queue = append(m.queue, i)
	}
	return m.pump()
}

// pump starts queued removals up to the concurrency limit.
func (m *model) pump() tea.Cmd {
	var cmds []tea.Cmd
	for m.inflight < maxConcurrentDeletes && len(m.queue) > 0 {
		i := m.queue[0]
		m.queue = m.queue[1:]
		m.inflight++
		cmds = append(cmds, removeCmd(i, m.items[i].res.Path))
	}
	return tea.Batch(cmds...)
}

// relPath returns path relative to dir, falling back to path itself.
func relPath(dir, path string) string {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return path
	}
	return rel
}

// rebuild recomputes the filtered + sorted view, keeping the cursor on the
// same item when it is still visible.
func (m *model) rebuild() {
	cur := -1
	if m.cursor >= 0 && m.cursor < len(m.view) {
		cur = m.view[m.cursor]
	}

	needle := strings.ToLower(m.filter)
	view := make([]int, 0, len(m.items))
	for i, it := range m.items {
		if needle == "" || strings.Contains(strings.ToLower(it.rel), needle) {
			view = append(view, i)
		}
	}

	now := time.Now()
	age := func(r scanner.Result) time.Duration {
		if r.ModTime.IsZero() {
			return 0
		}
		return now.Sub(r.ModTime)
	}
	sort.SliceStable(view, func(a, b int) bool {
		x, y := m.items[view[a]], m.items[view[b]]
		var c int
		switch m.sortBy {
		case sortPath:
			c = strings.Compare(x.rel, y.rel)
		case sortAge:
			c = cmp.Compare(age(x.res), age(y.res))
		default:
			c = cmp.Compare(x.res.Size, y.res.Size)
		}
		if m.sortDesc {
			c = -c
		}
		if c == 0 {
			c = strings.Compare(x.rel, y.rel)
		}
		return c < 0
	})
	m.view = view

	for pos, i := range view {
		if i == cur {
			m.cursor = pos
			break
		}
	}
	if m.cursor > len(view)-1 {
		m.cursor = len(view) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.clampOffset()
}

// moveCursor shifts the cursor by delta rows (clamped to the view range)
// and scrolls the list window so the cursor stays visible.
func (m *model) moveCursor(delta int) {
	m.cursor += delta
	if m.cursor > len(m.view)-1 {
		m.cursor = len(m.view) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.clampOffset()
}

// clampOffset keeps the cursor inside the visible list window and the
// window inside the view range.
func (m *model) clampOffset() {
	h := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	if maxOff := len(m.view) - h; m.offset > maxOff {
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
		return len(m.view)
	}
	// top chrome + scroll indicator line + blank line + footer
	h := m.height - strings.Count(m.headerView(), "\n") - 2 - lipgloss.Height(m.footerView())
	if h < 1 {
		return 1
	}
	return h
}

// pathWidth returns the available width for the path column given the
// current terminal width.
func (m model) pathWidth() int {
	fixed := iconWidth + markWidth + sizeColWidth + ageColWidth + 1 + warningColWidth + rowPadding
	w := m.width - fixed
	if w < minPathWidth {
		return minPathWidth
	}
	return w
}

func (m model) View() string {
	// 1. LOADING SCREEN
	if m.isScanning {
		return fmt.Sprintf("\n  %s Scanning %s…  %s\n\n", m.spinner.View(), grayStyle.Render(m.dir),
			grayStyle.Render(fmt.Sprintf("%d dirs · %d found · %s",
				m.scanStats.DirsVisited, m.scanStats.Found, formatSize(m.scanStats.Bytes))))
	}

	s := m.headerView()

	// 2. RESULTS
	if len(m.items) == 0 {
		return s + secondaryStyle.Render("✓  No .terraform or .terragrunt-cache directories found.\n")
	}
	if len(m.view) == 0 {
		return s + m.fitWidth(grayStyle.Render("  No folders match the filter.")) + "\n" +
			m.fitWidth(m.scrollIndicator(0)) + "\n" + m.footerView()
	}

	h := m.listHeight()
	end := m.offset + h
	if end > len(m.view) {
		end = len(m.view)
	}
	for i := m.offset; i < end; i++ {
		s += m.fitWidth(m.rowView(i)) + "\n"
	}
	s += m.fitWidth(m.scrollIndicator(end)) + "\n"

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

// rowView renders view row vi as a single line.
func (m model) rowView(vi int) string {
	it := m.items[m.view[vi]]
	pw := m.pathWidth()

	// Truncate path to pw runes before styling so lipgloss.Width()
	// returns the true visible length we need for dot-leader math.
	rawPath := it.rel
	pathRunes := []rune(rawPath)
	if len(pathRunes) > pw {
		rawPath = string(pathRunes[:pw-1]) + "…"
	}
	visLen := len([]rune(rawPath))

	icon := "  "
	var pathCol string
	atCursor := m.cursor == vi
	switch {
	case it.state != stateIdle && atCursor:
		icon = grayStyle.Render("▶ ")
		pathCol = grayStyle.Render(rawPath)
	case it.state == stateDeleted:
		icon = secondaryStyle.Render("✔ ")
		pathCol = grayStyle.Render(rawPath)
	case it.state == stateDeleting:
		pathCol = grayStyle.Render(rawPath)
	case atCursor && (m.confirmMode || m.batchConfirm):
		icon = selectedStyle.Render("▶ ")
		pathCol = warningStyle.Bold(true).Render(rawPath)
	case atCursor:
		icon = selectedStyle.Render("▶ ")
		pathCol = selectedStyle.Render(rawPath)
	default:
		pathCol = rawPath
	}

	mark := "  "
	if it.marked {
		mark = warningStyle.Render("● ")
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
		Render(formatSize(it.res.Size))

	ageCol := lipgloss.NewStyle().
		Width(ageColWidth).
		Align(lipgloss.Right).
		Foreground(subtle).
		Render(formatAge(it.res.ModTime, time.Now()))

	// Status column
	status := ""
	switch {
	case it.state == stateDeleting:
		status = warningStyle.Render(" deleting…")
	case it.res.HasState && it.state == stateIdle:
		status = warningStyle.Render(" ⚠  local state")
	}

	return icon + mark + pathCol + " " + dots + " " + sizeCol + " " + ageCol + status
}

// formatAge humanizes the time elapsed since t ("3d", "5mo", "2y").
func formatAge(t, now time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := now.Sub(t)
	switch {
	case d < time.Hour:
		return "<1h"
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d/(30*24*time.Hour)))
	default:
		return fmt.Sprintf("%dy", int(d/(365*24*time.Hour)))
	}
}

// scrollIndicator shows the visible range plus sort/filter/mark status.
func (m model) scrollIndicator(end int) string {
	total := len(m.view)
	var s string
	if m.offset == 0 && end == total {
		s = fmt.Sprintf("  %d folders", total)
	} else {
		more := ""
		if m.offset > 0 {
			more += "  ↑ more above"
		}
		if end < total {
			more += "  ↓ more below"
		}
		s = fmt.Sprintf("  %d–%d of %d%s", m.offset+1, end, total, more)
	}
	arrow := "↑"
	if m.sortDesc {
		arrow = "↓"
	}
	s += fmt.Sprintf(" · sort: %s %s", m.sortBy, arrow)
	if m.filter != "" {
		s += fmt.Sprintf(" · filter: %q", m.filter)
	}
	if n, _ := m.markedStats(); n > 0 {
		s += fmt.Sprintf(" · %d marked", n)
	}
	return grayStyle.Render(s)
}

func (m model) footerView() string {
	switch {
	case m.quitting:
		return m.fitWidth(warningStyle.Bold(true).Render(
			fmt.Sprintf("Waiting for %d deletion(s) to finish… ", m.inflight),
		) + renderHelp([][2]string{{"ctrl+c", "force quit"}}))
	case m.deleteErr != "":
		return warningStyle.Bold(true).Width(m.width).Render("✗  " + m.deleteErr)
	case m.filterInput:
		return m.fitWidth("  " + selectedStyle.Render("/") + m.filter + selectedStyle.Render("█") + "  " +
			renderHelp([][2]string{{"enter", "apply"}, {"esc", "clear"}}))
	case m.batchConfirm:
		n, size := m.markedStats()
		verb := "Delete"
		if m.dryRun {
			verb = "Mark as would-delete:"
		}
		txt := fmt.Sprintf("⚠  %s %d folders (%s)?", verb, n, formatSize(size))
		if h := m.markedHidden(); h > 0 {
			txt += fmt.Sprintf(" %d hidden by filter.", h)
		}
		if k := m.markedWithState(); k > 0 {
			txt += fmt.Sprintf(" %d contain local state.", k)
		}
		return m.fitWidth(warningStyle.Bold(true).Render(txt+" ") +
			renderHelp([][2]string{{"y", "yes"}, {"n / esc", "no"}}))
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
		action = "dry del"
	}
	return m.fitWidth(renderHelp([][2]string{
		{"↑↓", "nav"}, {"space", action}, {"v/a", "mark"}, {"d", "batch"},
		{"s/S", "sort"}, {"/", "filter"}, {"q", "quit"},
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
	for _, it := range m.items {
		if it.state == stateDeleted {
			deleted++
		}
	}
	return fmt.Sprintf("tfkill: %d of %d folders removed · %s: %s · releasable: %s",
		deleted, len(m.items), label, formatSize(m.totalSaved), formatSize(m.totalReleasable))
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
