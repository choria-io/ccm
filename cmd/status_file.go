// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/choria-io/ccm/model"
	fileresource "github.com/choria-io/ccm/resources/file"
	"github.com/choria-io/fisk"
	"github.com/goccy/go-yaml"
)

type statusFileCommand struct {
	name   string
	parent *statusCommand
}

func registerStatusFileCommand(ccm *fisk.CmdClause, parent *statusCommand) {
	cmd := &statusFileCommand{parent: parent}

	file := ccm.Command("file", "File status information").Action(cmd.action)
	file.Arg("name", "File name to inspect").Required().StringVar(&cmd.name)
}

func (c *statusFileCommand) action(_ *fisk.ParseContext) error {
	mgr, _, err := newManager("", "", false, true)
	if err != nil {
		return err
	}

	abs, err := filepath.Abs(c.name)
	if err != nil {
		return err
	}

	ft, err := fileresource.New(ctx, mgr, model.FileResourceProperties{
		CommonResourceProperties: model.CommonResourceProperties{
			Name:         abs,
			SkipValidate: true,
		},
	})
	if err != nil {
		return err
	}

	anyNfo, err := ft.Info(ctx)
	if err != nil {
		return err
	}

	if anyNfo == nil {
		return errors.New("no file state retrieved")
	}

	nfo, ok := anyNfo.(*model.FileState)
	if !ok {
		return fmt.Errorf("unexpected file state type %T", nfo)
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
