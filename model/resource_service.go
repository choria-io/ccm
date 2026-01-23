// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"fmt"
	"slices"

	"github.com/goccy/go-yaml"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/templates"
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
	Enable                   *bool    `json:"enable,omitempty" yaml:"enable,omitempty"`       // Enable indicates the service should be enabled on boot
	Subscribe                []string `json:"subscribe,omitempty" yaml:"subscribe,omitempty"` // Subscribe lists resource statusses to subscribe to in format type#name
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

func (f *ServiceState) CommonState() *CommonResourceState {
	return &f.CommonResourceState
}

func (p *ServiceResourceProperties) CommonProperties() *CommonResourceProperties {
	return &p.CommonResourceProperties
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
		return fmt.Errorf("%w: invalid ensure property %q expects %q or %q", ErrInvalidEnsureValue, p.Ensure, ServiceEnsureRunning, ServiceEnsureStopped)
	}

	// Validate service name to prevent shell injection
	if dangerousCharsRegex.MatchString(p.Name) {
		return fmt.Errorf("service name contains dangerous characters: %q", p.Name)
	}

	if !commonNameRegex.MatchString(p.Name) {
		return fmt.Errorf("service name contains invalid characters: %q (allowed: alphanumeric, ._+:~-)", p.Name)
	}

	if len(p.Subscribe) > 0 {
		if !iu.IsValidResourceRef(p.Subscribe...) {
			return fmt.Errorf("invalid subscribe format")
		}
	}

	return nil
}

// ResolveTemplates resolves template expressions in the package resource properties
func (p *ServiceResourceProperties) ResolveTemplates(env *templates.Env) error {
	err := p.CommonResourceProperties.ResolveTemplates(env)
	if err != nil {
		return err
	}

	if len(p.Subscribe) > 0 {
		subscribe := make([]string, len(p.Subscribe))
		for i, s := range p.Subscribe {
			val, err := templates.ResolveTemplateString(s, env)
			if err != nil {
				return err
			}
			subscribe[i] = val
		}
		p.Subscribe = subscribe
	}

	return nil
}

// ToYamlManifest returns the service resource properties as a yaml document
func (p *ServiceResourceProperties) ToYamlManifest() (yaml.RawMessage, error) {
	return yaml.Marshal(p)
}

// NewServiceResourcePropertiesFromYaml creates a new service resource properties object from a yaml document, does not validate or expand templates
func NewServiceResourcePropertiesFromYaml(raw yaml.RawMessage) ([]ResourceProperties, error) {
	return parseProperties(raw, ServiceTypeName, func() ResourceProperties { return &ServiceResourceProperties{} })
}
