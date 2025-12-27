// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package base

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/choria-io/ccm/healthcheck"
	"github.com/choria-io/ccm/metrics"
	"github.com/choria-io/ccm/model"
)

// EmbeddedResource is an interface that must be implemented by all resources that are based on this base
type EmbeddedResource interface {
	NewTransactionEvent() *model.TransactionEvent
	ApplyResource(ctx context.Context) (model.ResourceState, error)
	SelectProvider() (string, error)
	Type() string
}

type Base struct {
	Resource           EmbeddedResource
	CommonProperties   model.CommonResourceProperties
	ResourceProperties model.ResourceProperties
	Log                model.Logger
	UserLogger         model.Logger
	Manager            model.Manager

	sync.Mutex
}

func (b *Base) Validate() error {
	if b.ResourceProperties.CommonProperties().SkipValidate {
		return nil
	}

	return b.ResourceProperties.Validate()
}

func (b *Base) NewTransactionEvent() *model.TransactionEvent {
	event := model.NewTransactionEvent(b.CommonProperties.Type, b.CommonProperties.Name, b.CommonProperties.Alias)
	if b.ResourceProperties != nil {
		event.Properties = b.ResourceProperties
		event.RequestedEnsure = b.CommonProperties.Ensure
	}

	return event
}

func (b *Base) Healthcheck(ctx context.Context) (*model.TransactionEvent, error) {
	return b.applyOrHealthCheck(ctx, true)
}

func (b *Base) Apply(ctx context.Context) (*model.TransactionEvent, error) {
	return b.applyOrHealthCheck(ctx, false)
}

func (b *Base) applyOrHealthCheck(ctx context.Context, healthCheckOnly bool) (*model.TransactionEvent, error) {
	provName, err := b.Resource.SelectProvider()
	if err != nil {
		return nil, err
	}

	event := b.Resource.NewTransactionEvent()
	event.Provider = provName
	start := time.Now()
	defer func() {
		event.Duration = time.Since(start)
	}()

	var state model.ResourceState

	var unmet []string
	for _, r := range b.CommonProperties.Require {
		parts := strings.SplitN(r, "#", 2)
		failed, err := b.Manager.IsResourceFailed(parts[0], parts[1])
		if err != nil {
			return event, err
		}
		if failed {
			unmet = append(unmet, r)
			event.UnmetRequirements = append(event.UnmetRequirements, r)
		}
	}
	if len(unmet) > 0 {
		event.Skipped = true
		return event, nil
	}

	if !healthCheckOnly {
		timer := prometheus.NewTimer(metrics.ResourceApplyTime.WithLabelValues(b.CommonProperties.Type, provName, b.CommonProperties.Name))
		state, err = b.Resource.ApplyResource(ctx)
		timer.ObserveDuration()

		event.Duration = time.Since(start)
		if err != nil {
			event.Failed = true
			event.Errors = append(event.Errors, err.Error())
		}
	}

	// TODO: make helper in healthchecks
	for _, hc := range b.ResourceProperties.CommonProperties().HealthChecks {
		hc.TypeName = b.CommonProperties.Type
		hc.ResourceName = b.CommonProperties.Name

		res, err := healthcheck.Execute(ctx, b.Manager, &hc, b.UserLogger, b.Log)
		event.HealthChecks = append(event.HealthChecks, res)
		if err != nil {
			event.Failed = true
			event.Errors = append(event.Errors, err.Error())
		} else {
			if res.Status != model.HealthCheckOK {
				event.Failed = true
				event.Errors = append(event.Errors, fmt.Sprintf("health check status %q", res.Status.String()))
			}
		}
	}

	if state != nil {
		event.Status = state
		cs := state.CommonState()
		event.Changed = cs.Changed
		event.RequestedEnsure = b.CommonProperties.Ensure
		event.FinalEnsure = cs.Ensure
		event.Noop = cs.Noop
		event.NoopMessage = cs.NoopMessage
		event.Refreshed = cs.Refreshed
	}

	return event, nil
}

func (b *Base) Type() string {
	return b.CommonProperties.Type
}

func (b *Base) Name() string {
	return b.CommonProperties.Name
}

func (b *Base) Properties() model.ResourceProperties {
	return b.ResourceProperties
}

func (b *Base) String() string {
	return fmt.Sprintf("%s#%s", b.CommonProperties.Type, b.CommonProperties.Name)
}

// FinalizeState sets common fields on the resource state after applying changes.
// This reduces boilerplate in ApplyResource implementations.
func (b *Base) FinalizeState(state model.ResourceState, noop bool, noopMessage string, changed bool, stable bool, refreshed bool) {
	cs := state.CommonState()
	cs.Noop = noop
	cs.NoopMessage = noopMessage
	cs.Changed = changed
	cs.Stable = stable
	cs.Refreshed = refreshed
}

// ShouldRefresh checks if any of the subscribed resources have changed and should trigger a refresh.
// Returns true if a refresh should occur, the resource that triggered the refresh, and any error.
func (b *Base) ShouldRefresh(subscribe []string) (bool, string, error) {
	for _, s := range subscribe {
		parts := strings.SplitN(s, "#", 2)

		// validate already ensured its the right shape
		should, err := b.Manager.ShouldRefresh(parts[0], parts[1])
		if err != nil {
			return false, s, err
		}
		if should {
			return true, s, nil
		}
	}

	return false, "", nil
}
