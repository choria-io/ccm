// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/choria-io/ccm/manager"
	"github.com/choria-io/fisk"
	"github.com/goccy/go-yaml"
)

type applyCommand struct {
	manifest   string
	renderOnly bool
	report     bool
}

func registerApplyCommand(ccm *fisk.Application) {
	cmd := &applyCommand{}

	apply := ccm.Command("apply", "Apply a manifest").Action(cmd.applyAction)
	apply.Arg("manifest", "Path to manifest to apply").ExistingFileVar(&cmd.manifest)
	apply.Flag("render", "Do not apply, only render the resolved manifest").UnNegatableBoolVar(&cmd.renderOnly)
	apply.Flag("report", "Generate a report").Default("true").BoolVar(&cmd.report)

}

func (c *applyCommand) applyAction(_ *fisk.ParseContext) error {
	manifest, err := os.Open(c.manifest)
	if err != nil {
		return err
	}

	mgr, err := manager.NewManager(newLogger(), newOutputLogger())
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

	if c.report {
		summary, err := mgr.SessionSummary()
		if err != nil {
			return err
		}

		fmt.Println()
		fmt.Println("Manifest Run Summary")
		fmt.Println()
		fmt.Printf("             Run Time: %v\n", summary.TotalDuration.Round(time.Millisecond))
		fmt.Printf("      Total Resources: %d\n", summary.TotalResources)
		fmt.Printf("     Stable Resources: %d\n", summary.StableResources)
		fmt.Printf("    Changed Resources: %d\n", summary.ChangedResources)
		fmt.Printf("     Failed Resources: %d\n", summary.FailedResources)
		fmt.Printf("    Skipped Resources: %d\n", summary.SkippedResources)
		fmt.Printf("  Refreshed Resources: %d\n", summary.RefreshedCount)
		fmt.Printf("         Total Errors: %d\n", summary.TotalErrors)
	}

	return nil
}
