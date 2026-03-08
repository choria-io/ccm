// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package regpublisher

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"

	"github.com/choria-io/ccm/model"
)

type JetStreamPublisher struct {
	nats *NatsPublisher
}

func NewJetStreamPublisher(nc *nats.Conn, stream string, log model.Logger) (*JetStreamPublisher, error) {
	if nc == nil {
		return nil, fmt.Errorf("no nats connection provided")
	}

	natsPub := &NatsPublisher{
		nc:        nc,
		rawNC:     nc,
		log:       log,
		stream:    stream,
		reliable:  true,
		jsFactory: defaultJetStreamFactory,
	}

	return &JetStreamPublisher{nats: natsPub}, nil
}

func (j *JetStreamPublisher) Publish(ctx context.Context, entry *model.RegistrationEntry) error {
	return j.nats.Publish(ctx, entry)
}
