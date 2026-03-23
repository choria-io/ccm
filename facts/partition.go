// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package facts

import (
	"context"

	"github.com/choria-io/ccm/model"
	"github.com/shirou/gopsutil/v4/disk"
)

func getPartitionFacts(ctx context.Context, opts *model.FactsConfig) map[string]any {
	partitionFacts := map[string]any{
		"partitions": []any{},
		"usage":      []any{},
	}

	if opts.NoPartitionFacts {
		return partitionFacts
	}

	parts, err := disk.PartitionsWithContext(ctx, false)
	if err == nil {
		if len(parts) > 0 {
			var matchedParts []disk.PartitionStat
			var usages []*disk.UsageStat

			for _, part := range parts {
				matchedParts = append(matchedParts, part)
				u, err := disk.UsageWithContext(ctx, part.Mountpoint)
				if err != nil {
					continue
				}
				usages = append(usages, u)
			}

			partitionFacts["partitions"] = matchedParts
			partitionFacts["usage"] = usages
		}
	}

	return partitionFacts
}
