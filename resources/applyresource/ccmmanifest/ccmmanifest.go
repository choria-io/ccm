// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package ccmmanifest

import (
	"context"
	"sync"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources/apply"
)

const ProviderName = "ccmmanifest"

type Provider struct {
	savedNoop bool
	savedData map[string]any
	savedWd   string
	log       model.Logger
	runner    model.CommandRunner
	mu        sync.Mutex
}

func NewProvider(log model.Logger, runner model.CommandRunner) (*Provider, error) {
	return &Provider{log: log, runner: runner}, nil
}

func (p *Provider) ApplyManifest(ctx context.Context, mgr model.Manager, properties *model.ApplyResourceProperties, currentDepth int, healthCheckOnly bool, log model.Logger) (*model.ApplyState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.captureState(mgr)
	defer p.restoreState(mgr)

	if !mgr.NoopMode() && properties.Noop {
		log.Debug("Overriding noop mode", "noop", properties.Noop)
		mgr.SetNoopMode(properties.Noop)
	}

	effectiveHC := healthCheckOnly || properties.HealthCheckOnly

	opts := []apply.Option{
		apply.WithSkipSession(),
		apply.WithCurrentDepth(currentDepth + 1),
	}

	if properties.Data != nil {
		log.Debug("Overriding resolved data from apply resource")
		opts = append(opts, apply.WithOverridingResolvedData(properties.Data))
	}

	if !properties.AllowApply {
		opts = append(opts, apply.WithDenyApplyResources())
	}

	_, resolvedApply, _, err := apply.ResolveManifestUrl(ctx, mgr, properties.Name, log, opts...)
	if err != nil {
		return nil, err
	}

	_, err = resolvedApply.Execute(ctx, mgr, effectiveHC, log.With("manifest", properties.Name))
	if err != nil {
		return nil, err
	}

	state := &model.ApplyState{
		CommonResourceState: model.NewCommonResourceState(model.ResourceStatusApplyProtocol, model.ApplyTypeName, properties.Name, model.EnsurePresent),
		ResourceCount:       len(resolvedApply.Resources()),
	}

	return state, nil
}

func (p *Provider) captureState(mgr model.Manager) {
	p.savedNoop = mgr.NoopMode()
	p.savedWd = mgr.WorkingDirectory()
	p.savedData = mgr.Data()
}

func (p *Provider) restoreState(mgr model.Manager) {
	mgr.SetNoopMode(p.savedNoop)
	mgr.SetWorkingDirectory(p.savedWd)
	mgr.SetData(p.savedData)
}

func (p *Provider) Status(_ context.Context, properties *model.ApplyResourceProperties) (*model.ApplyState, error) {
	return &model.ApplyState{
		CommonResourceState: model.NewCommonResourceState(model.ResourceStatusApplyProtocol, model.ApplyTypeName, properties.Name, model.EnsurePresent),
	}, nil
}

func (p *Provider) Name() string {
	return ProviderName
}
