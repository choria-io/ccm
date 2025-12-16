// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"fmt"
	"slices"

	"github.com/choria-io/ccm/templates"
	"github.com/goccy/go-yaml"
)

const (
	ServiceEnsureRunning = "running"
	ServiceEnsureStopped = "stopped"
	// ResourceStatusServiceProtocol is the protocol identifier for service resource state
	ResourceStatusServiceProtocol = "io.choria.ccm.v1.resource.service.state"

	// ServiceTypeName is the type name for service resources
	ServiceTypeName = "service"
)

// ServiceResourceProperties defines the properties for a service resource
type ServiceResourceProperties struct {
	CommonResourceProperties `yaml:",inline"`
	Enable                   *bool  `json:"enable,omitempty" yaml:"enable,omitempty"`
	Subscribe                string `json:"subscribe,omitempty" yaml:"subscribe,omitempty"`
}

// ServiceMetadata contains detailed metadata about a service
type ServiceMetadata struct {
	Name     string `json:"name" yaml:"name"`
	Provider string `json:"provider,omitempty" yaml:"provider,omitempty"`
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Running  bool   `json:"running" yaml:"running"`
}

// ServiceState represents the current state of a service on the system
type ServiceState struct {
	CommonResourceState

	Metadata *ServiceMetadata `json:"metadata,omitempty"`
}

// Validate validates the package resource properties
func (p *ServiceResourceProperties) Validate() error {
	// Default ensure to running if not specified
	if p.Ensure == "" {
		p.Ensure = ServiceEnsureRunning
	}

	// First run common validation
	err := p.CommonResourceProperties.Validate()
	if err != nil {
		return err
	}

	if !slices.Contains([]string{ServiceEnsureRunning, ServiceEnsureStopped}, p.Ensure) {
		return fmt.Errorf("invalid service ensure property %q expects %q or %q", p.Ensure, ServiceEnsureRunning, ServiceEnsureStopped)
	}

	// Validate package name to prevent shell injection
	if dangerousCharsRegex.MatchString(p.Name) {
		return fmt.Errorf("service name contains dangerous characters: %q", p.Name)
	}

	if !commonNameRegex.MatchString(p.Name) {
		return fmt.Errorf("service name contains invalid characters: %q (allowed: alphanumeric, ._+:~-)", p.Name)
	}

	return nil
}

// ResolveTemplates resolves template expressions in the package resource properties
func (p *ServiceResourceProperties) ResolveTemplates(env *templates.Env) error {
	err := p.CommonResourceProperties.ResolveTemplates(env)
	if err != nil {
		return err
	}

	val, err := templates.ResolveTemplateString(p.Subscribe, env)
	if err != nil {
		return err
	}
	p.Subscribe = val

	return nil
}

// ToYamlManifest returns the service resource properties as a yaml document
func (p *ServiceResourceProperties) ToYamlManifest() (yaml.RawMessage, error) {
	return yaml.Marshal(p)
}

// NewServiceResourcePropertiesFromYaml creates a new service resource properties object from a yaml document, does not validate or expand templates
func NewServiceResourcePropertiesFromYaml(raw yaml.RawMessage) (*ServiceResourceProperties, error) {
	prop := &ServiceResourceProperties{}
	err := yaml.Unmarshal(raw, prop)
	if err != nil {
		return nil, err
	}

	prop.Type = ServiceTypeName

	return prop, nil
}
