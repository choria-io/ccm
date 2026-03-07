// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package regpublisher

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
	natsSubjectPrefix = "ccm.registration.v1"
	natsTTLHeader     = "Nats-Ttl"
)

// NatsMessagePublisher publishes a NATS message using core NATS
type NatsMessagePublisher interface {
	PublishMsg(msg *nats.Msg) error
}

// JetStreamMessagePublisher publishes a NATS message using JetStream
type JetStreamMessagePublisher interface {
	PublishMsg(ctx context.Context, msg *nats.Msg, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error)
}

// JetStreamFactory creates a JetStream connection from a NATS connection
type JetStreamFactory func(nc *nats.Conn) (JetStreamMessagePublisher, error)

func defaultJetStreamFactory(nc *nats.Conn) (JetStreamMessagePublisher, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, err
	}

	return js, nil
}

type NatsPublisher struct {
	log       model.Logger
	reliable  bool
	nc        NatsMessagePublisher
	rawNC     *nats.Conn
	js        JetStreamMessagePublisher
	jsFactory JetStreamFactory

	mu sync.Mutex
}

func NewNatsPublisher(nc *nats.Conn, log model.Logger) (*NatsPublisher, error) {
	if nc == nil {
		return nil, fmt.Errorf("no nats connection provided")
	}

	return &NatsPublisher{
		nc:        nc,
		rawNC:     nc,
		log:       log,
		jsFactory: defaultJetStreamFactory,
	}, nil
}

func (n *NatsPublisher) Publish(ctx context.Context, entry *model.RegistrationEntry) error {
	var err error
	var js JetStreamMessagePublisher

	n.mu.Lock()
	if n.js == nil {
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

	if js != nil {
		res, err := js.PublishMsg(ctx, msg)
		if err != nil {
			return err
		}
		n.log.Info("Published registration message", "subject", msg.Subject, "stream", res.Stream, "sequence", res.Sequence)
	} else {
		err = n.nc.PublishMsg(msg)
		if err != nil {
			return err
		}
		n.log.Info("Published registration message", "subject", msg.Subject)
	}

	return nil
}

func (n *NatsPublisher) message(e *model.RegistrationEntry) (*nats.Msg, error) {
	msg := nats.NewMsg(n.natsSubject(e, e.InstanceId()))
	if e.TTL > 0 {
		msg.Header.Add(natsTTLHeader, e.TTL.String())
	}

	var err error
	msg.Data, err = json.Marshal(e)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func (n *NatsPublisher) natsSubject(e *model.RegistrationEntry, instance string) string {
	return fmt.Sprintf("%s.%s.%s.%s.%s.%s", natsSubjectPrefix, e.Cluster, e.Protocol, e.Service, e.IP, instance)
}
