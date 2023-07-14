////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package server

import (
	"github.com/pkg/errors"

	"gitlab.com/elixxir/remoteSyncServer/store"
	"gitlab.com/xx_network/crypto/nonce"
)

// storeInstance stores an instance of a store.Store that only exists for the
// given TTL.
type storeInstance struct {
	username string
	nonce.Nonce
	store.Store
}

// newStoreInstance creates a new store for the user that will expire after the
// given TTL.
func newStoreInstance(storageDir, username string, n nonce.Nonce,
	newStore store.NewStore) (storeInstance, error) {
	s, err := newStore(storageDir, username)
	if err != nil {
		return storeInstance{}, errors.Wrapf(
			err, "Failed to create new store for user %q", username)
	}

	return storeInstance{
		username: username,
		Nonce:    n,
		Store:    s,
	}, nil
}
