// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package systemd

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

func (p *factory) TypeName() string { return "service" }
func (p *factory) Name() string     { return ProviderName }
func (p *factory) New(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
	return NewSystemdProvider(log, runner)
}
func (p *factory) IsManageable(_ map[string]any) (bool, int, error) {
	_, found, err := iu.ExecutableInPath("systemctl")
	if err != nil {
		return false, 0, err
	}
	if !found {
		return false, 0, nil
	}

	return true, 1, nil
}
