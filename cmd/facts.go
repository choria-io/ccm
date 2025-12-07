// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/choria-io/ccm/manager"
	"github.com/choria-io/fisk"
	"github.com/goccy/go-yaml"
	"github.com/tidwall/gjson"
)

type factsCommand struct {
	yamlFormat bool
	query      string
}

func registerFactsCommand(ccm *fisk.Application) {
	cmd := &factsCommand{}

	facts := ccm.Command("facts", "Shows system facts").Action(cmd.factsAction)
	facts.Arg("query", "Query to execute").StringVar(&cmd.query)
	facts.Flag("yaml", "Output facts in YAML format").UnNegatableBoolVar(&cmd.yamlFormat)
}

func (c *factsCommand) factsAction(_ *fisk.ParseContext) error {
	mgr, err := manager.NewManager(newLogger(), newOutputLogger())
	if err != nil {
		return err
	}

	f, err := mgr.FactsRaw(ctx)
	if err != nil {
		return err
	}

	if c.query != "" {
		res := gjson.GetBytes(f, c.query)
		f = []byte(res.Raw)
	}

	if c.yamlFormat {
		y, err := yaml.JSONToYAML(f)
		if err != nil {
			return err
		}

		fmt.Println(string(y))
		return nil
	}

	j := bytes.NewBuffer([]byte{})
	err = json.Indent(j, f, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(j.String())

	return nil
}
