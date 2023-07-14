////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package store

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"

	"gitlab.com/xx_network/primitives/utils"
)

// FileStore manages the file storage in a base directory. Adheres to the Store
// interface.
type FileStore struct {
	baseDir       string
	lastWritePath string

	mux sync.Mutex
}

// NewFileStore creates a new FileStore at the specified base directory. This
// function creates a new directory in the filesystem.
//
// Returns [NonLocalFileErr] if the file is outside the storage directory.
func NewFileStore(storageDir, baseDir string) (Store, error) {
	baseDir, err := readyPath(storageDir, baseDir)
	if err != nil {
		return nil, err
	}
	fs := &FileStore{baseDir: baseDir}

	err = os.MkdirAll(fs.baseDir, 0700)
	if err != nil {
		return nil, errors.Wrapf(
			err, "failed to make base directory %s", fs.baseDir)
	}

	return fs, nil
}

// Read reads from the provided file path and returns the data in the file at
// that path.
//
// An error is returned if it fails to read the file. Returns [NonLocalFileErr]
// if the file is outside the base path.
func (fs *FileStore) Read(path string) ([]byte, error) {
	path, err := fs.readyPath(path)
	if err != nil {
		return nil, err
	}
	return utils.ReadFile(path)
}

// Write writes the provided data to the file path.
//
// An error is returned if the write fails. Returns [NonLocalFileErr] if the
// file is outside the base path.
func (fs *FileStore) Write(path string, data []byte) error {
	path, err := fs.readyPath(path)
	if err != nil {
		return errors.WithStack(err)
	}

	err = utils.WriteFileDef(path, data)
	if err != nil {
		return errors.WithStack(err)
	}

	fs.mux.Lock()
	fs.lastWritePath = path
	fs.mux.Unlock()
	return nil
}

// GetLastModified returns the last modification time for the file at the given
// file.
//
// Returns [NonLocalFileErr] if the file is outside the base path.
func (fs *FileStore) GetLastModified(path string) (time.Time, error) {
	path, err := fs.readyPath(path)
	if err != nil {
		return time.Time{}, err
	}
	return fs.getLastModified(path)
}

func (fs *FileStore) getLastModified(path string) (time.Time, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}

	return fi.ModTime(), nil
}

// GetLastWrite returns the time of the most recent successful Write operation
// that was performed.
func (fs *FileStore) GetLastWrite() (time.Time, error) {
	fs.mux.Lock()
	defer fs.mux.Unlock()
	return fs.getLastModified(fs.lastWritePath)
}

// ReadDir reads the named directory, returning all its directory entries
// sorted by filename.
//
// Returns [NonLocalFileErr] if the file is outside the base path.
func (fs *FileStore) ReadDir(path string) ([]string, error) {
	path, err := fs.readyPath(path)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

// readyPath makes the path relative to the base directory and ensures it is
// local. Returns NonLocalFileErr if the file is outside the base path.
func (fs *FileStore) readyPath(path string) (string, error) {
	return readyPath(fs.baseDir, path)
}

// isLocalFile determines if the file path is local to the base directory.
// Returns NonLocalFileErr if the file is outside the base path.
func (fs *FileStore) isLocalFile(path string) bool {
	return isLocalFile(fs.baseDir, path)
}

func readyPath(baseDir, path string) (string, error) {
	path = filepath.Join(baseDir, path)
	if !isLocalFile(baseDir, path) {
		return "", NonLocalFileErr
	}
	return path, nil
}

func isLocalFile(baseDir, path string) bool {
	rel, err := filepath.Rel(baseDir, path)
	if err != nil {
		jww.WARN.Printf("Failed to get relative path of %s to base %s: %+v",
			path, baseDir, err)
		return false
	} else if strings.HasPrefix(rel, "..") {
		return false
	}

	return true
}
