// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/choria-io/ccm/manager"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/fisk"
)

type ensureCommand struct {
	session string
	out     model.Logger
}

func registerEnsureCommand(ccm *fisk.Application) {
	cmd := &ensureCommand{}

	ens := ccm.Command("ensure", "Manage individual resources")
	ens.Flag("session", "Session store to use").Envar("CCM_SESSION_STORE").StringVar(&cmd.session)

	registerPackageCommand(ens, cmd)
	registerServiceCommand(ens, cmd)
}

func (cmd *ensureCommand) manager() (model.Manager, error) {
	var opts []manager.Option

	if cmd.session != "" {
		opts = append(opts, manager.WithSessionDirectory(cmd.session))
	}

	cmd.out = newOutputLogger()

	return manager.NewManager(newLogger(), cmd.out, opts...)
}
