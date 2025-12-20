// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/MatusOllah/slogcolor"
	"github.com/choria-io/ccm/manager"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/fisk"
)

var (
	ctx     context.Context
	debug   bool
	info    bool
	Version string = "development"
)

func main() {
	app := fisk.New("ccm", "Choria Configuration Management")
	app.Version(Version)
	app.Author("https://choria.io")

	app.Flag("debug", "Enable debug logging").UnNegatableBoolVar(&debug)
	app.Flag("info", "Enable info logging").UnNegatableBoolVar(&info)

	registerApplyCommand(app)
	registerEnsureCommand(app)
	registerFactsCommand(app)
	registerHieraCommand(app)
	registerSessionCommand(app)
	registerStatusCommand(app)

	app.PreAction(func(_ *fisk.ParseContext) error {
		ctx, _ = signal.NotifyContext(context.Background(), os.Interrupt)
		return nil
	})

	app.MustParseWithUsage(os.Args[1:])
}

func newOutputLogger() model.Logger {
	return manager.NewSlogLogger(slog.New(slogcolor.NewHandler(os.Stdout, &slogcolor.Options{Level: slog.LevelInfo})))
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
