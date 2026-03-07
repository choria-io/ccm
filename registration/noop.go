// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"context"

	"github.com/choria-io/ccm/model"
)

type noopPublisher struct{}

func (n noopPublisher) Publish(ctx context.Context, entry *model.RegistrationEntry) error {
	return nil
}

func NewNoopPublisher() *noopPublisher {
	return &noopPublisher{}
}
