// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/fisk"
)

type ensureScaffoldCommand struct {
	name      string
	ensure    string
	source    string
	skipEmpty bool
	left      string
	right     string
	engine    string
	purge     bool
	post      map[string]string

	parent *ensureCommand
}

func registerEnsureScaffoldCommand(ccm *fisk.CmdClause, parent *ensureCommand) {
	cmd := &ensureScaffoldCommand{parent: parent}

	scaffold := ccm.Command("scaffold", "Scaffold management").Action(cmd.scaffoldAction)
	scaffold.Arg("name", "Target path to render into").Required().StringVar(&cmd.name)
	scaffold.Arg("source", "Source for the scaffold").ExistingDirVar(&cmd.source)
	scaffold.Arg("ensure", "Ensure value").Default(model.EnsurePresent).StringVar(&cmd.ensure)
	scaffold.Flag("skip-empty", "Do not create empty files").BoolVar(&cmd.skipEmpty)
	scaffold.Flag("left-delimiter", "Left template delimiter").StringVar(&cmd.left)
	scaffold.Flag("right-delimiter", "Right template delimiter").StringVar(&cmd.right)
	scaffold.Flag("engine", "Template engine to use (go, jet)").Default("jet").EnumVar(&cmd.engine, string(model.ScaffoldEngineGo), string(model.ScaffoldEngineJet))
	scaffold.Flag("post", "Post processing steps").PlaceHolder("PATTERN=TOOL").StringMapVar(&cmd.post)
	scaffold.Flag("purge", "Purge existing files").UnNegatableBoolVar(&cmd.purge)

	parent.addCommonFlags(scaffold)
}

func (c *ensureScaffoldCommand) scaffoldAction(_ *fisk.ParseContext) error {
	properties := model.ScaffoldResourceProperties{
		SkipEmpty:      c.skipEmpty,
		LeftDelimiter:  c.left,
		RightDelimiter: c.right,
		Source:         c.source,
		Purge:          c.purge,
		CommonResourceProperties: model.CommonResourceProperties{
			Name:     c.name,
			Ensure:   c.ensure,
			Provider: c.parent.provider,
		},
	}

	for k, v := range c.post {
		properties.Post = append(properties.Post, map[string]string{k: v})
	}

	switch c.engine {
	case "jet":
		properties.Engine = model.ScaffoldEngineJet
	case "go":
		properties.Engine = model.ScaffoldEngineGo
	default:
		return fmt.Errorf("unknown engine %q", c.engine)
	}

	return c.parent.commonEnsureResource(&properties)
}
