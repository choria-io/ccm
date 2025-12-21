// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"time"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/fisk"
)

type ensureCommand struct {
	session     string
	hieraFile   string
	readEnv     bool
	natsContext string

	healthCheckCommand string
	healthCheckTries   int
	healthCheckSleep   time.Duration

	noop bool

	out model.Logger
}

func registerEnsureCommand(ccm *fisk.Application) {
	cmd := &ensureCommand{}

	ens := ccm.Command("ensure", "Manage individual resources")
	ens.Flag("check", "Command to execute for additional health checks").StringVar(&cmd.healthCheckCommand)
	ens.Flag("check-tries", "Number of times to execute the health check command").Default("5").IntVar(&cmd.healthCheckTries)
	ens.Flag("check-sleep", "Time to sleep between health check tries").Default("1s").DurationVar(&cmd.healthCheckSleep)
	ens.Flag("noop", "Do not make any changes to the system").BoolVar(&cmd.noop)
	ens.Flag("session", "Session store to use").Envar("CCM_SESSION_STORE").PlaceHolder("DIRECTORY").StringVar(&cmd.session)
	ens.Flag("hiera", "Hiera data file to use as data source").Default(".hiera").Envar("CCM_HIERA_DATA").StringVar(&cmd.hieraFile)
	ens.Flag("read-env", "Read extra variables from .env file").Default("true").BoolVar(&cmd.readEnv)
	ens.Flag("context", "NATS Context to connect with").Envar("NATS_CONTEXT").StringVar(&cmd.natsContext)

	registerEnsureFileCommand(ens, cmd)
	registerEnsurePackageCommand(ens, cmd)
	registerEnsureServiceCommand(ens, cmd)
}

func (cmd *ensureCommand) manager() (model.Manager, error) {
	mgr, out, err := newManager(cmd.session, cmd.hieraFile, cmd.natsContext, cmd.readEnv, cmd.noop)
	if err != nil {
		return nil, err
	}

	cmd.out = out

	return mgr, nil
}

func (cmd *ensureCommand) healthCheckProperties() []model.CommonHealthCheck {
	if cmd.healthCheckCommand == "" {
		return nil
	}

	return []model.CommonHealthCheck{{
		Command:       cmd.healthCheckCommand,
		Tries:         cmd.healthCheckTries,
		ParseTrySleep: cmd.healthCheckSleep,
		TrySleep:      cmd.healthCheckSleep.String(),
	}}
}
