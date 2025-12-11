// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package serviceresource

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/choria-io/ccm/internal/registry"
	"github.com/choria-io/ccm/model"
)

type Type struct {
	prop     *model.ServiceResourceProperties
	mgr      model.Manager
	log      model.Logger
	provider model.Provider
	facts    map[string]any
	data     map[string]any

	subscribeType string
	subscribeName string

	mu sync.Mutex
}

var _ model.Resource = (*Type)(nil)

// New creates a new service resource with the given properties
func New(ctx context.Context, mgr model.Manager, properties model.ServiceResourceProperties) (*Type, error) {
	env, err := mgr.TemplateEnvironment(ctx)
	if err != nil {
		return nil, err
	}

	err = properties.ResolveTemplates(env)
	if err != nil {
		return nil, err
	}

	err = properties.Validate()
	if err != nil {
		return nil, err
	}

	logger, err := mgr.Logger("type", model.ServiceTypeName, "name", properties.Name)
	if err != nil {
		return nil, err
	}

	t := &Type{
		prop:  &properties,
		mgr:   mgr,
		log:   logger,
		facts: env.Facts,
		data:  env.Data,
	}

	if properties.Subscribe != "" {
		parts := strings.Split(properties.Subscribe, "#")
		if len(parts) == 2 {
			t.subscribeType = parts[0]
			t.subscribeName = parts[1]
		} else {
			return nil, fmt.Errorf("invalid subscribe property: %q", properties.Subscribe)
		}
	}

	err = t.validate()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", t.String(), model.ErrResourceInvalid, err)
	}

	t.log.Debug("Created resource instance")

	return t, nil
}

func (t *Type) newTransactionEvent() *model.TransactionEvent {
	event := model.NewTransactionEvent(model.ServiceTypeName, t.prop.Name)
	if t.prop != nil {
		event.Properties = t.prop
		event.Name = t.prop.Name
		event.Ensure = t.prop.Ensure
	}

	return event
}

func (t *Type) Apply(ctx context.Context) (*model.TransactionEvent, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	event := t.newTransactionEvent()
	start := time.Now()

	state, err := t.apply(ctx)
	event.Duration = time.Since(start)
	if err != nil {
		event.Failed = true
		event.Error = err.Error()
	}

	if state != nil {
		event.Status = state
		event.Changed = state.Changed
		event.ActualEnsure = state.Ensure
		event.Refreshed = state.Refreshed
	}

	event.Provider = t.providerUnlocked()

	return event, nil
}

func (t *Type) apply(ctx context.Context) (*model.ServiceState, error) {
	err := t.selectProvider()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", t.stringUnlocked(), err)
	}

	var (
		initialStatus             *model.ServiceState
		finalStatus               *model.ServiceState
		refreshState              bool
		p                         = t.provider.(ServiceProvider)
		properties                = t.prop
		shouldRefreshViaSubscribe bool
	)

	initialStatus, err = p.Status(ctx, properties.Name)
	if err != nil {
		return nil, err
	}

	if t.subscribeType != "" && t.subscribeName != "" {
		shouldRefreshViaSubscribe, err = t.mgr.ShouldRefresh(t.subscribeType, t.subscribeName)
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
		t.log.Info("Refreshing via subscribe", "subscribe", t.subscribeType+"#"+t.subscribeName)
		err = p.Restart(ctx, properties.Name)
		if err != nil {
			return nil, err
		}
		refreshState = true

	case properties.Ensure == model.ServiceEnsureStopped && initialStatus.Ensure == model.ServiceEnsureStopped:
		refreshState = false

	case properties.Ensure == model.ServiceEnsureStopped && initialStatus.Ensure != model.ServiceEnsureStopped:
		t.log.Info("Stopping service")
		err = p.Stop(ctx, properties.Name)
		if err != nil {
			return nil, err
		}
		refreshState = true

	case properties.Ensure == model.ServiceEnsureRunning && initialStatus.Ensure == model.ServiceEnsureRunning:
		refreshState = false

	case properties.Ensure == model.ServiceEnsureRunning && initialStatus.Ensure != model.ServiceEnsureRunning:
		t.log.Info("Starting service")
		err = p.Start(ctx, properties.Name)
		if err != nil {
			return nil, err
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
		err = p.Enable(ctx, properties.Name)
		if err != nil {
			return nil, err
		}
		refreshState = true
	case !*properties.Enable && initialStatus.Metadata.Enabled:
		t.log.Info("Disabling service")
		err = p.Disable(ctx, properties.Name)
		if err != nil {
			return nil, err
		}
		refreshState = true
	}

	if refreshState {
		finalStatus, err = p.Status(ctx, properties.Name)
		if err != nil {
			return nil, err
		}
	} else {
		finalStatus = initialStatus
	}

	if !t.isDesiredState(properties, finalStatus) {
		return nil, fmt.Errorf("failed to reach desired state %s", properties.Ensure)
	}

	finalStatus.Changed = refreshState
	finalStatus.Refreshed = shouldRefreshViaSubscribe

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
	err := t.selectProvider()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", t.String(), err)
	}

	return t.provider.(ServiceProvider).Status(ctx, t.prop.Name)
}

func (t *Type) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.stringUnlocked()
}

func (t *Type) stringUnlocked() string {
	return fmt.Sprintf("%s#%s", model.ServiceTypeName, t.prop.Name)
}

func (t *Type) validate() error {
	return t.prop.Validate()
}

// Type returns the resource type name
func (t *Type) Type() string {
	return model.ServiceTypeName
}

// Name returns the service name
func (t *Type) Name() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.prop.Name
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

// Properties returns the service resource properties
func (t *Type) Properties() any {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.prop
}

// TODO: extract to model or something
func (t *Type) selectProvider() error {
	if t.provider != nil {
		return nil
	}

	var selected model.ProviderFactory

	if t.prop.Provider == "" {
		provs, err := registry.SelectProviders(model.ServiceTypeName, t.facts, t.log)
		if err != nil {
			return fmt.Errorf("%w: %w", model.ErrProviderNotFound, err)
		}

		if len(provs) == 0 {
			return model.ErrNoSuitableProvider
		}

		if len(provs) != 1 {
			return model.ErrMultipleProviders
		}

		selected = provs[0]
	} else {
		prov, err := registry.SelectProvider(model.ServiceTypeName, t.prop.Provider, t.facts)
		if err != nil {
			return fmt.Errorf("%w: %w", model.ErrResourceInvalid, err)
		}

		selected = prov
	}

	if selected == nil {
		return model.ErrNoSuitableProvider
	}

	runner, err := t.mgr.NewRunner()
	if err != nil {
		return err
	}

	provider, err := selected.New(t.log, runner)
	if err != nil {
		return fmt.Errorf("%w: %w", model.ErrResourceInvalid, err)
	}
	t.log.Debug("Selected provider", "provider", provider.Name())
	t.provider = provider

	return nil
}
