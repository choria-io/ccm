// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/fisk"
)

type ensurePackageCommand struct {
	name   string
	ensure string
	parent *ensureCommand
}

func registerEnsurePackageCommand(ccm *fisk.CmdClause, parent *ensureCommand) {
	cmd := &ensurePackageCommand{parent: parent}

	pkg := ccm.Command("package", "Package management").Alias("pkg").Action(cmd.packageAction)
	pkg.Arg("name", "Package name to manage").Required().StringVar(&cmd.name)
	pkg.Arg("ensure", "Ensure value").Default(model.EnsurePresent).StringVar(&cmd.ensure)
	parent.addCommonFlags(pkg)
}

func (c *ensurePackageCommand) packageAction(_ *fisk.ParseContext) error {
	properties := model.PackageResourceProperties{
		CommonResourceProperties: model.CommonResourceProperties{
			Name:     c.name,
			Ensure:   c.ensure,
			Provider: c.parent.provider,
		},
	}

	return c.parent.commonEnsureResource(&properties)
}
