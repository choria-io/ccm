// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package choriascaffold

import (
	"github.com/choria-io/ccm/internal/registry"
	"github.com/choria-io/ccm/model"
)

// Register registers this provider with the registry
func Register() {
	registry.MustRegister(&factory{})
}

type factory struct{}

func (p *factory) TypeName() string { return model.ScaffoldTypeName }
func (p *factory) Name() string     { return ProviderName }
func (p *factory) New(log model.Logger, _ model.CommandRunner) (model.Provider, error) {
	return NewChoriaProvider(log)
}
func (p *factory) IsManageable(_ map[string]any, _ model.ResourceProperties) (bool, int, error) {
	return true, 1, nil
}
