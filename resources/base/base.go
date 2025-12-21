// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package base

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/choria-io/ccm/healthcheck"
	"github.com/choria-io/ccm/metrics"
	"github.com/choria-io/ccm/model"
	"github.com/prometheus/client_golang/prometheus"
)

// EmbeddedResource is an interface that must be implemented by all resources that are based on this base
type EmbeddedResource interface {
	NewTransactionEvent() *model.TransactionEvent
	ApplyResource(ctx context.Context) (model.ResourceState, error)
	SelectProvider() (string, error)
}

type Base struct {
	Resource           EmbeddedResource
	TypeName           string
	InstanceName       string
	Ensure             string
	ResourceProperties model.ResourceProperties
	Log                model.Logger
	UserLogger         model.Logger
	Manager            model.Manager

	sync.Mutex
}

func (b *Base) Validate() error {
	return b.ResourceProperties.Validate()
}

func (b *Base) NewTransactionEvent() *model.TransactionEvent {
	event := model.NewTransactionEvent(b.TypeName, b.InstanceName)
	if b.ResourceProperties != nil {
		event.Properties = b.ResourceProperties
		event.Name = b.InstanceName
		event.Ensure = b.Ensure
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
	start := time.Now()

	var state model.ResourceState

	if !healthCheckOnly {
		timer := prometheus.NewTimer(metrics.ResourceApplyTime.WithLabelValues(model.FileTypeName, provName, b.InstanceName))
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
		hc.TypeName = model.ServiceTypeName
		hc.ResourceName = b.InstanceName

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

	event.Duration = time.Since(start)

	if state != nil {
		event.Status = state

		cs := state.CommonState()
		event.Changed = cs.Changed
		event.ActualEnsure = cs.Ensure
		event.Noop = cs.Noop
		event.NoopMessage = cs.NoopMessage
		event.Refreshed = cs.Refreshed
	}
	event.Provider = provName

	return event, nil
}

func (b *Base) Type() string {
	return b.TypeName
}

func (b *Base) Name() string {
	return b.InstanceName
}

func (b *Base) Properties() model.ResourceProperties {
	return b.ResourceProperties
}

func (b *Base) String() string {
	return fmt.Sprintf("%s#%s", model.FileTypeName, b.InstanceName)
}
