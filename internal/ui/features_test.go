package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cesar32az/tfkill/internal/scanner"
	tea "github.com/charmbracelet/bubbletea"
)

func modelWith(t *testing.T, opts Options, res []scanner.Result) model {
	t.Helper()
	var tm tea.Model = InitialModel("/repo", opts)
	tm, _ = tm.Update(scanFinishedMsg{results: res})
	tm, _ = tm.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	return tm.(model)
}

func send(m model, msg tea.Msg) (model, tea.Cmd) {
	tm, cmd := m.Update(msg)
	return tm.(model), cmd
}

func sample() []scanner.Result {
	return []scanner.Result{
		{Path: "/repo/b/.terraform", Size: 100},
		{Path: "/repo/A-api/.terraform", Size: 300, HasState: true},
		{Path: "/repo/c/.terragrunt-cache", Size: 200},
	}
}

func order(m model) []string {
	var out []string
	for _, i := range m.view {
		out = append(out, m.items[i].rel)
	}
	return out
}

func TestSortCycleAndReverse(t *testing.T) {
	m := modelWith(t, Options{DryRun: true}, sample())
	if got := order(m); got[0] != "A-api/.terraform" || got[2] != "b/.terraform" {
		t.Fatalf("default size desc: %v", got)
	}
	m = press(m, "S", 1)
	if got := order(m); got[0] != "b/.terraform" {
		t.Fatalf("size asc: %v", got)
	}
	m = press(m, "s", 1) // path
	if got := order(m); got[0] != "A-api/.terraform" || got[1] != "b/.terraform" {
		t.Fatalf("path asc: %v", got)
	}
	if !strings.Contains(m.View(), "sort: path ↑") {
		t.Fatalf("indicator missing sort: %s", m.View())
	}
	m = press(m, "S", 1)
	if got := order(m); got[0] != "c/.terragrunt-cache" {
		t.Fatalf("path desc: %v", got)
	}
}

func TestSortKeepsCursorOnItem(t *testing.T) {
	m := modelWith(t, Options{DryRun: true}, sample())
	m = press(m, "j", 1) // c (200)
	m = press(m, "S", 1)
	i, _ := m.currentItem()
	if m.items[i].rel != "c/.terragrunt-cache" {
		t.Fatalf("cursor moved to %s", m.items[i].rel)
	}
}

func TestSortByAge(t *testing.T) {
	now := time.Now()
	res := []scanner.Result{
		{Path: "/repo/new/.terraform", ModTime: now.Add(-time.Hour)},
		{Path: "/repo/old/.terraform", ModTime: now.Add(-400 * 24 * time.Hour)},
		{Path: "/repo/zero/.terraform"},
	}
	m := modelWith(t, Options{DryRun: true}, res)
	m = press(m, "s", 2)
	if got := order(m); got[0] != "old/.terraform" {
		t.Fatalf("age desc (oldest first): %v", got)
	}
	if !strings.Contains(m.View(), "1y") || !strings.Contains(m.View(), "—") {
		t.Fatalf("age column missing:\n%s", m.View())
	}
}

