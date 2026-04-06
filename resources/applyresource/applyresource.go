// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package applyresource

import (
	"context"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources/applyresource/ccmmanifest"
)

type ApplyProvider interface {
	model.Provider

	// ApplyManifest resolves and executes a child manifest within the current session.
	// The provider handles manifest resolution, state save/restore, noop/health_check
	// strengthening, and recursion depth tracking.
	ApplyManifest(ctx context.Context, mgr model.Manager, properties *model.ApplyResourceProperties, currentDepth int, healthCheckOnly bool, log model.Logger) (*model.ApplyState, error)
}

func init() {
	ccmmanifest.Register()
}
