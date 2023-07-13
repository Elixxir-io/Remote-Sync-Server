////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package file

import (
	"bytes"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"gitlab.com/xx_network/primitives/netTime"
)

// Unit test of NewStore.
func TestNewStore(t *testing.T) {
	expected := &Store{baseDir: "baseDir/"}
	defer removeTestFile(t, expected.baseDir)
	s, err := NewStore(expected.baseDir)
	if err != nil {
		t.Errorf("Error creating new store: %+v", err)
	}

	if !reflect.DeepEqual(expected, s) {
		t.Errorf(
			"Unexpected new Store.\nexpected: %+v\nrecieved: %+v", expected, s)
	}

	fi, err := os.Stat(s.baseDir)
	if err != nil {
		t.Errorf("Failed to stat base directory %s: %+v", s.baseDir, err)
	} else if !fi.IsDir() {
		t.Errorf("Base directory path not directory.")
	}
}

// Error path: Tests that NewStore returns an error for an invalid path.
func TestNewStore_InvalidPathError(t *testing.T) {
	_, err := NewStore("/hello\000")
	if err == nil {
		t.Errorf("Failed to get error for invalid base file path: %+v", err)
	}
}

// Tests that Store.Read can only read files written to the base directory.
func TestStore_Read(t *testing.T) {
	testDir := "tmp"
	s := newTestStore("baseDir", testDir, t)
	defer removeTestFile(t, testDir)

	path1 := "file1.txt"
	path1Write := filepath.Join(s.baseDir, path1)
	path2 := filepath.Join(testDir, "file2.txt")
	expected := []byte("hello")

	if err := os.MkdirAll(testDir, os.ModePerm); err != nil {
		t.Fatal(err)
	} else if err = os.WriteFile(path1Write, expected, 0644); err != nil {
		t.Fatalf("Failed to write to %s: %+v", path1Write, err)
	} else if err = os.WriteFile(path2, []byte("hi"), 0644); err != nil {
		t.Fatalf("Failed to write to %s: %+v", path1, err)
	}

	content, err := s.Read(path1)
	if err != nil {
		t.Errorf("Failed to read %s: %+v", path1, err)
	} else if !bytes.Equal(expected, content) {
		t.Errorf("Failed to read expected content for %s."+
			"\nexpected: %q\nreceived: %q", path1, expected, content)
	}

	_, err = s.Read(path2)
	if err == nil {
		t.Errorf("Did not fail to read file %s outside basepath %s.",
			path2, s.baseDir)
	}
}

// Tests that all the files written by Store.Write can be properly read by
// Store.Read. Also checks that Store.lastWritePath is correctly updated on each
// write.
func TestStore_Write_Read(t *testing.T) {
	prng := rand.New(rand.NewSource(365785))
	testDir := "tmp"
	s := newTestStore("baseDir", testDir, t)
	defer removeTestFile(t, testDir)

	testFiles := map[string][]byte{
		"hello.txt":                       []byte(randString(1+prng.Intn(12), prng)),
		"dir/testFile.txt":                []byte(randString(1+prng.Intn(12), prng)),
		filepath.Join(s.baseDir, "f.txt"): []byte(randString(1+prng.Intn(12), prng)),
	}

	for path, data := range testFiles {
		err := s.Write(path, data)
		if err != nil {
			t.Errorf("Failed to write data for path %s: %+v", path, err)
		} else if s.lastWritePath != filepath.Join(s.baseDir, path) {
			t.Errorf("lastWritePath not updated.\nexpected: %s\nreceived: %s",
				filepath.Join(s.baseDir, path), s.lastWritePath)
		}
	}

	for path, expected := range testFiles {
		data, err := s.Read(path)
		if err != nil {
			t.Errorf("Failed to read data for path %s: %+v", path, err)
		} else if !bytes.Equal(expected, data) {
			t.Errorf("Read unexpected data for path %s."+
				"\nexpected: %q\nreceived: %q", path, expected, data)
		}
	}
}

// Error path: Tests that Store.Write returns an error for an invalid path.
func TestStore_Write_InvalidPathError(t *testing.T) {
	testDir := "tmp"
	s := newTestStore("baseDir", testDir, t)
	defer removeTestFile(t, testDir)

	s.baseDir = ""
	err := s.Write("~a/temp/temp2/test.txt", []byte{})
	if err == nil {
		t.Errorf("Failed to receive write error for invalid path.")
	}
}

