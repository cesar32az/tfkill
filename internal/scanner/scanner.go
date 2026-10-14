package scanner

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Kind identifies which tool produced a cache directory.
type Kind int

const (
	KindTerraform  Kind = iota // .terraform
	KindTerragrunt             // .terragrunt-cache
)

// Progress is a snapshot of an in-flight scan, reported through
// Options.OnProgress.
type Progress struct {
	DirsVisited int64 // directories walked so far
	Found       int   // cache directories found so far
	Bytes       int64 // total size of the cache directories sized so far
}

// Options tunes a scan. The zero value is valid.
type Options struct {
	// SkipDirs are directory names never descended into
	// (nil means DefaultSkipDirs).
	SkipDirs []string
	// OnProgress, when set, is called from scan goroutines: at most once
	// every ~100ms from the walk, once whenever a result is appended, and
	// once at the end with final totals. It may be called concurrently, so
	// it must be safe for concurrent use and must not block.
	OnProgress func(Progress)
}

// DefaultSkipDirs are heavy directories that never contain Terraform caches
// worth reporting.
var DefaultSkipDirs = []string{"node_modules", ".git", "vendor", ".venv", "venv", "__pycache__"}

// Result holds metadata about a discovered cache directory.
// It does not track UI-level state such as whether the entry has been deleted.
type Result struct {
	Path     string
	Size     int64
	HasState bool      // true when a terraform.tfstate file exists inside the directory (at any depth)
	Kind     Kind      // which tool produced the directory
	ModTime  time.Time // most recent modification time of any entry inside the directory
}

const (
	progressInterval = 100 * time.Millisecond
	stateFileName    = "terraform.tfstate"
)

// Scan walks rootDir and returns all .terraform and .terragrunt-cache directories
// it finds, along with a joined error of any directories that could not be accessed.
// Results are sorted by Size descending (ties broken by Path).
//
// Directories named in opts.SkipDirs are never descended into (rootDir itself is
// never skipped). The walk stops early if ctx is cancelled; results collected up
// to that point are still returned alongside ctx.Err().
func Scan(ctx context.Context, rootDir string, opts Options) ([]Result, error) {
	skipList := opts.SkipDirs
	if skipList == nil {
		skipList = DefaultSkipDirs
	}
	skip := make(map[string]struct{}, len(skipList))
	for _, n := range skipList {
		skip[n] = struct{}{}
	}

	var (
		results  []Result
		mu       sync.Mutex // guards results
		wg       sync.WaitGroup
		walkErrs []error // appended only from the walk goroutine

		dirs     atomic.Int64
		found    atomic.Int64
		total    atomic.Int64
		lastEmit atomic.Int64 // unix nanos of the last throttled emit
	)

	emit := func() {
		if opts.OnProgress != nil {
			opts.OnProgress(Progress{
				DirsVisited: dirs.Load(),
				Found:       int(found.Load()),
				Bytes:       total.Load(),
			})
		}
	}
	// emitThrottled emits at most once per progressInterval across goroutines.
	emitThrottled := func() {
		if opts.OnProgress == nil {
			return
		}
		now := time.Now().UnixNano()
		last := lastEmit.Load()
		if now-last < int64(progressInterval) || !lastEmit.CompareAndSwap(last, now) {
			return
		}
		emit()
	}

	// Semaphore: cap concurrent directory workers to avoid spawning an
	// unbounded number of goroutines on repos with thousands of cache dirs.
	sem := make(chan struct{}, runtime.NumCPU())

	filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return fs.SkipAll
		}
		if err != nil {
			walkErrs = append(walkErrs, err)
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		dirs.Add(1)
		emitThrottled()

		name := d.Name()
		if path != rootDir {
			if _, ok := skip[name]; ok {
				return filepath.SkipDir
			}
		}

		var kind Kind
		switch name {
		case ".terraform":
			kind = KindTerraform
		case ".terragrunt-cache":
			kind = KindTerragrunt
		default:
			return nil
		}

		sem <- struct{}{} // acquire slot before launching goroutine
		wg.Add(1)
		go func(p string, k Kind) {
			defer wg.Done()
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			// Single walk computes size, state and mtime outside the lock.
			res := inspect(ctx, p)
			res.Kind = k

			mu.Lock()
			results = append(results, res)
			mu.Unlock()

			found.Add(1)
			total.Add(res.Size)
			emit()
		}(path, kind)

		return filepath.SkipDir
	})

	wg.Wait()

	if ctxErr := ctx.Err(); ctxErr != nil {
		walkErrs = append(walkErrs, ctxErr)
	}
	emit()

	sort.Slice(results, func(i, j int) bool {
		if results[i].Size != results[j].Size {
			return results[i].Size > results[j].Size
		}
		return results[i].Path < results[j].Path
	})
	return results, errors.Join(walkErrs...)
}

// inspect walks dir once, computing apparent size, whether any file named
// terraform.tfstate exists at any depth, and the newest ModTime of any entry.
func inspect(ctx context.Context, dir string) Result {
	res := Result{Path: dir}
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, _ error) error {
		if ctx.Err() != nil {
			return fs.SkipAll
		}
		if d == nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if mt := info.ModTime(); mt.After(res.ModTime) {
			res.ModTime = mt
		}
		if !d.IsDir() {
			res.Size += info.Size()
			if d.Name() == stateFileName {
				res.HasState = true
			}
		}
		return nil
	})
	return res
}
