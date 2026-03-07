// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/choria-io/ccm/model"
)

// CreateOrUpdateStream creates or updates the JetStream stream used for registration data.
// Replicas must be between 1 and 5. The stream is configured with subject-level rollup
// to keep only the latest registration per unique subject key.
func CreateOrUpdateStream(ctx context.Context, mgr model.Manager, replicas int, maxAge time.Duration) (jetstream.Stream, error) {
	if replicas < 1 || replicas > 5 {
		return nil, fmt.Errorf("%w: replicas must be between 1 and 5", model.ErrResourceInvalid)
	}

	js, err := mgr.JetStream()
	if err != nil {
		return nil, fmt.Errorf("could not connect to JetStream: %w", err)
	}

	stream := mgr.RegistrationStream()

	cfg := jetstream.StreamConfig{
		Name:                   stream,
		Subjects:               []string{natsSubjectPrefix + ".>"},
		Retention:              jetstream.LimitsPolicy,
		MaxMsgsPerSubject:      1,
		AllowMsgTTL:            true,
		Replicas:               replicas,
		AllowRollup:            true,
		MaxAge:                 maxAge,
		SubjectDeleteMarkerTTL: maxAge,
		DenyDelete:             true,
		DenyPurge:              false,
	}

	s, err := js.CreateOrUpdateStream(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("could not create or update stream %s: %w", stream, err)
	}

	return s, nil
}
