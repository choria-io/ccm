// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"sort"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/synadia-io/orbit.go/jetstreamext"

	"github.com/choria-io/ccm/model"
)

// getLastMsgsFor is the function used to query last messages from a JetStream stream.
// It is a variable to allow replacement in tests.
var getLastMsgsFor = func(ctx context.Context, js jetstream.JetStream, stream string, subjects []string) (iter.Seq2[*jetstream.RawStreamMsg, error], error) {
	return jetstreamext.GetLastMsgsFor(ctx, js, stream, subjects)
}

// JetStreamLookup queries registration entries from a JetStream stream.
// Any of cluster, protocol, service, ip can be "*" or "" to wildcard that position.
// Returns entries sorted by Address (string compare), then by port (numeric).
func JetStreamLookup(ctx context.Context, mgr model.Manager, cluster, protocol, service, ip string) (model.RegistrationEntries, error) {
	js, err := mgr.JetStream()
	if err != nil {
		return nil, fmt.Errorf("could not connect to JetStream: %w", err)
	}

	stream := mgr.RegistrationStream()
	subject := filterSubject(cluster, protocol, service, ip)

	msgs, err := getLastMsgsFor(ctx, js, stream, []string{subject})
	if err != nil {
		return nil, fmt.Errorf("could not query registrations: %w", err)
	}

	var entries model.RegistrationEntries
	for msg, err := range msgs {
		if err != nil {
			if errors.Is(err, jetstreamext.ErrNoMessages) {
				return model.RegistrationEntries{}, nil
			}

			return nil, fmt.Errorf("could not read registration message: %w", err)
		}

		// Skip server-generated marker messages for expired, deleted, or
		// purged entries (see ADR-43 in nats-architecture-and-design)
		if isMarkerMessage(msg) {
			continue
		}

		entry := &model.RegistrationEntry{}
		err = json.Unmarshal(msg.Data, entry)
		if err != nil {
			return nil, fmt.Errorf("could not decode registration entry: %w", err)
		}

		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Address != entries[j].Address {
			return entries[i].Address < entries[j].Address
		}
		return portInt(entries[i].Port) < portInt(entries[j].Port)
	})

	return entries, nil
}

// isMarkerMessage returns true when the message is a server-generated marker
// for MaxAge expiry, Remove (delete API), or Purge operations.
func isMarkerMessage(msg *jetstream.RawStreamMsg) bool {
	reason := msg.Header.Get("Nats-Marker-Reason")
	return reason == "MaxAge" || reason == "Remove" || reason == "Purge"
}

// purgeSubject is the function used to purge messages by subject from a JetStream stream.
// It is a variable to allow replacement in tests.
var purgeSubject = func(ctx context.Context, js jetstream.JetStream, streamName string, subject string) error {
	stream, err := js.Stream(ctx, streamName)
	if err != nil {
		return err
	}

	return stream.Purge(ctx, jetstream.WithPurgeSubject(subject))
}

// JetStreamRemove purges a specific registration entry from the JetStream stream.
func JetStreamRemove(ctx context.Context, mgr model.Manager, entry *model.RegistrationEntry) error {
	js, err := mgr.JetStream()
	if err != nil {
		return fmt.Errorf("could not connect to JetStream: %w", err)
	}

	instanceID := entry.InstanceId()
	subject := publishSubject(entry, instanceID)

	err = purgeSubject(ctx, js, mgr.RegistrationStream(), subject)
	if err != nil {
		return fmt.Errorf("could not purge registration entry: %w", err)
	}

	return nil
}

func portInt(v any) int64 {
	switch p := v.(type) {
	case int64:
		return p
	case float64:
		return int64(p)
	case json.Number:
		n, _ := p.Int64()
		return n
	default:
		return 0
	}
}
