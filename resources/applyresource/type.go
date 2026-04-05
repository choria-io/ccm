// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package applyresource

import (
	"context"
	"fmt"
	"sync"

	"github.com/choria-io/ccm/internal/registry"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources/applyresource/ccmmanifest"
	"github.com/choria-io/ccm/resources/base"
)

type Type struct {
	*base.Base

	prop     *model.ApplyResourceProperties
	mgr      model.Manager
	log      model.Logger
	provider model.Provider

	mu sync.Mutex
}

var _ model.Resource = (*Type)(nil)
var _ ApplyProvider = (*ccmmanifest.Provider)(nil)

// New creates a new apply resource with the given properties
func New(ctx context.Context, mgr model.Manager, properties model.ApplyResourceProperties) (*Type, error) {
	env, err := mgr.TemplateEnvironment(ctx)
	if err != nil {
		return nil, err
	}

	err = properties.ResolveTemplates(env)
	if err != nil {
		return nil, err
	}

	loggerArgs := []any{"type", model.ApplyTypeName, "name", properties.Name}
	logger, err := mgr.Logger(loggerArgs...)
	if err != nil {
		return nil, err
	}

	properties.CommonResourceProperties.Type = model.ApplyTypeName

	t := &Type{
		prop: &properties,
		mgr:  mgr,
		log:  logger,
	}
	t.Base = &base.Base{
		Resource:           t,
		CommonProperties:   properties.CommonResourceProperties,
		ResourceProperties: &properties,
		Log:                logger,
		UserLogger:         mgr.UserLogger().With(loggerArgs...),
		Manager:            mgr,
		Facts:              env.Facts,
		Data:               env.Data,
	}

	err = t.Base.Validate()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", t.String(), model.ErrResourceInvalid, err)
	}

	t.log.Debug("Created resource instance")

	return t, nil
}

func (t *Type) ApplyResource(ctx context.Context) (model.ResourceState, error) {
	var (
		p          = t.provider.(ApplyProvider)
		properties = t.prop
		noop       = t.mgr.NoopMode()
	)

	initialStatus, err := p.Status(ctx, properties)
	if err != nil {
		return nil, err
	}

	if noop {
		t.log.Info("Skipping apply as noop")
		t.FinalizeState(initialStatus, true, "Would have applied child manifest", true, false, false)
		return initialStatus, nil
	}

	state, err := p.ApplyManifest(ctx, t.mgr, properties, 0, false, t.log)
	if err != nil {
		return nil, err
	}

	t.FinalizeState(state, false, "", true, false, false)

	return state, nil
}

func (t *Type) selectProviderUnlocked() error {
	if t.provider != nil {
		return nil
	}

	runner, err := t.mgr.NewRunner()
	if err != nil {
		return err
	}

	selected, err := registry.FindSuitableProvider(model.ApplyTypeName, t.prop.Provider, t.Facts, t.prop, t.log, runner)
	if err != nil {
		return err
	}

	if selected == nil {
		return model.ErrNoSuitableProvider
	}

	t.log.Debug("Selected provider", "provider", selected.Name())
	t.provider = selected

	return nil
}

func (t *Type) Info(_ context.Context) (any, error) {
	return nil, fmt.Errorf("apply resources do not support info queries")
}

func (t *Type) SelectProvider() (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	err := t.selectProviderUnlocked()
	if err != nil {
		return "", err
	}

	return t.providerUnlocked(), nil
}

func (t *Type) providerUnlocked() string {
	if t.provider == nil {
		return ""
	}

	return t.provider.Name()
}

func (t *Type) Provider() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.providerUnlocked()
}
