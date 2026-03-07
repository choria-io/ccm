// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package regpublisher

import (
	"context"

	"github.com/choria-io/ccm/model"
)

type NoopPublisher struct{}

func (n NoopPublisher) Publish(ctx context.Context, entry *model.RegistrationEntry) error {
	return nil
}

func NewNoopPublisher() *NoopPublisher {
	return &NoopPublisher{}
}
