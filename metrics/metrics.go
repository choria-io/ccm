// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/choria-io/ccm/model"
)

var (
	NameSpace = "choria"
	Subsystem = "ccm"

	//ManifestApplyTime is a summary of the time taken to apply an entire manifest
	ManifestApplyTime = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "manifest_apply_duration_seconds"),
		Help: "Time taken to apply an entire manifest",
	}, []string{"source"})

	// ResourceApplyTime is a summary of the time taken to apply a particular resource
	ResourceApplyTime = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "resource_apply_duration_seconds"),
		Help: "Time taken to apply a particular resource",
	}, []string{"type", "provider", "name"})

	// HealthCheckTime is a summary of the time taken to health check a particular resource
	HealthCheckTime = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "healthcheck_duration_seconds"),
		Help: "Time taken to health check a particular resource",
	}, []string{"type", "name", "check"})

	// HealthStatusCount is how many checks are in a certain state
	HealthStatusCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "healthcheck_status_count"),
		Help: "How many resources are in a certain state",
	}, []string{"type", "name", "status", "check"})

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

	// ResourceStateStable counts how many resources were in stable state
	ResourceStateStable = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "resource_state_stable_count"),
		Help: "How many resources were in stable state",
	}, []string{"type", "name"})

	// FactGatherTime is a summary of the time taken to gather facts
	FactGatherTime = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "facts_gather_duration_seconds"),
		Help: "Time taken to gather facts",
	}, []string{})

	AgentApplyTime = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "agent_apply_duration_seconds"),
		Help: "Time taken to apply manifests in agents",
	}, []string{"manifest"})

	AgentHealthCheckTime = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "agent_healthcheck_duration_seconds"),
		Help: "How long health check runs took",
	}, []string{"manifests"})

	AgentHealthCheckRemediation = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "agent_healthcheck_remediations_count"),
		Help: "How many times health checks triggered remediating runs",
	}, []string{"manifest"})

	AgentDataResolveTime = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "agent_data_resolve_duration_seconds"),
		Help: "Time taken to resolve data in agents",
	}, []string{})

	AgentFactsResolveTime = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "agent_facts_resolve_duration_seconds"),
		Help: "Time taken to resolve facts in agents",
	}, []string{})

	AgentDataResolveFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "agent_data_resolve_error_count"),
		Help: "How many times resolving data failed in agents",
	}, []string{"url"})

	AgentFactsResolveFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "agent_facts_resolve_error_count"),
		Help: "How many times resolving facts failed in agents",
	}, []string{})

	AgentManifestFetchCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "agent_manifest_fetch_count"),
		Help: "How many times manifests were fetch from remote",
	}, []string{"manifest"})

	AgentManifestFetchFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: prometheus.BuildFQName(NameSpace, Subsystem, "agent_manifest_fetch_error_count"),
		Help: "How many times fetching manifests failed in agents",
	}, []string{"manifest"})
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
	prometheus.MustRegister(ResourceStateStable)
	prometheus.MustRegister(FactGatherTime)
	prometheus.MustRegister(AgentApplyTime)
	prometheus.MustRegister(AgentDataResolveTime)
	prometheus.MustRegister(AgentFactsResolveTime)
	prometheus.MustRegister(AgentDataResolveFailureCount)
	prometheus.MustRegister(AgentFactsResolveFailureCount)
	prometheus.MustRegister(AgentManifestFetchCount)
	prometheus.MustRegister(AgentManifestFetchFailureCount)
	prometheus.MustRegister(AgentHealthCheckTime)
	prometheus.MustRegister(AgentHealthCheckRemediation)
}

func ListenAndServe(port int, log model.Logger) {
	if port <= 0 {
		return
	}

	go func() {
		log.Info("Starting monitoring server", "port", port)
		http.Handle("/metrics", promhttp.Handler())
		err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
		if err != nil {
			log.Error("HTTP Listener failed", "error", err)
		}
	}()
}
