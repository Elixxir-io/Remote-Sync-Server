////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package store

import (
	"bytes"
	"errors"
	"gitlab.com/xx_network/primitives/utils"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"gitlab.com/xx_network/primitives/netTime"
)

// Tests that FileStore adheres to the Store interface.
var _ Store = (*FileStore)(nil)

// Unit test of NewFileStore.
func TestNewFileStore(t *testing.T) {
	testDir := "tmp"
	expected := &FileStore{baseDir: filepath.Join(testDir, "baseDir")}
	defer removeTestFile(t, testDir)

	fs, err := NewFileStore(testDir, "baseDir")
	if err != nil {
		t.Errorf("Error creating new store: %+v", err)
	}

	if !reflect.DeepEqual(expected, fs) {
		t.Errorf("Unexpected new FileStore.\nexpected: %+v\nrecieved: %+v",
			expected, fs)
	}

	fi, err := os.Stat(fs.(*FileStore).baseDir)
	if err != nil {
		t.Errorf("Failed to stat base directory %s: %+v",
			fs.(*FileStore).baseDir, err)
	} else if !fi.IsDir() {
		t.Errorf("Base directory path not directory.")
	}
}

// Error path: Tests that NewFileStore returns an error for an invalid path.
func TestNewFileStore_InvalidPathError(t *testing.T) {
	_, err := NewFileStore("tmp", "/hello\000")
	if err == nil {
		t.Errorf("Failed to get error for invalid base file path: %+v", err)
	}
}

// Error path: Tests that NewFileStore returns an error when the base directory
// already exists as a file.
func TestNewFileStore_BaseDirectoryIsFileError(t *testing.T) {
	testDir := "tmp"
	path := filepath.Join(testDir, "file")
	defer removeTestFile(t, testDir)

	err := utils.WriteFileDef(path, []byte("data"))
	if err != nil {
		t.Errorf("Failed to write file: %+v", err)
	}

	_, err = NewFileStore(testDir, "file")
	if err == nil {
		t.Errorf("Failed to get error for invalid base file path: %+v", err)
	}
}

// Error path: Tests that NewFileStore returns NonLocalFileErr when the path is
// not local to the base directory.
func TestNewFileStore_NonLocalPathError(t *testing.T) {
	_, err := NewFileStore("tmp", "../file")
	if !errors.Is(err, NonLocalFileErr) {
		t.Errorf("Unexpected error for non-local file."+
			"\nexpected: %v\nreceived: %v", NonLocalFileErr, err)
	}
}

// Tests that FileStore.Read can only read files written to the base directory.
func TestFileStore_Read(t *testing.T) {
	testDir := "tmp"
	fs := newTestFileStore("baseDir", testDir, t)
	defer removeTestFile(t, testDir)

	path1 := "file1.txt"
	path1Write := filepath.Join(fs.baseDir, path1)
	path2 := filepath.Join(testDir, "file2.txt")
	expected := []byte("hello")

	if err := os.MkdirAll(testDir, os.ModePerm); err != nil {
		t.Fatal(err)
	} else if err = os.WriteFile(path1Write, expected, 0644); err != nil {
		t.Fatalf("Failed to write to %s: %+v", path1Write, err)
	} else if err = os.WriteFile(path2, []byte("hi"), 0644); err != nil {
		t.Fatalf("Failed to write to %s: %+v", path1, err)
	}

	content, err := fs.Read(path1)
	if err != nil {
		t.Errorf("Failed to read %s: %+v", path1, err)
	} else if !bytes.Equal(expected, content) {
		t.Errorf("Failed to read expected content for %s."+
			"\nexpected: %q\nreceived: %q", path1, expected, content)
	}

	_, err = fs.Read(path2)
	if err == nil {
		t.Errorf("Did not fail to read file %s outside basepath %s.",
			path2, fs.baseDir)
	}
}

// Error path: Tests that FileStore.Read returns NonLocalFileErr when the path
// is not local to the base directory.
func TestFileStore_Read_NonLocalPathError(t *testing.T) {
	fs := &FileStore{baseDir: "baseDir"}
	_, err := fs.Read("../file")
	if !errors.Is(err, NonLocalFileErr) {
		t.Errorf("Unexpected error for non-local file."+
			"\nexpected: %v\nreceived: %v", NonLocalFileErr, err)
	}
}

// Tests that all the files written by FileStore.Write can be properly read by
// FileStore.Read. Also checks that FileStore.lastWritePath is correctly updated
// on each write.
func TestFileStore_Write_Read(t *testing.T) {
	prng := rand.New(rand.NewSource(365785))
	testDir := "tmp"
	fs := newTestFileStore("baseDir", testDir, t)
	defer removeTestFile(t, testDir)

	testFiles := map[string][]byte{
		"hello.txt":                        []byte(randString(1+prng.Intn(12), prng)),
		"dir/testFile.txt":                 []byte(randString(1+prng.Intn(12), prng)),
		filepath.Join(fs.baseDir, "f.txt"): []byte(randString(1+prng.Intn(12), prng)),
	}

	for path, data := range testFiles {
		err := fs.Write(path, data)
		if err != nil {
			t.Errorf("Failed to write data for path %s: %+v", path, err)
		} else if fs.lastWritePath != filepath.Join(fs.baseDir, path) {
			t.Errorf("lastWritePath not updated.\nexpected: %s\nreceived: %s",
				filepath.Join(fs.baseDir, path), fs.lastWritePath)
		}
	}

	for path, expected := range testFiles {
		data, err := fs.Read(path)
		if err != nil {
			t.Errorf("Failed to read data for path %s: %+v", path, err)
		} else if !bytes.Equal(expected, data) {
			t.Errorf("Read unexpected data for path %s."+
				"\nexpected: %q\nreceived: %q", path, expected, data)
		}
	}
}

