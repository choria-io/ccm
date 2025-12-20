// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/choria-io/ccm/hiera"
	"github.com/choria-io/ccm/metrics"
	"github.com/choria-io/ccm/model"
	fileresource "github.com/choria-io/ccm/resources/file"
	packageresource "github.com/choria-io/ccm/resources/package"
	serviceresource "github.com/choria-io/ccm/resources/service"
	"github.com/choria-io/ccm/templates"
	"github.com/goccy/go-yaml"
	"github.com/prometheus/client_golang/prometheus"
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

// ResolveManifestReader reads and resolves a manifest using Hiera, returning the resolved data and parsed manifest
func ResolveManifestReader(ctx context.Context, mgr model.Manager, manifest io.Reader) (map[string]any, model.Apply, error) {
	mb, err := io.ReadAll(manifest)
	if err != nil {
		return nil, nil, err
	}

	facts, err := mgr.Facts(ctx)
	if err != nil {
		return nil, nil, err
	}

	hieraLogger, err := mgr.Logger("hiera", "resources")
	if err != nil {
		return nil, nil, err
	}

	resolved, err := hiera.ResolveYaml(mb, facts, hiera.DefaultOptions, hieraLogger)
	if err != nil {
		return nil, nil, err
	}

	data := mgr.SetData(resolved)
	env, err := mgr.TemplateEnvironment(ctx)
	if err != nil {
		return nil, nil, err
	}

	var manifestRaw map[string]any
	err = yaml.Unmarshal(mb, &manifestRaw)
	if err != nil {
		return nil, nil, err
	}

	manifestData := map[string]any{
		"data":      data,
		"resources": []map[string]any{},
	}

	ccm, ok := manifestRaw["ccm"].(map[string]any)
	if ok {
		manifestData["resources"] = ccm["resources"]
	}

	apply, err := ParseManifestHiera(manifestData, env, hieraLogger)
	if err != nil {
		return nil, nil, err
	}

	return manifestData, apply, err
}

func (a *Apply) Execute(ctx context.Context, mgr model.Manager, healthCheckOnly bool, userLog model.Logger) (model.SessionStore, error) {
	timer := prometheus.NewTimer(metrics.ManifestApplyTime.WithLabelValues())
	defer timer.ObserveDuration()

	session, err := mgr.StartSession(a)
	if err != nil {
		return nil, err
	}

	log, err := mgr.Logger("component", "apply")
	if err != nil {
		return nil, err
	}

	for _, r := range a.Resources() {
		for _, prop := range r {
			var event *model.TransactionEvent
			var resource model.Resource
			var err error

			// TODO: error here should rather create a TransactionEvent with an error status
			// TODO: this stuff should be stored in the registry so it knows when to call what so its automatic

			switch rprop := prop.(type) {
			case *model.PackageResourceProperties:
				resource, err = packageresource.New(ctx, mgr, *rprop)
			case *model.ServiceResourceProperties:
				resource, err = serviceresource.New(ctx, mgr, *rprop)
			case *model.FileResourceProperties:
				resource, err = fileresource.New(ctx, mgr, *rprop)
			default:
				return nil, fmt.Errorf("unsupported resource property type %T", rprop)
			}
			if err != nil {
				return nil, err
			}

			if healthCheckOnly {
				event, err = resource.Healthcheck(ctx)
			} else {
				event, err = resource.Apply(ctx)
			}
			if err != nil {
				return nil, err
			}

			event.LogStatus(userLog)

			err = mgr.RecordEvent(event)
			if err != nil {
				log.Error("Could not save event", "event", event.String())
			}
		}
	}

	return session, nil
}

// Manifest represents the raw structure of a manifest file
type Manifest struct {
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

			prop, err := model.NewValidatedResourcePropertiesFromYaml(typeName, rawProperties, env)
			if err != nil {
				return nil, fmt.Errorf("invalid manifest resource %d: %w", i+1, err)
			}

			res.resources = append(res.resources, map[string]model.ResourceProperties{typeName: prop})
		}
	}

	return res, nil
}
