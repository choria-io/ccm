// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/choria-io/ccm/templates"
	"github.com/goccy/go-yaml"
)

const (
	// ResourceStatusFileProtocol is the protocol identifier for file resource state
	ResourceStatusFileProtocol = "io.choria.ccm.v1.resource.file.state"

	// FileTypeName is the type name for file resources
	FileTypeName = "file"

	FileEnsureDirectory = "directory"
)

type FileResourceProperties struct {
	CommonResourceProperties `yaml:",inline"`
	Contents                 string    `json:"content,omitempty" yaml:"content,omitempty"`
	Source                   string    `json:"source,omitempty" yaml:"source,omitempty"`
	Owner                    string    `json:"owner" yaml:"owner"`
	Group                    string    `json:"group" yaml:"group"`
	Mode                     string    `json:"mode" yaml:"mode"`
	MTime                    time.Time `json:"mtime,omitempty" yaml:"mtime,omitempty"`
}

// FileMetadata contains detailed metadata about a file
type FileMetadata struct {
	Name     string         `json:"name" yaml:"name"`
	Checksum string         `json:"checksum,omitempty" yaml:"checksum,omitempty"`
	Owner    string         `json:"owner" yaml:"owner"`
	Group    string         `json:"group" yaml:"group"`
	Mode     string         `json:"mode" yaml:"mode"`
	Provider string         `json:"provider,omitempty" yaml:"provider,omitempty"`
	MTime    time.Time      `json:"mtime,omitempty" yaml:"mtime,omitempty"`
	Size     int64          `json:"size,omitempty" yaml:"size,omitempty"`
	Extended map[string]any `json:"extended,omitempty" yaml:"extended,omitempty"`
}

// FileState represents the current state of a file on the system
type FileState struct {
	CommonResourceState

	Metadata any `json:"metadata,omitempty"`
}

func (f *FileState) CommonState() CommonResourceState {
	return f.CommonResourceState
}

func (p *FileResourceProperties) CommonProperties() *CommonResourceProperties {
	return &p.CommonResourceProperties
}

// Validate validates the package resource properties
func (p *FileResourceProperties) Validate() error {
	if p.SkipValidate {
		return nil
	}

	// First run common validation
	err := p.CommonResourceProperties.Validate()
	if err != nil {
		return err
	}

	// Validate ensure value if it's a version string
	if p.Ensure != EnsurePresent && p.Ensure != EnsureAbsent && p.Ensure != FileEnsureDirectory {
		return fmt.Errorf("ensure must be one of %q, %q or %q", EnsurePresent, EnsureAbsent, FileEnsureDirectory)
	}

	if filepath.Clean(p.Name) != p.Name {
		return fmt.Errorf("file path must be absolute")
	}

	if p.Owner == "" {
		return fmt.Errorf("owner cannot be empty")
	}
	if p.Group == "" {
		return fmt.Errorf("group cannot be empty")
	}
	if p.Mode == "" {
		return fmt.Errorf("mode cannot be empty")
	}

	return nil
}

// ResolveTemplates resolves template expressions in the package resource properties
func (p *FileResourceProperties) ResolveTemplates(env *templates.Env) error {
	err := p.CommonResourceProperties.ResolveTemplates(env)
	if err != nil {
		return err
	}

	val, err := templates.ResolveTemplateString(p.Owner, env)
	if err != nil {
		return err
	}
	p.Owner = val

	val, err = templates.ResolveTemplateString(p.Group, env)
	if err != nil {
		return err
	}
	p.Group = val

	val, err = templates.ResolveTemplateString(p.Mode, env)
	if err != nil {
		return err
	}
	p.Mode = val

	if p.Contents != "" {
		val, err = templates.ResolveTemplateString(p.Contents, env)
		if err != nil {
			return err
		}
		p.Contents = val
	}

	if p.Source != "" {
		val, err = templates.ResolveTemplateString(p.Source, env)
		if err != nil {
			return err
		}
		p.Source = filepath.Clean(val)
	}

	return nil
}

// ToYamlManifest returns the package resource properties as a yaml document
func (p *FileResourceProperties) ToYamlManifest() (yaml.RawMessage, error) {
	return yaml.Marshal(p)
}

// NewFileResourcePropertiesFromYaml creates a new file resource properties object from a yaml document, does not validate or expand templates
func NewFileResourcePropertiesFromYaml(raw yaml.RawMessage) (*FileResourceProperties, error) {
	prop := &FileResourceProperties{}
	err := yaml.Unmarshal(raw, prop)
	if err != nil {
		return nil, err
	}

	prop.Type = FileTypeName

	return prop, nil
}
