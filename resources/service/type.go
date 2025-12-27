// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package serviceresource

import (
	"context"
	"fmt"
	"sync"

	"github.com/choria-io/ccm/internal/registry"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources/base"
)

type Type struct {
	*base.Base

	prop     *model.ServiceResourceProperties
	mgr      model.Manager
	log      model.Logger
	provider model.Provider
	facts    map[string]any
	data     map[string]any

	mu sync.Mutex
}

var _ model.Resource = (*Type)(nil)

// New creates a new service resource with the given properties
func New(ctx context.Context, mgr model.Manager, properties model.ServiceResourceProperties) (*Type, error) {
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

	loggerArgs := []any{"type", model.ServiceTypeName, "name", properties.Name}
	logger, err := mgr.Logger(loggerArgs...)
	if err != nil {
		return nil, err
	}

	t := &Type{
		prop:  &properties,
		mgr:   mgr,
		log:   logger,
		facts: facts,
		data:  data,
	}
	t.Base = &base.Base{
		Resource:           t,
		ResourceProperties: &properties,
		CommonProperties:   properties.CommonResourceProperties,
		Log:                logger,
		UserLogger:         mgr.UserLogger().With(loggerArgs...),
		Manager:            mgr,
	}

	err = t.Validate()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", t.String(), model.ErrResourceInvalid, err)
	}

	t.log.Debug("Created resource instance")

	return t, nil
}

func (t *Type) ApplyResource(ctx context.Context) (model.ResourceState, error) {
	var (
		initialStatus             *model.ServiceState
		finalStatus               *model.ServiceState
		refreshState              bool
		p                         = t.provider.(ServiceProvider)
		properties                = t.prop
		shouldRefreshViaSubscribe bool
		refreshResource           string
		noop                      = t.mgr.NoopMode()
		noopMessage               string
		err                       error
	)

	initialStatus, err = p.Status(ctx, properties.Name)
	if err != nil {
		return nil, err
	}

	if len(properties.Subscribe) > 0 {
		shouldRefreshViaSubscribe, refreshResource, err = t.ShouldRefresh(properties.Subscribe)
		if err != nil {
			return nil, err
		}

		if properties.Ensure != model.ServiceEnsureRunning {
			shouldRefreshViaSubscribe = false
		}

		// its not running and we have ensure running, so we just make it run
		if properties.Ensure == model.ServiceEnsureRunning && initialStatus.Ensure == model.ServiceEnsureStopped {
			shouldRefreshViaSubscribe = false
		}
	}

	switch {
	case shouldRefreshViaSubscribe:
		t.log.Info("Refreshing via subscribe", "subscribe", refreshResource)
		if !noop {
			err = p.Restart(ctx, properties.Name)
			if err != nil {
				return nil, err
			}
		} else {
			t.log.Info("Skipping restart as noop")
			noopMessage = "Would have restarted via subscribe"
		}
		refreshState = true

	case properties.Ensure == model.ServiceEnsureStopped && initialStatus.Ensure == model.ServiceEnsureStopped:
		refreshState = false

	case properties.Ensure == model.ServiceEnsureStopped && initialStatus.Ensure != model.ServiceEnsureStopped:
		t.log.Info("Stopping service")
		if !noop {
			err = p.Stop(ctx, properties.Name)
			if err != nil {
				return nil, err
			}
		} else {
			t.log.Info("Skipping stop as noop")
			noopMessage = "Would have stopped"
		}
		refreshState = true

	case properties.Ensure == model.ServiceEnsureRunning && initialStatus.Ensure == model.ServiceEnsureRunning:
		refreshState = false

	case properties.Ensure == model.ServiceEnsureRunning && initialStatus.Ensure != model.ServiceEnsureRunning:
		t.log.Info("Starting service")
		if !noop {
			err = p.Start(ctx, properties.Name)
			if err != nil {
				return nil, err
			}
		} else {
			t.log.Info("Skipping start as noop")
			noopMessage = "Would have started"
		}
		refreshState = true

	default:
		return nil, fmt.Errorf("invalid state encountered")
	}

	switch {
	case properties.Enable == nil:
		// noop, leave refresh state from ensure alone
	case *properties.Enable && initialStatus.Metadata.Enabled:
		// noop, leave refresh state from ensure alone
	case *properties.Enable && !initialStatus.Metadata.Enabled:
		t.log.Info("Enabling service")
		if !noop {
			err = p.Enable(ctx, properties.Name)
			if err != nil {
				return nil, err
			}
		} else {
			t.log.Info("Skipping enable as noop")
			if noopMessage != "" {
				noopMessage += ", would have enabled"
			} else {
				noopMessage = "Would have enabled"
			}
		}
		refreshState = true
	case !*properties.Enable && initialStatus.Metadata.Enabled:
		t.log.Info("Disabling service")
		if !noop {
			err = p.Disable(ctx, properties.Name)
			if err != nil {
				return nil, err
			}
		} else {
			t.log.Info("Skipping disable as noop")
			if noopMessage != "" {
				noopMessage += ", would have disabled"
			} else {
				noopMessage = "Would have disabled"
			}
		}
		refreshState = true
	}

	if refreshState && !noop {
		finalStatus, err = p.Status(ctx, properties.Name)
		if err != nil {
			return nil, err
		}
	} else {
		finalStatus = initialStatus
	}

	if !noop {
		if !t.isDesiredState(properties, finalStatus) {
			return nil, fmt.Errorf("failed to reach desired state %s", properties.Ensure)
		}
	}

	changed := refreshState
	if noop && refreshState {
		changed = true
	}
	t.FinalizeState(finalStatus, noop, noopMessage, changed, !refreshState, shouldRefreshViaSubscribe)

	return finalStatus, nil
}

func (t *Type) isDesiredState(properties *model.ServiceResourceProperties, state *model.ServiceState) bool {
	switch {
	case properties.Ensure == model.ServiceEnsureStopped && state.Ensure != model.ServiceEnsureStopped:
		return false

	case properties.Ensure == model.ServiceEnsureRunning && state.Ensure != model.ServiceEnsureRunning:
		return false
	}

	switch {
	case properties.Enable == nil:
		// just leave it alone
	case *properties.Enable && !state.Metadata.Enabled:
		return false
	case !*properties.Enable && state.Metadata.Enabled:
		return false
	}

	return true
}

func (t *Type) Info(ctx context.Context) (any, error) {
	_, err := t.SelectProvider()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", t.String(), err)
	}

	return t.provider.(ServiceProvider).Status(ctx, t.prop.Name)
}

func (t *Type) providerUnlocked() string {
	if t.provider == nil {
		return ""
	}

	return t.provider.Name()
}

// Provider returns the name of the selected provider
func (t *Type) Provider() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.providerUnlocked()
}

func (t *Type) selectProviderUnlocked() error {
	if t.provider != nil {
		return nil
	}

	runner, err := t.mgr.NewRunner()
	if err != nil {
		return err
	}

	selected, err := registry.FindSuitableProvider(model.ServiceTypeName, t.prop.Provider, t.facts, t.log, runner)
	if err != nil {
		return err
	}

	if selected == nil {
		return fmt.Errorf("%s#%s: %w", model.ServiceTypeName, t.prop.Name, model.ErrNoSuitableProvider)
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
