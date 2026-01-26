// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"log"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/adrg/xdg"
	"github.com/goccy/go-yaml"

	"github.com/choria-io/appbuilder/commands"

	"github.com/choria-io/appbuilder/builder"
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
	plugins := map[string]string{}

	err := findPluginsInDir(filepath.Join("/etc", "choria", "ccm", "plugins"), plugins)
	if err != nil {
		return nil
	}

	if xdg.ConfigHome != "" {
		err := findPluginsInDir(filepath.Join(xdg.ConfigHome, "choria", "ccm", "plugins"), plugins)
		if err != nil {
			return nil
		}
	}

	if len(plugins) == 0 {
		return nil
	}

	commands.MustRegisterStandardCommands()

	for _, p := range slices.Sorted(maps.Keys(plugins)) {
		def, err := os.ReadFile(plugins[p])
		if err == nil {
			var plugin builder.Definition
			err = yaml.Unmarshal(def, &plugin)
			if err == nil {
				return builder.MountAsCommand(ctx, app, def, nil)
			}
		}
	}

	return nil
}

func findPluginsInDir(dir string, plugins map[string]string) error {
	os.MkdirAll(dir, 0755) // this is just best efforts, ok to fail

	pluginFileRe := regexp.MustCompile("^(.+)-plugin.yaml")

	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if pluginFileRe.MatchString(d.Name()) {
			plugins[pluginFileRe.FindStringSubmatch(d.Name())[1]] = path
		}

		return nil
	})
}
