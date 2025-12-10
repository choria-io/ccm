// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package serviceresource

import (
	"context"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources/service/systemd"
)

func init() {
	systemd.Register()
}

type ServiceProvider interface {
	model.Provider

	Enable(ctx context.Context, service string) error
	Disable(ctx context.Context, service string) error
	Start(ctx context.Context, service string) error
	Stop(ctx context.Context, service string) error
	Restart(ctx context.Context, service string) error
	Status(ctx context.Context, service string) (*model.ServiceState, error)
}
