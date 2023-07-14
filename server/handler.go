////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package server

import (
	"bytes"
	"sync"
	"time"

	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"

	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/elixxir/remoteSyncServer/store"
	"gitlab.com/xx_network/comms/messages"
	"gitlab.com/xx_network/primitives/netTime"
)

var (
	// InvalidTokenErr is returned when passed a token for a user that does
	// not exist.
	InvalidTokenErr = errors.New("Invalid token, log in required")

	// StoreAlreadyExistsErr is returned when passed a store with the given
	// token already exists.
	StoreAlreadyExistsErr = errors.New("store with token already exists")

	// ExpiredTokenErr is returned if the user's token has reached the TTL
	// duration and has been deleted.
	ExpiredTokenErr = errors.New("token expired; log in again")

	// InvalidUsernameErr is returned when a username does not match a
	// registered user.
	InvalidUsernameErr = errors.New("username does not match known user")

	// InvalidPasswordErr is returned when a password hashed with a salt does
	// not match the expected password hash.
	InvalidPasswordErr = errors.New("invalid password")
)

// handler handles the server stores for each token/user.
type handler struct {
	storageDir    string
	tokenTTL      time.Duration
	stores        map[Token]storeInstance
	userTokens    map[string]Token  // Map of username to token
	userPasswords map[string]string // Map of username to password (from CSV)
	mux           sync.Mutex
}

// newHandler generates a new store handler.
func newHandler(
	storageDir string, tokenTTL time.Duration, userRecords [][]string) *handler {
	return &handler{
		storageDir:    storageDir,
		tokenTTL:      tokenTTL,
		stores:        make(map[Token]storeInstance),
		userTokens:    make(map[string]Token),
		userPasswords: userRecordsToMap(userRecords),
	}
}

// userRecordsToMap converts the username/password records from a CSV to a map
// of passwords keyed on each username. Note that this will overwrite any
// passwords with duplicate usernames.
func userRecordsToMap(records [][]string) map[string]string {
	users := make(map[string]string, len(records))
	for _, line := range records {
		users[line[0]] = line[1]
	}
	jww.DEBUG.Printf(
		"Imported %d users from %d records.", len(users), len(records))

	return users
}

// Login is called when a new [mixmessages.RsAuthenticationRequest] is received.
// It authenticates the username and password, initializes storage for the user,
// and returns to them a unique token used to interact with the server and an
// expiration time. When a token expires, a user must log in again to get issues
// a new token.
func (h *handler) Login(
	msg *pb.RsAuthenticationRequest) (*pb.RsAuthenticationResponse, error) {
	jww.DEBUG.Printf("Received Login message: %s", msg)

	// Verify user exists and password is correct
	err := h.verifyUser(msg.GetUsername(), msg.GetPasswordHash(), msg.GetSalt())
	if err != nil {
		return nil, err
	}

	// Generate token
	genTime := netTime.Now()
	token := GenerateToken(msg.GetUsername(), msg.GetPasswordHash(), genTime)

	// Add token to store
	s, err := h.addStore(msg.GetUsername(), genTime, h.tokenTTL, token)
	if err != nil {
		return nil, err
	}

	jww.INFO.Printf("Added store for user %s that expires at %s",
		msg.GetUsername(), s.expiryTime)

	return &pb.RsAuthenticationResponse{
		Token:     string(token),
		ExpiresAt: s.expiryTime.UnixNano(),
	}, nil
}

// Read reads from the provided file path and returns the data in the file
// at that path.
//
// An error is returned if it fails to read the file. Returns
// [store.NonLocalFileErr] if the file is outside the base path,
// [NoStoreForTokenErr] for an invalid token, and [ExpiredTokenErr] if the token
// has expired.
func (h *handler) Read(msg *pb.RsReadRequest) (*pb.RsReadResponse, error) {
	jww.TRACE.Printf("Received Read message: %s", msg)

	s, err := h.getStore(Token(msg.GetToken()))
	if err != nil {
		return nil, err
	}

	data, err := s.Read(msg.GetPath())
	if err != nil {
		return nil, err
	}
	jww.TRACE.Printf("Received Read message: %s", msg)

	return &pb.RsReadResponse{Data: data}, nil
}

// Write writes the provided data to the file path.
//
// An error is returned if the write fails. Returns [store.NonLocalFileErr] if
// the file is outside the base path, [NoStoreForTokenErr] for an invalid token,
// and [ExpiredTokenErr] if the token has expired.
func (h *handler) Write(msg *pb.RsWriteRequest) (*messages.Ack, error) {
	jww.DEBUG.Printf("Received Write message: %s", msg)

	s, err := h.getStore(Token(msg.GetToken()))
	if err != nil {
		return nil, err
	}

	err = s.Write(msg.GetPath(), msg.GetData())
	if err != nil {
		return nil, err
	}

	return &messages.Ack{}, nil
}

