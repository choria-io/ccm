// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/choria-io/ccm/model"
	execresource "github.com/choria-io/ccm/resources/exec"
	"github.com/choria-io/fisk"
)

type ensureExecCommand struct {
	command     string
	creates     string
	returns     []int
	timeout     string
	cwd         string
	environment []string
	path        string
	subscribe   []string
	refreshOnly bool
	logoutput   bool
	parent      *ensureCommand
}

func registerEnsureExecCommand(ccm *fisk.CmdClause, parent *ensureCommand) {
	cmd := &ensureExecCommand{returns: []int{}, parent: parent}

	exec := ccm.Command("exec", "Execution management").Action(cmd.execAction)
	exec.Arg("command", "Command to execute").Required().StringVar(&cmd.command)
	exec.Flag("creates", "Skip execution if this file exists").PlaceHolder("FILE").StringVar(&cmd.creates)
	exec.Flag("returns", "Expected return codes").Default("0").IntsVar(&cmd.returns)
	exec.Flag("timeout", "Command timeout").Default("1m").StringVar(&cmd.timeout)
	exec.Flag("cwd", "Working directory for command execution").PlaceHolder("DIR").StringVar(&cmd.cwd)
	exec.Flag("environment", "Environment variables in KEY=VALUE format").Short('e').PlaceHolder("KEY=VALUE").StringsVar(&cmd.environment)
	exec.Flag("path", "Search path for executables (colon-separated)").PlaceHolder("PATH").StringVar(&cmd.path)
	exec.Flag("refresh-only", "Only run when notified by a subscribed resource").UnNegatableBoolVar(&cmd.refreshOnly)
	exec.Flag("subscribe", "Subscribe to changes in other resources").PlaceHolder("type#name").Short('S').StringsVar(&cmd.subscribe)
	exec.Flag("logoutput", "Log output of the command").UnNegatableBoolVar(&cmd.logoutput)

	parent.addCommonFlags(exec)
}

func (c *ensureExecCommand) execAction(_ *fisk.ParseContext) error {
	properties := model.ExecResourceProperties{
		CommonResourceProperties: model.CommonResourceProperties{
			Name:     c.command,
			Ensure:   model.EnsurePresent,
			Provider: c.parent.provider,
			Control:  c.parent.control(),
		},
		Returns:     c.returns,
		Timeout:     c.timeout,
		Cwd:         c.cwd,
		Environment: c.environment,
		Path:        c.path,
		Creates:     c.creates,
		RefreshOnly: c.refreshOnly,
		Subscribe:   c.subscribe,
		LogOutput:   c.logoutput,
	}

	err := c.parent.setCommonProperties(&properties.CommonResourceProperties)
	if err != nil {
		return err
	}

	mgr, err := c.parent.manager()
	if err != nil {
		return err
	}

	exec, err := execresource.New(ctx, mgr, properties)
	if err != nil {
		return err
	}

	status, err := exec.Apply(ctx)
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
