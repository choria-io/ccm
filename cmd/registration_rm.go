// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"

	"github.com/AlecAivazis/survey/v2"

	"github.com/choria-io/fisk"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/registration"
)

type registrationRmCommand struct {
	cluster  string
	service  string
	protocol string
	address  string
	port     int64
	force    bool
	parent   *registrationCommand
}

func registerRegistrationRmCommand(reg *fisk.CmdClause, parent *registrationCommand) {
	cmd := &registrationRmCommand{parent: parent}

	c := reg.Command("rm", "Remove a registration entry").Alias("delete").Action(cmd.rmAction)
	c.Flag("cluster", "The cluster name").Required().StringVar(&cmd.cluster)
	c.Flag("service", "The service name").Required().StringVar(&cmd.service)
	c.Flag("protocol", "The protocol name").Required().StringVar(&cmd.protocol)
	c.Flag("address", "The ip or host").Required().StringVar(&cmd.address)
	c.Flag("port", "The port number").Required().Int64Var(&cmd.port)
	c.Flag("force", "Skip confirmation prompt").UnNegatableBoolVar(&cmd.force)
	c.Flag("registration", "The NATS Stream holding registration data").Default("REGISTRATION").Short('R').StringVar(&cmd.parent.registrationStream)
}

func (c *registrationRmCommand) rmAction(_ *fisk.ParseContext) error {
	entry, err := model.NewRegistrationEntry(c.cluster, c.service, c.protocol, c.address, c.port, 1, nil)
	if err != nil {
		return err
	}

	err = entry.Validate()
	if err != nil {
		return fmt.Errorf("invalid registration entry: %w", err)
	}

	if !c.force {
		confirmed := false
		err = survey.AskOne(
			&survey.Confirm{
				Message: fmt.Sprintf("Remove registration entry %s/%s/%s at %s:%d?", c.cluster, c.service, c.protocol, c.address, c.port),
				Default: false,
			},
			&confirmed,
		)
		if err != nil {
			return err
		}

		if !confirmed {
			return nil
		}
	}

	mgr, userlog, err := newManager("", "", c.parent.natsContext, false, false, c.parent.registrationStream, nil)
	if err != nil {
		return err
	}

	err = registration.JetStreamRemove(context.Background(), mgr, entry)
	if err != nil {
		return err
	}

	userlog.Info("Registration entry removed",
		"cluster", entry.Cluster,
		"service", entry.Service,
		"protocol", entry.Protocol,
		"address", entry.Address,
		"port", entry.Port,
	)

	return nil
}
