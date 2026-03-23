// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package facts

import (
	"context"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/metrics"
	"github.com/choria-io/ccm/model"
	"github.com/prometheus/client_golang/prometheus"
)

// Gather gathers facts from the system
func Gather(ctx context.Context, opts model.FactsConfig, log model.Logger) (map[string]any, error) {
	timer := prometheus.NewTimer(metrics.FactGatherTime.WithLabelValues())
	defer timer.ObserveDuration()

	facts := map[string]any{
		"host":      getHostFacts(ctx, &opts),
		"network":   getNetworkFacts(ctx, &opts),
		"partition": getPartitionFacts(ctx, &opts),
		"cpu":       getCpuFacts(ctx, &opts),
		"memory":    getMemoryFacts(ctx, &opts),
	}

	for _, p := range append(opts.ExtraFactSources, gatherFileFacts) {
		f, err := p(ctx, opts, log)
		if err != nil {
			log.Error("Could not gather facts", "error", err)
			continue
		}

		facts = iu.DeepMergeMap(facts, f)
	}

	return facts, nil
}
