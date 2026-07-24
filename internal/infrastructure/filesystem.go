package infrastructure

import (
	"errors"
	"io/fs"
	"os"
)

type FileSystem struct{}

func (FileSystem) MkdirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}

func (FileSystem) Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func (FileSystem) WriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
