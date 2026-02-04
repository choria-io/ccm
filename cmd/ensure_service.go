// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/choria-io/ccm/model"
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
	properties := model.ServiceResourceProperties{
		Subscribe: c.subscribe,
		CommonResourceProperties: model.CommonResourceProperties{
			Name:     c.name,
			Ensure:   c.ensure,
			Provider: c.parent.provider,
		},
	}

	switch {
	case !c.parent.sessionIsSet && len(c.subscribe) > 0:
		return fmt.Errorf("session store should be set when using subscribe")
	case c.enableIsSet && c.disableIsSet:
		return fmt.Errorf("cannot specify both enable and disable flags")
	case c.enableIsSet:
		t := true
		properties.Enable = &t
	case c.disableIsSet:
		f := false
		properties.Enable = &f
	}

	return c.parent.commonEnsureResource(&properties)
}
