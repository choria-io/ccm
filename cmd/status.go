// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/choria-io/ccm/internal/registry"
	"github.com/choria-io/fisk"
)

type statusCommand struct {
	json     bool
	typeName string
	name     string
}

func registerStatusCommand(ccm *fisk.Application) {
	cmd := &statusCommand{}

	status := ccm.Command("status", "Get resource status").Alias("info").Action(cmd.statusAction)
	status.Arg("type", fmt.Sprintf("Type to get status for (%s)", strings.Join(registry.Types(), ","))).Required().EnumVar(&cmd.typeName, registry.Types()...)
	status.Arg("name", "Resource name to get status for").Required().StringVar(&cmd.name)
	status.Flag("json", "Output status in JSON format").UnNegatableBoolVar(&cmd.json)
}

func (c *statusCommand) statusAction(_ *fisk.ParseContext) error {
	mgr, _, err := newManager("", "", "", false, true, nil)
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
