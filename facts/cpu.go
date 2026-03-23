// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package facts

import (
	"context"

	"github.com/choria-io/ccm/model"
	"github.com/shirou/gopsutil/v4/cpu"
)

func getCpuFacts(ctx context.Context, opts *model.FactsConfig) map[string]any {
	cpuFacts := map[string]any{
		"info": []any{},
	}

	if opts.NoCPUFacts {
		return cpuFacts
	}

	cpuFacts["info"], _ = cpu.InfoWithContext(ctx)

	return cpuFacts
}
