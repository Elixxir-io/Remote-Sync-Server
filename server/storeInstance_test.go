////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package server

import (
	"gitlab.com/xx_network/primitives/netTime"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"gitlab.com/elixxir/remoteSyncServer/store"
)

// Unit test of newStoreInstance.
func Test_newStoreInstance(t *testing.T) {
	expected := storeInstance{
		username:   "username",
		genTime:    time.Date(1955, 11, 5, 12, 30, 2, 16, time.UTC),
		expiryTime: time.Time{},
		ttl:        5 * time.Minute,
	}
	expected.expiryTime = expected.genTime.Add(expected.ttl)
	expected.Store, _ = store.NewMemStore("", "")

	si, err := newStoreInstance(
		"", expected.username, expected.genTime, expected.ttl, store.NewMemStore)
	if err != nil {
		t.Errorf("Failed to make new storeInstance: %+v", err)
	}

	if !reflect.DeepEqual(expected, si) {
		t.Errorf("Unexpected new storeInstance.\nexpected: %+v\nreceived: %+v",
			expected, si)
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
		valid := storeInstance{expiryTime: expiryTime}.isValid()
		if valid != expected {
			t.Errorf("Unexpected IsValid evaltuion for %s at time %s."+
				"\nexpected: %t\nreceived: %t",
				expiryTime, netTime.Now(), expected, valid)
		}
	}
}
