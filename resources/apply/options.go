// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package apply

type Option func(*Apply) error

// WithOverridingHieraData provides an additional hiera url will be applied after the manifest data is resolved but before resources are parsed
func WithOverridingHieraData(u string) Option {
	return func(a *Apply) error {
		a.overridingHieraData = u
		return nil
	}
}

// WithOverridingResolvedData provides an additional map of data that will be merged into the resolved data
func WithOverridingResolvedData(d map[string]any) Option {
	return func(a *Apply) error {
		a.overridingResolvedData = d
		return nil
	}
}
