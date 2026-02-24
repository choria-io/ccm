// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"time"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources"
	"github.com/choria-io/fisk"
)

type ensureCommand struct {
	session      string
	sessionIsSet bool
	hieraFile    string
	readEnv      bool
	natsContext  string

	healthCheckCommand string
	healthCheckTries   int
	healthCheckSleep   time.Duration

	conditionIf     string
	conditionUnless string

	alias    string
	noop     bool
	provider string
	requires []string

	out model.Logger
}

func registerEnsureCommand(ccm *fisk.Application) {
	cmd := &ensureCommand{}

	ens := ccm.Command("ensure", "Manage individual resources")
	ens.Flag("noop", "Do not make any changes to the system").UnNegatableBoolVar(&cmd.noop)
	ens.Flag("session", "Session store to use").Envar("CCM_SESSION_STORE").PlaceHolder("DIR").IsSetByUser(&cmd.sessionIsSet).StringVar(&cmd.session)
	ens.Flag("hiera", "Hiera data file to use as data source").Default(".hiera").Envar("CCM_HIERA_DATA").StringVar(&cmd.hieraFile)
	ens.Flag("read-env", "Read extra variables from .env file").Default("true").BoolVar(&cmd.readEnv)
	ens.Flag("context", "NATS Context to connect with").Envar("NATS_CONTEXT").Default("CCM").StringVar(&cmd.natsContext)

	registerEnsureArchiveCommand(ens, cmd)
	registerEnsureExecCommand(ens, cmd)
	registerEnsureFileCommand(ens, cmd)
	registerEnsurePackageCommand(ens, cmd)
	registerEnsureScaffoldCommand(ens, cmd)
	registerEnsureServiceCommand(ens, cmd)
	registerEnsureApiCommand(ens, cmd)
}

// we do it like this in each child so it shows up in the main sub command help without needing explicit --help
func (cmd *ensureCommand) addCommonFlags(app *fisk.CmdClause) {
	app.Flag("alias", "Resource alias").StringVar(&cmd.alias)
	app.Flag("check", "Command to execute for additional health checks").PlaceHolder("COMMAND").StringVar(&cmd.healthCheckCommand)
	app.Flag("check-tries", "Number of times to execute the health check command").Default("5").IntVar(&cmd.healthCheckTries)
	app.Flag("check-sleep", "Time to sleep between health check tries").Default("1s").DurationVar(&cmd.healthCheckSleep)
	app.Flag("if", "Manage resource if it matches this condition").PlaceHolder("CONDITION").StringVar(&cmd.conditionIf)
	app.Flag("unless", "Manage resource unless it matches this condition").PlaceHolder("CONDITION").StringVar(&cmd.conditionUnless)
	app.Flag("provider", "Resource provider").PlaceHolder("NAME").StringVar(&cmd.provider)
	app.Flag("require", "Require success on an earlier resource").PlaceHolder("type#name").StringsVar(&cmd.requires)
}

func (cmd *ensureCommand) manager() (model.Manager, error) {
	if cmd.sessionIsSet && cmd.session == "" {
		return nil, fmt.Errorf("session store should not be empty")
	}

	if cmd.session == "" && len(cmd.requires) > 0 {
		return nil, fmt.Errorf("session store should be set when using requires")
	}

	mgr, out, err := newManager(cmd.session, cmd.hieraFile, cmd.natsContext, cmd.readEnv, cmd.noop, nil)
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

func (cmd *ensureCommand) control() *model.CommonResourceControl {
	if cmd.conditionIf == "" && cmd.conditionUnless == "" {
		return nil
	}

	return &model.CommonResourceControl{
		ManageIf:     cmd.conditionIf,
		ManageUnless: cmd.conditionUnless,
	}
}

func (cmd *ensureCommand) commonEnsureResource(properties model.ResourceProperties) error {
	mgr, err := cmd.manager()
	if err != nil {
		return err
	}

	cp := properties.CommonProperties()

	cp.HealthChecks = cmd.healthCheckProperties()
	cp.Control = cmd.control()
	cp.Require = cmd.requires
	cp.Alias = cmd.alias

	svc, err := resources.NewResourceFromProperties(ctx, mgr, properties)
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

	status.LogStatus(cmd.out)

	return nil
}
