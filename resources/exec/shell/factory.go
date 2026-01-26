// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package shell

import (
	"github.com/choria-io/ccm/internal/registry"
	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/model"
)

// Register registers this provider with the registry
func Register() {
	registry.MustRegister(&factory{})
}

type factory struct{}

func (p *factory) TypeName() string { return model.ExecTypeName }
func (p *factory) Name() string     { return ProviderName }
func (p *factory) New(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
	if runner == nil {
		log.Warn("factory called with no runner")
	}

	return NewShellProvider(log, runner)
}
func (p *factory) IsManageable(_ map[string]any, _ model.ResourceProperties) (bool, int, error) {
	return iu.FileExists(shellPath), 99, nil
}
