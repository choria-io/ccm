// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/choria-io/ccm/model"
	packageresource "github.com/choria-io/ccm/resources/package"
	"github.com/choria-io/fisk"
)

type packageCommand struct {
	name   string
	ensure string
	parent *ensureCommand
}

func registerEnsurePackageCommand(ccm *fisk.CmdClause, parent *ensureCommand) {
	cmd := &packageCommand{parent: parent}

	pkg := ccm.Command("package", "Package management").Alias("pkg").Action(cmd.packageAction)
	pkg.Arg("name", "Package name to manage").Required().StringVar(&cmd.name)
	pkg.Arg("ensure", "Ensure value").Default(model.EnsurePresent).StringVar(&cmd.ensure)
	parent.addCommonFlags(pkg)
}

func (c *packageCommand) packageAction(_ *fisk.ParseContext) error {
	mgr, err := c.parent.manager()
	if err != nil {
		return err
	}

	pkg, err := packageresource.New(ctx, mgr, model.PackageResourceProperties{
		CommonResourceProperties: model.CommonResourceProperties{
			Name:         c.name,
			Ensure:       c.ensure,
			Provider:     c.parent.provider,
			HealthChecks: c.parent.healthCheckProperties(),
		},
	})
	if err != nil {
		return err
	}

	status, err := pkg.Apply(ctx)
	if err != nil {
		return err
	}

	err = mgr.RecordEvent(status)
	if err != nil {
		return err
	}

	status.LogStatus(c.parent.out)

	return nil
}
