package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cesar32az/tfkill/internal/scanner"
	tea "github.com/charmbracelet/bubbletea"
)

func scrolledModel(t *testing.T, n, width, height int) model {
	t.Helper()
	res := make([]scanner.Result, n)
	for i := range res {
		res[i] = scanner.Result{Path: fmt.Sprintf("/repo/stack-%03d/.terraform", i), Size: 1024}
	}
	var tm tea.Model = InitialModel("/repo", true)
	tm, _ = tm.Update(scanFinishedMsg{results: res})
	tm, _ = tm.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return tm.(model)
}

func press(m model, key string, times int) model {
	var tm tea.Model = m
	for i := 0; i < times; i++ {
		tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	}
	return tm.(model)
}

func TestViewFitsTerminalAndKeepsCursorVisible(t *testing.T) {
	for _, height := range []int{12, 30, 50} {
		m := scrolledModel(t, 200, 100, height)
		for _, step := range []struct {
			key   string
			times int
		}{{"j", 75}, {"k", 30}, {"G", 1}, {"g", 1}} {
			m = press(m, step.key, step.times)
			v := m.View()
			if got := strings.Count(v, "\n") + 1; got > height {
				t.Fatalf("height %d after %q: view has %d lines", height, step.key, got)
			}
			if !strings.Contains(v, "Releasable:") {
				t.Fatalf("height %d after %q: header not visible", height, step.key)
			}
			cur := fmt.Sprintf("stack-%03d", m.cursor)
			if !strings.Contains(v, cur) {
				t.Fatalf("height %d after %q: cursor row %s not visible", height, step.key, cur)
			}
		}
	}
}

func TestCursorMovesWithJK(t *testing.T) {
	m := scrolledModel(t, 5, 100, 40)
	m = press(m, "j", 3)
	if m.cursor != 3 {
		t.Fatalf("cursor = %d, want 3", m.cursor)
	}
	m = press(m, "j", 10)
	if m.cursor != 4 {
		t.Fatalf("cursor = %d, want 4 (clamped)", m.cursor)
	}
	m = press(m, "k", 10)
	if m.cursor != 0 {
		t.Fatalf("cursor = %d, want 0 (clamped)", m.cursor)
	}
}
