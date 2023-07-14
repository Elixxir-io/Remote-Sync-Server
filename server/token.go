////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package server

import (
	"encoding/base64"
	"encoding/binary"
	"time"

	"gitlab.com/elixxir/crypto/hash"
)

// Token that identifies a user. It is unique and generated from a user's
// username and password.
type Token string

// GenerateToken generates a unique token from the username and password.
func GenerateToken(username string, passwordHash []byte, genTime time.Time) Token {
	h := hash.CMixHash.New()

	h.Write([]byte(username))
	h.Write(passwordHash)
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(genTime.UnixNano()))
	h.Write(b)

	tokenBytes := h.Sum(nil)
	return Token(base64.StdEncoding.EncodeToString(tokenBytes))
}
