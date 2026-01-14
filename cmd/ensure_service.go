// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/choria-io/ccm/model"
	serviceresource "github.com/choria-io/ccm/resources/service"
	"github.com/choria-io/fisk"
)

type ensureServiceCommand struct {
	name         string
	ensure       string
	enable       bool
	enableIsSet  bool
	disable      bool
	disableIsSet bool
	subscribe    []string
	parent       *ensureCommand
}

func registerEnsureServiceCommand(ccm *fisk.CmdClause, parent *ensureCommand) {
	cmd := &ensureServiceCommand{parent: parent}

	svc := ccm.Command("service", "Service management").Alias("svc").Action(cmd.serviceAction)
	svc.Arg("name", "Service name to manage").Required().StringVar(&cmd.name)
	svc.Arg("ensure", "Ensure value").Default(model.ServiceEnsureRunning).StringVar(&cmd.ensure)
	svc.Flag("enable", "Enable the service").IsSetByUser(&cmd.enableIsSet).UnNegatableBoolVar(&cmd.enable)
	svc.Flag("disable", "Disable the service").IsSetByUser(&cmd.disableIsSet).UnNegatableBoolVar(&cmd.disable)
	svc.Flag("subscribe", "Subscribe to changes in other resources").PlaceHolder("type#name").Short('S').StringsVar(&cmd.subscribe)
	parent.addCommonFlags(svc)
}

func (c *ensureServiceCommand) serviceAction(_ *fisk.ParseContext) error {
	mgr, err := c.parent.manager()
	if err != nil {
		return err
	}

	properties := model.ServiceResourceProperties{
		Subscribe: c.subscribe,
		CommonResourceProperties: model.CommonResourceProperties{
			Name:         c.name,
			Ensure:       c.ensure,
			Provider:     c.parent.provider,
			HealthChecks: c.parent.healthCheckProperties(),
		},
	}

	switch {
	case c.enableIsSet && c.disableIsSet:
		return fmt.Errorf("cannot specify both enable and disable flags")
	case c.enableIsSet:
		t := true
		properties.Enable = &t
	case c.disableIsSet:
		f := false
		properties.Enable = &f
	}

	err = c.parent.setCommonProperties(&properties.CommonResourceProperties)
	if err != nil {
		return err
	}

	svc, err := serviceresource.New(ctx, mgr, properties)
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
