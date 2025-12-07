// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnf

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

// Name returns the provider name
func (p *DnfProvider) Name() string {
	return "dnf"
}

func (p *factory) TypeName() string { return "package" }
func (p *factory) Name() string     { return "dnf" }
func (p *factory) New(log model.Logger, runner model.CommandRunner) (model.Provider, error) {
	return NewDnfProvider(log, runner)
}
func (p *factory) IsManageable(_ map[string]any) (bool, error) {
	for _, path := range []string{"dnf", "rpm"} {
		_, found, err := iu.ExecutableInPath(path)
		if err != nil {
			return false, err
		}
		if !found {
			return false, nil
		}
	}

	return true, nil
}
