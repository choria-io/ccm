// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"github.com/choria-io/ccm/templates"
	"github.com/goccy/go-yaml"
)

// ResourceStatusPackageProtocol is the protocol identifier for package resource state
const ResourceStatusPackageProtocol = "io.choria.ccm.v1.resource.package.state"

// PackageTypeName is the type name for package resources
const PackageTypeName = "package"

// PackageResourceProperties defines the properties for a package resource
type PackageResourceProperties struct {
	CommonResourceProperties `yaml:",inline"`
}

// PackageMetadata contains detailed metadata about a package
type PackageMetadata struct {
	Name        string         `json:"name" yaml:"name"`
	Version     string         `json:"version" yaml:"version"`
	Arch        string         `json:"arch,omitempty" yaml:"arch,omitempty"`
	License     string         `json:"license,omitempty" yaml:"license,omitempty"`
	URL         string         `json:"url,omitempty" yaml:"url,omitempty"`
	Summary     string         `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Provider    string         `json:"provider,omitempty" yaml:"provider,omitempty"`
	Extended    map[string]any `json:"extended,omitempty" yaml:"extended,omitempty"`
}

// PackageState represents the current state of a package on the system
type PackageState struct {
	CommonResourceState

	Metadata *PackageMetadata `json:"metadata,omitempty"`
}

// Validate validates the package resource properties
func (p *PackageResourceProperties) Validate() error {
	return p.CommonResourceProperties.Validate()
}

// ResolveTemplates resolves template expressions in the package resource properties
func (p *PackageResourceProperties) ResolveTemplates(env *templates.Env) error {
	return p.CommonResourceProperties.ResolveTemplates(env)
}

// NewPackageResourcePropertiesFromYaml creates a new package resource properties object from a yaml document, does not validate or expand templates
func NewPackageResourcePropertiesFromYaml(raw yaml.RawMessage) (*PackageResourceProperties, error) {
	prop := &PackageResourceProperties{}
	err := yaml.Unmarshal(raw, prop)
	if err != nil {
		return nil, err
	}

	prop.Type = "package"

	return prop, nil
}
