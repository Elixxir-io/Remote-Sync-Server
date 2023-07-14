////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package server

import (
	"encoding/base64"
	"math/rand"
	"reflect"
	"testing"
	"time"
)

// Unit test of newHandler.
func Test_newHandler(t *testing.T) {
	expected := &handler{
		storageDir:    "storageDir",
		tokenTTL:      5 * time.Hour,
		stores:        make(map[Token]storeInstance),
		userTokens:    make(map[string]Token),
		userPasswords: map[string]string{"user": "pass"},
	}

	h, err := newHandler(expected.storageDir, expected.tokenTTL,
		[][]string{{"user", "pass"}}, nil)
	if err != nil {
		t.Errorf("Failed to make new handler: %+v", err)
	}

	if !reflect.DeepEqual(expected, h) {
		t.Errorf("Unexpected new handler.\nexpected: %#v\nreceived: %#v",
			expected, h)
	}
}

// Tests that userRecordsToMap returns the expected map.
func Test_userRecordsToMap(t *testing.T) {
	prng := rand.New(rand.NewSource(3459806))
	const numTests = 100
	records := make([][]string, numTests)
	expected := make(map[string]string, numTests)
	for i := range records {
		usernameBytes := make([]byte, 3+prng.Intn(7))
		passwordBytes := make([]byte, 3+prng.Intn(32))
		prng.Read(usernameBytes)
		prng.Read(passwordBytes)
		username := base64.StdEncoding.EncodeToString(usernameBytes)
		password := base64.StdEncoding.EncodeToString(passwordBytes)
		records[i] = []string{username, password}
		expected[username] = password
	}

	recordsMap, _ := userRecordsToMap(records)
	if !reflect.DeepEqual(expected, recordsMap) {
		t.Errorf("Unexpected records map.\nexpected: %s\nreceived: %s",
			expected, recordsMap)
	}
}

func Test_handler_Login(t *testing.T) {
}

func Test_handler_Read(t *testing.T) {
}

func Test_handler_Write(t *testing.T) {
}

func Test_handler_GetLastModified(t *testing.T) {
}

func Test_handler_GetLastWrite(t *testing.T) {
}

func Test_handler_ReadDir(t *testing.T) {
}

func Test_handler_verifyUser(t *testing.T) {

	// h, _ := newHandler("storeDir", 64*time.Hour,
	// 	[][]string{{"user", "pass"}}, store.NewMemStore)
}

func Test_handler_getStore(t *testing.T) {
}

func Test_handler_addStore(t *testing.T) {
}
