// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	NameSpace = "choria"
	Subsystem = "ccm"

	//ManifestApplyTime is a summary of the time taken to apply an entire manifest
	ManifestApplyTime = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "manifest_apply_duration_seconds"),
		Help: "Time taken to apply an entire manifest",
	}, []string{})

	// ResourceApplyTime is a summary of the time taken to apply a particular resource
	ResourceApplyTime = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "resource_apply_duration_seconds"),
		Help: "Time taken to apply a particular resource",
	}, []string{"type", "provider", "name"})

	// HealthCheckTime is a summary of the time taken to health check a particular resource
	HealthCheckTime = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "healthcheck_duration_seconds"),
		Help: "Time taken to health check a particular resource",
	}, []string{"type", "name"})

	// HealthStatusCount is how many checks are in a certain state
	HealthStatusCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "healthcheck_status_count"),
		Help: "How many resources are in a certain state",
	}, []string{"type", "name", "status"})

	// ResourceStateChanged counts how many resources were changed
	ResourceStateChanged = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "resource_state_changed_count"),
		Help: "How many resources were changed",
	}, []string{"type", "name"})

	// ResourceStateRefreshed counts how many resources were refreshed
	ResourceStateRefreshed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "resource_state_refreshed_count"),
		Help: "How many resources were refreshed",
	}, []string{"type", "name"})

	// ResourceStateFailed counts how many resources failed
	ResourceStateFailed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "resource_state_failed_count"),
		Help: "How many resources failed",
	}, []string{"type", "name"})

	// ResourceStateError counts how many resources had errors
	ResourceStateError = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "resource_state_error_count"),
		Help: "How many resources had errors",
	}, []string{"type", "name"})

	// ResourceStateSkipped counts how many resources were skipped
	ResourceStateSkipped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "resource_state_skipped_count"),
		Help: "How many resources were skipped",
	}, []string{"type", "name"})

	// ResourceStateNoop counts how many resources were in noop mode
	ResourceStateNoop = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "resource_state_noop_count"),
		Help: "How many resources were in noop mode",
	}, []string{"type", "name"})

	// ResourceStateTotal counts how many resources were processed
	ResourceStateTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "resource_state_total_count"),
		Help: "How many resources were processed",
	}, []string{"type", "name"})
)

func RegisterMetrics() {
	prometheus.MustRegister(ManifestApplyTime)
	prometheus.MustRegister(ResourceApplyTime)
	prometheus.MustRegister(HealthCheckTime)
	prometheus.MustRegister(HealthStatusCount)
	prometheus.MustRegister(ResourceStateChanged)
	prometheus.MustRegister(ResourceStateRefreshed)
	prometheus.MustRegister(ResourceStateFailed)
	prometheus.MustRegister(ResourceStateError)
	prometheus.MustRegister(ResourceStateSkipped)
	prometheus.MustRegister(ResourceStateNoop)
	prometheus.MustRegister(ResourceStateTotal)
}
