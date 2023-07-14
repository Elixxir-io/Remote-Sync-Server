////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package store

import (
	"bytes"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/pkg/errors"

	"gitlab.com/xx_network/primitives/netTime"
)

// Tests that MemStore adheres to the Store interface.
var _ Store = (*MemStore)(nil)

// Unit test of NewMemStore.
func TestNewMemStore(t *testing.T) {
	expected := &MemStore{store: make(map[string]memFile)}
	ms, _ := NewMemStore("", "")

	if !reflect.DeepEqual(expected, ms) {
		t.Errorf("Unexpected new MemStore.\nexpected: %+v\nrecieved: %+v",
			expected, ms)
	}
}

// Tests that all the files written by MemStore.Write can be properly read by
// MemStore.Read. Also checks that MemStore.lastWritePath is correctly updated
// on each write.
func TestMemStore_Write_Read(t *testing.T) {
	prng := rand.New(rand.NewSource(365785))
	ms, _ := NewMemStore("", "")

	testFiles := map[string][]byte{
		"hello.txt":                    []byte(randString(1+prng.Intn(12), prng)),
		"dir/testFile.txt":             []byte(randString(1+prng.Intn(12), prng)),
		filepath.Join("dir2", "f.txt"): []byte(randString(1+prng.Intn(12), prng)),
	}

	for path, data := range testFiles {
		err := ms.Write(path, data)
		if err != nil {
			t.Errorf("Failed to write data for path %s: %+v", path, err)
		} else if ms.(*MemStore).lastWritePath != path {
			t.Errorf("lastWritePath not updated.\nexpected: %s\nreceived: %s",
				path, ms.(*MemStore).lastWritePath)
		}
	}

	for path, expected := range testFiles {
		data, err := ms.Read(path)
		if err != nil {
			t.Errorf("Failed to read data for path %s: %+v", path, err)
		} else if !bytes.Equal(expected, data) {
			t.Errorf("Read unexpected data for path %s."+
				"\nexpected: %q\nreceived: %q", path, expected, data)
		}
	}
}

// Error path: Tests that MemStore.Read returns os.ErrNotExist if the file does
// not exist.
func TestMemStore_Read_ErrNotExist(t *testing.T) {
	ms, _ := NewMemStore("", "")
	_, err := ms.Read("no file")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Unexpected error for non-local file."+
			"\nexpected: %v\nreceived: %v", os.ErrNotExist, err)
	}
}

// Tests that MemStore.GetLastModified returns a modified time close to the time
// taken before MemStore.Write is called.
func TestMemStore_GetLastModified(t *testing.T) {
	prng := rand.New(rand.NewSource(365785))
	ms, _ := NewMemStore("", "")

	testFiles := make(map[string]time.Time)
	for i := 0; i < 20; i++ {
		path := randString(1+prng.Intn(6), prng) + ".txt"
		testFiles[path] = netTime.Now()
		err := ms.Write(path, []byte(randString(1+prng.Intn(12), prng)))
		if err != nil {
			t.Errorf("Failed to write data for path %s: %+v", path, err)
		}
	}

	for path, expected := range testFiles {
		lastModified, err := ms.GetLastModified(path)
		if err != nil {
			t.Errorf("Failed to get last modified for path %s: %+v", path, err)
		} else if !lastModified.Round(100 * time.Millisecond).Equal(
			expected.Round(100 * time.Millisecond)) {
			t.Errorf("Last modified on path %s is not close to expected time "+
				"(Δ%s).\nexpected: %s\nreceived: %s",
				path, expected.Sub(lastModified), expected, lastModified)
		}
	}
}

// Error path: Tests that MemStore.GetLastModified returns os.ErrNotExist if the
// file does not exist.
func TestMemStore_GetLastModified_ErrNotExist(t *testing.T) {
	ms, _ := NewMemStore("", "")
	_, err := ms.GetLastModified("no file")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Unexpected error for non-local file."+
			"\nexpected: %v\nreceived: %v", os.ErrNotExist, err)
	}
}

// Tests that MemStore.GetLastWrite returns a modified time close to the time
// taken before MemStore.Write is called on the most recent write.
func TestMemStore_GetLastWrite(t *testing.T) {
	prng := rand.New(rand.NewSource(365785))
	ms, _ := NewMemStore("", "")

	for i := 0; i < 20; i++ {
		timeNow := netTime.Now()
		err := ms.Write(randString(1+prng.Intn(6), prng)+".txt",
			[]byte(randString(1+prng.Intn(12), prng)))
		if err != nil {
			t.Errorf("Failed to write data (%d): %+v", i, err)
		}

		const round = 250 * time.Millisecond
		lastModified, err := ms.GetLastWrite()
		if err != nil {
			t.Errorf("Failed to get last modified (%d): %+v", i, err)
		} else if !lastModified.Round(round).Equal(timeNow.Round(round)) {
			t.Errorf("Last modified is not close to expected time (Δ%s) (%d)."+
				"\nexpected: %s\nreceived: %s",
				timeNow.Sub(lastModified), i, timeNow, lastModified)
		}
	}
}

// Tests that MemStore.ReadDir returns all the expected directories.
func TestMemStore_ReadDir(t *testing.T) {
	ms, _ := NewMemStore("", "")

	tests := []struct {
		path string
		dirs []string
	}{
		{"", []string{"dir1", "dir2", "dirD"}},
		{"dir1", []string{"dirA", "dirB", "dirC"}},
		{"dir1/dirB", []string{"dirB1", "dirB2"}},
		{"dir1/dirB/dirB2", []string{}},
	}

	for i, path := range []string{"file", "dir1", "dir1/file", "dir1/dirA/a",
		"dir1/dirB/dirB1/a", "dir1/dirB/dirB2/a", "dir1/dirC/file",
		"dir2/dirC/a", "dirD/"} {
		if err := ms.Write(path, []byte("data")); err != nil {
			t.Errorf("Failed to write data for path %s (%d): %+v", path, i, err)
		}
	}

	for i, tt := range tests {
		dirs, err := ms.ReadDir(tt.path)
		if err != nil {
			t.Errorf("Failed to read directory: %+v", err)
		}

		if !reflect.DeepEqual(dirs, tt.dirs) {
			t.Errorf("Unexpected directory list for %s (%d)."+
				"\nexpected: %s\nreceived: %s", tt.path, i, tt.dirs, dirs)
		}
	}
}
