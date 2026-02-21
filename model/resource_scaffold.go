// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"fmt"
	"path/filepath"

	"github.com/goccy/go-yaml"

	"github.com/choria-io/ccm/templates"
)

const (
	// ResourceStatusScaffoldProtocol is the protocol identifier for scaffold resource state
	ResourceStatusScaffoldProtocol = "io.choria.ccm.v1.resource.scaffold.state"

	// ScaffoldTypeName is the type name for scaffold resources
	ScaffoldTypeName = "scaffold"
)

type ScaffoldResourceEngine string

const (
	ScaffoldEngineGo  ScaffoldResourceEngine = "go"
	ScaffoldEngineJet ScaffoldResourceEngine = "jet"
)

// ScaffoldResourceProperties defines the properties for a scaffold resource
type ScaffoldResourceProperties struct {
	CommonResourceProperties `yaml:",inline"`
	Source                   string                 `json:"source" yaml:"source"`
	SkipEmpty                bool                   `json:"skip_empty,omitempty" yaml:"skip_empty,omitempty"`
	LeftDelimiter            string                 `json:"left_delimiter,omitempty" yaml:"left_delimiter,omitempty"`
	RightDelimiter           string                 `json:"right_delimiter,omitempty" yaml:"right_delimiter,omitempty"`
	Engine                   ScaffoldResourceEngine `json:"engine,omitempty" yaml:"engine,omitempty"`
	Purge                    bool                   `json:"purge,omitempty" yaml:"purge,omitempty"`
	Post                     []map[string]string    `json:"post,omitempty" yaml:"post,omitempty"`
}

// ScaffoldMetadata contains detailed metadata about a scaffold
type ScaffoldMetadata struct {
	Name         string                 `json:"name" yaml:"name"`
	Provider     string                 `json:"provider,omitempty" yaml:"provider,omitempty"`
	TargetExists bool                   `json:"target_exists,omitempty" yaml:"target_exists,omitempty"`
	Changed      []string               `json:"changed,omitempty" yaml:"changed,omitempty"`
	Purged       []string               `json:"purged,omitempty" yaml:"purged,omitempty"`
	Stable       []string               `json:"stable,omitempty" yaml:"stable,omitempty"`
	Engine       ScaffoldResourceEngine `json:"engine,omitempty" yaml:"engine,omitempty"`
}

// ScaffoldState represents the current state of a scaffold on the system
type ScaffoldState struct {
	CommonResourceState

	Metadata *ScaffoldMetadata `json:"metadata,omitempty"`
}

func (f *ScaffoldState) CommonState() *CommonResourceState {
	return &f.CommonResourceState
}

func (p *ScaffoldResourceProperties) CommonProperties() *CommonResourceProperties {
	return &p.CommonResourceProperties
}

// Validate validates the package resource properties
func (p *ScaffoldResourceProperties) Validate() error {
	if p.SkipValidate {
		return nil
	}

	err := p.CommonResourceProperties.Validate()
	if err != nil {
		return err
	}

	if p.Ensure != EnsurePresent && p.Ensure != EnsureAbsent {
		return fmt.Errorf("%w: must be one of %q or %q", ErrInvalidEnsureValue, EnsurePresent, EnsureAbsent)
	}

	if !filepath.IsAbs(p.Name) {
		return fmt.Errorf("name must be an absolute path")
	}

	if filepath.Clean(p.Name) != p.Name {
		return fmt.Errorf("name must be a canonical path")
	}

	if p.Source == "" {
		return fmt.Errorf("source cannot be empty")
	}

	if p.Engine != ScaffoldEngineGo && p.Engine != ScaffoldEngineJet {
		return fmt.Errorf("engine must be one of %q or %q", ScaffoldEngineGo, ScaffoldEngineJet)
	}

	for _, entry := range p.Post {
		for key, val := range entry {
			if key == "" {
				return fmt.Errorf("post keys cannot be empty")
			}
			if val == "" {
				return fmt.Errorf("post value for key %q cannot be empty", key)
			}
		}
	}

	return nil
}

// ResolveTemplates resolves template expressions in the package resource properties
func (p *ScaffoldResourceProperties) ResolveTemplates(env *templates.Env) error {
	err := p.CommonResourceProperties.ResolveTemplates(env)
	if err != nil {
		return err
	}

	val, err := templates.ResolveTemplateString(p.Source, env)
	if err != nil {
		return err
	}
	p.Source = val

	return nil
}

// ToYamlManifest returns the file resource properties as a yaml document
func (p *ScaffoldResourceProperties) ToYamlManifest() (yaml.RawMessage, error) {
	return yaml.Marshal(p)
}

// NewScaffoldResourcePropertiesFromYaml creates a new scaffold resource properties object from a yaml document, does not validate or expand templates
func NewScaffoldResourcePropertiesFromYaml(raw yaml.RawMessage) ([]ResourceProperties, error) {
	return parseProperties(raw, ScaffoldTypeName, func() ResourceProperties { return &ScaffoldResourceProperties{} })
}
