////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package server

import (
	"time"

	"github.com/pkg/errors"

	"gitlab.com/elixxir/remoteSyncServer/store"
	"gitlab.com/xx_network/primitives/netTime"
)

// storeInstance stores an instance of a store.Store that only exists for the
// given TTL.
type storeInstance struct {
	username   string
	genTime    time.Time
	expiryTime time.Time
	ttl        time.Duration
	store.Store
}

// newStoreInstance creates a new store for the user that will expire after the
// given TTL.
func newStoreInstance(storageDir, username string, genTime time.Time,
	ttl time.Duration, newStore store.NewStore) (storeInstance, error) {
	s, err := newStore(storageDir, username)
	if err != nil {
		return storeInstance{}, errors.Wrapf(
			err, "Failed to create new store for user %q", username)
	}

	return storeInstance{
		username:   username,
		genTime:    genTime,
		expiryTime: genTime.Add(ttl),
		ttl:        ttl,
		Store:      s,
	}, nil
}

// isValid checks that the nonce has not expired
func (si storeInstance) isValid() bool {
	return netTime.Now().Before(si.expiryTime)
}
