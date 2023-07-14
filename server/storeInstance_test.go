////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package server

import (
	"errors"
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"

	"gitlab.com/elixxir/remoteSyncServer/store"
	"gitlab.com/xx_network/crypto/nonce"
	"gitlab.com/xx_network/primitives/netTime"
)

// Unit test of newStoreInstance.
func Test_newStoreInstance(t *testing.T) {
	n, err := nonce.NewNonce(uint(5*time.Minute + 56*time.Nanosecond))
	if err != nil {
		t.Errorf("Failed to generate new nonce: %+v", err)
	}
	expected := storeInstance{
		username: "username",
		Nonce:    n,
		Store:    nil,
	}
	expected.Store, _ = store.NewMemStore("", "")

	si, err := newStoreInstance("", expected.username, n, store.NewMemStore)
	if err != nil {
		t.Errorf("Failed to make new storeInstance: %+v", err)
	}

	if !reflect.DeepEqual(expected, si) {
		t.Errorf("Unexpected new storeInstance.\nexpected: %+v\nreceived: %+v",
			expected, si)
	}
}

// Error path: Tests that newStoreInstance returns store.NonLocalFileErr for a
// non-local path.
func Test_newStoreInstance_NonLocalFileError(t *testing.T) {
	testDir := "tmp"
	defer func() {
		if err := os.RemoveAll(testDir); err != nil {
			t.Fatalf("Failed to remove %s: %+v", testDir, err)
		}
	}()
	_, err := newStoreInstance(
		testDir, "user/../..", nonce.Nonce{}, store.NewFileStore)
	if !errors.Is(err, store.NonLocalFileErr) {
		t.Errorf("Failed to make new storeInstance: %+v", err)
	}
}

// Tests determined times if they are valid via storeInstance.isValid
func Test_storeInstance_isValid(t *testing.T) {
	prng := rand.New(rand.NewSource(4035390))

	times := map[time.Time]bool{}
	for i := 0; i < 100; i++ {
		switch prng.Intn(2) {
		case 0:
			d := 1*time.Minute + time.Duration(prng.Int63())
			times[netTime.Now().Add(d)] = true
		case 1:
			d := -1*time.Minute - time.Duration(prng.Int63())
			times[netTime.Now().Add(d)] = false
		}
	}

	for expiryTime, expected := range times {
		valid := storeInstance{Nonce: nonce.Nonce{ExpiryTime: expiryTime}}.IsValid()
		if valid != expected {
			t.Errorf("Unexpected IsValid evaltuion for %s at time %s."+
				"\nexpected: %t\nreceived: %t",
				expiryTime, netTime.Now(), expected, valid)
		}
	}
}
