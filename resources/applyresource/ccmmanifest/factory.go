// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package ccmmanifest

import (
	"github.com/choria-io/ccm/internal/registry"
	"github.com/choria-io/ccm/model"
)

func Register() {
	registry.MustRegister(&factory{})
}

type factory struct{}

func (p *factory) TypeName() string { return model.ApplyTypeName }
func (p *factory) Name() string     { return ProviderName }
func (p *factory) New(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
	return NewProvider(log, runner)
}
func (p *factory) IsManageable(_ map[string]any, prop model.ResourceProperties) (bool, int, error) {
	return true, 1, nil
}
