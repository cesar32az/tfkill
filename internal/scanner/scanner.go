package scanner

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Result holds metadata about a discovered cache directory.
// It does not track UI-level state such as whether the entry has been deleted.
type Result struct {
	Path     string
	Size     int64
	HasState bool // true when a terraform.tfstate file exists inside the directory
}

// Scan walks rootDir and returns all .terraform and .terragrunt-cache directories
// it finds, along with a joined error of any directories that could not be accessed.
//
// The walk stops early if ctx is cancelled; results collected up to that point
// are still returned alongside ctx.Err().
func Scan(ctx context.Context, rootDir string) ([]Result, error) {
	var (
		results  []Result
		mu       sync.Mutex
		wg       sync.WaitGroup
		walkErrs []error
	)

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

		if d.Name() == ".terraform" || d.Name() == ".terragrunt-cache" {
			sem <- struct{}{} // acquire slot before launching goroutine
			wg.Add(1)
			go func(p string) {
				defer wg.Done()
				defer func() { <-sem }()

				if ctx.Err() != nil {
					return
				}

				// Compute size and state check before taking the lock
				// to avoid holding it during slow I/O.
				size := calculateSize(p)
				_, stateErr := os.Stat(filepath.Join(p, "terraform.tfstate"))

				mu.Lock()
				results = append(results, Result{
					Path:     p,
					Size:     size,
					HasState: stateErr == nil,
				})
				mu.Unlock()
			}(path)

			return filepath.SkipDir
		}
		return nil
	})

	wg.Wait()

	if ctxErr := ctx.Err(); ctxErr != nil {
		walkErrs = append(walkErrs, ctxErr)
	}
	return results, errors.Join(walkErrs...)
}

func calculateSize(path string) int64 {
	var size int64
	filepath.WalkDir(path, func(_ string, d fs.DirEntry, _ error) error {
		if d != nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				size += info.Size()
			}
		}
		return nil
	})
	return size
}
