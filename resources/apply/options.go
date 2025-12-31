// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
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
