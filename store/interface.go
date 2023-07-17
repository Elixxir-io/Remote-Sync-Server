////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package store

import (
	"time"

	"github.com/pkg/errors"
)

var (
	// NonLocalFileErr is returned when attempting to read or write to file or
	// directory outside the base directory.
	NonLocalFileErr = errors.New("file path not in local base directory")
)

// NewStore generates a new Store for the given base directory that will be
// created in the storage directory.
//
// Returns [NonLocalFileErr] if the file is outside the storage directory.
type NewStore func(storageDir, baseDir string) (Store, error)

// Store copies the [collective.RemoteStore] interface.
type Store interface {
	// Read reads from the provided file path and returns the data in the file
	// at that path.
	//
	// An error is returned if it fails to read the file. Returns
	// [NonLocalFileErr] if the file is outside the base path.
	Read(path string) ([]byte, error)

	// Write writes the provided data to the file path.
	//
	// An error is returned if the write fails. Returns [NonLocalFileErr] if the
	// file is outside the base path.
	Write(path string, data []byte) error

	// GetLastModified returns the last modification time for the file at the
	// given file.
	//
	// Returns [NonLocalFileErr] if the file is outside the base path.
	GetLastModified(path string) (time.Time, error)

	// GetLastWrite returns the time of the most recent successful Write
	// operation that was performed.
	GetLastWrite() (time.Time, error)

	// ReadDir reads the named directory, returning all its directory entries
	// sorted by filename.
	//
	// Returns [NonLocalFileErr] if the file is outside the base path.
	ReadDir(path string) ([]string, error)
}
