package infrastructure

import (
	"os"
	"path/filepath"
)

type Filesystem struct{}

func (Filesystem) MkdirAll(path string) error { return os.MkdirAll(path, 0o755) }

func (Filesystem) WriteFile(path, content string, force bool) (bool, error) {
	if _, err := os.Stat(path); err == nil && !force {
		return false, nil
	} else if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, []byte(content), 0o644)
}
