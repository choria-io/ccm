// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package facts

import (
	"context"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// StandardFacts returns a map of standard facts
func StandardFacts(ctx context.Context) (map[string]any, error) {
	return standardFacts(ctx)
}

func standardFacts(ctx context.Context) (map[string]any, error) {
	var err error

	swapFacts := map[string]any{
		"info":    map[string]any{},
		"devices": map[string]any{},
	}
	memoryFacts := map[string]any{
		"swap":    swapFacts,
		"virtual": map[string]any{},
	}
	cpuFacts := map[string]any{
		"info": []any{},
	}
	partitionFacts := map[string]any{
		"partitions": []any{},
		"usage":      []any{},
	}
	hostFacts := map[string]any{
		"info": map[string]any{},
	}
	networkFacts := map[string]any{
		"interfaces": []any{},
	}

	virtual, err := mem.VirtualMemoryWithContext(ctx)
	if err == nil {
		memoryFacts["virtual"] = virtual
	}

	swap, err := mem.SwapMemoryWithContext(ctx)
	if err == nil {
		swapFacts["info"] = swap
	}
	swapDev, err := mem.SwapDevicesWithContext(ctx)
	if err == nil {
		swapFacts["devices"] = swapDev
	}

	cpuInfo, err := cpu.InfoWithContext(ctx)
	if err == nil {
		cpuFacts["info"] = cpuInfo
	}

	parts, err := disk.PartitionsWithContext(ctx, false)
	if err == nil {
		if len(parts) > 0 {
			matchedParts := []disk.PartitionStat{}
			usages := []*disk.UsageStat{}

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

	hostInfo, err := host.InfoWithContext(ctx)
	if err == nil {
		hostFacts["info"] = hostInfo
	}

	interfaces, err := net.InterfacesWithContext(ctx)
	if err == nil {
		networkFacts["interfaces"] = interfaces
	}

	return map[string]any{
		"memory":    memoryFacts,
		"cpu":       cpuFacts,
		"partition": partitionFacts,
		"host":      hostFacts,
		"network":   networkFacts,
	}, nil
}
