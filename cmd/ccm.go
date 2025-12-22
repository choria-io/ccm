// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/MatusOllah/slogcolor"
	"github.com/adrg/xdg"
	"github.com/choria-io/appbuilder/builder"
	"github.com/choria-io/appbuilder/commands/exec"
	"github.com/choria-io/appbuilder/commands/parent"
	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/manager"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/fisk"
)

var (
	ctx     context.Context
	debug   bool
	info    bool
	Version = "development"
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

	ctx, _ = signal.NotifyContext(context.Background(), os.Interrupt)
	err := extendCli(app)
	if err != nil {
		log.Fatalf("Could not load CLI extensions: %s", err)
	}

	app.MustParseWithUsage(os.Args[1:])
}

func extendCli(app *fisk.Application) error {
	var path string
	var userFile = filepath.Join(xdg.ConfigHome, "choria", "ccm", "cli-extension.yaml")
	var systemFile = "/etc/choria/ccm/cli-extension.yaml"

	if xdg.ConfigHome != "" {
		os.MkdirAll(filepath.Dir(userFile), 0755)
	}

	os.MkdirAll(filepath.Dir(systemFile), 0755)

	if iu.FileExists(userFile) {
		path = userFile
	} else if iu.FileExists(systemFile) {
		path = systemFile
	}

	if path == "" {
		return nil
	}

	def, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	parent.MustRegister()
	exec.MustRegister()

	ext := app.Command("plugin", "External CLI plugin commands").Alias("ext")

	return builder.MountAsCommand(ctx, ext, def, nil)
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
