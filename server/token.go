////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package server

// Token that identifies a user. It is unique and generated from a user's
// username and password.
type Token string

// GenerateToken generates a unique token from the username and password.
func GenerateToken(username, password string) Token {
	return Token(username + password)
}
