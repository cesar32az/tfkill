package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func write(t *testing.T, path string, n int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, n), 0o644); err != nil {
		t.Fatal(err)
	}
}

func byPath(rs []Result) map[string]Result {
	m := map[string]Result{}
	for _, r := range rs {
		m[r.Path] = r
	}
	return m
}

func TestFindsBothKinds(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a", ".terraform", "f"), 10)
	write(t, filepath.Join(root, "b", ".terragrunt-cache", "f"), 10)
	rs, err := Scan(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 2 {
		t.Fatalf("want 2 results, got %d", len(rs))
	}
	m := byPath(rs)
	if m[filepath.Join(root, "a", ".terraform")].Kind != KindTerraform {
		t.Error("wrong kind for .terraform")
	}
	if m[filepath.Join(root, "b", ".terragrunt-cache")].Kind != KindTerragrunt {
		t.Error("wrong kind for .terragrunt-cache")
	}
}

func TestSkipDirs(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "node_modules", "x", ".terraform", "f"), 1)
	write(t, filepath.Join(root, ".git", ".terraform", "f"), 1)
	write(t, filepath.Join(root, "ok", ".terraform", "f"), 1)
	rs, _ := Scan(context.Background(), root, Options{})
	if len(rs) != 1 || rs[0].Path != filepath.Join(root, "ok", ".terraform") {
		t.Fatalf("unexpected results: %+v", rs)
	}
	// A custom skip list replaces the default.
	rs, _ = Scan(context.Background(), root, Options{SkipDirs: []string{"ok"}})
	if len(rs) != 2 {
		t.Fatalf("custom skip: want 2, got %d", len(rs))
	}
}

func TestRootNotSkipped(t *testing.T) {
	root := filepath.Join(t.TempDir(), "node_modules")
	write(t, filepath.Join(root, ".terraform", "f"), 1)
	rs, _ := Scan(context.Background(), root, Options{})
	if len(rs) != 1 {
		t.Fatalf("root must not be skipped, got %d", len(rs))
	}
}

func TestStateDetection(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "nested", ".terragrunt-cache", "h1", "h2", "terraform.tfstate"), 1)
	write(t, filepath.Join(root, "top", ".terraform", "terraform.tfstate"), 1)
	write(t, filepath.Join(root, "backup", ".terraform", "terraform.tfstate.backup"), 1)
	rs, _ := Scan(context.Background(), root, Options{})
	m := byPath(rs)
	if !m[filepath.Join(root, "nested", ".terragrunt-cache")].HasState {
		t.Error("nested state not detected")
	}
	if !m[filepath.Join(root, "top", ".terraform")].HasState {
		t.Error("top-level state not detected")
	}
	if m[filepath.Join(root, "backup", ".terraform")].HasState {
		t.Error(".backup alone must not count")
	}
}

func TestSizeAndModTime(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "p", ".terraform")
	write(t, filepath.Join(dir, "a"), 100)
	write(t, filepath.Join(dir, "sub", "b"), 50)
	old := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	newest := time.Now().Add(-time.Hour).Truncate(time.Second)
	// Set children first: touching entries updates parent mtimes only on
	// create/remove, so Chtimes order does not disturb the others.
	for _, p := range []string{filepath.Join(dir, "a"), filepath.Join(dir, "sub"), dir} {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(filepath.Join(dir, "sub", "b"), newest, newest); err != nil {
		t.Fatal(err)
	}
	rs, _ := Scan(context.Background(), root, Options{})
	if len(rs) != 1 || rs[0].Size != 150 {
		t.Fatalf("size: %+v", rs)
	}
	if !rs[0].ModTime.Equal(newest) {
		t.Errorf("modtime = %v, want %v", rs[0].ModTime, newest)
	}
}

func TestProgress(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a", ".terraform", "f"), 10)
	write(t, filepath.Join(root, "b", ".terraform", "f"), 20)
	var (
		mu    sync.Mutex
		last  Progress
		calls int
	)
	rs, err := Scan(context.Background(), root, Options{OnProgress: func(p Progress) {
		mu.Lock()
		defer mu.Unlock()
		last = p
		calls++
	}})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls == 0 || last.Found != len(rs) || last.Bytes != 30 || last.DirsVisited < 3 {
		t.Errorf("calls=%d last=%+v", calls, last)
	}
}

func TestCancelledContext(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a", ".terraform", "f"), 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Scan(ctx, root, Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestSortedBySizeDesc(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "s", ".terraform", "f"), 10)
	write(t, filepath.Join(root, "l", ".terraform", "f"), 1000)
	write(t, filepath.Join(root, "m", ".terraform", "f"), 100)
	rs, _ := Scan(context.Background(), root, Options{})
	want := []int64{1000, 100, 10}
	if len(rs) != 3 {
		t.Fatalf("want 3, got %d", len(rs))
	}
	for i, r := range rs {
		if r.Size != want[i] {
			t.Fatalf("order wrong: %+v", rs)
		}
	}
}