// GetLastModified returns the last modification time for the file at the
// given file.
//
// Returns [store.NonLocalFileErr] if the file is outside the base path,
// [NoStoreForTokenErr] for an invalid token, and [ExpiredTokenErr] if the token
// has expired.
func (h *handler) GetLastModified(
	msg *pb.RsReadRequest) (*pb.RsTimestampResponse, error) {
	jww.DEBUG.Printf("Received GetLastModified message: %s", msg)

	s, err := h.getStore(Token(msg.GetToken()))
	if err != nil {
		return nil, err
	}

	lastModified, err := s.GetLastModified(msg.GetPath())
	if err != nil {
		return nil, err
	}

	return &pb.RsTimestampResponse{Timestamp: lastModified.UnixNano()}, nil
}

// GetLastWrite returns the time of the most recent successful Write
// operation that was performed.
//
// Returns [NoStoreForTokenErr] for an invalid token, and [ExpiredTokenErr] if
// the token has expired.
func (h *handler) GetLastWrite(
	msg *pb.RsLastWriteRequest) (*pb.RsTimestampResponse, error) {
	jww.DEBUG.Printf("Received GetLastWrite message: %s", msg)

	s, err := h.getStore(Token(msg.GetToken()))
	if err != nil {
		return nil, err
	}

	lastModified, err := s.GetLastWrite()
	if err != nil {
		return nil, err
	}

	return &pb.RsTimestampResponse{Timestamp: lastModified.UnixNano()}, nil
}

// ReadDir reads the named directory, returning all its directory entries
// sorted by filename.
//
// Returns [store.NonLocalFileErr] if the file is outside the base path,
// [NoStoreForTokenErr] for an invalid token, and [ExpiredTokenErr] if the token
// has expired.
func (h *handler) ReadDir(msg *pb.RsReadRequest) (*pb.RsReadDirResponse, error) {
	jww.DEBUG.Printf("Received ReadDir message: %s", msg)

	s, err := h.getStore(Token(msg.GetToken()))
	if err != nil {
		return nil, err
	}

	directories, err := s.ReadDir(msg.GetToken())
	if err != nil {
		return nil, err
	}

	return &pb.RsReadDirResponse{Data: directories}, nil
}

// verifyUser verifies the username and password are correct.
func (h *handler) verifyUser(username string, passwordHash, salt []byte) error {
	h.mux.Lock()
	defer h.mux.Unlock()

	clearTextPassword, exists := h.userPasswords[username]
	if !exists {
		return InvalidUsernameErr
	}

	hh := hash.CMixHash.New()
	hh.Write([]byte(clearTextPassword))
	hh.Write(salt)
	if !bytes.Equal(hh.Sum(nil), passwordHash) {
		return InvalidPasswordErr
	}

	return nil
}

// getStore returns the store for the given token. Returns [NoStoreForTokenErr]
// if it does not exist or [ExpiredTokenErr] if the token has expired.
func (h *handler) getStore(t Token) (store.Store, error) {
	h.mux.Lock()
	defer h.mux.Unlock()

	s, exists := h.stores[t]
	if !exists {
		return nil, InvalidTokenErr
	}

	// If the store is no longer valid, then delete it and its token from their
	// respective maps
	if !s.IsValid() {
		delete(h.stores, t)
		delete(h.userTokens, s.username)
		return nil, ExpiredTokenErr
	}

	return s, nil
}

// addStore adds a new store for the given token. Returns StoreAlreadyExistsErr
// if one already exists for the token.
func (h *handler) addStore(username string, genTime time.Time,
	tokenTTL time.Duration, token Token) (storeInstance, error) {
	h.mux.Lock()
	defer h.mux.Unlock()

	if _, exists := h.stores[token]; !exists {
		return storeInstance{}, StoreAlreadyExistsErr
	}

	s, err := newStoreInstance(h.storageDir, username, genTime, tokenTTL)
	if err != nil {
		return storeInstance{}, err
	}

	// If a token has been previously registered for this user, delete its store
	// from the map
	if oldToken, exists := h.userTokens[username]; exists {
		jww.DEBUG.Printf(
			"Deleting old store for %s after overwriting token.", username)
		delete(h.stores, oldToken)
	}

	h.stores[token] = s
	h.userTokens[username] = token

	return s, nil
}
