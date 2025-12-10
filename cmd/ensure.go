// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/manager"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/fisk"
	"github.com/choria-io/tinyhiera"
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

	mgr, err := manager.NewManager(newLogger(), cmd.out, opts...)
	if err != nil {
		return nil, err
	}

	abs, err := filepath.Abs(".hiera")
	if err != nil {
		return nil, err
	}

	if iu.FileExists(abs) {
		// TODO: should this soft fail?
		facts, err := mgr.Facts(ctx)
		if err != nil {
			return nil, err
		}

		if iu.FileExists(abs) {
			logger, err := mgr.Logger("file", abs)
			if err != nil {
				return nil, err
			}

			data, err := cmd.hieraData(abs, facts, logger)
			if err != nil {
				return nil, err
			}

			mgr.SetData(data)
		}
	}

	return mgr, nil
}

func (cmd *ensureCommand) hieraData(file string, facts map[string]any, log model.Logger) (map[string]any, error) {
	log.Info("Loading hiera data")
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	res, err := tinyhiera.ResolveYaml(raw, facts, tinyhiera.DefaultOptions, newLogger())
	if err != nil {
		return nil, err
	}

	log.Debug("Resolved hiera data", "data", res)

	return res, nil
}
