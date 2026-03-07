// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/choria-io/ccm/model"
)

// WatchAction indicates whether a registration event is a new registration or a removal
type WatchAction int

const (
	// Register indicates a new or updated registration entry
	Register WatchAction = iota
	// Remove indicates a registration entry was removed (expired, deleted, or purged)
	Remove
)

func (a WatchAction) String() string {
	switch a {
	case Register:
		return "register"
	case Remove:
		return "remove"
	default:
		return "unknown"
	}
}

// WatchEvent represents a registration change observed by the watcher
type WatchEvent struct {
	Action WatchAction
	Entry  *model.RegistrationEntry
	Reason string
}

// JetStreamWatch subscribes to registration changes using a NATS ordered consumer.
// Events are sent to the returned channel until the context is canceled.
// The channel is closed when the watcher stops.
func JetStreamWatch(ctx context.Context, mgr model.Manager, cluster, protocol, service, ip string) (<-chan *WatchEvent, error) {
	js, err := mgr.JetStream()
	if err != nil {
		return nil, fmt.Errorf("could not connect to JetStream: %w", err)
	}

	stream := mgr.RegistrationStream()
	subject := filterSubject(cluster, protocol, service, ip)

	consumer, err := js.OrderedConsumer(ctx, stream, jetstream.OrderedConsumerConfig{
		FilterSubjects: []string{subject},
		DeliverPolicy:  jetstream.DeliverLastPerSubjectPolicy,
	})
	if err != nil {
		return nil, fmt.Errorf("could not create ordered consumer: %w", err)
	}

	events := make(chan *WatchEvent, 100)

	cctx, err := consumer.Consume(func(msg jetstream.Msg) {
		msg.Ack()

		event, err := parseWatchMessage(msg)
		if err != nil {
			return
		}

		select {
		case events <- event:
		case <-ctx.Done():
		}
	})
	if err != nil {
		close(events)
		return nil, fmt.Errorf("could not start consumer: %w", err)
	}

	go func() {
		<-ctx.Done()
		cctx.Stop()
		close(events)
	}()

	return events, nil
}

func parseWatchMessage(msg jetstream.Msg) (*WatchEvent, error) {
	reason := msg.Headers().Get("Nats-Marker-Reason")

	if reason != "" {
		return &WatchEvent{
			Action: Remove,
			Entry:  parseSubject(msg.Subject()),
			Reason: reason,
		}, nil
	}

	entry := &model.RegistrationEntry{}
	err := json.Unmarshal(msg.Data(), entry)
	if err != nil {
		return nil, fmt.Errorf("could not decode registration entry: %w", err)
	}

	return &WatchEvent{
		Action: Register,
		Entry:  entry,
	}, nil
}
