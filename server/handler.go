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

	// InvalidCredentialsErr is returned when a username does not match a
	// registered user or the password hashed with a salt does not match the
	// expected password hash.
	InvalidCredentialsErr = errors.New("invalid username or password")
)

// handler handles the server stores for each token/user.
type handler struct {
	storageDir    string
	tokenTTL      time.Duration
	sessions      map[Token]*userSession
	userTokens    map[string]Token  // Map of username to token
	userPasswords map[string]string // Map of username to password (from CSV)
	newStore      store.NewStore
	mux           sync.Mutex
}

// newHandler generates a new server handler.
//
// Pass in store.NewMemStore into newStore for testing.
func newHandler(storageDir string, tokenTTL time.Duration,
	userRecords [][]string, newStore store.NewStore) (*handler, error) {
	userPasswords, err := userRecordsToMap(userRecords)
	if err != nil {
		return nil, err
	}

	return &handler{
		storageDir:    storageDir,
		tokenTTL:      tokenTTL,
		sessions:      make(map[Token]*userSession),
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
//
// Returns [InvalidCredentialsErr] for invalid username or password.
func (h *handler) Login(
	msg *pb.RsAuthenticationRequest) (*pb.RsAuthenticationResponse, error) {
	jww.DEBUG.Printf("Received Login message: %s", msg)

	// Verify user exists and password is correct
	err := h.verifyUser(msg.GetUsername(), msg.GetPasswordHash(), msg.GetSalt())
	if err != nil {
		return nil, err
	}

	// Add token and initialize user directory in storage
	us, err := h.addSession(msg.GetUsername())
	if err != nil {
		jww.WARN.Printf(
			"Failed to add session for user %q: %+v", msg.GetUsername(), err)
		return nil, err
	}

	jww.INFO.Printf("Added session for user %s that expires at %s",
		msg.GetUsername(), us.ExpiryTime)

	return &pb.RsAuthenticationResponse{
		Token:     us.Value[:],
		ExpiresAt: us.ExpiryTime.UnixNano(),
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

	us, err := h.getSession(UnmarshalToken(msg.GetToken()))
	if err != nil {
		return nil, err
	}

	data, err := us.Read(msg.GetPath())
	if err != nil {
		jww.WARN.Printf("Failed to read \"%s\" for user %q: %+v",
			msg.GetPath(), us.username, err)
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

	us, err := h.getSession(UnmarshalToken(msg.GetToken()))
	if err != nil {
		return nil, err
	}

	err = us.Write(msg.GetPath(), msg.GetData())
	if err != nil {
		jww.WARN.Printf("Failed to write to \"%s\" for user %q: %+v",
			msg.GetPath(), us.username, err)
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

	us, err := h.getSession(UnmarshalToken(msg.GetToken()))
	if err != nil {
		return nil, err
	}

	lastModified, err := us.GetLastModified(msg.GetPath())
	if err != nil {
		jww.WARN.Printf("Failed to get last modified time of \"%s\" for "+
			"user %q: %+v", msg.GetPath(), us.username, err)
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

	us, err := h.getSession(UnmarshalToken(msg.GetToken()))
	if err != nil {
		return nil, err
	}

	lastModified, err := us.GetLastWrite()
	if err != nil {
		jww.WARN.Printf(
			"Failed to get last write for user %q: %+v", us.username, err)
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

	us, err := h.getSession(UnmarshalToken(msg.GetToken()))
	if err != nil {
		return nil, err
	}

	directories, err := us.ReadDir(msg.GetPath())
	if err != nil {
		jww.WARN.Printf("Failed to get read dir \"%s\" for user %q: %+v",
			msg.GetPath(), us.username, err)
		return nil, err
	}

	return &pb.RsReadDirResponse{Data: directories}, nil
}

// verifyUser verifies the username and password are correct. Returns
// InvalidCredentialsErr for incorrect username or password.
func (h *handler) verifyUser(username string, passwordHash, salt []byte) error {
	h.mux.Lock()
	defer h.mux.Unlock()

	clearTextPassword, exists := h.userPasswords[username]
	if !exists {
		jww.WARN.Printf("Failed to find username %q", username)
		return InvalidCredentialsErr
	}

	if !bytes.Equal(hashPassword(clearTextPassword, salt), passwordHash) {
		jww.WARN.Printf("Incorrect password hash for user %q", username)
		return InvalidCredentialsErr
	}

	return nil
}

func hashPassword(clearTextPassword string, salt []byte) []byte {
	h := hash.CMixHash.New()
	h.Write([]byte(clearTextPassword))
	h.Write(salt)
	return h.Sum(nil)
}

// getSession returns the user session for the given token. Returns
// InvalidTokenErr for an invalid token.
func (h *handler) getSession(token Token) (*userSession, error) {
	h.mux.Lock()
	defer h.mux.Unlock()

	us, exists := h.sessions[token]
	if !exists {
		jww.WARN.Printf("Failed to find session for token %X", token)
		return nil, InvalidTokenErr
	}

	// If the session is no longer valid, then delete it and its token from
	// their respective maps
	if !us.IsValid() {
		jww.WARN.Printf("Session for user %q expired", us.username)
		delete(h.sessions, token)
		delete(h.userTokens, us.username)
		return nil, InvalidTokenErr
	}

	return us, nil
}

// addSession generates a new Token and expiration time. On first login, it
// initializes a new storage directory for user. On subsequent logins, it
// overwrites the token with the new token gives access to the user's directory.
func (h *handler) addSession(username string) (*userSession, error) {
	h.mux.Lock()
	defer h.mux.Unlock()

	var token Token
	var n nonce.Nonce
	var err error
	for exists := true; exists; _, exists = h.sessions[token] {
		// Generate a new nonce and token
		n, err = nonce.NewNonce(uint(h.tokenTTL.Seconds()))
		if err != nil {
			// This error cannot currently happen
			return nil, err
		}
		token = Token(n.Value)
	}

	if oldToken, exists := h.userTokens[username]; exists {
		// If an old token is registered, update the token in the sessions map
		jww.DEBUG.Printf("Updating token for user %s.", username)
		h.sessions[token] = h.sessions[oldToken]
		h.sessions[token].Value = nonce.Value(token)
		delete(h.sessions, oldToken)
	} else {
		// If no token exists, create a new session and put in the map
		jww.DEBUG.Printf("Creating new token for user %s.", username)

		us, err := newUserSession(h.storageDir, username, n, h.newStore)
		if err != nil {
			return nil, err
		}
		h.sessions[token] = &us
	}

	// Update to the newest token
	h.userTokens[username] = token

	return h.sessions[token], nil
}