func TestFilterNarrowsAndNavigates(t *testing.T) {
	m := modelWith(t, Options{DryRun: true}, sample())
	m = press(m, "/", 1)
	if !m.filterInput {
		t.Fatal("filter input not active")
	}
	// j/k/q are text while typing
	for _, k := range []string{"A", "-", "j", "q"} {
		m = press(m, k, 1)
	}
	if m.filter != "A-jq" {
		t.Fatalf("filter = %q", m.filter)
	}
	m, _ = send(m, tea.KeyMsg{Type: tea.KeyBackspace})
	m, _ = send(m, tea.KeyMsg{Type: tea.KeyBackspace})
	m = press(m, "a", 1) // case-insensitive
	m, _ = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.filterInput || m.filter != "A-a" {
		t.Fatalf("state after enter: input=%v filter=%q", m.filterInput, m.filter)
	}
	if len(m.view) != 1 {
		t.Fatalf("view = %v", order(m))
	}
	m = press(m, "j", 5)
	if m.cursor != 0 {
		t.Fatalf("cursor out of range: %d", m.cursor)
	}
	if !strings.Contains(m.View(), `filter: "A-a"`) {
		t.Fatal("indicator missing filter")
	}
	m, _ = send(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.filter != "" || len(m.view) != 3 {
		t.Fatalf("esc did not clear filter")
	}
}

func TestFilterEscClears(t *testing.T) {
	m := modelWith(t, Options{DryRun: true}, sample())
	m = press(m, "/", 1)
	m = press(m, "c", 1)
	if len(m.view) != 1 {
		t.Fatalf("view = %v", order(m))
	}
	m, _ = send(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.filterInput || m.filter != "" || len(m.view) != 3 {
		t.Fatal("esc should clear and exit")
	}
}

func TestRelativePaths(t *testing.T) {
	m := modelWith(t, Options{DryRun: true}, sample())
	v := m.View()
	if !strings.Contains(v, "b/.terraform") || strings.Contains(v, "/repo/b") {
		t.Fatalf("expected relative paths:\n%s", v)
	}
	if got := relPath("rel", "/abs/x"); got != "/abs/x" {
		t.Fatalf("fallback = %q", got)
	}
}

func TestMultiSelectBatchDryRun(t *testing.T) {
	m := modelWith(t, Options{DryRun: true}, sample())
	m = press(m, "a", 1)
	if n, size := m.markedStats(); n != 3 || size != 600 {
		t.Fatalf("marked %d size %d", n, size)
	}
	m = press(m, "a", 1)
	if n, _ := m.markedStats(); n != 0 {
		t.Fatal("a should toggle off")
	}
	m = press(m, "v", 1) // A-api (300, local state)
	m = press(m, "j", 1)
	m = press(m, "v", 1) // c (200)
	m = press(m, " ", 1)
	if !m.batchConfirm {
		t.Fatal("batch confirm not shown")
	}
	if v := m.View(); !strings.Contains(v, "2 folders") || !strings.Contains(v, "1 contain local state") {
		t.Fatalf("confirm text:\n%s", v)
	}
	m = press(m, "y", 1)
	if m.batchConfirm || m.totalSaved != 500 {
		t.Fatalf("saved = %d", m.totalSaved)
	}
	if !strings.Contains(m.Summary(), "2 of 3") {
		t.Fatal(m.Summary())
	}
}

func TestSingleDeleteStateConfirm(t *testing.T) {
	m := modelWith(t, Options{DryRun: true}, sample())
	m = press(m, " ", 1) // A-api has local state
	if !m.confirmMode {
		t.Fatal("expected confirm")
	}
	m = press(m, "y", 1)
	if m.totalSaved != 300 {
		t.Fatalf("saved %d", m.totalSaved)
	}
}

func TestAsyncDelete(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a", ".terraform")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	var tm tea.Model = InitialModel(dir, Options{})
	tm, _ = tm.Update(scanFinishedMsg{results: []scanner.Result{{Path: p, Size: 10}}})
	m := tm.(model)
	space := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")}
	m, cmd := send(m, space)
	if cmd == nil || m.items[0].state != stateDeleting {
		t.Fatal("expected in-flight delete")
	}
	if !strings.Contains(m.View(), "deleting…") {
		t.Fatal("deleting marker missing")
	}
	if _, c2 := send(m, space); c2 != nil {
		t.Fatal("second delete should be a no-op")
	}
	msg := cmd()
	if _, ok := msg.(deleteFinishedMsg); !ok {
		t.Fatalf("unexpected msg %T", msg)
	}
	m, _ = send(m, msg)
	if m.items[0].state != stateDeleted || m.totalSaved != 10 {
		t.Fatal("row not marked deleted")
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("folder still exists")
	}
	// error path restores the row
	m2 := modelWith(t, Options{}, sample())
	m2.items[0].state = stateDeleting
	m2.inflight = 1
	m2, _ = send(m2, deleteFinishedMsg{index: 0, err: fmt.Errorf("boom")})
	if m2.items[0].state != stateIdle || m2.deleteErr == "" {
		t.Fatal("error should restore row")
	}
}

func TestOlderThan(t *testing.T) {
	now := time.Now()
	res := []scanner.Result{
		{Path: "/repo/new/.terraform", Size: 1, ModTime: now.Add(-24 * time.Hour)},
		{Path: "/repo/old/.terraform", Size: 2, ModTime: now.Add(-60 * 24 * time.Hour)},
		{Path: "/repo/zero/.terraform", Size: 4},
	}
	m := modelWith(t, Options{DryRun: true, OlderThan: 30 * 24 * time.Hour}, res)
	if got := order(m); len(got) != 2 {
		t.Fatalf("view = %v", got)
	}
	if m.totalReleasable != 6 {
		t.Fatalf("releasable = %d", m.totalReleasable)
	}
}

func TestParseAge(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"30d": 30 * 24 * time.Hour, "2w": 14 * 24 * time.Hour, "12h": 12 * time.Hour, "90m": 90 * time.Minute,
	} {
		got, err := ParseAge(in)
		if err != nil || got != want {
			t.Fatalf("ParseAge(%q) = %v, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "xd", "-3d", "abc"} {
		if _, err := ParseAge(in); err == nil {
			t.Fatalf("ParseAge(%q) should fail", in)
		}
	}
}

func TestProgressTick(t *testing.T) {
	m := InitialModel("/repo", Options{})
	m.progress.Set(scanner.Progress{DirsVisited: 42, Found: 3, Bytes: 5 << 20})
	m, cmd := send(m, progressTickMsg{})
	if cmd == nil {
		t.Fatal("tick should reschedule")
	}
	if v := m.View(); !strings.Contains(v, "42 dirs · 3 found · 5.00 MB") {
		t.Fatalf("scan view: %s", v)
	}
}

func TestViewHeightInAllModes(t *testing.T) {
	for _, height := range []int{12, 24, 40} {
		m := scrolledModel(t, 100, 100, height)
		check := func(name string, m model) {
			if got := strings.Count(m.View(), "\n") + 1; got > height {
				t.Fatalf("height %d %s: %d lines", height, name, got)
			}
		}
		check("normal", m)
		f := press(m, "/", 1)
		f = press(f, "9", 1)
		check("filter-input", f)
		f = press(f, "z", 3) // no matches
		check("filter-empty", f)
		b := press(m, "a", 1)
		b = press(b, "d", 1)
		if !b.batchConfirm {
			t.Fatal("batch confirm expected")
		}
		check("batch-confirm", b)
	}
	m := modelWith(t, Options{DryRun: true}, sample())
	m.height = 12
	m = press(m, " ", 1)
	if !m.confirmMode {
		t.Fatal("confirm expected")
	}
	if got := strings.Count(m.View(), "\n") + 1; got > 12 {
		t.Fatalf("confirm mode: %d lines", got)
	}
}
