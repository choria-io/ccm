// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package shell

import (
	"bufio"
	"bytes"
	"context"
	"fmt"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/model"
)

const ProviderName = "shell"

var shellPath = "/bin/sh"

type Provider struct {
	log    model.Logger
	runner model.CommandRunner
}

func NewShellProvider(log model.Logger, runner model.CommandRunner) (*Provider, error) {
	return &Provider{log: log, runner: runner}, nil
}

func (p *Provider) Execute(ctx context.Context, properties *model.ExecResourceProperties, log model.Logger) (int, error) {
	if p.runner == nil {
		return -1, fmt.Errorf("no command runner configured")
	}

	cmd := properties.Name
	if properties.Command != "" {
		cmd = properties.Command
	}
	if cmd == "" {
		return -1, fmt.Errorf("no command to execute")
	}

	stdout, _, exitCode, err := p.runner.ExecuteWithOptions(ctx, model.ExtendedExecOptions{
		Command:     shellPath,
		Args:        append([]string{}, "-c", cmd),
		Cwd:         properties.Cwd,
		Environment: properties.Environment,
		Path:        properties.Path,
		Timeout:     properties.ParsedTimeout,
	})

	p.log.Info("Command finished", "command", properties.Name, "exitcode", exitCode)

	if properties.LogOutput && log != nil {
		scanner := bufio.NewScanner(bytes.NewReader(stdout))
		for scanner.Scan() {
			log.Info(scanner.Text())
		}
	}

	return exitCode, err
}

func (p *Provider) Status(ctx context.Context, properties *model.ExecResourceProperties) (*model.ExecState, error) {
	res := &model.ExecState{
		CommonResourceState: model.NewCommonResourceState(model.ResourceStatusExecProtocol, model.ExecTypeName, properties.Name, model.EnsurePresent),
		CreatesSatisfied:    properties.Creates != "" && iu.FileExists(properties.Creates),
	}

	return res, nil
}

func (p *Provider) Name() string {
	return ProviderName
}
