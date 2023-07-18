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

// userSession stores a nonce with a unique token and an instance of a
// store.Store for a user that only exists for the given TTL.
type userSession struct {
	username string
	nonce.Nonce
	store.Store
}

// newUserSession creates a new session for the user that will expire after the
// given TTL.
//
// Returns store.NonLocalFileErr if the file is outside the storage directory.
func newUserSession(storageDir, username string, n nonce.Nonce,
	newStore store.NewStore) (userSession, error) {
	s, err := newStore(storageDir, username)
	if err != nil {
		return userSession{}, errors.Wrapf(
			err, "Failed to create new session storage for user %q", username)
	}

	return userSession{
		username: username,
		Nonce:    n,
		Store:    s,
	}, nil
}
