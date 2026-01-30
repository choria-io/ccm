// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources"
	"github.com/choria-io/fisk"
)

type ensureApiCommand struct {
	yaml   bool
	parent *ensureCommand
}

func registerEnsureApiCommand(ccm *fisk.CmdClause, parent *ensureCommand) {
	cmd := &ensureApiCommand{parent: parent}

	api := ccm.Command("api", "Resource management API")

	piped := api.Command("piped", "STDIN/STDOUT based piped API").Alias("pipe").Action(cmd.pipedAction)
	piped.Flag("yaml", "Output YAML instead of JSON").BoolVar(&cmd.yaml)
}

func (c *ensureApiCommand) pipedAction(_ *fisk.ParseContext) error {
	event, _, err := c.apply()

	format := model.JsonRequestEncoding
	if c.yaml {
		format = model.YamlRequestEncoding
	}

	b, err := model.MarshalResourceEnsureApiResponse(format, event, err)
	if err != nil {
		return err
	}

	fmt.Println(string(b))
	return nil
}

func (c *ensureApiCommand) apply() (*model.TransactionEvent, model.RequestEncoding, error) {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, model.JsonRequestEncoding, err
	}

	req, err := model.UnmarshalResourceEnsureApiRequest(input)
	if err != nil {
		return nil, model.JsonRequestEncoding, err
	}

	mgr, err := c.parent.manager()
	if err != nil {
		return nil, req.Encoding, err
	}

	env, err := mgr.TemplateEnvironment(ctx)
	if err != nil {
		return nil, req.Encoding, err
	}

	props, err := model.NewValidatedResourcePropertiesFromYaml(req.Type, req.Properties, env)
	if err != nil {
		return nil, req.Encoding, err
	}
	if len(props) == 0 {
		return nil, req.Encoding, fmt.Errorf("no resources found")
	}
	if len(props) > 1 {
		return nil, req.Encoding, fmt.Errorf("multiple resources found")
	}

	resource, err := resources.NewResourceFromProperties(ctx, mgr, props[0])
	if err != nil {
		return nil, req.Encoding, err
	}

	event, err := resource.Apply(ctx)
	return event, req.Encoding, err
}
