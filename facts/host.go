// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package facts

import (
	"context"

	"github.com/choria-io/ccm/model"
	"github.com/shirou/gopsutil/v4/host"
)

func getHostFacts(ctx context.Context, opts *model.FactsConfig) map[string]any {
	hostFacts := map[string]any{
		"info": map[string]any{},
	}

	if opts.NoHostFacts {
		return hostFacts
	}

	hostFacts["info"], _ = host.InfoWithContext(ctx)

	return hostFacts
}
