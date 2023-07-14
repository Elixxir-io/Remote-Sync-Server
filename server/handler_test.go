////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package server

import (
	"bytes"
	"encoding/base64"
	"errors"
	"math/rand"
	"reflect"
	"testing"
	"time"

	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/remoteSyncServer/store"
	"gitlab.com/xx_network/crypto/nonce"
	"gitlab.com/xx_network/primitives/netTime"
)

// Unit test of newHandler.
func Test_newHandler(t *testing.T) {
	expected := &handler{
		storageDir:    "storageDir",
		tokenTTL:      5 * time.Hour,
		stores:        make(map[Token]*storeInstance),
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

// Error path: Tests that newHandler returns an error for invalid user records
func Test_newHandler_UserError(t *testing.T) {
	_, err := newHandler("", 0, [][]string{{"user", "pass"}, {"user2"}}, nil)
	if err == nil {
		t.Errorf("Failed to error for invalid records.")
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

		// Half of the time, add extra records
		if prng.Intn(2) == 0 {
			extraRecords := 1 + prng.Intn(15)
			for j := 0; j < extraRecords; j++ {
				extraRecord := make([]byte, 3+prng.Intn(26))
				prng.Read(extraRecord)
				records[i] = append(records[i],
					base64.StdEncoding.EncodeToString(extraRecord))
			}
		}

		expected[username] = password
	}

	recordsMap, err := userRecordsToMap(records)
	if err != nil {
		t.Errorf("Failed to convert records: %+v", err)
	}
	if !reflect.DeepEqual(expected, recordsMap) {
		t.Errorf("Unexpected records map.\nexpected: %s\nreceived: %s",
			expected, recordsMap)
	}
}

// Error path: Tests that userRecordsToMap returns an error for an invalid
// record.
func Test_userRecordsToMap_InvalidRecordError(t *testing.T) {
	_, err := userRecordsToMap([][]string{{"user", "pass"}, {"user2"}})
	if err == nil {
		t.Errorf("Failed to error for invalid records.")
	}
}

// Tests that handler.Login properly hashes the password and checks the username
// and that the message returns makes sense.
func Test_handler_Login(t *testing.T) {
	prng := rand.New(rand.NewSource(44477))
	username := "waldo"
	password := "hunter2"
	salt := make([]byte, 32)
	prng.Read(salt)

	passwordHash := hashPassword(password, salt)

	h, _ := newHandler(
		"tmp", time.Hour, [][]string{{username, password}}, store.NewMemStore)

	msg, err := h.Login(&pb.RsAuthenticationRequest{
		Username:     username,
		PasswordHash: passwordHash,
		Salt:         salt,
	})
	if err != nil {
		t.Errorf("Login error: %+v", err)
	}

	var token Token
	if msg.GetToken() == nil ||
		len(msg.GetToken()) != nonce.NonceLen ||
		bytes.Equal(msg.GetToken(), token.Marshal()) {
		t.Errorf("Received invalid token: %X", msg.GetToken())
	}

	if now := netTime.Now().Unix(); msg.ExpiresAt < now {
		t.Errorf("ExpiresAt %d before now %d.", msg.ExpiresAt, now)
	}
}

// Error path: Tests that handler.Login returns InvalidCredentialsErr for an
// invalid username.
func Test_handler_Login_InvalidUsernameError(t *testing.T) {
	prng := rand.New(rand.NewSource(44477))
	username := "waldo"
	password := "hunter2"
	salt := make([]byte, 32)
	prng.Read(salt)

	passwordHash := hashPassword(password, salt)

	h, _ := newHandler(
		"tmp", time.Hour, [][]string{{username, password}}, store.NewMemStore)

	_, err := h.Login(&pb.RsAuthenticationRequest{
		Username:     username + "extra junk",
		PasswordHash: passwordHash,
		Salt:         salt,
	})
	if !errors.Is(err, InvalidCredentialsErr) {
		t.Errorf("Unexpected error for invalid username."+
			"\nexpected: %v\nreceived: %+v", InvalidCredentialsErr, err)
	}
}

func Test_handler_Write_Read(t *testing.T) {
	h, token := newHandlerLogin(
		time.Hour, "waldo", "hunter2", rand.New(rand.NewSource(4596)), t)

	filePath := "dir1/dir2/fileA.txt"
	contents := []byte("Lorem ipsum and such as it goes.")
	ack, err := h.Write(&pb.RsWriteRequest{
		Path:  filePath,
		Data:  contents,
		Token: token.Marshal(),
	})
	if err != nil {
		t.Errorf("Failed to write: %+v", err)
	} else if ack == nil {
		t.Errorf("Received no ack: %+v", ack)
	}

	response, err := h.Read(&pb.RsReadRequest{
		Path:  filePath,
		Token: token.Marshal(),
	})
	if err != nil {
		t.Errorf("Failed to read: %+v", err)
	}

	if !bytes.Equal(contents, response.GetData()) {
		t.Errorf("Unexpected contents.\nexpected: %q\nreceived: %q",
			contents, response.GetData())
	}
}

func Test_handler_GetLastModified(t *testing.T) {
	h, token := newHandlerLogin(
		time.Hour, "waldo", "hunter2", rand.New(rand.NewSource(4596)), t)

	filePath := "dir1/dir2/fileA.txt"
	_, err := h.Write(&pb.RsWriteRequest{
		Path:  filePath,
		Data:  []byte("Lorem ipsum and such as it goes."),
		Token: token.Marshal(),
	})
	if err != nil {
		t.Errorf("Failed to write: %+v", err)
	}

	msg, err := h.GetLastModified(&pb.RsReadRequest{
		Path:  filePath,
		Token: token.Marshal(),
	})
	if err != nil {
		t.Errorf("Failed to get last modified time: %+v", err)
	}

	ts := time.Unix(0, msg.GetTimestamp())
	now := netTime.Now()
	if !ts.Round(time.Second).Equal(now.Round(time.Second)) || now.Before(ts) {
		t.Errorf("Modification time not near or before now."+
			"\nnow:      %s\nreceived: %s", now, ts)
	}
}

func Test_handler_GetLastWrite(t *testing.T) {
	h, token := newHandlerLogin(
		time.Hour, "waldo", "hunter2", rand.New(rand.NewSource(4596)), t)

	_, err := h.Write(&pb.RsWriteRequest{
		Path:  "dir1/dir2/fileA.txt",
		Data:  []byte("Lorem ipsum and such as it goes."),
		Token: token.Marshal(),
	})
	if err != nil {
		t.Errorf("Failed to write: %+v", err)
	}

	msg, err := h.GetLastWrite(&pb.RsLastWriteRequest{Token: token.Marshal()})
	if err != nil {
		t.Errorf("Failed to get last write: %+v", err)
	}

	ts := time.Unix(0, msg.GetTimestamp())
	now := netTime.Now()
	if !ts.Round(time.Second).Equal(now.Round(time.Second)) || now.Before(ts) {
		t.Errorf("Modification time not near or before now."+
			"\nnow:      %s\nreceived: %s", now, ts)
	}
}

func Test_handler_ReadDir(t *testing.T) {
	h, token := newHandlerLogin(
		time.Hour, "waldo", "hunter2", rand.New(rand.NewSource(4596)), t)

	_, err := h.Write(&pb.RsWriteRequest{
		Path:  "dir1/dir2/fileA.txt",
		Data:  []byte("Lorem ipsum and such as it goes."),
		Token: token.Marshal(),
	})
	if err != nil {
		t.Errorf("Failed to write: %+v", err)
	}

	msg, err := h.ReadDir(&pb.RsReadRequest{
		Path:  "dir1/",
		Token: token.Marshal(),
	})
	if err != nil {
		t.Errorf("Failed to read dir %s: %+v", "dir1/", err)
	}

	expected := []string{"dir2"}
	if !reflect.DeepEqual(msg.GetData(), expected) {
		t.Errorf("Unexpected directories.\nexpected: %s\nreceived: %s",
			expected, msg.GetData())
	}

}

// Tests handler.verifyUser with valid user.
func Test_handler_verifyUser(t *testing.T) {
	prng := rand.New(rand.NewSource(2))
	username := "waldo"
	password := "hunter2"
	salt := make([]byte, 32)
	prng.Read(salt)
	passwordHash := hashPassword(password, salt)
	h := &handler{
		userPasswords: map[string]string{
			username: password,
		},
	}

	err := h.verifyUser(username, passwordHash, salt)
	if err != nil {
		t.Errorf("Failed to verify user %s: %+v", username, err)
	}
}

// Error path: Tests that handler.verifyUser returns InvalidCredentialsErr for
// an invalid username.
func Test_handler_verifyUser_InvalidUsernameError(t *testing.T) {
	prng := rand.New(rand.NewSource(2))
	username := "waldo"
	password := "hunter2"
	salt := make([]byte, 32)
	prng.Read(salt)
	passwordHash := hashPassword(password, salt)
	h := &handler{
		userPasswords: map[string]string{
			username: password,
		},
	}

	err := h.verifyUser(username+"junk", passwordHash, salt)
	if !errors.Is(err, InvalidCredentialsErr) {
		t.Errorf("Unexpected error.\nexpected: %v\nreceived: %+v",
			InvalidCredentialsErr, err)
	}
}

// Error path: Tests that handler.verifyUser returns InvalidCredentialsErr for
// an invalid password.
func Test_handler_verifyUser_InvalidPasswordError(t *testing.T) {
	prng := rand.New(rand.NewSource(2))
	username := "waldo"
	password := "hunter2"
	salt := make([]byte, 32)
	prng.Read(salt)
	passwordHash := hashPassword(password, salt)
	h := &handler{
		userPasswords: map[string]string{
			username: password,
		},
	}

	err := h.verifyUser(username, append(passwordHash, []byte("junk")...), salt)
	if !errors.Is(err, InvalidCredentialsErr) {
		t.Errorf("Unexpected error.\nexpected: %v\nreceived: %+v",
			InvalidCredentialsErr, err)
	}
}

// Unit test of handler.getStore.
func Test_handler_getStore(t *testing.T) {
	h := &handler{
		tokenTTL:   time.Hour,
		stores:     make(map[Token]*storeInstance),
		userTokens: make(map[string]Token),
		newStore:   store.NewMemStore,
	}
	si1, err := h.addStore("waldo")
	if err != nil {
		t.Errorf("Failed to add store with the same username: %+v", err)
	}

	si2, err := h.getStore(Token(si1.Value))
	if err != nil {
		t.Errorf("Failed to get store for token %X: %+v", si1.Value, err)
	}

	if si1 != si2 {
		t.Errorf("Got wrong storeInstance.\nexpected: %+v\nreceived: %+v",
			si1, si2)
	}
}

// Error path: Tests that handler.getStore returns InvalidTokenErr for a token
// that is not found.
func Test_handler_getStore_InvalidTokenError(t *testing.T) {
	h := &handler{
		stores: make(map[Token]*storeInstance),
	}

	_, err := h.getStore(Token{1, 2, 3})
	if !errors.Is(err, InvalidTokenErr) {
		t.Errorf("Unexpected error for invalid token."+
			"\nexpected: %v\nreceived: %+v", InvalidTokenErr, err)
	}
}

// Error path: Tests that handler.getStore returns InvalidTokenErr for an
// expired token.
func Test_handler_getStore_ExpiredTokenError(t *testing.T) {
	h := &handler{
		tokenTTL:   time.Second,
		stores:     make(map[Token]*storeInstance),
		userTokens: make(map[string]Token),
		newStore:   store.NewMemStore,
	}

	si, err := h.addStore("waldo")
	if err != nil {
		t.Errorf("Failed to add store with the same username: %+v", err)
	}

	time.Sleep(time.Second)

	_, err = h.getStore(Token(si.Value))
	if !errors.Is(err, InvalidTokenErr) {
		t.Errorf("Unexpected error for expired token."+
			"\nexpected: %v\nreceived: %+v", InvalidTokenErr, err)
	}
}

// Tests that when called twice on the same username, handler.addStore returns
// the same storeInstance with a different token.
func Test_handler_addStore(t *testing.T) {
	h := &handler{
		tokenTTL:   time.Hour,
		stores:     make(map[Token]*storeInstance),
		userTokens: make(map[string]Token),
		newStore:   store.NewMemStore,
	}

	si1, err := h.addStore("waldo")
	if err != nil {
		t.Errorf("Failed to add store with the same username: %+v", err)
	}
	oldToken := si1.Value

	si2, err := h.addStore("waldo")
	if err != nil {
		t.Errorf("Failed to add store with the same username: %+v", err)
	}

	if oldToken == si2.Value {
		t.Errorf("Did not get new token.\nold: %X\nnew: %X", oldToken, si2.Value)
	}

	if si1 != si2 {
		t.Errorf("New storeInstance created.\nold: %+v\nnew: %+v", si1, si2)
	}
}

func newHandlerLogin(ttl time.Duration, username, password string,
	prng *rand.Rand, t testing.TB) (*handler, Token) {
	salt := make([]byte, 32)
	prng.Read(salt)
	passwordHash := hashPassword(password, salt)

	h, err := newHandler(
		"tmp", ttl, [][]string{{username, password}}, store.NewMemStore)
	if err != nil {
		t.Fatalf("Failed to make new handler: %+v", err)
	}
	msg, err := h.Login(&pb.RsAuthenticationRequest{
		Username:     username,
		PasswordHash: passwordHash,
		Salt:         salt,
	})
	if err != nil {
		t.Fatalf("Failed to login: %+v", err)
	}
	return h, UnmarshalToken(msg.GetToken())
}
