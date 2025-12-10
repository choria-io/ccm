// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package systemd

import (
	"context"
	"fmt"
	"strings"

	"github.com/choria-io/ccm/model"
)

const (
	ProviderName = "systemd"
)

type Provider struct {
	log    model.Logger
	runner model.CommandRunner
}

// NewSystemdProvider creates a new DNF package provider
func NewSystemdProvider(log model.Logger, runner model.CommandRunner) (*Provider, error) {
	return &Provider{log: log, runner: runner}, nil
}

func (p *Provider) Name() string {
	return ProviderName
}

func (p *Provider) Enable(ctx context.Context, service string) error {
	_, _, _, err := p.runner.Execute(ctx, "systemctl", "enable", "--system", service)

	return err
}

func (p *Provider) Disable(ctx context.Context, service string) error {
	_, _, _, err := p.runner.Execute(ctx, "systemctl", "disable", "--system", service)

	return err
}

func (p *Provider) Restart(ctx context.Context, service string) error {
	_, _, _, err := p.runner.Execute(ctx, "systemctl", "restart", "--system", service)

	return err
}

func (p *Provider) Start(ctx context.Context, service string) error {
	_, _, _, err := p.runner.Execute(ctx, "systemctl", "start", "--system", service)

	return err
}

func (p *Provider) Stop(ctx context.Context, service string) error {
	_, _, _, err := p.runner.Execute(ctx, "systemctl", "stop", "--system", service)

	return err
}

func (p *Provider) Status(ctx context.Context, service string) (*model.ServiceState, error) {
	isActive, err := p.isActive(ctx, service)
	if err != nil {
		return nil, err
	}

	isEnabled, err := p.isEnabled(ctx, service)
	if err != nil {
		return nil, err
	}

	ensure := model.ServiceEnsureStopped
	if isActive {
		ensure = model.ServiceEnsureRunning
	}

	return &model.ServiceState{
		CommonResourceState: model.NewCommonResourceState(model.ResourceStatusServiceProtocol, model.ServiceTypeName, service, ensure),
		Metadata: &model.ServiceMetadata{
			Name:     service,
			Provider: ProviderName,
			Enabled:  isEnabled,
			Running:  isActive,
		},
	}, nil
}

func (p *Provider) isEnabled(ctx context.Context, service string) (bool, error) {
	stdout, _, _, err := p.runner.Execute(ctx, "systemctl", "is-enabled", "--system", service)
	if err != nil {
		return false, err
	}

	switch strings.Trim(string(stdout), "\n") {
	case "enabled", "enabled-runtime", "alias", "static", "indirect", "generated", "transient":
		return true, nil
	case "linked", "linked-runtime", "masked", "masked-runtime", "disabled":
		return false, nil
	default:
		return false, fmt.Errorf("invalid systemctl is-enabled output: %s", string(stdout))
	}
}

func (p *Provider) isActive(ctx context.Context, service string) (bool, error) {
	stdout, _, _, err := p.runner.Execute(ctx, "systemctl", "is-active", "--system", service)
	if err != nil {
		return false, err
	}

	switch strings.Trim(string(stdout), "\n") {
	case "active":
		return true, nil
	case "inactive", "failed", "activating":
		return false, nil
	default:
		return false, fmt.Errorf("invalid systemctl is-active output: %s", string(stdout))
	}
}
