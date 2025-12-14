// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"fmt"
	"time"

	"github.com/choria-io/ccm/templates"
	"github.com/goccy/go-yaml"
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
	Properties() any
	Apply(context.Context) (*TransactionEvent, error)
	Info(context.Context) (any, error)
}

// ResourceProperties defines the interface for resource property validation and template resolution
type ResourceProperties interface {
	Validate() error
	ResolveTemplates(*templates.Env) error
	ToYamlManifest() (yaml.RawMessage, error)
}

// CommonResourceProperties contains properties shared by all resource types
type CommonResourceProperties struct {
	Type        string             `json:"-" yaml:"-"`
	Name        string             `json:"name" yaml:"name"`
	Ensure      string             `json:"ensure,omitempty" yaml:"ensure"`
	Provider    string             `json:"provider,omitempty" yaml:"provider"`
	HealthCheck *CommonHealthCheck `json:"health_check,omitempty" yaml:"health_check,omitempty"`
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

	return nil
}

// Validate validates common resource properties
func (p *CommonResourceProperties) Validate() error {
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

// CommonResourceState contains state information shared by all resource types
type CommonResourceState struct {
	TimeStamp    time.Time          `json:"timestamp" yaml:"timestamp"`
	Protocol     string             `json:"protocol" yaml:"protocol"`
	ResourceType string             `json:"type" yaml:"type"`
	Name         string             `json:"name" yaml:"name"`
	Ensure       string             `json:"ensure" yaml:"ensure"`
	Changed      bool               `json:"changed" yaml:"changed"`
	Refreshed    bool               `json:"refreshed" yaml:"refreshed"`
	HealthCheck  *HealthCheckResult `json:"health_check,omitempty" yaml:"health_check,omitempty"`
}

// NewResourcePropertiesFromYaml creates a new resource properties object from a yaml document, it validates the properties and expands any templates
func NewResourcePropertiesFromYaml(typeName string, rawProperties yaml.RawMessage, env *templates.Env) (ResourceProperties, error) {
	var prop ResourceProperties
	var err error

	switch typeName {
	case PackageTypeName:
		prop, err = NewPackageResourcePropertiesFromYaml(rawProperties)
		if err != nil {
			return nil, err
		}
	case ServiceTypeName:
		prop, err = NewServiceResourcePropertiesFromYaml(rawProperties)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("%w: %s %s", ErrResourceInvalid, ErrUnknownType, typeName)
	}

	err = prop.ResolveTemplates(env)
	if err != nil {
		return nil, err
	}

	err = prop.Validate()
	if err != nil {
		return nil, err
	}

	return prop, nil
}
