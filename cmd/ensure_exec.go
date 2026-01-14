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
	subscribe   []string
	refreshOnly bool
	parent      *ensureCommand
}

func registerEnsureExecCommand(ccm *fisk.CmdClause, parent *ensureCommand) {
	cmd := &ensureExecCommand{returns: []int{}, parent: parent}

	exec := ccm.Command("exec", "Execution management").Action(cmd.execAction)
	exec.Arg("command", "Command to execute").Required().StringVar(&cmd.command)
	exec.Flag("creates", "File to check for existence").StringVar(&cmd.creates)
	exec.Flag("returns", "Expected return codes").IntsVar(&cmd.returns)
	exec.Flag("timeout", "Command timeout").Default("1m").StringVar(&cmd.timeout)
	exec.Flag("refresh-only", "Only run on subscribed resources").UnNegatableBoolVar(&cmd.refreshOnly)
	exec.Flag("subscribe", "Subscribe to changes in other resources").PlaceHolder("type#name").Short('S').StringsVar(&cmd.subscribe)
	parent.addCommonFlags(exec)
}

func (c *ensureExecCommand) execAction(_ *fisk.ParseContext) error {
	properties := model.ExecResourceProperties{
		CommonResourceProperties: model.CommonResourceProperties{
			Name:     c.command,
			Ensure:   model.EnsurePresent,
			Provider: c.parent.provider,
		},
		Returns:     c.returns,
		Timeout:     c.timeout,
		Creates:     c.creates,
		RefreshOnly: c.refreshOnly,
		Subscribe:   c.subscribe,
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
