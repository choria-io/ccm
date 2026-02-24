// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"github.com/choria-io/ccm/metrics"
	"github.com/choria-io/ccm/model"
)

func updateMetrics(event model.SessionEvent) {
	e, ok := event.(*model.TransactionEvent)
	if !ok {
		return
	}

	name := e.Name
	if e.Alias != "" {
		name = e.Alias
	}

	metrics.ResourceStateTotal.WithLabelValues(e.ResourceType, name).Inc()

	switch {
	case e.Noop:
		metrics.ResourceStateNoop.WithLabelValues(e.ResourceType, name).Inc()
	case e.Changed:
		metrics.ResourceStateChanged.WithLabelValues(e.ResourceType, name).Inc()
	case e.Skipped:
		metrics.ResourceStateSkipped.WithLabelValues(e.ResourceType, name).Inc()
	case e.Refreshed:
		metrics.ResourceStateRefreshed.WithLabelValues(e.ResourceType, name).Inc()
	case e.Failed:
		metrics.ResourceStateFailed.WithLabelValues(e.ResourceType, name).Inc()
	case len(e.Errors) > 0:
		metrics.ResourceStateError.WithLabelValues(e.ResourceType, name).Inc()
	default:
		metrics.ResourceStateStable.WithLabelValues(e.ResourceType, name).Inc()
	}
}

func filterEvents(allEvents []model.SessionEvent, resourceType string, resourceName string) ([]model.TransactionEvent, error) {
	// Filter for the specific resource
	var filtered []model.TransactionEvent
	for _, event := range allEvents {
		// Only include TransactionEvents (skip SessionStartEvent)
		txEvent, ok := event.(*model.TransactionEvent)
		if !ok {
			continue
		}

		// Filter by resourceType and name
		if txEvent.ResourceType == resourceType && (txEvent.Name == resourceName || txEvent.Alias == resourceName) {
			filtered = append(filtered, *txEvent)
		}
	}

	return filtered, nil
}
