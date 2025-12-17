// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/choria-io/ccm/model"
	packageresource "github.com/choria-io/ccm/resources/package"
	"github.com/choria-io/fisk"
	"github.com/goccy/go-yaml"
)

type statusPackageCommand struct {
	name   string
	parent *statusCommand
}

func registerStatusPackageCommand(ccm *fisk.CmdClause, parent *statusCommand) {
	cmd := &statusPackageCommand{parent: parent}

	file := ccm.Command("package", "Package status information").Action(cmd.action)
	file.Arg("name", "Package name to inspect").Required().StringVar(&cmd.name)
}

func (c *statusPackageCommand) action(_ *fisk.ParseContext) error {
	mgr, _, err := newManager("", "", false, true)
	if err != nil {
		return err
	}

	st, err := packageresource.New(ctx, mgr, model.PackageResourceProperties{
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
		return errors.New("no state retrieved")
	}

	nfo, ok := anyNfo.(*model.PackageState)
	if !ok {
		return fmt.Errorf("unexpected state type %T", nfo)
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
