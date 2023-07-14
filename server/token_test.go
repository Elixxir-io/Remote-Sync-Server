////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package server

import (
	"bytes"
	"math/rand"
	"testing"

	"gitlab.com/xx_network/crypto/nonce"
)

// Tests that UnmarshalToken unmarshalls bytes slices of all sizes into the
// expected Token.
func TestUnmarshalToken(t *testing.T) {
	prng := rand.New(rand.NewSource(123456789))

	tests := make([]struct{ b, expected []byte }, 100)
	for i := range tests {
		tests[i].b = make([]byte, prng.Intn(2*nonce.NonceLen))
		tests[i].expected = make([]byte, nonce.NonceLen)
		copy(tests[i].expected, tests[i].b)
	}

	for i, tt := range tests {
		token := UnmarshalToken(tt.b)
		if !bytes.Equal(tt.expected, token[:]) {
			t.Errorf("Unexpected token (%d).\nexpected: %X\nreceived: %X",
				i, tt.expected, token)
		}
	}
}
