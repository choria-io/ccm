// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/choria-io/ccm/model"
	serviceresource "github.com/choria-io/ccm/resources/service"
	"github.com/choria-io/fisk"
)

type ensureServiceCommand struct {
	name      string
	ensure    string
	enable    *bool
	subscribe []string
	parent    *ensureCommand
}

func registerEnsureServiceCommand(ccm *fisk.CmdClause, parent *ensureCommand) {
	cmd := &ensureServiceCommand{parent: parent}

	svc := ccm.Command("service", "Service management").Alias("svc").Action(cmd.serviceAction)
	svc.Arg("name", "Service name to manage").Required().StringVar(&cmd.name)
	svc.Arg("ensure", "Ensure value").Default(model.ServiceEnsureRunning).StringVar(&cmd.ensure)
	cmd.enable = svc.Flag("enable", "Enable the service").Default("true").Bool()
	svc.Flag("subscribe", "Subscribe to changes in other resources (type#name)").PlaceHolder("RESOURCE").Short('S').StringsVar(&cmd.subscribe)
	parent.addCommonFlags(svc)
}

func (c *ensureServiceCommand) serviceAction(_ *fisk.ParseContext) error {
	mgr, err := c.parent.manager()
	if err != nil {
		return err
	}

	props := model.ServiceResourceProperties{
		Subscribe: c.subscribe,
		CommonResourceProperties: model.CommonResourceProperties{
			Name:         c.name,
			Ensure:       c.ensure,
			Provider:     c.parent.provider,
			HealthChecks: c.parent.healthCheckProperties(),
		},
	}

	if c.enable != nil {
		props.Enable = c.enable
	}

	svc, err := serviceresource.New(ctx, mgr, props)
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

	status.LogStatus(c.parent.out)

	return nil
}
