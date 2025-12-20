// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/choria-io/ccm/hiera"
	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/manager"
	"github.com/choria-io/ccm/model"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/synadia-io/orbit.go/natscontext"
)

func newManager(session string, hieraFile string, natsContext string, readEnv bool, noop bool) (model.Manager, model.Logger, error) {
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

	if noop {
		opts = append(opts, manager.WithNoop())
	}

	mgr, err := manager.NewManager(logger, out, opts...)
	if err != nil {
		return nil, nil, err
	}

	if hieraFile != "" {
		facts, err := mgr.Facts(ctx)
		if err != nil {
			return nil, nil, err
		}

		data, err := getHieraData(hieraFile, natsContext, hiera.DefaultOptions, facts, logger)
		if err != nil {
			return nil, nil, err
		}

		mgr.SetData(data)
	}

	return mgr, out, nil
}

func getHieraData(hieraSource string, natsContext string, opts hiera.Options, facts map[string]any, logger model.Logger) (map[string]any, error) {
	// TODO: we probably need a helper for this in hiera package

	if hieraSource == "" {
		return nil, nil
	}

	uri, err := url.Parse(hieraSource)
	if err != nil {
		return nil, err
	}

	var res map[string]any

	switch uri.Scheme {
	case "kv":
		logger.Info("Using kv hiera data source")
		res, err = hieraDataFromJetStream(natsContext, uri, facts, opts, logger)

	case "file", "":
		logger.Info("Using file hiera data source")
		res, err = hieraDataFromFile(uri.Path, facts, opts, logger)

	default:
		return nil, fmt.Errorf("unsupported hiera data source: %s", hieraSource)
	}
	if err != nil {
		return nil, err
	}

	logger.Debug("Resolved hiera data", "data", res)

	return res, nil
}

func hieraDataFromJetStream(natsContext string, uri *url.URL, facts map[string]any, opts hiera.Options, log model.Logger) (map[string]any, error) {
	if natsContext == "" {
		return nil, fmt.Errorf("nats context is required for kv hiera data source")
	}

	if uri.Host == "" {
		return nil, fmt.Errorf("bucket name is required for kv hiera data source")
	}
	if uri.Path == "" {
		return nil, fmt.Errorf("key is required for kv hiera data source")
	}

	nc, _, err := natscontext.Connect(natsContext)
	if err != nil {
		return nil, err
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, err
	}

	opts.JetStream = js

	return hiera.ResolveKeyValue(ctx, uri.Host, strings.TrimPrefix(uri.Path, "/"), facts, opts, log)
}

func hieraDataFromFile(file string, facts map[string]any, opts hiera.Options, logger model.Logger) (map[string]any, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, err
	}

	if !iu.FileExists(abs) {
		return nil, nil
	}

	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}

	return hiera.ResolveYaml(raw, facts, opts, newLogger())
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
