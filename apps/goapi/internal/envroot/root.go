package envroot

import (
	"os"
	"path/filepath"
)

// MonorepoRootFrom walks parents of wd until it finds a directory that contains
// pnpm-workspace.yaml and apps/goapi/go.mod (chexi-trading layout). Returns ("", nil)
// if not found.
func MonorepoRootFrom(wd string) (string, error) {
	d, err := filepath.Abs(wd)
	if err != nil {
		return "", err
	}
	for {
		if fileExists(filepath.Join(d, "pnpm-workspace.yaml")) && fileExists(filepath.Join(d, "apps", "goapi", "go.mod")) {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", nil
		}
		d = parent
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
