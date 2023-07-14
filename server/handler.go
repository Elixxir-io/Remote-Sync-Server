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

	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/elixxir/remoteSyncServer/store"
	"gitlab.com/xx_network/comms/messages"
	"gitlab.com/xx_network/primitives/netTime"
)

var (
	// NoStoreForTokenErr is returned when passed a token for a user that does
	// not exist.
	NoStoreForTokenErr = errors.New("no storage registered for the token")

	// StoreAlreadyExistsErr is returned when passed a store with the given
	// token already exists.
	StoreAlreadyExistsErr = errors.New("store with token already exists")

	// ExpiredTokenErr is returned if the user's token has reached the TTL
	// duration and has been deleted.
	ExpiredTokenErr = errors.New("token expired; log in again")
)

// handler handles the server stores for each token/user.
type handler struct {
	storageDir    string
	tokenTTL      time.Duration
	stores        map[Token]storeInstance
	userTokens    map[string]Token
	userPasswords map[string]string
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
	return users
}

func (h *handler) Login(
	msg *pb.RsAuthenticationRequest) (*pb.RsAuthenticationResponse, error) {
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

	return &pb.RsAuthenticationResponse{
		Token:     string(token),
		ExpiresAt: s.expiryTime.UnixNano(),
	}, nil
}

func (h *handler) Read(msg *pb.RsReadRequest) (*pb.RsReadResponse, error) {
	s, err := h.getStore(Token(msg.GetToken()))
	if err != nil {
		return nil, err
	}

	data, err := s.Read(msg.GetPath())
	if err != nil {
		return nil, err
	}

	return &pb.RsReadResponse{Data: data}, nil
}

func (h *handler) Write(msg *pb.RsWriteRequest) (*messages.Ack, error) {
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

func (h *handler) GetLastModified(
	msg *pb.RsReadRequest) (*pb.RsTimestampResponse, error) {
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

func (h *handler) GetLastWrite(
	msg *pb.RsLastWriteRequest) (*pb.RsTimestampResponse, error) {
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

func (h *handler) ReadDir(msg *pb.RsReadRequest) (*pb.RsReadDirResponse, error) {
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
		return errors.Errorf("no user registered with username %q", username)
	}

	hh := hash.CMixHash.New()
	hh.Write([]byte(clearTextPassword))
	hh.Write(salt)
	if !bytes.Equal(hh.Sum(nil), passwordHash) {
		return errors.New("invalid password")
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
		return nil, NoStoreForTokenErr
	}

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

	// If a token has been previously registered for this user, delete it and
	// its storage
	if oldToken, exists := h.userTokens[username]; exists {
		delete(h.stores, oldToken)
		delete(h.userTokens, username)
	}

	h.stores[token] = s
	h.userTokens[username] = token

	return s, nil
}
