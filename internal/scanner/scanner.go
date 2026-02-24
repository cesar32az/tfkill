package scanner

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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
func Scan(rootDir string) ([]Result, error) {
	var (
		results  []Result
		mu       sync.Mutex
		wg       sync.WaitGroup
		walkErrs []error
	)

	filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			walkErrs = append(walkErrs, err)
			return nil
		}
		if !d.IsDir() {
			return nil
		}

		if d.Name() == ".terraform" || d.Name() == ".terragrunt-cache" {
			wg.Add(1)
			go func(p string) {
				defer wg.Done()

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
