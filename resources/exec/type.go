// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package execresource

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/choria-io/ccm/internal/registry"
	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/resources/base"
	"github.com/choria-io/ccm/resources/exec/posix"
)

type Type struct {
	*base.Base

	prop     *model.ExecResourceProperties
	mgr      model.Manager
	log      model.Logger
	provider model.Provider
	facts    map[string]any
	data     map[string]any

	mu sync.Mutex
}

var _ model.Resource = (*Type)(nil)
var _ ExecProvider = (*posix.Provider)(nil)

// New creates a new exec resource with the given properties
func New(ctx context.Context, mgr model.Manager, properties model.ExecResourceProperties) (*Type, error) {
	env, err := mgr.TemplateEnvironment(ctx)
	if err != nil {
		return nil, err
	}

	err = properties.ResolveTemplates(env)
	if err != nil {
		return nil, err
	}

	loggerArgs := []any{"type", model.ExecTypeName, "name", properties.Name}
	logger, err := mgr.Logger(loggerArgs...)
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
	t.Base = &base.Base{
		Resource:           t,
		TypeName:           model.ExecTypeName,
		InstanceName:       properties.Name,
		Ensure:             properties.Ensure,
		InstanceAlias:      properties.Alias,
		ResourceProperties: &properties,
		Log:                logger,
		UserLogger:         mgr.UserLogger().With(loggerArgs...),
		Manager:            mgr,
	}

	err = t.validate()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", t.String(), model.ErrResourceInvalid, err)
	}

	t.log.Debug("Created resource instance")

	return t, nil
}

func (t *Type) validate() error {
	if t.prop.SkipValidate {
		return nil
	}

	return t.prop.Validate()
}

func (t *Type) ApplyResource(ctx context.Context) (model.ResourceState, error) {
	var (
		initialStatus             *model.ExecState
		finalStatus               *model.ExecState
		refreshState              bool
		p                         = t.provider.(ExecProvider)
		properties                = t.prop
		noop                      = t.mgr.NoopMode()
		noopMessage               string
		shouldRefreshViaSubscribe bool
		refreshResource           string
		exitCodePtr               *int
		exitCode                  int
		err                       error
	)

	initialStatus, err = p.Status(ctx, properties)
	if err != nil {
		return nil, err
	}

	shouldRefreshViaSubscribe, refreshResource, err = t.shouldRefresh()
	if err != nil {
		return nil, err
	}

	isStable := t.isDesiredState(properties, initialStatus)
	t.log.Info("Checking desired state", "stable", isStable, "refresh", shouldRefreshViaSubscribe)
	switch {
	case shouldRefreshViaSubscribe:
		t.log.Info("Refreshing via subscribe", "subscribe", refreshResource)

		if noop {
			t.log.Info("Skipping execution as noop")
			noopMessage = "Would have executed via subscribe"
		} else {
			exitCode, err = p.Execute(ctx, properties, t.mgr.UserLogger())
			exitCodePtr = &exitCode
			t.log.Info("Executed", "exitcode", exitCode)
		}

		refreshState = true

	case isStable:
		t.log.Info("Skipping execution as already in desired state", "stable", isStable)

	default:
		if properties.RefreshOnly {
			t.log.Info("Skipping execution as refresh only")
		} else if noop {
			t.log.Info("Skipping execution as noop")
			noopMessage = "Would have executed"
			refreshState = true
		} else {
			exitCode, err = p.Execute(ctx, properties, t.mgr.UserLogger())
			exitCodePtr = &exitCode
			refreshState = true
			t.log.Info("Executed", "exitcode", exitCode)
		}

	}
	if err != nil {
		return nil, err
	}

	if refreshState && !noop {
		finalStatus, err = p.Status(ctx, properties)
		if err != nil {
			return nil, err
		}
		finalStatus.ExitCode = exitCodePtr
	} else {
		finalStatus = initialStatus
	}

	if !noop {
		if !t.isDesiredState(properties, finalStatus) {
			return nil, fmt.Errorf("failed to reach desired state exit code %d", exitCode)
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
	finalStatus.Stable = isStable

	return finalStatus, nil
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

func (t *Type) isDesiredState(properties *model.ExecResourceProperties, status *model.ExecState) bool {
	if properties.Creates != "" && status.CreatesSatisfied {
		return true
	}

	if status.ExitCode == nil && properties.RefreshOnly {
		return true
	}

	returns := []int{0}
	if len(properties.Returns) > 0 {
		returns = properties.Returns
	}

	if status.ExitCode != nil {
		return slices.Contains(returns, *status.ExitCode)
	}

	return false
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

	selected, err := registry.FindSuitableProvider(model.ExecTypeName, t.prop.Provider, t.facts, t.log, runner)
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

func (t *Type) Info(ctx context.Context) (any, error) {
	return nil, fmt.Errorf("not implemented")
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
