// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package facts

import (
	"context"

	"github.com/choria-io/ccm/model"
	"github.com/shirou/gopsutil/v4/mem"
)

func getMemoryFacts(ctx context.Context, opts *model.FactsConfig) map[string]any {
	swapFacts := map[string]any{
		"info":    map[string]any{},
		"devices": map[string]any{},
	}
	memFacts := map[string]any{
		"swap":    swapFacts,
		"virtual": map[string]any{},
	}

	if opts.NoMemoryFacts {
		return memFacts
	}

	memFacts["virtual"], _ = mem.VirtualMemoryWithContext(ctx)
	if !opts.NoSwapFacts {
		swapFacts["info"], _ = mem.SwapMemoryWithContext(ctx)
		swapFacts["devices"], _ = mem.SwapDevicesWithContext(ctx)
	}

	return memFacts
}
