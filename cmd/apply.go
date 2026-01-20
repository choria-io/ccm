// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"

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
	factsFile   string
}

func registerApplyCommand(ccm *fisk.Application) {
	cmd := &applyCommand{
		facts: make(map[string]string),
	}

	applyCmd := ccm.Command("apply", "Apply a manifest").Action(cmd.applyAction)
	applyCmd.HelpLong(`Paths to manifests can be given in a few ways

   * manifest.json: A path to a local file on disk
   * obj://BUCKET/file.tgz: Downloads a gzipped tarball from a NATS Object Store
   * https://example.com/file.tgz: Downloads a gzipped tarball from a HTTP Server

When accessing manifests via NATS use the --context flag to provide 
server urls and authentication parameters.
`)
	applyCmd.Arg("manifest", "Path to manifest to apply").PlaceHolder("URL").Required().StringVar(&cmd.manifest)
	applyCmd.Flag("fact", "Set additional facts to merge with the system facts").StringMapVar(&cmd.facts)
	applyCmd.Flag("facts", "File holding additional facts to merge with the system facts").PlaceHolder("FILE").ExistingFileVar(&cmd.factsFile)
	applyCmd.Flag("hiera", "Hiera data file to use as overriding data source").Envar("CCM_HIERA_DATA").StringVar(&cmd.hieraFile)
	applyCmd.Flag("read-env", "Read extra variables from .env file").Default("true").BoolVar(&cmd.readEnv)
	applyCmd.Flag("noop", "Do not make changes, only show what would be done").UnNegatableBoolVar(&cmd.noop)
	applyCmd.Flag("monitor-only", "Only perform monitoring").UnNegatableBoolVar(&cmd.monitorOnly)
	applyCmd.Flag("render", "Do not apply, only render the resolved manifest").UnNegatableBoolVar(&cmd.renderOnly)
	applyCmd.Flag("report", "Generate a report").Default("true").BoolVar(&cmd.report)
	applyCmd.Flag("context", "NATS Context to connect with").Envar("NATS_CONTEXT").Default("CCM").StringVar(&cmd.natsContext)
}

func (c *applyCommand) applyAction(_ *fisk.ParseContext) error {
	finalFacts := iu.MapStringsToMapStringAny(c.facts)

	if c.factsFile != "" {
		fc, err := os.ReadFile(c.factsFile)
		if err != nil {
			return err
		}

		facts := map[string]any{}

		if iu.IsJsonObject(fc) {
			err = json.Unmarshal(fc, &facts)
		} else {
			err = yaml.Unmarshal(fc, &facts)
		}
		if err != nil {
			return err
		}
		finalFacts = iu.DeepMergeMap(finalFacts, facts)
	}

	mgr, userLogger, err := newManager("", "", c.natsContext, c.readEnv, c.noop, finalFacts)
	if err != nil {
		return err
	}

	opts := []apply.Option{
		apply.WithOverridingHieraData(c.hieraFile),
	}

	_, manifest, wd, err := apply.ResolveManifestUrl(ctx, mgr, c.manifest, userLogger, opts...)
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

	if manifest.PreMessage() != "" {
		fmt.Println()
		fmt.Println(manifest.PreMessage())
	}

	_, err = manifest.Execute(ctx, mgr, c.monitorOnly, userLogger)
	if err != nil {
		return err
	}

	if manifest.PostMessage() != "" {
		fmt.Println(manifest.PostMessage())
	}

	if c.report {
		summary, err := mgr.SessionSummary()
		if err != nil {
			return err
		}

		fmt.Println()
		summary.RenderText(os.Stdout)
	}

	return nil
}
