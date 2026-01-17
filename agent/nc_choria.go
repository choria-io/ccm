// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/choria-io/tokens"
	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

type choriaNatsProvider struct {
	tokenFile   string
	seedFile    string
	collective  string
	tlsCA       string
	tlsInsecure bool
	logger      *logrus.Entry
	nc          *nats.Conn
	mu          sync.Mutex
}

func newChoriaNatsProvider(cfg *Config) *choriaNatsProvider {
	log := logrus.New()
	log.SetLevel(logrus.WarnLevel)
	log.SetOutput(io.Discard)

	return &choriaNatsProvider{
		tokenFile:   cfg.ChoriaTokenFile,
		seedFile:    cfg.ChoriaSeedFile,
		collective:  cfg.ChoriaCollective,
		tlsCA:       cfg.NatsTLSCA,
		tlsInsecure: cfg.NatsTLSInsecure,
		logger:      logrus.NewEntry(log),
	}
}

func (p *choriaNatsProvider) Connect(servers string, opts ...nats.Option) (*nats.Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.nc != nil {
		return p.nc, nil
	}

	if strings.TrimSpace(servers) == "" {
		return nil, fmt.Errorf("nats servers not set")
	}

	token, err := os.ReadFile(p.tokenFile)
	if err != nil {
		return nil, err
	}
	tokenStr := strings.TrimSpace(string(token))

	inbox, jwth, sigh, err := tokens.NatsConnectionHelpers(tokenStr, p.collective, p.seedFile, p.logger)
	if err != nil {
		return nil, err
	}

	opts = append(opts,
		nats.CustomInboxPrefix(inbox),
		nats.Token(tokenStr),
		nats.UserJWT(jwth, sigh),
	)

	if p.tlsCA != "" || p.tlsInsecure {
		tlsc := &tls.Config{InsecureSkipVerify: p.tlsInsecure}
		if p.tlsCA != "" {
			caCert, err := os.ReadFile(p.tlsCA)
			if err != nil {
				return nil, err
			}
			caPool := x509.NewCertPool()
			if !caPool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("failed to parse nats_tls_ca: %s", p.tlsCA)
			}
			tlsc.RootCAs = caPool
		}
		opts = append(opts, nats.Secure(tlsc))
	}

	p.nc, err = nats.Connect(servers, opts...)
	if err != nil {
		return nil, err
	}

	return p.nc, nil
}
