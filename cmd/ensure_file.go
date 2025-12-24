// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/choria-io/ccm/model"
	fileresource "github.com/choria-io/ccm/resources/file"
	"github.com/choria-io/fisk"
)

type ensureFileCommand struct {
	name         string
	ensure       string
	contentsFile string
	contents     string
	source       string
	owner        string
	mode         string
	parent       *ensureCommand
}

func registerEnsureFileCommand(ccm *fisk.CmdClause, parent *ensureCommand) {
	cmd := &ensureFileCommand{parent: parent}

	file := ccm.Command("file", "File management").Action(cmd.fileAction)
	file.Arg("name", "File name to manage").Required().StringVar(&cmd.name)
	file.Arg("ensure", "Ensure value").Default(model.EnsurePresent).StringVar(&cmd.ensure)
	file.Flag("owner", "File and group (owner:group)").StringVar(&cmd.owner)
	file.Flag("mode", "File mode (octal)").Default("0644").StringVar(&cmd.mode)
	file.Flag("contents", "Contents of the file, will be template parsed").PlaceHolder("STRING").StringVar(&cmd.contents)
	file.Flag("contents-file", "File containing the contents of the file, will be template parsed").PlaceHolder("FILE").ExistingFileVar(&cmd.contentsFile)
	file.Flag("source", "File to copy in place verbatim").PlaceHolder("FILE").ExistingFileVar(&cmd.source)
	parent.addCommonFlags(file)
}

func (c *ensureFileCommand) fileAction(_ *fisk.ParseContext) error {
	if c.contentsFile != "" && c.contents != "" {
		return fmt.Errorf("cannot specify both contents and contents-file")
	}
	if c.source != "" && (c.contents != "" || c.contentsFile != "") {
		return fmt.Errorf("cannot specify both source and contents or contents-file")
	}

	var owner string
	var group string

	parts := strings.SplitN(c.owner, ":", 1)
	if len(parts) == 2 {
		owner = parts[0]
		group = parts[1]
	} else {
		usr, err := user.LookupId(strconv.Itoa(os.Getuid()))
		if err != nil {
			return fmt.Errorf("failed to get current user: %v", err)
		}
		owner = usr.Username

		grp, err := user.LookupGroupId(strconv.Itoa(os.Getgid()))
		if err != nil {
			return fmt.Errorf("failed to get current group: %v", err)
		}
		group = grp.Name
	}

	mgr, err := c.parent.manager()
	if err != nil {
		return err
	}

	properties := model.FileResourceProperties{
		Owner: owner,
		Group: group,
		Mode:  c.mode,
		CommonResourceProperties: model.CommonResourceProperties{
			Name:         c.name,
			Ensure:       c.ensure,
			Provider:     c.parent.provider,
			HealthChecks: c.parent.healthCheckProperties(),
		},
	}

	switch {
	case c.contentsFile != "":
		cbytes, err := os.ReadFile(c.contentsFile)
		if err != nil {
			return err
		}
		properties.Contents = string(cbytes)

	case c.contents != "":
		properties.Contents = c.contents

	case c.source != "":
		properties.Source = c.source
	}

	file, err := fileresource.New(ctx, mgr, properties)
	if err != nil {
		return err
	}

	status, err := file.Apply(ctx)
	if err != nil {
		return err
	}

	err = mgr.RecordEvent(status)
	if err != nil {
		return err
	}

	status.LogStatus(c.parent.out)

	return nil
}
