// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/choria-io/ccm/model"
	serviceresource "github.com/choria-io/ccm/resources/service"
	"github.com/choria-io/fisk"
	"github.com/goccy/go-yaml"
)

type statusServiceCommand struct {
	name   string
	parent *statusCommand
}

func registerStatusServiceCommand(ccm *fisk.CmdClause, parent *statusCommand) {
	cmd := &statusServiceCommand{parent: parent}

	file := ccm.Command("service", "Service status information").Action(cmd.action)
	file.Arg("name", "Service name to inspect").Required().StringVar(&cmd.name)
}

func (c *statusServiceCommand) action(_ *fisk.ParseContext) error {
	mgr, _, err := newManager("", "", false, true)
	if err != nil {
		return err
	}

	st, err := serviceresource.New(ctx, mgr, model.ServiceResourceProperties{
		CommonResourceProperties: model.CommonResourceProperties{
			Name:         c.name,
			SkipValidate: true,
		},
	})
	if err != nil {
		return err
	}

	anyNfo, err := st.Info(ctx)
	if err != nil {
		return err
	}

	if anyNfo == nil {
		return errors.New("no service state retrieved")
	}

	nfo, ok := anyNfo.(*model.ServiceState)
	if !ok {
		return fmt.Errorf("unexpected service state type %T", nfo)
	}

	if c.parent.json {
		out, err := json.MarshalIndent(nfo.Metadata, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
	} else {
		out, err := yaml.Marshal(nfo.Metadata)
		if err != nil {
			return err
		}
		fmt.Println(string(out))
	}

	return nil
}
