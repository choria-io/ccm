// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package posix

import (
	"bufio"
	"bytes"
	"context"
	"fmt"

	"github.com/kballard/go-shellquote"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/model"
)

const ProviderName = "posix"

type Provider struct {
	log    model.Logger
	runner model.CommandRunner
}

func NewPosixProvider(log model.Logger, runner model.CommandRunner) (*Provider, error) {
	return &Provider{log: log, runner: runner}, nil
}

func (p *Provider) Execute(ctx context.Context, properties *model.ExecResourceProperties, log model.Logger) (int, error) {
	words, err := shellquote.Split(properties.Name)
	if err != nil {
		return -1, err
	}

	var command string
	var args []string

	switch len(words) {
	case 0:
		return -1, fmt.Errorf("no command specified")
	case 1:
		command = words[0]
	default:
		command = words[0]
		args = words[1:]
	}

	if p.runner == nil {
		return -1, fmt.Errorf("no command runner configured")
	}

	stdout, _, exitCode, err := p.runner.ExecuteWithOptions(ctx, model.ExtendedExecOptions{
		Command:     command,
		Args:        args,
		Cwd:         properties.Cwd,
		Environment: properties.Environment,
		Path:        properties.Path,
		Timeout:     properties.ParsedTimeout,
	})

	p.log.Info("Command finished", "command", command, "exitcode", exitCode)

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
		CommonResourceState: model.NewCommonResourceState(model.ResourceStatusServiceProtocol, model.ServiceTypeName, properties.Name, model.EnsurePresent),
		CreatesSatisfied:    properties.Creates != "" && iu.FileExists(properties.Creates),
	}

	return res, nil
}

func (p *Provider) Name() string {
	return ProviderName
}
