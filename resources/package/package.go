// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package packageresource

import (
	"context"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources/package/apt"
	"github.com/choria-io/ccm/resources/package/dnf"
)

func init() {
	dnf.Register()
	apt.Register()
}

type PackageProvider interface {
	model.Provider

	Install(ctx context.Context, pkg string, version string) error
	Upgrade(ctx context.Context, pkg string, version string) error
	Downgrade(ctx context.Context, pkg string, version string) error
	Uninstall(ctx context.Context, pkg string) error
	Status(ctx context.Context, pkg string) (*model.PackageState, error)
	VersionCmp(versionA, versionB string, ignoreTrailingZeroes bool) (int, error)
}
