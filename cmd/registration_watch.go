// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/choria-io/ccm/registration"
	"github.com/choria-io/fisk"
)

type registrationWatchCommand struct {
	cluster  string
	protocol string
	service  string
	ip       string

	json   bool
	parent *registrationCommand
}

func registerRegistrationWatchCommand(reg *fisk.CmdClause, parent *registrationCommand) {
	cmd := &registrationWatchCommand{parent: parent}

	w := reg.Command("watch", "Watch registration updated in real time").Action(cmd.watchAction)
	w.Arg("cluster", "The cluster to query").Default("*").StringVar(&cmd.cluster)
	w.Arg("protocol", "the protocol to query").Default("*").StringVar(&cmd.protocol)
	w.Arg("service", "The service to query").Default("*").StringVar(&cmd.service)
	w.Arg("ip", "The ip or host to query").Default("*").StringVar(&cmd.ip)
	w.Flag("json", "Render results in JSON format").Default("false").UnNegatableBoolVar(&cmd.json)
	w.Flag("registration", "The NATS Stream holding registration data").Default("REGISTRATION").Short('R').StringVar(&cmd.parent.registrationStream)
}

func (c *registrationWatchCommand) watchAction(_ *fisk.ParseContext) error {
	mgr, userlog, err := newManager("", "", c.parent.natsContext, false, false, c.parent.registrationStream, nil)
	if err != nil {
		return err
	}

	res, err := registration.JetStreamWatch(ctx, mgr, c.cluster, c.protocol, c.service, c.ip)
	if err != nil {
		return err
	}

	for entry := range res {
		r := entry.Entry
		switch entry.Action {
		case registration.Remove:
			userlog.Warn("Removed entry", "cluster", r.Cluster, "service", r.Service, "protocol", r.Protocol, "address", r.Address, "reason", entry.Reason)
		case registration.Register:
			userlog.Info("Updated entry", "cluster", r.Cluster, "service", r.Service, "protocol", r.Protocol, "address", r.Address, "port", r.Port)
		}
	}

	return nil
}
