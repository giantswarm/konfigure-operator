package ccr

import (
	"os"
	"path"
	"path/filepath"
)

type CCR struct {
	Dir string
}

func New(dir string) (*CCR, error) {
	absolutePath, err := filepath.Abs(dir)

	if err != nil {
		return nil, err
	}

	return &CCR{
		Dir: absolutePath,
	}, nil
}

func (ccr *CCR) ListApps() ([]string, error) {
	entries, err := os.ReadDir(path.Join(ccr.Dir, "default", "apps"))

	if err != nil {
		return []string{}, err
	}

	apps := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			apps = append(apps, entry.Name())
		}
	}

	return apps, nil
}