// Error path: Tests that FileStore.Write returns an error for an invalid path.
func TestFileStore_Write_InvalidPathError(t *testing.T) {
	testDir := "tmp"
	fs := newTestFileStore("baseDir", testDir, t)
	defer removeTestFile(t, testDir)

	fs.baseDir = ""
	err := fs.Write("~a/temp/temp2/test.txt", []byte{})
	if err == nil {
		t.Errorf("Failed to receive write error for invalid path.")
	}
}

// Error path: Tests that FileStore.Write returns NonLocalFileErr when the path
// is not local to the base directory.
func TestFileStore_Write_NonLocalPathError(t *testing.T) {
	fs := &FileStore{baseDir: "baseDir"}
	err := fs.Write("../file", nil)
	if !errors.Is(err, NonLocalFileErr) {
		t.Errorf("Unexpected error for non-local file."+
			"\nexpected: %v\nreceived: %v", NonLocalFileErr, err)
	}
}

// Tests that FileStore.GetLastModified returns a modified time close to the
// time taken before FileStore.Write is called.
func TestFileStore_GetLastModified(t *testing.T) {
	prng := rand.New(rand.NewSource(365785))
	testDir := "tmp"
	fs := newTestFileStore("baseDir", testDir, t)
	defer removeTestFile(t, testDir)

	testFiles := make(map[string]time.Time)
	for i := 0; i < 20; i++ {
		path := randString(1+prng.Intn(6), prng) + ".txt"
		testFiles[path] = netTime.Now()
		err := fs.Write(path, []byte(randString(1+prng.Intn(12), prng)))
		if err != nil {
			t.Errorf("Failed to write data for path %s: %+v", path, err)
		}
	}

	for path, expected := range testFiles {
		lastModified, err := fs.GetLastModified(path)
		if err != nil {
			t.Errorf("Failed to get last modified for path %s: %+v", path, err)
		} else if !lastModified.Round(500 * time.Millisecond).Equal(
			expected.Round(500 * time.Millisecond)) {
			t.Errorf("Last modified on path %s is not close to expected time "+
				"(Δ%s).\nexpected: %s\nreceived: %s",
				path, expected.Sub(lastModified), expected, lastModified)
		}
	}
}

// Error path: Tests that FileStore.GetLastModified returns NonLocalFileErr when
// the path is not local to the base directory.
func TestFileStore_GetLastModified_NonLocalPathError(t *testing.T) {
	fs := &FileStore{baseDir: "baseDir"}
	_, err := fs.GetLastModified("../file")
	if !errors.Is(err, NonLocalFileErr) {
		t.Errorf("Unexpected error for non-local file."+
			"\nexpected: %v\nreceived: %v", NonLocalFileErr, err)
	}
}

// Error path: Tests that FileStore.GetLastModified returns an error when the
// file does not exist.
func TestFileStore_GetLastModified_InvalidPathError(t *testing.T) {
	testDir := "tmp"
	fs := newTestFileStore("baseDir", testDir, t)
	defer removeTestFile(t, testDir)

	_, err := fs.GetLastModified("file")
	if err == nil {
		t.Errorf("Failed to receive error for invalid path.")
	}
}

// Tests that FileStore.GetLastWrite returns a modified time close to the time
// taken before FileStore.Write is called on the most recent write.
func TestFileStore_GetLastWrite(t *testing.T) {
	prng := rand.New(rand.NewSource(365785))
	testDir := "tmp"
	fs := newTestFileStore("baseDir", testDir, t)
	defer removeTestFile(t, testDir)

	for i := 0; i < 20; i++ {
		timeNow := netTime.Now()
		err := fs.Write(randString(1+prng.Intn(6), prng)+".txt",
			[]byte(randString(1+prng.Intn(12), prng)))
		if err != nil {
			t.Errorf("Failed to write data (%d): %+v", i, err)
		}

		const round = 250 * time.Millisecond
		lastModified, err := fs.GetLastWrite()
		if err != nil {
			t.Errorf("Failed to get last modified (%d): %+v", i, err)
		} else if !lastModified.Round(round).Equal(timeNow.Round(round)) {
			t.Errorf("Last modified is not close to expected time (Δ%s) (%d)."+
				"\nexpected: %s\nreceived: %s",
				timeNow.Sub(lastModified), i, timeNow, lastModified)
		}
	}
}

