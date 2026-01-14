// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/adrg/xdg"

	"github.com/choria-io/appbuilder/builder"
	"github.com/choria-io/appbuilder/commands/exec"
	"github.com/choria-io/appbuilder/commands/parent"
	iu "github.com/choria-io/ccm/internal/util"
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

	registerAgentCommand(app)
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
