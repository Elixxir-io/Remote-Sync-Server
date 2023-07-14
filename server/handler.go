////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package server

import (
	"sync"

	"github.com/pkg/errors"

	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/remoteSyncServer/store"
)

// handler handles the server stores for each token/user.
type handler struct {
	stores map[Token]store.Store
	mux    sync.Mutex
}

var (
	// NoStoreForTokenErr is returned when passed a token for a user that does
	// not exist.
	NoStoreForTokenErr = errors.New("no storage registered for the token")

	// StoreAlreadyExistsErr is returned when passed a store with the given
	// token already exists.
	StoreAlreadyExistsErr = errors.New("store with token already exists")
)

// getStore returns the store for the given token or NoStoreForTokenErr if it
// does not exist.
func (h *handler) getStore(t Token) (store.Store, error) {
	h.mux.Lock()
	defer h.mux.Unlock()

	s, exists := h.stores[t]
	if !exists {
		return nil, NoStoreForTokenErr
	}

	return s, nil
}

// addStore adds a new store for the given token. Returns StoreAlreadyExistsErr
// if one already exists for the token.
func (h *handler) addStore(t Token) error {
	h.mux.Lock()
	defer h.mux.Unlock()

	if _, exists := h.stores[t]; !exists {
		return StoreAlreadyExistsErr
	}

	s, err := store.NewFileStore(string(t))
	if err != nil {
		return err
	}

	h.stores[t] = s

	return nil
}

func (h *handler) Login(
	msg *pb.RsAuthenticationRequest) (*pb.RsAuthenticationResponse, error) {
	// TODO generate token
	token := GenerateToken(msg.GetPath(), msg.GetPassword())

	err := h.addStore(token)
	if err != nil {
		// TODO: add error
		return &pb.RsAuthenticationResponse{}, nil
	}

	return &pb.RsAuthenticationResponse{Token: string(token)}, nil
}

func (h *handler) Read(msg *pb.RsReadRequest) (*pb.RsReadResponse, error) {
	s, err := h.getStore(Token(msg.GetToken()))
	if err != nil {
		return &pb.RsReadResponse{Error: err.Error()}, nil
	}

	data, err := s.Read(msg.GetPath())
	if err != nil {
		return &pb.RsReadResponse{Error: err.Error()}, nil
	}

	return &pb.RsReadResponse{Data: data}, nil
}

func (h *handler) Write(msg *pb.RsWriteRequest) (*pb.RsWriteResponse, error) {
	s, err := h.getStore(Token(msg.GetToken()))
	if err != nil {
		return &pb.RsWriteResponse{Error: err.Error()}, nil
	}

	err = s.Write(msg.GetPath(), msg.GetData())
	if err != nil {
		return &pb.RsWriteResponse{Error: err.Error()}, nil
	}

	return &pb.RsWriteResponse{}, nil
}

func (h *handler) GetLastModified(
	msg *pb.RsReadRequest) (*pb.RsTimestampResponse, error) {
	s, err := h.getStore(Token(msg.GetToken()))
	if err != nil {
		return &pb.RsTimestampResponse{Error: err.Error()}, nil
	}

	lastModified, err := s.GetLastModified(msg.GetPath())
	if err != nil {
		return &pb.RsTimestampResponse{Error: err.Error()}, nil
	}

	return &pb.RsTimestampResponse{Timestamp: lastModified.UnixNano()}, nil
}

func (h *handler) GetLastWrite(
	msg *pb.RsLastWriteRequest) (*pb.RsTimestampResponse, error) {
	s, err := h.getStore(Token(msg.GetToken()))
	if err != nil {
		return &pb.RsTimestampResponse{Error: err.Error()}, nil
	}

	lastModified, err := s.GetLastWrite()
	if err != nil {
		return &pb.RsTimestampResponse{Error: err.Error()}, nil
	}

	return &pb.RsTimestampResponse{Timestamp: lastModified.UnixNano()}, nil
}

func (h *handler) ReadDir(msg *pb.RsReadRequest) (*pb.RsReadDirResponse, error) {
	s, err := h.getStore(Token(msg.GetToken()))
	if err != nil {
		return &pb.RsReadDirResponse{Error: err.Error()}, nil
	}

	directories, err := s.ReadDir(msg.GetToken())
	if err != nil {
		return &pb.RsReadDirResponse{Error: err.Error()}, nil
	}

	return &pb.RsReadDirResponse{Data: directories}, nil
}
