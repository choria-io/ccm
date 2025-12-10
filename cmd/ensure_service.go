// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"

	"github.com/choria-io/ccm/model"
	serviceresource "github.com/choria-io/ccm/resources/service"
	"github.com/choria-io/fisk"
)

type serviceCommand struct {
	name     string
	ensure   string
	enable   *bool
	provider string
	parent   *ensureCommand
}

func registerServiceCommand(ccm *fisk.CmdClause, parent *ensureCommand) {
	cmd := &serviceCommand{parent: parent}

	pkg := ccm.Command("service", "Service management").Alias("pkg").Action(cmd.serviceAction)
	pkg.Arg("name", "Service name to manage").Required().StringVar(&cmd.name)
	pkg.Arg("ensure", "Ensure value").Default("present").StringVar(&cmd.ensure)
	pkg.Flag("enable", "Enable the service").Default("true").BoolVar(cmd.enable)
	pkg.Flag("provider", "Service provider").StringVar(&cmd.provider)
}

func (c *serviceCommand) serviceAction(_ *fisk.ParseContext) error {
	mgr, err := c.parent.manager()
	if err != nil {
		return err
	}

	svc, err := serviceresource.New(ctx, mgr, model.ServiceResourceProperties{
		CommonResourceProperties: model.CommonResourceProperties{
			Name:     c.name,
			Ensure:   c.ensure,
			Provider: c.provider,
		},
		Enable: c.enable,
	})
	if err != nil {
		return err
	}

	status, err := svc.Apply(ctx)
	if err != nil {
		return err
	}

	err = mgr.RecordEvent(status)
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
