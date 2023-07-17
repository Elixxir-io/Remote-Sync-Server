////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package server

import (
	"gitlab.com/xx_network/crypto/nonce"
)

// Token that identifies a user. It is unique and generated from a user's
// username and password.
type Token nonce.Value

// Marshal marshals the Token into a byte slice.
func (t Token) Marshal() []byte {
	return t[:]
}

// UnmarshalToken unmarshalls the byte slice into a Token.
func UnmarshalToken(b []byte) Token {
	var t Token
	copy(t[:], b)
	return t
}