// Tests that FileStore.ReadDir returns all the expected directories.
func TestFileStore_ReadDir(t *testing.T) {
	testDir := "tmp"
	fs := newTestFileStore("baseDir", testDir, t)
	defer removeTestFile(t, testDir)

	tests := []struct {
		path string
		dirs []string
	}{
		{"", []string{"dir1", "dir2", "dirD"}},
		{"dir1", []string{"dirA", "dirB", "dirC"}},
		{"dir1/dirB", []string{"dirB1", "dirB2"}},
		{"dir1/dirB/dirB2", []string{}},
	}

	for i, path := range []string{"file", "dir1/a", "dir1/file", "dir1/dirA/a",
		"dir1/dirB/dirB1/a", "dir1/dirB/dirB2/a", "dir1/dirC/file",
		"dir2/dirC/a", "dirD/a"} {
		if err := fs.Write(path, []byte("data")); err != nil {
			t.Errorf("Failed to write data for path %s (%d): %+v", path, i, err)
		}
	}

	for i, tt := range tests {
		dirs, err := fs.ReadDir(tt.path)
		if err != nil {
			t.Errorf("Failed to read directory: %+v", err)
		}

		if !reflect.DeepEqual(dirs, tt.dirs) {
			t.Errorf("Unexpected directory list for %s (%d)."+
				"\nexpected: %s\nreceived: %s", tt.path, i, tt.dirs, dirs)
		}
	}
}

// Error path: Tests that FileStore.ReadDir returns NonLocalFileErr when the
// path is not local to the base directory.
func TestFileStore_ReadDir_NonLocalPathError(t *testing.T) {
	fs := &FileStore{baseDir: "baseDir"}
	_, err := fs.ReadDir("../file")
	if !errors.Is(err, NonLocalFileErr) {
		t.Errorf("Unexpected error for non-local file."+
			"\nexpected: %v\nreceived: %v", NonLocalFileErr, err)
	}
}

// Error path: Tests that FileStore.ReadDir returns an error when the file
// does not exist.
func TestFileStore_ReadDir_InvalidPathError(t *testing.T) {
	testDir := "tmp"
	fs := newTestFileStore("baseDir", testDir, t)
	defer removeTestFile(t, testDir)

	_, err := fs.ReadDir("file")
	if err == nil {
		t.Errorf("Failed to receive error for invalid path.")
	}
}

func TestFileStore_readyPath(t *testing.T) {
	fs := &FileStore{baseDir: "baseDir"}
	tests := []struct {
		path, expected string
		err            error
	}{
		{"dir/file", filepath.Join(fs.baseDir, "dir/file"), nil},
		{"../dir/file", "", NonLocalFileErr},
	}

	for i, tt := range tests {
		path, err := fs.readyPath(tt.path)
		if tt.err == nil {
			if err != nil {
				t.Errorf(
					"Failed to ready valid path %s (%d): %+v", tt.path, i, err)
			} else if path != tt.expected {
				t.Errorf("Unexpected path for %s (%d)."+
					"\nexpected: %s\nreceived: %s",
					tt.path, i, tt.expected, path)
			}
		} else if !errors.Is(err, tt.err) {
			t.Errorf("Unexpected error for invalid path %s (%d)."+
				"\nexpected: %s\nreceived: %v", tt.path, i, tt.err, err)
		}
	}
}

// Tests that FileStore.isLocalFile returns nil for all local files and
// NonLocalFileErr for non-local files.
func TestFileStore_isLocalFile(t *testing.T) {
	fs := &FileStore{baseDir: "baseDir"}
	testPaths := map[string]bool{
		filepath.Join(fs.baseDir, "dir", "file"):              true,
		filepath.Join("bob", "dir", "file"):                   false,
		filepath.Join(fs.baseDir, "..", "bob", "dir", "file"): false,
		filepath.Join(fs.baseDir, "file"):                     true,
		"file":                                                false,
		"./file":                                              false,
		"~/file":                                              false,
		`C:\`:                                                 false,
	}

	for path, expected := range testPaths {
		isLocal := fs.isLocalFile(path)
		if expected != isLocal {
			t.Errorf("Did not get expected result for path %s."+
				"\nexpected: %t\nreceoved: %t", path, expected, isLocal)
		}
	}
}

// newTestFileStore creates a new FileStore for testing purposes.
func newTestFileStore(baseDir, testDir string, t testing.TB) *FileStore {
	fs, err := NewFileStore(testDir, baseDir)
	if err != nil {
		t.Fatalf("Failed to create new FileStore: %+v", err)
	}

	return fs.(*FileStore)
}

// removeTestFile removes all passed in paths. Use in a defer function before
// file creation.
func removeTestFile(t testing.TB, paths ...string) {
	for i, path := range paths {
		if err := os.RemoveAll(path); err != nil {
			t.Errorf("Failed to remove path %s (%d of %d): %+v",
				path, i, len(paths), err)
		}
	}
}

const randStringChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// randString generates a random string of length n consisting of the characters
// in randStringChars.
func randString(n int, prng *rand.Rand) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = randStringChars[prng.Intn(len(randStringChars))]
	}
	return string(b)
}
