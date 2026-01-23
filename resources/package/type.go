// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package packageresource

import (
	"context"
	"fmt"
	"sync"

	"github.com/choria-io/ccm/internal/registry"
	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources/base"
)

// Type represents a package resource that manages software package installation
type Type struct {
	*base.Base

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

	loggerArgs := []any{"type", model.PackageTypeName, "name", properties.Name}
	logger, err := mgr.Logger(loggerArgs...)
	if err != nil {
		return nil, err
	}

	properties.CommonResourceProperties.Type = model.PackageTypeName

	t := &Type{
		prop:  &properties,
		mgr:   mgr,
		log:   logger,
		facts: env.Facts,
		data:  env.Data,
	}
	t.Base = &base.Base{
		Resource:           t,
		ResourceProperties: &properties,
		CommonProperties:   properties.CommonResourceProperties,
		Log:                logger,
		UserLogger:         mgr.UserLogger().With(loggerArgs...),
		Manager:            mgr,
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
		initialStatus *model.PackageState
		finalStatus   *model.PackageState
		refreshState  bool
		p             = t.provider.(PackageProvider)
		properties    = t.prop
		noop          = t.mgr.NoopMode()
		noopMessage   string
	)

	initialStatus, err := p.Status(ctx, t.prop.Name)
	if err != nil {
		return nil, err
	}

	switch {
	case properties.Ensure == "":
		return nil, model.ErrInvalidEnsureValue

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
		t.log.Debug("Comparing versions", "initial", initialStatus, "provider", p.Name(), "ensure", properties.Ensure)
		vc, err := p.VersionCmp(initialStatus.Ensure, properties.Ensure, false)
		if err != nil {
			return nil, err
		}

		t.log.Debug("Package version comparison", "version", initialStatus.Ensure, "provider", p.Name(), "ensure", properties.Ensure, "comparison", vc)

		switch vc {
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
			return nil, fmt.Errorf("%w: %s", model.ErrDesiredStateFailed, properties.Ensure)
		}
	}

	changed := initialStatus.Ensure != finalStatus.Ensure
	if noop && refreshState {
		changed = true
	}
	t.FinalizeState(finalStatus, noop, noopMessage, changed, !refreshState, false)

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
	_, err := t.SelectProvider()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", t.String(), err)
	}

	return t.provider.(PackageProvider).Status(ctx, t.prop.Name)
}

func (t *Type) SelectProvider() (string, error) {
	// TODO: move to base

	t.mu.Lock()
	defer t.mu.Unlock()

	err := t.selectProviderUnlocked()
	if err != nil {
		return "", err
	}

	return t.providerUnlocked(), nil
}

func (t *Type) selectProviderUnlocked() error {
	// TODO: move to base

	if t.provider != nil {
		return nil
	}

	runner, err := t.mgr.NewRunner()
	if err != nil {
		return err
	}

	selected, err := registry.FindSuitableProvider(model.PackageTypeName, t.prop.Provider, t.facts, t.log, runner)
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
