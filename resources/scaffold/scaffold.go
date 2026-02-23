// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package scaffoldresource

import (
	"context"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources/scaffold/choriascaffold"
	"github.com/choria-io/ccm/templates"
)

func init() {
	choriascaffold.Register()
}

type ScaffoldProvider interface {
	model.Provider
	Remove(ctx context.Context, prop *model.ScaffoldResourceProperties, state *model.ScaffoldState) error
	Scaffold(ctx context.Context, env *templates.Env, prop *model.ScaffoldResourceProperties, noop bool) (*model.ScaffoldState, error)
	Status(ctx context.Context, env *templates.Env, prop *model.ScaffoldResourceProperties) (*model.ScaffoldState, error)
}
