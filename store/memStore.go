////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package store

import (
	"gitlab.com/xx_network/primitives/netTime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemStore manages the storage in a base directory. It saves everything in
// memory instead of to the file system. Adheres to the Store interface.
type MemStore struct {
	lastWritePath string
	store         map[string]memFile

	mux sync.Mutex
}

type memFile struct {
	data     []byte
	modified time.Time
}

// NewMemStore creates a new MemStore at the specified base directory.
func NewMemStore(_ string, _ string) (Store, error) {
	ms := &MemStore{
		store: make(map[string]memFile),
	}

	return ms, nil
}

// Read reads from the provided file path and returns the data in the file at
// that path.
//
// An error is returned if it fails to read the file. Returns [os.ErrNotExist]
// if the file cannot be found.
func (ms *MemStore) Read(path string) ([]byte, error) {
	ms.mux.Lock()
	defer ms.mux.Unlock()
	f, exists := ms.store[path]
	if !exists {
		return nil, os.ErrNotExist
	}
	return f.data, nil
}

// Write writes the provided data to the file path. Does not return any errors.
func (ms *MemStore) Write(path string, data []byte) error {
	ms.mux.Lock()
	defer ms.mux.Unlock()
	ms.store[path] = memFile{data, netTime.Now()}
	ms.lastWritePath = path
	return nil
}

// GetLastModified returns the last modification time for the file at the given
// file.
//
// Returns [NonLocalFileErr] if the file is outside the base path.
func (ms *MemStore) GetLastModified(path string) (time.Time, error) {
	ms.mux.Lock()
	defer ms.mux.Unlock()
	return ms.getLastModified(path)
}

func (ms *MemStore) getLastModified(path string) (time.Time, error) {
	f, exists := ms.store[path]
	if !exists {
		return time.Time{}, os.ErrNotExist
	}
	return f.modified, nil
}

// GetLastWrite returns the time of the most recent successful Write operation
// that was performed.
func (ms *MemStore) GetLastWrite() (time.Time, error) {
	ms.mux.Lock()
	defer ms.mux.Unlock()
	return ms.getLastModified(ms.lastWritePath)
}

// ReadDir reads the named directory, returning all its directory entries
// sorted by filename.
//
// Returns [NonLocalFileErr] if the file is outside the base path.
func (ms *MemStore) ReadDir(path string) ([]string, error) {
	ms.mux.Lock()
	defer ms.mux.Unlock()

	dirMap := make(map[string]struct{})
	if path != "" {
		path = filepath.Clean(path)
	}

	for fPath := range ms.store {
		fPath = filepath.Dir(fPath) + string(os.PathSeparator)
		if fPath != string(os.PathSeparator) &&
			fPath != "."+string(os.PathSeparator) &&
			strings.HasPrefix(fPath, path) {
			dir := strings.TrimPrefix(fPath, path+string(os.PathSeparator))
			dir = strings.Split(dir, string(os.PathSeparator))[0]
			if dir != "" && dir != path {
				dirMap[dir] = struct{}{}
			}
		}
	}

	dirList := make([]string, 0, len(dirMap))
	for dir := range dirMap {
		dirList = append(dirList, dir)
	}
	sort.Strings(dirList)

	return dirList, nil
}
