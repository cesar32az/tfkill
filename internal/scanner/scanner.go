package scanner

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

type Result struct {
	Path     string
	Size     int64
	HasState bool // ⚠️ Indicates if there is a local terraform.tfstate
	Deleted  bool
}

func Scan(rootDir string) []Result {
	var results []Result
	var mu sync.Mutex
	var wg sync.WaitGroup

	filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() { return nil }

		if d.Name() == ".terraform" || d.Name() == ".terragrunt-cache" {
			wg.Add(1)
			go func(p string) {
				defer wg.Done()
				// Security check: Is there a local state?
				_, stateErr := os.Stat(filepath.Join(p, "terraform.tfstate"))
				
				mu.Lock()
				results = append(results, Result{
					Path:     p,
					Size:     calculateSize(p),
					HasState: stateErr == nil,
					Deleted:  false,
				})
				mu.Unlock()
			}(path)
			return filepath.SkipDir
		}
		return nil
	})
	wg.Wait()
	return results
}

func calculateSize(path string) int64 {
	var size int64
	filepath.WalkDir(path, func(_ string, d fs.DirEntry, _ error) error {
		if d != nil && !d.IsDir() {
			if info, err := d.Info(); err == nil { size += info.Size() }
		}
		return nil
	})
	return size
}
