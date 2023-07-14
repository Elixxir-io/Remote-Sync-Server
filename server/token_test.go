////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package server

import (
	"math/rand"
	"testing"
	"time"
)

// Consistency test of GenerateToken.
func TestGenerateToken_Consistency(t *testing.T) {
	prng := rand.New(rand.NewSource(98432))

	expectedTokens := []Token{
		"fRADyBHYCAQ0twsaMe6n57uApn7fB+1YnFDzjUxFDP0=",
		"Xhcock1WiY0XoYHVcMj0q+LGjo1FcS9A8XMUdl2myU8=",
		"Y9FaZnvdOSLr9vD3hp81dtZfeRLlW6sRybC7Yip9g9c=",
		"y2NoDBJxlHQxHHroaZsIAPFm2cR8gycDfF7+BRXp9+c=",
		"ZwK4pHcECndbedllZWzgbiAgx2C3EM28zXmpDhr9Nco=",
		"mYoOmplPBCVEIojuf+EhyLtewBTwO8dsvc132knG5Ug=",
		"UaoSKlRRecGDNvB93tFfH2KMRfOVQ9YoSsGwVXbokZ4=",
		"FHItxXXp3wMNXY6/vchEknfRX3ORjl32lNp29FsCBbU=",
		"Yjtq5ZyoZdwLCNbM3W1d8VhfGLh5uZD9V5hPw5TKgN0=",
		"Z+CmYcje0ocHNRv3SQ1nnMGBjWkzA4sd7hP5U7BAkJY=",
		"csLidu4p0Z00Eda4Y31DqpOq+tvFitNe8KoQJw/ViFA=",
		"PApUzDjlSuVk86hzhMcAKHn6MoyD9SmmMb4ecsHjIHs=",
		"yYvoQ+moa4UehWDcxPkcGmDFRQr2sg+UKtPMSXy5F1Q=",
		"rFs4fs5DZqwHnSkQtIUaiwVSa9RvWhE2pZbpIKzmUzQ=",
		"3jQikxpUR8vkGDT9tCgcpbBvfymkFJGZ6aW+v7gMtMw=",
		"o19hZu+J6wgZOVQHpkfum9U/kY7zZDPtEuz2A5BW4Yw=",
		"mXNZoyJdcwwfhU0lHszf3O+ikVW4eY10TDIQfBsstaQ=",
		"FZlRyVpqFgIp6NbWTBAh1XbcI3ATrfQrFFrrmwTO6v0=",
		"y9hZPbRMq9i88wRVxajLpObLFFYUCgCybABLxfitnqw=",
		"EYt6OT+MOgjBvdI/NZUb2RjTmdBgVlsHbXlhrCDH2ow=",
		"wRH3Tk2+xdd6PMzTqZ4QVh8jBxzKtIhkZ4etUxyOeIc=",
		"MtvVYTHwvMbW+JPF/mcZgjk+hzQk0wQS/5a863BG5l8=",
		"31nNuCBTn8J2mEUYXrBgL0G3Pfx0UdvgCz1b+D9BAK8=",
		"ygZp7DrhOKxtNWGb2FZnQVzbtDN5cmWfuTSaK7g3/94=",
		"rJW7o79xDnzGiM8kRWBHas815HwLZriEq3JnsLsQ54s=",
	}

	for i, expected := range expectedTokens {
		username, passwordHash := make([]byte, 3+prng.Intn(7)), make([]byte, 32)
		prng.Read(username)
		prng.Read(passwordHash)
		genTime := time.Unix(0, rand.Int63())

		token := GenerateToken(string(username), passwordHash, genTime)
		if expected != token {
			t.Errorf("Unexpected token for username:%X passwordHash:%X "+
				"genTime:%s (%d).\nexpected: %s\nreceived: %s",
				username, passwordHash, genTime, i, expected, token)
		}
	}
}

// Uniqueness test of GenerateToken.
func TestGenerateToken_Unique(t *testing.T) {
	prng := rand.New(rand.NewSource(98432))
	const numTests = 25

	usernames := make([]string, numTests)
	passwordHashes := make([][]byte, numTests)
	genTimes := make([]time.Time, numTests)

	tokens := make(map[Token]struct{}, numTests*numTests*numTests)

	for i := range usernames {
		username := make([]byte, 3+prng.Intn(7))
		prng.Read(username)
		usernames[i] = string(username)
		passwordHashes[i] = make([]byte, 32)
		prng.Read(passwordHashes[i])
		genTimes[i] = time.Unix(0, rand.Int63())
	}

	for i, username := range usernames {
		for j, passwordHash := range passwordHashes {
			for k, genTime := range genTimes {
				token := GenerateToken(username, passwordHash, genTime)
				if _, exists := tokens[token]; exists {
					t.Errorf("Duplicate token for username:%X passwordHash:%X "+
						"genTime:%s (%d, %d, %d): %s",
						username, passwordHash, genTime, i, j, k, token)
				} else {
					tokens[token] = struct{}{}
				}

			}
		}
	}
}