// Tests that Store.GetLastModified returns a modified time close to the time
// taken before Store.Write is called.
func TestStore_GetLastModified(t *testing.T) {
	prng := rand.New(rand.NewSource(365785))
	testDir := "tmp"
	s := newTestStore("baseDir", testDir, t)
	defer removeTestFile(t, testDir)

	testFiles := make(map[string]time.Time)
	for i := 0; i < 20; i++ {
		path := randString(1+prng.Intn(6), prng) + ".txt"
		testFiles[path] = netTime.Now()
		err := s.Write(path, []byte(randString(1+prng.Intn(12), prng)))
		if err != nil {
			t.Errorf("Failed to write data for path %s: %+v", path, err)
		}
	}

	for path, expected := range testFiles {
		lastModified, err := s.GetLastModified(path)
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

// Error path: Tests that Store.GetLastModified returns an error when the file
// does not exist.
func TestStore_GetLastModified_InvalidPathError(t *testing.T) {
	testDir := "tmp"
	s := newTestStore("baseDir", testDir, t)
	defer removeTestFile(t, testDir)

	_, err := s.GetLastModified("file")
	if err == nil {
		t.Errorf("Failed to receive error for invalid path.")
	}
}

// Tests that Store.GetLastWrite returns a modified time close to the time
// taken before Store.Write is called on the most recent write.
func TestStore_GetLastWrite(t *testing.T) {
	prng := rand.New(rand.NewSource(365785))
	testDir := "tmp"
	s := newTestStore("baseDir", testDir, t)
	defer removeTestFile(t, testDir)

	for i := 0; i < 20; i++ {
		timeNow := netTime.Now()
		err := s.Write(randString(1+prng.Intn(6), prng)+".txt",
			[]byte(randString(1+prng.Intn(12), prng)))
		if err != nil {
			t.Errorf("Failed to write data (%d): %+v", i, err)
		}

		lastModified, err := s.GetLastWrite()
		if err != nil {
			t.Errorf("Failed to get last modified (%d): %+v", i, err)
		} else if !lastModified.Round(100 * time.Millisecond).Equal(
			timeNow.Round(100 * time.Millisecond)) {
			t.Errorf("Last modified is not close to expected time (Δ%s) (%d)."+
				"\nexpected: %s\nreceived: %s",
				timeNow.Sub(lastModified), i, timeNow, lastModified)
		}
	}
}

// Tests that Store.ReadDir returns all the expected directories.
func TestStore_ReadDir(t *testing.T) {
	prng := rand.New(rand.NewSource(365785))
	testDir := "tmp"
	s := newTestStore("baseDir", testDir, t)
	defer removeTestFile(t, testDir)

	expectedDirs := []string{"dir1", "dir2", "dir3", filepath.Dir(s.baseDir)}
	testFiles := []string{
		"hello.txt",
		filepath.Join(expectedDirs[0], "test.txt"),
		filepath.Join(expectedDirs[0], "dir2", "test.txt"),
		filepath.Join(expectedDirs[1], "test.txt"),
		filepath.Join(expectedDirs[2], "test.txt"),
		filepath.Join(s.baseDir, "test.txt"),
	}

	for _, path := range testFiles {
		if err := s.Write(path, []byte(randString(1+prng.Intn(12), prng))); err != nil {
			t.Errorf("Failed to write data for path %s: %+v", path, err)
		}
	}

	dirs, err := s.ReadDir("")
	if err != nil {
		t.Errorf("Failed to read directory: %+v", err)
	}

	if !reflect.DeepEqual(expectedDirs, dirs) {
		t.Errorf("Unexpected directories.\nexpected: %s\nreceived: %s",
			expectedDirs, dirs)
	}
}

// Error path: Tests that Store.ReadDir returns an error when the file
// does not exist.
func TestStore_ReadDir_InvalidPathError(t *testing.T) {
	testDir := "tmp"
	s := newTestStore("baseDir", testDir, t)
	defer removeTestFile(t, testDir)

	_, err := s.ReadDir("file")
	if err == nil {
		t.Errorf("Failed to receive error for invalid path.")
	}
}

// newTestStore creates a new Store for testing purposes.
func newTestStore(baseDir, testDir string, t testing.TB) *Store {
	s, err := NewStore(filepath.Join(testDir, baseDir))
	if err != nil {
		t.Fatalf("Failed to create new Store: %+v", err)
	}

	return s
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
