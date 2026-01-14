// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/synadia-io/orbit.go/natscontext"
)

type cachingNatsProvider struct {
	nc *nats.Conn
	mu sync.Mutex
}

func (m *cachingNatsProvider) Connect(natsContext string, opts ...nats.Option) (*nats.Conn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.nc != nil {
		return m.nc, nil
	}

	var err error

	m.nc, _, err = natscontext.Connect(natsContext, opts...)
	if err != nil {
		return nil, err
	}

	return m.nc, nil
}
