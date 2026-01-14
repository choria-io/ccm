// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package facts

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/goccy/go-yaml"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/metrics"
	"github.com/choria-io/ccm/model"
)

// StandardFacts returns a map of standard facts
func StandardFacts(ctx context.Context, log model.Logger) (map[string]any, error) {
	timer := prometheus.NewTimer(metrics.FactGatherTime.WithLabelValues())
	defer timer.ObserveDuration()

	sf, err := standardFacts(ctx)
	if err != nil {
		return nil, err
	}

	sysConfigDir := "/etc/choria/ccm"
	userconfigDir := filepath.Join(xdg.ConfigHome, "choria", "ccm")

	for _, dir := range []string{sysConfigDir, userconfigDir} {
		jf := filepath.Join(dir, "facts.json")
		yf := filepath.Join(dir, "facts.yaml")

		if iu.FileExists(jf) {
			log.Debug("Reading facts", "file", jf)
			jb, err := os.ReadFile(jf)
			if err != nil {
				log.Error("Failed to read facts file", "file", jf, "error", err)
			} else {
				var f map[string]any
				err = json.Unmarshal(jb, &f)
				if err != nil {
					log.Error("Failed to unmarshal facts file", "file", jf, "error", err)
				} else {
					sf = iu.DeepMergeMap(sf, f)
				}
			}
		}

		if iu.FileExists(yf) {
			log.Debug("Reading facts", "file", yf)
			jb, err := os.ReadFile(yf)
			if err == nil {
				var f map[string]any
				err = yaml.Unmarshal(jb, &f)
				if err != nil {
					log.Error("Failed to unmarshal facts file", "file", jf, "error", err)
				} else {
					sf = iu.DeepMergeMap(sf, f)
				}
			}
		}
	}

	return sf, nil
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
		"host":      hostFacts,
		"network":   networkFacts,
		"partition": partitionFacts,
		"cpu":       cpuFacts,
		"memory":    memoryFacts,
	}, nil
}
