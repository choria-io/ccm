// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package scaffoldresource

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"sync"

	"github.com/choria-io/ccm/internal/registry"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources/base"
	"github.com/choria-io/ccm/resources/scaffold/choriascaffold"
)

type Type struct {
	*base.Base

	prop     *model.ScaffoldResourceProperties
	mgr      model.Manager
	log      model.Logger
	provider model.Provider

	mu sync.Mutex
}

var _ model.Resource = (*Type)(nil)
var _ ScaffoldProvider = (*choriascaffold.Provider)(nil)

// New creates a new scaffold resource with the given properties
func New(ctx context.Context, mgr model.Manager, properties model.ScaffoldResourceProperties) (*Type, error) {
	var facts map[string]any
	var data map[string]any

	env, err := mgr.TemplateEnvironment(ctx)
	if err != nil {
		return nil, err
	}
	facts = env.Facts
	data = env.Data

	err = properties.ResolveTemplates(env)
	if err != nil {
		return nil, err
	}

	loggerArgs := []any{"type", model.ScaffoldTypeName, "name", properties.Name}
	logger, err := mgr.Logger(loggerArgs...)
	if err != nil {
		return nil, err
	}

	properties.CommonResourceProperties.Type = model.ScaffoldTypeName

	t := &Type{
		prop: &properties,
		mgr:  mgr,
		log:  logger,
	}
	t.Base = &base.Base{
		Resource:           t,
		ResourceProperties: &properties,
		CommonProperties:   properties.CommonResourceProperties,
		Log:                logger,
		UserLogger:         mgr.UserLogger().With(loggerArgs...),
		Manager:            mgr,
		Facts:              facts,
		Data:               data,
	}

	parsed, _ := url.Parse(properties.Source)
	if parsed == nil || parsed.Scheme == "" {
		if !filepath.IsAbs(properties.Source) {
			t.prop.Source = filepath.Join(mgr.WorkingDirectory(), properties.Source)
		}
	}

	err = t.validate()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", t.String(), model.ErrResourceInvalid, err)
	}

	t.log.Debug("Created resource instance")

	return t, nil
}

func (t *Type) ApplyResource(ctx context.Context) (model.ResourceState, error) {
	var (
		initialStatus *model.ScaffoldState
		finalStatus   *model.ScaffoldState
		refreshState  bool
		p             = t.provider.(ScaffoldProvider)
		properties    = t.prop
		noop          = t.mgr.NoopMode()
		noopMessage   string
		err           error
	)

	env, err := t.mgr.TemplateEnvironment(ctx)
	if err != nil {
		return nil, err
	}

	initialStatus, err = p.Status(ctx, env, t.prop)
	if err != nil {
		return nil, err
	}

	isStable, err := t.isDesiredState(properties, initialStatus, true)
	if err != nil {
		return nil, err
	}

	switch {
	case isStable:
		t.log.Debug("Scaffold is already in desired state")
		refreshState = false
	case noop:
		t.log.Debug("Skipping render in noop mode")
		refreshState = false
	case properties.Ensure == model.EnsureAbsent:
		refreshState = true
		t.log.Debug("Removing scaffold")
		err = p.Remove(ctx, properties, initialStatus)
		if err != nil {
			return nil, err
		}
	case properties.Ensure == model.EnsurePresent:
		refreshState = true
		_, err = p.Scaffold(ctx, env, properties, noop)
		if err != nil {
			return nil, err
		}
	}

	if refreshState {
		finalStatus, err = p.Status(ctx, env, properties)
		if err != nil {
			return nil, err
		}
	} else {
		finalStatus = initialStatus
	}

	if !noop {
		desired, err := t.isDesiredState(properties, finalStatus, false)
		if err != nil {
			return nil, err
		}
		if !desired {
			return nil, fmt.Errorf("%w: %s", model.ErrDesiredStateFailed, properties.Ensure)
		}
	}

	changed := refreshState
	if noop {
		changed = true
	}

	t.FinalizeState(finalStatus, noop, noopMessage, changed, !refreshState, false)

	return finalStatus, nil
}

func (t *Type) isDesiredState(properties *model.ScaffoldResourceProperties, state *model.ScaffoldState, fromStatus bool) (bool, error) {
	meta := state.Metadata

	if properties.Ensure == model.EnsureAbsent {
		stable := len(meta.Stable) == 0 && len(meta.Changed) == 0 && len(meta.Purged) == 0
		exists := meta.TargetExists

		if !exists {
			return true, nil
		}

		return stable, nil
	}

	if fromStatus {
		return len(meta.Changed) == 0 && len(meta.Purged) == 0 && len(meta.Stable) > 0, nil
	} else {
		return len(meta.Changed) > 0 || len(meta.Purged) > 0 || len(meta.Stable) > 0, nil
	}
}

func (t *Type) Info(ctx context.Context) (any, error) {
	_, err := t.SelectProvider()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", t.String(), err)
	}

	env, err := t.mgr.TemplateEnvironment(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", t.String(), err)
	}

	return t.provider.(ScaffoldProvider).Status(ctx, env, t.prop)
}

func (t *Type) validate() error {
	if t.prop.SkipValidate {
		return nil
	}

	err := t.Base.Validate()
	if err != nil {
		return err
	}

	if t.prop.Engine == "" {
		t.prop.Engine = model.ScaffoldEngineJet
	}

	if t.prop.Engine == model.ScaffoldEngineGo {
		if t.prop.LeftDelimiter == "" {
			t.prop.LeftDelimiter = "{{"
		}
		if t.prop.RightDelimiter == "" {
			t.prop.RightDelimiter = "}}"
		}
	} else if t.prop.Engine == model.ScaffoldEngineJet {
		if t.prop.LeftDelimiter == "" {
			t.prop.LeftDelimiter = "[["
		}
		if t.prop.RightDelimiter == "" {
			t.prop.RightDelimiter = "]]"
		}
	}

	return t.prop.Validate()
}

func (t *Type) Provider() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.providerUnlocked()
}

func (t *Type) providerUnlocked() string {
	if t.provider == nil {
		return ""
	}

	return t.provider.Name()
}

func (t *Type) selectProviderUnlocked() error {
	if t.provider != nil {
		return nil
	}

	runner, err := t.mgr.NewRunner()
	if err != nil {
		return err
	}

	selected, err := registry.FindSuitableProvider(model.ScaffoldTypeName, t.prop.Provider, t.Facts, t.prop, t.log, runner)
	if err != nil {
		return err
	}

	if selected == nil {
		return fmt.Errorf("%s#%s: %w", model.ScaffoldTypeName, t.prop.Name, model.ErrNoSuitableProvider)
	}

	t.log.Debug("Selected provider", "provider", selected.Name())
	t.provider = selected

	return nil
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
