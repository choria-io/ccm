// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/choria-io/ccm/model"
)

const (
	natsTTLHeader            = "Nats-TTL"
	natsRollupHeader         = "Nats-Rollup"
	natsExpectedStreamHeader = "Nats-Expected-Stream"
	natsSubRollup            = "sub"
)

// natsMessagePublisher publishes a NATS message using core NATS
type natsMessagePublisher interface {
	PublishMsg(msg *nats.Msg) error
}

// jetStreamMessagePublisher publishes a NATS message using JetStream
type jetStreamMessagePublisher interface {
	PublishMsg(ctx context.Context, msg *nats.Msg, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error)
}

// jetStreamFactory creates a JetStream connection from a NATS connection
type jetStreamFactory func(nc *nats.Conn) (jetStreamMessagePublisher, error)

func defaultJetStreamFactory(nc *nats.Conn) (jetStreamMessagePublisher, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, err
	}

	return js, nil
}

type natsPublisher struct {
	log       model.Logger
	reliable  bool
	stream    string
	nc        natsMessagePublisher
	rawNC     *nats.Conn
	js        jetStreamMessagePublisher
	jsFactory jetStreamFactory

	mu sync.Mutex
}

func newNatsPublisher(nc *nats.Conn, log model.Logger) (*natsPublisher, error) {
	if nc == nil {
		return nil, fmt.Errorf("no nats connection provided")
	}

	return &natsPublisher{
		nc:        nc,
		rawNC:     nc,
		log:       log,
		jsFactory: defaultJetStreamFactory,
	}, nil
}

func (n *natsPublisher) Publish(ctx context.Context, entry *model.RegistrationEntry) error {
	var err error
	var js jetStreamMessagePublisher

	n.mu.Lock()
	if n.reliable && n.js == nil {
		n.log.Debug("Creating JetStream connection")
		n.js, err = n.jsFactory(n.rawNC)
		if err != nil {
			n.mu.Unlock()
			return err
		}
	}
	js = n.js
	n.mu.Unlock()

	msg, err := n.message(entry)
	if err != nil {
		return err
	}

	if n.reliable {
		res, err := js.PublishMsg(ctx, msg)
		if err != nil {
			return err
		}
		n.log.Debug("Published registration message", "subject", msg.Subject, "stream", res.Stream, "sequence", res.Sequence)
	} else {
		err = n.nc.PublishMsg(msg)
		if err != nil {
			return err
		}
		n.log.Debug("Published registration message", "subject", msg.Subject)
	}

	return nil
}

func (n *natsPublisher) message(e *model.RegistrationEntry) (*nats.Msg, error) {
	msg := nats.NewMsg(n.natsSubject(e, e.InstanceId()))
	if n.reliable {
		if e.TTL > 0 {
			msg.Header.Add(natsTTLHeader, e.TTL.String())
		}
		msg.Header.Add(natsRollupHeader, natsSubRollup)

		if n.stream != "" {
			msg.Header.Add(natsExpectedStreamHeader, n.stream)
		}
	}

	var err error
	msg.Data, err = json.Marshal(e)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func (n *natsPublisher) natsSubject(e *model.RegistrationEntry, instance string) string {
	return publishSubject(e, instance)
}
