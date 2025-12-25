// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"fmt"
	"regexp"

	"github.com/choria-io/ccm/templates"
	"github.com/goccy/go-yaml"
)

const (
	// ResourceStatusPackageProtocol is the protocol identifier for package resource state
	ResourceStatusPackageProtocol = "io.choria.ccm.v1.resource.package.state"

	// PackageTypeName is the type name for package resources
	PackageTypeName = "package"

	PackageEnsureLatest = "latest"
)

var (
	// commonNameRegex allows alphanumeric, hyphens, underscores, dots, plus signs, colons, and tildes
	// which are common in package and service names across different package managers
	commonNameRegex = regexp.MustCompile(`^[a-zA-Z0-9._+:~-]+$`)

	// dangerousCharsRegex detects shell metacharacters that could be used for injection
	dangerousCharsRegex = regexp.MustCompile(`[;&|$` + "`" + `()\[\]{}<>*?'"\\!\n\t\r]`)
)

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

func (f *PackageState) CommonState() *CommonResourceState {
	return &f.CommonResourceState
}

func (p *PackageResourceProperties) CommonProperties() *CommonResourceProperties {
	return &p.CommonResourceProperties
}

// Validate validates the package resource properties
func (p *PackageResourceProperties) Validate() error {
	// First run common validation
	err := p.CommonResourceProperties.Validate()
	if err != nil {
		return err
	}

	// Validate package name to prevent shell injection
	if dangerousCharsRegex.MatchString(p.Name) {
		return fmt.Errorf("package name contains dangerous characters: %q", p.Name)
	}

	if !commonNameRegex.MatchString(p.Name) {
		return fmt.Errorf("package name contains invalid characters: %q (allowed: alphanumeric, ._+:~-)", p.Name)
	}

	// Validate ensure value if it's a version string
	if p.Ensure != "" && p.Ensure != EnsurePresent && p.Ensure != EnsureAbsent && p.Ensure != PackageEnsureLatest {
		// It's a version string, validate it
		if dangerousCharsRegex.MatchString(p.Ensure) {
			return fmt.Errorf("package version/ensure contains dangerous characters: %q", p.Ensure)
		}
	}

	return nil
}

// ResolveTemplates resolves template expressions in the package resource properties
func (p *PackageResourceProperties) ResolveTemplates(env *templates.Env) error {
	return p.CommonResourceProperties.ResolveTemplates(env)
}

// ToYamlManifest returns the package resource properties as a yaml document
func (p *PackageResourceProperties) ToYamlManifest() (yaml.RawMessage, error) {
	return yaml.Marshal(p)
}

// NewPackageResourcePropertiesFromYaml creates a new package resource properties object from a yaml document, does not validate or expand templates
func NewPackageResourcePropertiesFromYaml(raw yaml.RawMessage) (*PackageResourceProperties, error) {
	prop := &PackageResourceProperties{}
	err := yaml.Unmarshal(raw, prop)
	if err != nil {
		return nil, err
	}

	prop.Type = PackageTypeName

	return prop, nil
}
