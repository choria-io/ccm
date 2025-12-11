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
	"github.com/choria-io/tinyhiera"
)

func newManager(session string, hieraFile string, readEnv bool) (model.Manager, model.Logger, error) {
	var opts []manager.Option

	if session != "" {
		opts = append(opts, manager.WithSessionDirectory(session))
	}

	out := newOutputLogger()
	logger := newLogger()

	data, err := dotEnvData(readEnv, logger)
	if err != nil {
		return nil, nil, err
	}

	opts = append(opts, manager.WithEnvironmentData(data))

	mgr, err := manager.NewManager(logger, out, opts...)
	if err != nil {
		return nil, nil, err
	}

	if hieraFile != "" {
		abs, err := filepath.Abs(hieraFile)
		if err != nil {
			return nil, nil, err
		}
		if iu.FileExists(abs) {
			// TODO: should this soft fail?
			facts, err := mgr.Facts(ctx)
			if err != nil {
				return nil, nil, err
			}

			if iu.FileExists(abs) {
				data, err := hieraData(abs, facts, logger.With("file", abs))
				if err != nil {
					return nil, nil, err
				}

				mgr.SetData(data)
			}
		}
	}

	return mgr, out, nil
}

func hieraData(file string, facts map[string]any, log model.Logger) (map[string]any, error) {
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

func dotEnvData(readEnv bool, log model.Logger) (map[string]string, error) {
	environ := os.Environ()
	res := make(map[string]string)
	re := regexp.MustCompile(`^(.+?)="*(.+)"*$`)

	if readEnv {
		file, err := filepath.Abs(".env")
		if err != nil {
			return nil, err
		}

		if iu.FileExists(file) {
			log.With("file", file).Info("Reading environment variables from .env file")

			env, err := os.Open(file)
			if err != nil {
				return res, err
			}
			defer env.Close()

			scanner := bufio.NewScanner(env)
			for scanner.Scan() {
				line := scanner.Text()
				matches := re.FindStringSubmatch(line)
				if len(matches) == 3 {
					environ = append(environ, line)
				}
			}
		}
	}

	for _, line := range environ {
		matches := re.FindStringSubmatch(line)
		if len(matches) == 3 {
			res[matches[1]] = matches[2]
		}
	}

	log.Debug("Resolved environment data", "data", res)

	return res, nil
}
