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

	"github.com/choria-io/ccm/healthcheck"
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

	if t.prop.HealthCheck != nil {
		res, err := healthcheck.Execute(ctx, t.mgr, t.prop.HealthCheck, t.log)
		event.HealthCheck = res
		if err != nil {
			event.Failed = true
			event.Error = err.Error()
		} else {
			if res.Status != model.HealthCheckOK {
				event.Failed = true
				event.Error = fmt.Sprintf("health check status %q", res.Status.String())
			}
		}
	}

	if state != nil {
		event.Status = state
		event.Changed = state.Changed
		event.ActualEnsure = state.Ensure
		event.Refreshed = state.Refreshed
		event.Noop = state.Noop
		event.NoopMessage = state.NoopMessage
	}

	event.Provider = t.providerUnlocked()

	return event, nil
}

func (t *Type) apply(ctx context.Context) (*model.ServiceState, error) {
	err := t.selectProviderUnlocked()
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
		refreshResource           string
		noop                      = t.mgr.NoopMode()
		noopMessage               string
	)

	initialStatus, err = p.Status(ctx, properties.Name)
	if err != nil {
		return nil, err
	}

	if len(t.prop.Subscribe) > 0 {
		shouldRefreshViaSubscribe, refreshResource, err = t.shouldRefresh()
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

	finalStatus.Noop = noop
	finalStatus.NoopMessage = noopMessage
	if noop && refreshState {
		finalStatus.Changed = true
	} else {
		finalStatus.Changed = refreshState
	}
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
		return fmt.Errorf("%s#%s: %w", model.PackageTypeName, t.prop.Name, model.ErrNoSuitableProvider)
	}

	t.log.Debug("Selected provider", "provider", selected.Name())
	t.provider = selected

	return nil
}

func (t *Type) selectProvider() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.selectProviderUnlocked()
}

func (t *Type) shouldRefresh() (bool, string, error) {
	for _, s := range t.prop.Subscribe {
		parts := strings.Split(s, "#")

		// validate already ensured its the right shape
		should, err := t.mgr.ShouldRefresh(parts[0], parts[1])
		if err != nil {
			return false, s, err
		}
		if should {
			return true, s, nil
		}
	}

	return false, "", nil
}
