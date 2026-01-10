package server

import (
	"os"
	"path/filepath"
	"sync"
)

type memoryFS struct {
	files map[string][]byte
	mu    sync.RWMutex
}

func newMemoryFS(rootPath string) (*memoryFS, error) {
	mfs := &memoryFS{
		files: make(map[string][]byte),
	}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		mfs.files["/"+filepath.ToSlash(relPath)] = data
		return nil
	})

	return mfs, err
}

func (mfs *memoryFS) Open(name string) ([]byte, error) {
	mfs.mu.RLock()
	defer mfs.mu.RUnlock()

	data, exists := mfs.files[name]
	if !exists {
		return nil, os.ErrNotExist
	}
	return data, nil
}
