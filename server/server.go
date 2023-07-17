////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package server

import (
	"crypto/tls"
	"time"

	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"

	"gitlab.com/elixxir/comms/remoteSync/server"
	"gitlab.com/elixxir/remoteSyncServer/store"
	"gitlab.com/xx_network/primitives/id"
)

// Server contains the comms server and handler.
type Server struct {
	h       *handler
	comms   *server.Comms
	keyPair tls.Certificate
}

// NewServer generates a new server with a remote sync comms server. Returns an
// error if the key pair cannot be generated.
func NewServer(storageDir string, tokenTTL time.Duration, userRecords [][]string,
	id *id.ID, localServer string, certPem, keyPem []byte) (*Server, error) {
	keyPair, err := tls.X509KeyPair(certPem, keyPem)
	if err != nil {
		return nil, errors.Errorf("failed to generate a public/private TLS "+
			"key pair from the cert and key: %+v", err)
	}

	h, err := newHandler(storageDir, tokenTTL, userRecords, store.NewFileStore)
	if err != nil {
		return nil, errors.Errorf("failed to initialize new handler: %+v", err)
	}

	jww.INFO.Printf("Starting remote sync server %s in \"%s\" with sessions "+
		"lasting %s.", localServer, storageDir, tokenTTL)
	s := &Server{
		h:       h,
		comms:   server.StartRemoteSync(id, localServer, h, certPem, keyPem),
		keyPair: keyPair,
	}

	return s, nil
}

// Start starts the comms HTTPS server.
func (s *Server) Start() error {
	jww.INFO.Printf("Serving HTTPS on %s.", s.comms)
	return s.comms.ServeHttps(s.keyPair)
}
