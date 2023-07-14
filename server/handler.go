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
	"gitlab.com/xx_network/crypto/nonce"
)

var (
	// InvalidTokenErr is returned when passed a token for a user that does
	// not exist.
	InvalidTokenErr = errors.New("Invalid token, login required")

	// StoreAlreadyExistsErr is returned when passed a store with the given
	// token already exists.
	StoreAlreadyExistsErr = errors.New("store with token already exists")

	// InvalidCredentialsErr is returned when a username does not match a
	// registered user or the password hashed with a salt does not match the
	// expected password hash.
	InvalidCredentialsErr = errors.New("invalid password or password")
)

// handler handles the server stores for each token/user.
type handler struct {
	storageDir    string
	tokenTTL      time.Duration
	stores        map[Token]storeInstance
	userTokens    map[string]Token  // Map of username to token
	userPasswords map[string]string // Map of username to password (from CSV)
	newStore      store.NewStore
	mux           sync.Mutex
}

// newHandler generates a new store handler.
//
// Pass in Store.NewMemStore into newStore for testing.
func newHandler(storageDir string, tokenTTL time.Duration,
	userRecords [][]string, newStore store.NewStore) (*handler, error) {
	userPasswords, err := userRecordsToMap(userRecords)
	if err != nil {
		return nil, err
	}

	return &handler{
		storageDir:    storageDir,
		tokenTTL:      tokenTTL,
		stores:        make(map[Token]storeInstance),
		userTokens:    make(map[string]Token),
		userPasswords: userPasswords,
		newStore:      newStore,
	}, nil
}

// userRecordsToMap converts the username/password records from a CSV to a map
// of passwords keyed on each username. Note that this will overwrite any
// passwords with duplicate usernames.
func userRecordsToMap(records [][]string) (map[string]string, error) {
	users := make(map[string]string, len(records))
	for i, line := range records {
		if len(line) < 2 {
			return nil, errors.Errorf("could not process record %d of %d",
				i, len(records))
		}
		users[line[0]] = line[1]
	}
	jww.DEBUG.Printf(
		"Imported %d users from %d records.", len(users), len(records))

	return users, nil
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

	// Add token and initialize user directory in storage
	s, err := h.addStore(msg.GetUsername(), h.tokenTTL)
	if err != nil {
		return nil, err
	}

	jww.INFO.Printf("Added store for user %s that expires at %s",
		msg.GetUsername(), s.ExpiryTime)

	return &pb.RsAuthenticationResponse{
		Token:     s.Value[:],
		ExpiresAt: s.ExpiryTime.UnixNano(),
	}, nil
}

// Read reads from the provided file path and returns the data in the file
// at that path.
//
// An error is returned if it fails to read the file. Returns
// [store.NonLocalFileErr] if the file is outside the base path,
// [InvalidTokenErr] for an invalid token.
func (h *handler) Read(msg *pb.RsReadRequest) (*pb.RsReadResponse, error) {
	jww.TRACE.Printf("Received Read message: %s", msg)

	s, err := h.getStore(UnmarshalToken(msg.GetToken()))
	if err != nil {
		return nil, err
	}

	data, err := s.Read(msg.GetPath())
	if err != nil {
		return nil, err
	}

	return &pb.RsReadResponse{Data: data}, nil
}

// Write writes the provided data to the file path.
//
// An error is returned if the write fails. Returns [store.NonLocalFileErr] if
// the file is outside the base path, [InvalidTokenErr] for an invalid token.
func (h *handler) Write(msg *pb.RsWriteRequest) (*messages.Ack, error) {
	jww.TRACE.Printf("Received Write message: %s", msg)

	s, err := h.getStore(UnmarshalToken(msg.GetToken()))
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
// [InvalidTokenErr] for an invalid token.
func (h *handler) GetLastModified(
	msg *pb.RsReadRequest) (*pb.RsTimestampResponse, error) {
	jww.TRACE.Printf("Received GetLastModified message: %s", msg)

	s, err := h.getStore(UnmarshalToken(msg.GetToken()))
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
// Returns [InvalidTokenErr] for an invalid token.
func (h *handler) GetLastWrite(
	msg *pb.RsLastWriteRequest) (*pb.RsTimestampResponse, error) {
	jww.TRACE.Printf("Received GetLastWrite message: %s", msg)

	s, err := h.getStore(UnmarshalToken(msg.GetToken()))
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
// [InvalidTokenErr] for an invalid token.
func (h *handler) ReadDir(
	msg *pb.RsReadRequest) (*pb.RsReadDirResponse, error) {
	jww.TRACE.Printf("Received ReadDir message: %s", msg)

	s, err := h.getStore(UnmarshalToken(msg.GetToken()))
	if err != nil {
		return nil, err
	}

	directories, err := s.ReadDir(msg.GetPath())
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
		return InvalidCredentialsErr
	}

	hh := hash.CMixHash.New()
	hh.Write([]byte(clearTextPassword))
	hh.Write(salt)
	if !bytes.Equal(hh.Sum(nil), passwordHash) {
		return InvalidCredentialsErr
	}

	return nil
}

// getStore returns the store for the given token. Returns [InvalidTokenErr] for
// an invalid token.
func (h *handler) getStore(token Token) (store.Store, error) {
	h.mux.Lock()
	defer h.mux.Unlock()

	s, exists := h.stores[token]
	if !exists {
		return nil, InvalidTokenErr
	}

	// If the store is no longer valid, then delete it and its token from their
	// respective maps
	if !s.IsValid() {
		delete(h.stores, token)
		delete(h.userTokens, s.username)
		return nil, InvalidTokenErr
	}

	return s, nil
}

// addStore generates a new Token and expiration time. On first login, it
// initializes a new storage directory for user. On subsequent logins, it
// overwrites the token with the new token gives access to the user's directory.
// Returns StoreAlreadyExistsErr if one already exists for the token.
func (h *handler) addStore(username string, tokenTTL time.Duration) (
	storeInstance, error) {
	h.mux.Lock()
	defer h.mux.Unlock()

	// Generate a new nonce and token
	n, err := nonce.NewNonce(uint(tokenTTL.Seconds()))
	if err != nil {
		return storeInstance{}, err
	}
	token := Token(n.Value)

	// The token should always be unique; this error should never occur
	if _, exists := h.stores[token]; !exists {
		return storeInstance{}, StoreAlreadyExistsErr
	}

	if oldToken, exists := h.userTokens[username]; exists {
		// If an old token is registered, update the token in the stores map
		jww.DEBUG.Printf(
			"Deleting old store for %s after overwriting token.", username)
		h.stores[token] = h.stores[oldToken]
		delete(h.stores, oldToken)
	} else {
		// If no token exists, create a new store instance and put in the map
		si, err := newStoreInstance(h.storageDir, username, n, h.newStore)
		if err != nil {
			return storeInstance{}, err
		}
		h.stores[token] = si
	}

	// Update to the newest token
	h.userTokens[username] = token

	return h.stores[token], nil
}
