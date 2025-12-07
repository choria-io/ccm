// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/templates"
	"github.com/goccy/go-yaml"
)

// Apply represents a parsed and resolved manifest ready for execution
type Apply struct {
	resources []map[string]model.ResourceProperties
	data      map[string]any

	mu sync.Mutex
}

func (a *Apply) MarshalYAML() ([]byte, error) {
	return yaml.Marshal(a.toMap())
}

func (a *Apply) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.toMap())
}

func (a *Apply) toMap() map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()

	return map[string]any{
		"resources": a.resources,
		"data":      a.data,
	}
}

// Resources returns the list of resources in the manifest
func (a *Apply) Resources() []map[string]model.ResourceProperties {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.resources
}

// Data returns the Hiera data associated with the manifest
func (a *Apply) Data() map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.data
}

// ApplyManifest represents the raw structure of a manifest file
type ApplyManifest struct {
	Resources []map[string]yaml.RawMessage `json:"resources" yaml:"resources"`
}

// ParseManifestHiera parses a resolved Hiera manifest and returns an Apply instance
func ParseManifestHiera(resolved map[string]any, env *templates.Env, log model.Logger) (*Apply, error) {
	resourcesRaw, ok := resolved["resources"]
	if !ok {
		return nil, fmt.Errorf("invalid manifest: no resources found")
	}

	resources, ok := resourcesRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("invalid manifest: resources must be an array got %T", resourcesRaw)
	}

	res := &Apply{
		data: env.Data,
	}

	for i, resource := range resources {
		resourceMap, ok := resource.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid manifest: resources must be an array of maps got %T", resource)
		}

		for typeName, v := range resourceMap {
			var rawProperties yaml.RawMessage
			rawProperties, err := yaml.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("invalid manifest resource %d: %w", i+1, err)
			}

			prop, err := model.NewResourcePropertiesFromYaml(typeName, rawProperties, env)
			if err != nil {
				return nil, fmt.Errorf("invalid manifest resource %d: %w", i+1, err)
			}

			res.resources = append(res.resources, map[string]model.ResourceProperties{typeName: prop})
		}
	}

	return res, nil
}
