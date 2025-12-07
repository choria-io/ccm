// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

// Provider is an interface for a resource provider
type Provider interface {
	Name() string
}

type ProviderFactory interface {
	IsManageable(map[string]any) (bool, error)
	TypeName() string
	Name() string
	New(Logger, CommandRunner) (Provider, error)
}
