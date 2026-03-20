// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/registration"
	"github.com/choria-io/fisk"
)

type registrationCreateCommand struct {
	cluster     string
	service     string
	protocol    string
	address     string
	port        int64
	priority    int
	ttl         string
	annotations map[string]string
	parent      *registrationCommand
}

func registerRegistrationCreateCommand(reg *fisk.CmdClause, parent *registrationCommand) {
	cmd := &registrationCreateCommand{parent: parent}

	c := reg.Command("create", "Create a registration entry").Alias("c").Action(cmd.createAction)
	c.Flag("cluster", "The cluster name").Required().StringVar(&cmd.cluster)
	c.Flag("service", "The service name").Required().StringVar(&cmd.service)
	c.Flag("protocol", "The protocol name").Required().StringVar(&cmd.protocol)
	c.Flag("address", "The ip or host").Required().StringVar(&cmd.address)
	c.Flag("port", "The port number").Required().Int64Var(&cmd.port)
	c.Flag("priority", "The priority (1-255)").Default("100").IntVar(&cmd.priority)
	c.Flag("ttl", "Time to live for the entry").Default("30s").StringVar(&cmd.ttl)
	c.Flag("annotation", "Annotations in key=value format").Short('A').StringMapVar(&cmd.annotations)
	c.Flag("registration", "The NATS Stream holding registration data").Default("REGISTRATION").Short('R').StringVar(&cmd.parent.registrationStream)
}

func (c *registrationCreateCommand) createAction(_ *fisk.ParseContext) error {
	var ttl *model.RegistrationTTL
	var err error

	if c.ttl != "" {
		ttl, err = model.ParseRegistrationTTL(c.ttl)
		if err != nil {
			return err
		}
	}

	entry, err := model.NewRegistrationEntry(c.cluster, c.service, c.protocol, c.address, c.port, c.priority, ttl)
	if err != nil {
		return err
	}

	entry.Annotations = c.annotations

	err = entry.Validate()
	if err != nil {
		return fmt.Errorf("invalid registration entry: %w", err)
	}

	mgr, userlog, err := newManager("", "", c.parent.natsContext, false, false, c.parent.registrationStream, nil)
	if err != nil {
		return err
	}

	pub, err := registration.New(mgr, model.JetStreamRegistrationDestination)
	if err != nil {
		return err
	}

	err = pub.Publish(context.Background(), entry)
	if err != nil {
		return err
	}

	userlog.Info("Registration entry created",
		"cluster", entry.Cluster,
		"service", entry.Service,
		"protocol", entry.Protocol,
		"address", entry.Address,
		"port", entry.Port,
		"priority", entry.Priority,
	)

	return nil
}
