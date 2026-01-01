// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/goccy/go-yaml"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/resources/apply"
	"github.com/choria-io/fisk"
)

type applyCommand struct {
	manifest    string
	renderOnly  bool
	report      bool
	hieraFile   string
	readEnv     bool
	noop        bool
	monitorOnly bool
	natsContext string
	facts       map[string]string
}

func registerApplyCommand(ccm *fisk.Application) {
	cmd := &applyCommand{
		facts: make(map[string]string),
	}

	apply := ccm.Command("apply", "Apply a manifest").Action(cmd.applyAction)
	apply.HelpLong(`Paths to manifests can be given in a few ways

   * manifest.json: A path to a local file on disk
   * obj://BUCKET/file.tgz: Downloads a gzipped tarball from a NATS Object Store

When accessing manifests via NATS use the --context flag to provide 
server urls and authentication parameters.
`)
	apply.Arg("manifest", "Path to manifest to apply").PlaceHolder("URL").Required().StringVar(&cmd.manifest)
	apply.Flag("render", "Do not apply, only render the resolved manifest").UnNegatableBoolVar(&cmd.renderOnly)
	apply.Flag("report", "Generate a report").Default("true").BoolVar(&cmd.report)
	apply.Flag("fact", "Set additional facts to merge with the system facts").StringMapVar(&cmd.facts)
	apply.Flag("read-env", "Read extra variables from .env file").Default("true").BoolVar(&cmd.readEnv)
	apply.Flag("noop", "Do not make changes, only show what would be done").UnNegatableBoolVar(&cmd.noop)
	apply.Flag("monitor-only", "Only perform monitoring").UnNegatableBoolVar(&cmd.monitorOnly)
	apply.Flag("hiera", "Hiera data file to use as overriding data source").Envar("CCM_HIERA_DATA").StringVar(&cmd.hieraFile)

	apply.Flag("context", "NATS Context to connect with").Envar("NATS_CONTEXT").Default("CCM").StringVar(&cmd.natsContext)
}

func (c *applyCommand) applyAction(_ *fisk.ParseContext) error {
	mgr, userLogger, err := newManager("", "", c.natsContext, c.readEnv, c.noop, iu.MapStringsToMapStringAny(c.facts))
	if err != nil {
		return err
	}

	_, manifest, wd, err := apply.ResolveManifestUrl(ctx, mgr, c.manifest, userLogger, apply.WithOverridingHieraData(c.hieraFile))
	if err != nil {
		return err
	}
	if wd != "" {
		mgr.SetWorkingDirectory(wd)
		defer os.RemoveAll(wd)
	}

	if c.renderOnly {
		resolvedYaml, err := yaml.Marshal(manifest)
		if err != nil {
			return err
		}

		fmt.Println(string(resolvedYaml))

		return nil
	}

	_, err = manifest.Execute(ctx, mgr, c.monitorOnly, userLogger)
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
		fmt.Printf("   Unmet Requirements: %d\n", summary.RequirementsUnMetCount)
		if summary.HealthCheckOKCount > 0 {
			fmt.Printf("    Checked Resources: %d (ok: %d, critical: %d, warning: %d unknown: %d)\n", summary.HealthCheckedCount, summary.HealthCheckOKCount, summary.HealthCheckCriticalCount, summary.HealthCheckWarningCount, summary.HealthCheckUnknownCount)
		} else {
			fmt.Printf("    Checked Resources: %d\n", summary.HealthCheckedCount)
		}
		fmt.Printf("         Total Errors: %d\n", summary.TotalErrors)
	}

	return nil
}
