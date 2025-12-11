// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/manager"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/fisk"
	"github.com/choria-io/tinyhiera"
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
	ens.Flag("hiera", "Hiera data file to use as data source").Default(".hiera").StringVar(&cmd.hieraFile)
	ens.Flag("read-env", "Read extra variables from .env file").Default("true").BoolVar(&cmd.readEnv)

	registerPackageCommand(ens, cmd)
	registerServiceCommand(ens, cmd)
}

func (cmd *ensureCommand) manager() (model.Manager, error) {
	var opts []manager.Option

	if cmd.session != "" {
		opts = append(opts, manager.WithSessionDirectory(cmd.session))
	}

	cmd.out = newOutputLogger()
	logger := newLogger()

	if cmd.readEnv {
		abs, err := filepath.Abs(".env")
		if err != nil {
			return nil, err
		}
		if iu.FileExists(abs) {
			data, err := cmd.dotEnvData(abs, logger.With("file", abs))
			if err != nil {
				return nil, err
			}

			opts = append(opts, manager.WithExtraData("env", data))
		}
	}

	mgr, err := manager.NewManager(logger, cmd.out, opts...)
	if err != nil {
		return nil, err
	}

	if cmd.hieraFile != "" {
		abs, err := filepath.Abs(cmd.hieraFile)
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
				data, err := cmd.hieraData(abs, facts, logger.With("file", abs))
				if err != nil {
					return nil, err
				}

				mgr.SetData(data)
			}
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

func (cmd *ensureCommand) dotEnvData(file string, log model.Logger) (map[string]any, error) {
	log.Info("Loading .env data")

	res := make(map[string]any)

	env, err := os.Open(file)
	if err != nil {
		return res, err
	}
	defer env.Close()

	re := regexp.MustCompile(`^(.+?)="(.+)"$`)

	scanner := bufio.NewScanner(env)
	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if len(matches) == 3 {
			res[matches[1]] = matches[2]
		}
	}

	log.Debug("Resolved env data", "data", res)

	return res, nil
}
