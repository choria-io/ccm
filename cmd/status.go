// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"

	"github.com/choria-io/fisk"
	"github.com/goccy/go-yaml"
)

type statusCommand struct {
	json     bool
	typeName string
	name     string
}

func registerStatusCommand(ccm *fisk.Application) {
	cmd := &statusCommand{}

	status := ccm.Command("status", "Get resource status").Alias("info").Action(cmd.statusAction)
	status.Arg("type", "Type to get status for").Required().EnumVar(&cmd.typeName, "file", "package", "service")
	status.Arg("name", "Resource name to get status for").Required().StringVar(&cmd.name)
	status.Flag("json", "Output status in JSON format").UnNegatableBoolVar(&cmd.json)
}

func (c *statusCommand) statusAction(_ *fisk.ParseContext) error {
	mgr, _, err := newManager("", "", false, true)
	if err != nil {
		return err
	}

	nfo, err := mgr.ResourceInfo(ctx, c.typeName, c.name)
	if err != nil {
		return fmt.Errorf("could not get status: %s", err)
	}

	if c.json {
		out, err := json.MarshalIndent(nfo, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
	} else {
		out, err := yaml.Marshal(nfo)
		if err != nil {
			return err
		}
		fmt.Println(string(out))
	}

	return nil
}
