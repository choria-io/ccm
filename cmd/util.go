// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"

	"github.com/MatusOllah/slogcolor"

	"github.com/choria-io/ccm/hiera"
	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/manager"
	"github.com/choria-io/ccm/model"
)

func newManager(session string, hieraSource string, natsContext string, readEnv bool, noop bool, facts map[string]any) (model.Manager, model.Logger, error) {
	var opts []manager.Option

	if session != "" {
		opts = append(opts, manager.WithSessionDirectory(session))
	}

	if natsContext != "" {
		opts = append(opts, manager.WithNatsContext(natsContext))
	}

	logger := newLogger()
	out := newOutputLogger()

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

	if len(facts) > 0 {
		mgr.MergeFacts(ctx, facts)
	}

	if hieraSource != "" && iu.FileExists(hieraSource) {
		facts, err := mgr.Facts(ctx)
		if err != nil {
			return nil, nil, err
		}

		resolved, err := hiera.ResolveUrl(ctx, hieraSource, mgr, facts, hiera.DefaultOptions, logger)
		if err != nil {
			return nil, nil, err
		}

		mgr.SetData(resolved)
	}

	return mgr, out, nil
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

	return res, nil
}

func newOutputLogger() model.Logger {
	var level slog.Level

	switch {
	case debug:
		level = slog.LevelDebug
	default:
		level = slog.LevelInfo
	}

	return manager.NewSlogLogger(slog.New(slogcolor.NewHandler(os.Stdout, &slogcolor.Options{Level: level})))
}

func newLogger() model.Logger {
	var level slog.Level

	switch {
	case debug:
		level = slog.LevelDebug
	case info:
		level = slog.LevelInfo
	default:
		level = slog.LevelWarn
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	return manager.NewSlogLogger(logger)
}
