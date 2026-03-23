// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"path/filepath"

	"github.com/adrg/xdg"
)

type FactProvider func(ctx context.Context, opts FactsConfig, log Logger) (map[string]any, error)

type FactsConfig struct {
	SystemConfigDirectory string `json:"system_config_directory" yaml:"system_config_directory"`           // SystemConfigDirectory is the directory where system wide facts are stored in facts.yaml|json, empty disables
	UserConfigDirectory   string `json:"user_config_directory" yaml:"user_config_directory"`               //  UserConfigDirectory is the directory where user specific facts are stored in facts.yaml|json, empty disables
	NoMemoryFacts         bool   `json:"no_memory_facts,omitempty" yaml:"no_memory_facts,omitempty"`       // NoMemoryFacts disables built-in memory fact gathering
	NoSwapFacts           bool   `json:"no_swap_facts,omitempty" yaml:"no_swap_facts,omitempty"`           // NoSwapFacts disables built-in swap facts gathering
	NoCPUFacts            bool   `json:"no_cpu_facts,omitempty" yaml:"no_cpu_facts,omitempty"`             // NoCPUFacts disables built-in cpu facts gathering
	NoPartitionFacts      bool   `json:"no_partition_facts,omitempty" yaml:"no_partition_facts,omitempty"` // NoPartitionFacts disables built-in disk facts gathering
	NoHostFacts           bool   `json:"no_host_facts,omitempty" yaml:"no_host_facts,omitempty"`           // NoHostFacts disables built-in host facts gathering
	NoNetworkFacts        bool   `json:"no_network_facts,omitempty" yaml:"no_network_facts,omitempty"`     // NoNetworkFacts disables built-in network interface facts gathering

	ExtraFactSources []FactProvider
}

// NewFactsConfig creates a new facts config with defaults options set
func NewFactsConfig() *FactsConfig {
	return &FactsConfig{
		SystemConfigDirectory: "/etc/choria/ccm",
		UserConfigDirectory:   filepath.Join(xdg.ConfigHome, "choria", "ccm"),
	}
}
