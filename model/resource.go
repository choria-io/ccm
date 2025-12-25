// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"fmt"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/choria-io/ccm/templates"
)

const (
	// EnsurePresent indicates a resource should be present on the system
	EnsurePresent string = "present"
	// EnsureAbsent indicates a resource should be removed from the system
	EnsureAbsent string = "absent"
)

// Resource represents a system resource that can be managed
type Resource interface {
	Type() string
	Name() string
	Provider() string
	Properties() ResourceProperties
	Apply(context.Context) (*TransactionEvent, error)
	Info(context.Context) (any, error)
	Healthcheck(ctx context.Context) (*TransactionEvent, error)
}

type ResourceState interface {
	CommonState() CommonResourceState
}

// ResourceProperties defines the interface for resource property validation and template resolution
type ResourceProperties interface {
	CommonProperties() *CommonResourceProperties
	Validate() error
	ResolveTemplates(*templates.Env) error
	ToYamlManifest() (yaml.RawMessage, error)
}

// CommonResourceProperties contains properties shared by all resource types
type CommonResourceProperties struct {
	Type         string              `json:"-" yaml:"-"`
	Name         string              `json:"name" yaml:"name"`
	Alias        string              `json:"alias,omitempty" yaml:"alias,omitempty"`
	Ensure       string              `json:"ensure,omitempty" yaml:"ensure"`
	Provider     string              `json:"provider,omitempty" yaml:"provider"`
	HealthChecks []CommonHealthCheck `json:"health_checks,omitempty" yaml:"health_checks,omitempty"`
	SkipValidate bool                `json:"-" yaml:"-"`
}

// ResolveTemplates resolves template expressions in common resource properties
func (p *CommonResourceProperties) ResolveTemplates(env *templates.Env) error {
	val, err := templates.ResolveTemplateString(p.Ensure, env)
	if err != nil {
		return err
	}
	p.Ensure = val

	val, err = templates.ResolveTemplateString(p.Name, env)
	if err != nil {
		return err
	}
	p.Name = val

	val, err = templates.ResolveTemplateString(p.Provider, env)
	if err != nil {
		return err
	}
	p.Provider = val

	for _, hc := range p.HealthChecks {
		val, err = templates.ResolveTemplateString(hc.Command, env)
		if err != nil {
			return err
		}
		hc.Command = val
	}

	return nil
}

// Validate validates common resource properties
func (p *CommonResourceProperties) Validate() error {
	if p.SkipValidate {
		return nil
	}

	if p.Name == "" {
		return ErrResourceNameRequired
	}

	if p.Ensure == "" {
		return ErrResourceEnsureRequired
	}

	return nil
}

// NewCommonResourceState creates a new common resource state with the given properties
func NewCommonResourceState(protocol string, resourceType string, name string, ensure string) CommonResourceState {
	return CommonResourceState{
		TimeStamp:    time.Now().UTC(),
		Protocol:     protocol,
		ResourceType: resourceType,
		Name:         name,
		Ensure:       ensure,
	}
}

// TODO: seems redundant

// CommonResourceState contains state information shared by all resource types
type CommonResourceState struct {
	TimeStamp    time.Time          `json:"timestamp" yaml:"timestamp"`
	Protocol     string             `json:"protocol" yaml:"protocol"`
	ResourceType string             `json:"type" yaml:"type"`
	Name         string             `json:"name" yaml:"name"`
	Ensure       string             `json:"ensure" yaml:"ensure"`
	Changed      bool               `json:"changed" yaml:"changed"`
	Refreshed    bool               `json:"refreshed" yaml:"refreshed"`
	Stable       bool               `json:"stable" yaml:"stable"`
	Noop         bool               `json:"noop" yaml:"noop"`
	NoopMessage  string             `json:"noop_message,omitempty" yaml:"noop_message,omitempty"`
	HealthCheck  *HealthCheckResult `json:"health_check,omitempty" yaml:"health_check,omitempty"`
}

// NewResourcePropertiesFromYaml creates a new resource properties object from a yaml document, it validates the properties and expands any templates
func NewResourcePropertiesFromYaml(typeName string, rawProperties yaml.RawMessage, env *templates.Env) (ResourceProperties, error) {
	var prop ResourceProperties
	var err error

	// TODO: extend the registry to include these so its automatic and doesnt need maintenance

	switch typeName {
	case PackageTypeName:
		prop, err = NewPackageResourcePropertiesFromYaml(rawProperties)
	case ServiceTypeName:
		prop, err = NewServiceResourcePropertiesFromYaml(rawProperties)
	case FileTypeName:
		prop, err = NewFileResourcePropertiesFromYaml(rawProperties)
	case ExecTypeName:
		prop, err = NewExecResourcePropertiesFromYaml(rawProperties)
	default:
		return nil, fmt.Errorf("%w: %s %s", ErrResourceInvalid, ErrUnknownType, typeName)
	}
	if err != nil {
		return nil, err
	}

	err = prop.ResolveTemplates(env)
	if err != nil {
		return nil, err
	}

	return prop, nil
}

// NewValidatedResourcePropertiesFromYaml creates and validates a new resource properties object from a yaml document, it validates the properties and expands any templates
func NewValidatedResourcePropertiesFromYaml(typeName string, rawProperties yaml.RawMessage, env *templates.Env) (ResourceProperties, error) {
	prop, err := NewResourcePropertiesFromYaml(typeName, rawProperties, env)
	if err != nil {
		return nil, err
	}

	err = prop.Validate()
	if err != nil {
		return nil, err
	}

	return prop, nil
}
