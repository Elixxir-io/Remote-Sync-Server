////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package file

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"

	"gitlab.com/xx_network/primitives/utils"
)

// Store manages the storage in a base directory.
type Store struct {
	baseDir       string
	lastWritePath string

	mux sync.Mutex
}

// NewStore creates a new Store at the specified base directory. This function
// creates a new directory.
func NewStore(baseDir string) (*Store, error) {
	s := &Store{
		baseDir: baseDir,
	}

	err := os.MkdirAll(s.baseDir, 0700)
	if err != nil {
		return nil, errors.Wrapf(
			err, "failed to make base directory %s", s.baseDir)
	}

	return s, nil
}

// Read reads from the provided file path and returns the data in the file at
// that path. An error is returned if it fails to read the file.
func (s *Store) Read(path string) ([]byte, error) {
	return os.ReadFile(filepath.Join(s.baseDir, path))
}

// Write writes the provided data to the file path. An error is returned if the
// write fails.
func (s *Store) Write(path string, data []byte) error {
	path = filepath.Join(s.baseDir, path)

	err := utils.WriteFileDef(path, data)
	if err != nil {
		return err
	}

	s.mux.Lock()
	s.lastWritePath = path
	s.mux.Unlock()
	return nil
}

// GetLastModified returns the last modification time for the file at the given
// file.
func (s *Store) GetLastModified(path string) (time.Time, error) {
	return s.getLastModified(filepath.Join(s.baseDir, path))
}

func (s *Store) getLastModified(path string) (time.Time, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}

	return fi.ModTime(), nil
}

// GetLastWrite returns the time of the most recent successful Write operation
// that was performed.
func (s *Store) GetLastWrite() (time.Time, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.getLastModified(s.lastWritePath)
}

// ReadDir reads the named directory, returning all its directory entries
// sorted by filename.
func (s *Store) ReadDir(path string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(s.baseDir, path))
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}
