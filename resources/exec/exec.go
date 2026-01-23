// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package execresource

import (
	"context"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources/exec/posix"
	"github.com/choria-io/ccm/resources/exec/shell"
)

func init() {
	posix.Register()
	shell.Register()
}

type ExecProvider interface {
	model.Provider

	Execute(ctx context.Context, properties *model.ExecResourceProperties, log model.Logger) (int, error)
	Status(ctx context.Context, properties *model.ExecResourceProperties) (*model.ExecState, error)
}
