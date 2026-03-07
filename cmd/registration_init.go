// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/choria-io/ccm/registration"
	"github.com/choria-io/fisk"
)

type registrationInitCommand struct {
	replicas int
	maxAge   time.Duration
	parent   *registrationCommand
}

func registerRegistrationInitCommand(reg *fisk.CmdClause, parent *registrationCommand) {
	cmd := &registrationInitCommand{parent: parent}

	i := reg.Command("init", "Initialize the registration JetStream stream").Action(cmd.initAction)
	i.Flag("replicas", "Number of stream replicas").Default("1").IntVar(&cmd.replicas)
	i.Flag("max-age", "Maximum age of messages in the stream").Default("1m").DurationVar(&cmd.maxAge)
	i.Flag("registration", "The NATS Stream holding registration data").Default("REGISTRATION").Short('R').StringVar(&cmd.parent.registrationStream)
}

func (c *registrationInitCommand) initAction(_ *fisk.ParseContext) error {
	if c.replicas < 1 || c.replicas > 5 {
		return fmt.Errorf("replicas must be between 1 and 5")
	}

	mgr, userlog, err := newManager("", "", c.parent.natsContext, false, false, c.parent.registrationStream, nil)
	if err != nil {
		return err
	}

	s, err := registration.CreateOrUpdateStream(context.Background(), mgr, c.replicas, c.maxAge)
	if err != nil {
		return err
	}

	info, err := s.Info(context.Background())
	if err != nil {
		return err
	}

	userlog.Info("Registration stream ready", "stream", info.Config.Name, "subjects", info.Config.Subjects, "replicas", info.Config.Replicas)

	return nil
}
