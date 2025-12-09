// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/choria-io/ccm/manager"
	"github.com/choria-io/fisk"
	"github.com/goccy/go-yaml"
)

type applyCommand struct {
	manifest   string
	renderOnly bool
	session    string
}

func registerApplyCommand(ccm *fisk.Application) {
	cmd := &applyCommand{}

	apply := ccm.Command("apply", "Apply a manifest").Action(cmd.applyAction)
	apply.Arg("manifest", "Path to manifest to apply").ExistingFileVar(&cmd.manifest)
	apply.Flag("render", "Do not apply, only render the resolved manifest").UnNegatableBoolVar(&cmd.renderOnly)
	apply.Flag("session", "Session store to use").Envar("CCM_SESSION_STORE").StringVar(&cmd.session)

}

func (c *applyCommand) applyAction(_ *fisk.ParseContext) error {
	manifest, err := os.Open(c.manifest)
	if err != nil {
		return err
	}

	var opts []manager.Option

	if c.session != "" {
		opts = append(opts, manager.WithSessionDirectory(c.session))
	}

	mgr, err := manager.NewManager(newLogger(), newOutputLogger(), opts...)
	if err != nil {
		return err
	}

	_, apply, err := mgr.ResolveManifestReader(ctx, manifest)
	if err != nil {
		return err
	}

	if c.renderOnly {
		resolvedYaml, err := yaml.Marshal(apply)
		if err != nil {
			return err
		}

		fmt.Println(string(resolvedYaml))

		return nil
	}

	_, err = mgr.ApplyManifest(ctx, apply)
	if err != nil {
		return err
	}

	return nil
}
