// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/fisk"
)

type ensureCommand struct {
	session   string
	hieraFile string
	readEnv   bool
	out       model.Logger
}

func registerEnsureCommand(ccm *fisk.Application) {
	cmd := &ensureCommand{}

	ens := ccm.Command("ensure", "Manage individual resources")
	ens.Flag("session", "Session store to use").Envar("CCM_SESSION_STORE").PlaceHolder("DIRECTORY").StringVar(&cmd.session)
	ens.Flag("hiera", "Hiera data file to use as data source").Default(".hiera").Envar("CCM_HIERA_DATA").StringVar(&cmd.hieraFile)
	ens.Flag("read-env", "Read extra variables from .env file").Default("true").BoolVar(&cmd.readEnv)

	registerPackageCommand(ens, cmd)
	registerServiceCommand(ens, cmd)
}

func (cmd *ensureCommand) manager() (model.Manager, error) {
	mgr, out, err := newManager(cmd.session, cmd.hieraFile, cmd.readEnv)
	if err != nil {
		return nil, err
	}

	cmd.out = out

	return mgr, nil
}
