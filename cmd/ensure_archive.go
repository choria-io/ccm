// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
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
	archiveresource "github.com/choria-io/ccm/resources/archive"
	"github.com/choria-io/fisk"
)

type ensureArchiveCommand struct {
	name     string
	url      string
	hdr      map[string]string
	username string
	password string
	checksum string
	extract  string
	cleanup  bool
	owner    string
	creates  string

	parent *ensureCommand
}

func registerEnsureArchiveCommand(ccm *fisk.CmdClause, parent *ensureCommand) {
	cmd := &ensureArchiveCommand{parent: parent}

	archive := ccm.Command("archive", "Archive management").Action(cmd.archiveAction)
	archive.Arg("file", "File to store resulting archive in").Required().StringVar(&cmd.name)
	archive.Arg("url", "URL to download archive from").Required().StringVar(&cmd.url)
	archive.Flag("header", "Add headers to the HTTP requests").Short('H').PlaceHolder("K:V").StringMapVar(&cmd.hdr)
	archive.Flag("username", "HTTP username to use for authentication").PlaceHolder("USER").StringVar(&cmd.username)
	archive.Flag("password", "HTTP password to use for authentication").PlaceHolder("PASS").Envar("HTTP_PASSWORD").StringVar(&cmd.password)
	archive.Flag("checksum", "Hex encoded sha256 checksum of the archive").PlaceHolder("SHA256SUM").StringVar(&cmd.checksum)
	archive.Flag("extract", "Parent directory to extract to").PlaceHolder("DIR").ExistingDirVar(&cmd.extract)
	archive.Flag("creates", "Skip extraction if this file exists").PlaceHolder("FILE").StringVar(&cmd.creates)
	archive.Flag("cleanup", "Removes the archive after extraction").UnNegatableBoolVar(&cmd.cleanup)
	archive.Flag("owner", "User who should own the archive (user:group)").StringVar(&cmd.owner)

	parent.addCommonFlags(archive)
}

func (c *ensureArchiveCommand) archiveAction(_ *fisk.ParseContext) error {
	properties := model.ArchiveResourceProperties{
		CommonResourceProperties: model.CommonResourceProperties{
			Name:     c.name,
			Ensure:   model.EnsurePresent,
			Provider: c.parent.provider,
			Control:  c.parent.control(),
		},
		Url:           c.url,
		Headers:       c.hdr,
		Username:      c.username,
		Password:      c.password,
		Checksum:      c.checksum,
		ExtractParent: c.extract,
		Creates:       c.creates,
		Cleanup:       c.cleanup,
	}

	err := c.parent.setCommonProperties(&properties.CommonResourceProperties)
	if err != nil {
		return err
	}

	parts := strings.SplitN(c.owner, ":", 2)
	if len(parts) == 2 {
		properties.Owner = parts[0]
		properties.Group = parts[1]
	} else {
		usr, err := user.LookupId(strconv.Itoa(os.Getuid()))
		if err != nil {
			return fmt.Errorf("failed to get current user: %v", err)
		}
		properties.Owner = usr.Username

		grp, err := user.LookupGroupId(strconv.Itoa(os.Getgid()))
		if err != nil {
			return fmt.Errorf("failed to get current group: %v", err)
		}
		properties.Group = grp.Name
	}

	mgr, err := c.parent.manager()
	if err != nil {
		return err
	}

	archive, err := archiveresource.New(ctx, mgr, properties)
	if err != nil {
		return err
	}

	status, err := archive.Apply(ctx)
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
