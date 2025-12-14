// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package packageresource

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/choria-io/ccm/healthcheck"
	"github.com/choria-io/ccm/internal/registry"
	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/model"
)

// Type represents a package resource that manages software package installation
type Type struct {
	prop     *model.PackageResourceProperties
	mgr      model.Manager
	log      model.Logger
	provider model.Provider
	facts    map[string]any
	data     map[string]any

	mu sync.Mutex
}

const (
	// EnsurePresent indicates the package should be installed
	EnsurePresent = model.EnsurePresent
	// EnsureAbsent indicates the package should be removed
	EnsureAbsent = model.EnsureAbsent
	// EnsureLatest indicates the package should be upgraded to the latest version
	EnsureLatest = "latest"
)

var _ model.Resource = (*Type)(nil)

// New creates a new package resource with the given properties
func New(ctx context.Context, mgr model.Manager, properties model.PackageResourceProperties) (*Type, error) {
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

	logger, err := mgr.Logger("type", model.PackageTypeName, "name", properties.Name)
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
	event := model.NewTransactionEvent(model.PackageTypeName, t.prop.Name)
	if t.prop != nil {
		event.Properties = t.prop
		event.Name = t.prop.Name
		event.Ensure = t.prop.Ensure
	}

	return event
}

// Apply executes the package resource, ensuring it matches the desired state
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
		event.Status = *state
		event.Changed = state.Changed
		event.ActualEnsure = state.Ensure
		event.Noop = state.Noop
		event.NoopMessage = state.NoopMessage
	}
	event.Provider = t.providerUnlocked()

	return event, nil
}

func (t *Type) apply(ctx context.Context) (*model.PackageState, error) {
	err := t.selectProvider()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", t.stringUnlocked(), err)
	}

	var (
		initialStatus *model.PackageState
		finalStatus   *model.PackageState
		refreshState  bool
		p             = t.provider.(PackageProvider)
		properties    = t.prop
		noop          = t.mgr.NoopMode()
		noopMessage   string
	)

	initialStatus, err = p.Status(ctx, t.prop.Name)
	if err != nil {
		return nil, err
	}

	switch {
	case properties.Ensure == "":
		return nil, fmt.Errorf("invalid value for ensure")

	case properties.Ensure == EnsureLatest:
		if initialStatus.Ensure == EnsureAbsent {
			t.log.Info("Installing package", "version", initialStatus.Ensure, "provider", p.Name(), "ensure", properties.Ensure)
			if !noop {
				err := p.Install(ctx, properties.Name, properties.Ensure)
				if err != nil {
					return nil, err
				}
			} else {
				t.log.Info("Skipping install as noop")
				noopMessage = "Would have installed latest"
			}
		} else {
			t.log.Info("Upgrading package to latest", "version", initialStatus.Ensure, "provider", p.Name(), "ensure", properties.Ensure)
			if !noop {
				err := p.Upgrade(ctx, properties.Name, properties.Ensure)
				if err != nil {
					return nil, err
				}
			} else {
				t.log.Info("Skipping upgrade as noop")
				noopMessage = "Would have upgraded to latest"
			}
		}

		refreshState = true

	case t.isDesiredState(properties, initialStatus):
		// nothing to do

	case properties.Ensure == EnsureAbsent:
		t.log.Info("Uninstalling package", "version", initialStatus.Ensure, "provider", p.Name(), "ensure", properties.Ensure)

		if !noop {
			err := p.Uninstall(ctx, properties.Name)
			if err != nil {
				return nil, err
			}
		} else {
			t.log.Info("Skipping uninstall as noop")
			noopMessage = "Would have uninstalled"
		}

		refreshState = true

	case initialStatus.Ensure == EnsureAbsent:
		t.log.Info("Installing package", "version", initialStatus.Ensure, "provider", p.Name(), "ensure", properties.Ensure)

		if !noop {
			err := p.Install(ctx, properties.Name, properties.Ensure)
			if err != nil {
				return nil, err
			}
		} else {
			t.log.Info("Skipping install as noop")
			noopMessage = fmt.Sprintf("Would have installed version %s", properties.Ensure)
		}

		refreshState = true

	default:
		switch iu.VersionCmp(initialStatus.Ensure, properties.Ensure, false) {
		case 0:
			t.log.Debug("Package already present", "version", initialStatus.Ensure, "provider", p.Name(), "ensure", properties.Ensure)
			refreshState = false

		case -1:
			t.log.Info("Upgrading package", "version", initialStatus.Ensure, "provider", p.Name(), "ensure", properties.Ensure)

			if !noop {
				err := p.Upgrade(ctx, properties.Name, properties.Ensure)
				if err != nil {
					return nil, err
				}
			} else {
				t.log.Info("Skipping upgrade as noop")
				noopMessage = fmt.Sprintf("Would have upgraded to %s", properties.Ensure)
			}

			refreshState = true

		case 1:
			t.log.Info("Downgrading package", "version", initialStatus.Ensure, "provider", p.Name(), "ensure", properties.Ensure)

			if !noop {
				err := p.Downgrade(ctx, properties.Name, properties.Ensure)
				if err != nil {
					return nil, err
				}
			} else {
				t.log.Info("Skipping downgrade as noop")
				noopMessage = fmt.Sprintf("Would have downgraded to %s", properties.Ensure)
			}

			refreshState = true
		}
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
		finalStatus.Changed = initialStatus.Ensure != finalStatus.Ensure
	}

	return finalStatus, nil
}

func (t *Type) isDesiredState(properties *model.PackageResourceProperties, state *model.PackageState) bool {
	switch properties.Ensure {
	case EnsurePresent: // anything but absent is ok
		return state.Ensure != EnsureAbsent

	case EnsureAbsent: // only absent is ok
		return state.Ensure == EnsureAbsent

	case EnsureLatest: // we dont really know if its latest, OS can lie about it, latest is a bad idea so we check absent
		return state.Ensure != EnsureAbsent

	default:
		if iu.VersionCmp(state.Ensure, properties.Ensure, false) == 0 {
			return true
		}
	}

	return false
}

// Info returns the current status of the package
func (t *Type) Info(ctx context.Context) (any, error) {
	err := t.selectProvider()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", t.String(), err)
	}

	return t.provider.(PackageProvider).Status(ctx, t.prop.Name)
}

// TODO: extract to model or something
func (t *Type) selectProvider() error {
	if t.provider != nil {
		return nil
	}

	var selected model.ProviderFactory

	if t.prop.Provider == "" {
		provs, err := registry.SelectProviders(model.PackageTypeName, t.facts, t.log)
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
		prov, err := registry.SelectProvider(model.PackageTypeName, t.prop.Provider, t.facts)
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

func (t *Type) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.stringUnlocked()
}

func (t *Type) stringUnlocked() string {
	return fmt.Sprintf("%s#%s", model.PackageTypeName, t.prop.Name)
}

func (t *Type) validate() error {
	return t.prop.Validate()
}

// Type returns the resource type name
func (t *Type) Type() string {
	return model.PackageTypeName
}

// Name returns the package name
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

// Properties returns the package resource properties
func (t *Type) Properties() any {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.prop
}
