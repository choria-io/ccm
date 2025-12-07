// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/manager"
	"github.com/choria-io/ccm/model"
	packageresource "github.com/choria-io/ccm/resources/package"
	"github.com/choria-io/fisk"
)

type packageCommand struct {
	name     string
	ensure   string
	provider string
}

func registerPackageCommand(ccm *fisk.Application) {
	cmd := &packageCommand{}

	pkg := ccm.Command("package", "Package management").Alias("pkg")

	ensureCmd := pkg.Command("ensure", "Ensure a package is in a given state").Action(cmd.ensureAction)
	ensureCmd.Arg("name", "Package name to manage").Required().StringVar(&cmd.name)
	ensureCmd.Arg("ensure", "Ensure value").Default("present").StringVar(&cmd.ensure)
	ensureCmd.Flag("provider", "Package provider").StringVar(&cmd.provider)

	infoCmd := pkg.Command("info", "Show information about a package").Alias("show").Alias("i").Action(cmd.infoAction)
	infoCmd.Arg("name", "Package name to show information about").Required().StringVar(&cmd.name)
	infoCmd.Flag("provider", "Package provider").StringVar(&cmd.provider)

	uninstallCmd := pkg.Command("uninstall", "Removes a package").Alias("remove").Alias("rm").Action(cmd.rmAction)
	uninstallCmd.Arg("name", "Package name to remove").Required().StringVar(&cmd.name)
	uninstallCmd.Flag("provider", "Package provider").StringVar(&cmd.provider)
}

func (c *packageCommand) rmAction(_ *fisk.ParseContext) error {
	c.ensure = model.EnsureAbsent
	return c.ensureAction(nil)
}

func (c *packageCommand) infoAction(_ *fisk.ParseContext) error {
	mgr, err := manager.NewManager(newLogger(), newOutputLogger())
	if err != nil {
		return err
	}

	pkg, err := packageresource.New(ctx, mgr, model.PackageResourceProperties{
		CommonResourceProperties: model.CommonResourceProperties{
			Name:     c.name,
			Provider: c.provider,
			Ensure:   packageresource.EnsurePresent,
		},
	})
	if err != nil {
		return err
	}

	nfo, err := pkg.Info(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("%v\n", string(iu.DumpJson(nfo)))

	return nil
}

func (c *packageCommand) ensureAction(_ *fisk.ParseContext) error {
	mgr, err := manager.NewManager(newLogger(), newOutputLogger())
	if err != nil {
		return err
	}

	pkg, err := packageresource.New(ctx, mgr, model.PackageResourceProperties{
		CommonResourceProperties: model.CommonResourceProperties{
			Name:     c.name,
			Ensure:   c.ensure,
			Provider: c.provider,
		},
	})
	if err != nil {
		return err
	}

	status, err := pkg.Apply(ctx)
	if err != nil {
		return err
	}

	j, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(j))

	return nil
}
