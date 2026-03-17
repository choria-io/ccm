// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"

	"github.com/choria-io/ccm/model"
)

type jetStreamPublisher struct {
	nats *natsPublisher
}

func newJetStreamPublisher(nc *nats.Conn, stream string, log model.Logger) (*jetStreamPublisher, error) {
	if nc == nil {
		return nil, fmt.Errorf("no nats connection provided")
	}

	natsPub := &natsPublisher{
		nc:        nc,
		rawNC:     nc,
		log:       log,
		stream:    stream,
		reliable:  true,
		jsFactory: defaultJetStreamFactory,
	}

	return &jetStreamPublisher{nats: natsPub}, nil
}

func (j *jetStreamPublisher) Publish(ctx context.Context, entry *model.RegistrationEntry) error {
	return j.nats.Publish(ctx, entry)
}
