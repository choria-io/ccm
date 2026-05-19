// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/choria-io/ccm/templates"
)

const (
	// ResourceStatusFileProtocol is the protocol identifier for file resource state
	ResourceStatusFileProtocol = "io.choria.ccm.v1.resource.file.state"

	// FileTypeName is the type name for file resources
	FileTypeName = "file"

	FileEnsureDirectory = "directory"
)

// FileResourceProperties defines the properties for a file resource
type FileResourceProperties struct {
	CommonResourceProperties `yaml:",inline"`
	Contents                 *string `json:"content,omitempty" yaml:"content,omitempty" template:"deferred"` // Contents specifies the desired file contents as a string; mutually exclusive with Source. When nil, file contents are not managed and only owner/group/mode are enforced.
	Source                   string  `json:"source,omitempty" yaml:"source,omitempty" template:"deferred"`   // Source specifies a local file path to use as the source for the file contents; mutually exclusive with Contents
	Owner                    string  `json:"owner,omitempty" yaml:"owner,omitempty"`                         // Owner specifies the user that should own the file; required unless ensure is absent
	Group                    string  `json:"group,omitempty" yaml:"group,omitempty"`                         // Group specifies the group that should own the file; required unless ensure is absent
	Mode                     string  `json:"mode,omitempty" yaml:"mode,omitempty"`                           // Mode specifies the file permissions in octal notation (e.g., "0644"); required unless ensure is absent
	Force                    bool    `json:"force,omitempty" yaml:"force,omitempty"`                         // Force allows removal of non-empty directories when Ensure is absent; has no effect on regular files
}

// ManagesContent reports whether this resource manages the file's contents.
// When false the resource only enforces owner/group/mode on an existing file
// and creates an empty file with those attributes if it does not yet exist.
func (p *FileResourceProperties) ManagesContent() bool {
	return p.Contents != nil || p.Source != ""
}

// Content returns the desired file contents as a string, or an empty string
// when contents are not explicitly set.
func (p *FileResourceProperties) Content() string {
	if p.Contents == nil {
		return ""
	}
	return *p.Contents
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

	Metadata *FileMetadata `json:"metadata,omitempty"`
}

func (f *FileState) CommonState() *CommonResourceState {
	return &f.CommonResourceState
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
		return fmt.Errorf("%w: must be one of %q, %q or %q", ErrInvalidEnsureValue, EnsurePresent, EnsureAbsent, FileEnsureDirectory)
	}

	if filepath.Clean(p.Name) != p.Name {
		return fmt.Errorf("file path must be canonical")
	}

	if !filepath.IsAbs(p.Name) {
		return fmt.Errorf("file path must be absolute")
	}

	if p.Force {
		if p.Ensure != EnsureAbsent {
			return fmt.Errorf("'force: true' is only valid with 'ensure: absent', got 'ensure: %s'", p.Ensure)
		}
		if p.Name == "/" {
			return fmt.Errorf("'force: true' cannot be used with the filesystem root")
		}
	}

	if p.Contents != nil && p.Source != "" {
		return fmt.Errorf("'content' and 'source' are mutually exclusive")
	}

	// owner/group/mode describe a desired on-disk state and are not
	// consulted on the removal path, so they are optional when the
	// resource is being removed.
	if p.Ensure != EnsureAbsent {
		if p.Owner == "" {
			return fmt.Errorf("owner cannot be empty")
		}
		if p.Group == "" {
			return fmt.Errorf("group cannot be empty")
		}
		if p.Mode == "" {
			return fmt.Errorf("mode cannot be empty")
		}
	}

	return nil
}

// ResolveTemplates resolves template expressions in the file resource properties
func (p *FileResourceProperties) ResolveTemplates(env *templates.Env) error {
	err := templates.ResolveStructTemplates(p, env, false)
	if err != nil {
		return err
	}

	return p.resolveRegistrations(env)
}

// ResolveDeferredTemplates resolves content and source templates after control evaluation.
// This allows controls like if/unless to prevent template errors in content when the
// resource would be skipped.
func (p *FileResourceProperties) ResolveDeferredTemplates(env *templates.Env) error {
	err := templates.ResolveStructTemplates(p, env, true)
	if err != nil {
		return err
	}

	if p.Source != "" {
		p.Source = filepath.Clean(p.Source)
	}

	return nil
}

// ToYamlManifest returns the file resource properties as a yaml document
func (p *FileResourceProperties) ToYamlManifest() (yaml.RawMessage, error) {
	return yaml.Marshal(p)
}

// NewFileResourcePropertiesFromYaml creates a new file resource properties object from a yaml document, does not validate or expand templates
func NewFileResourcePropertiesFromYaml(raw yaml.RawMessage) ([]ResourceProperties, error) {
	return parseProperties(raw, FileTypeName, func() ResourceProperties { return &FileResourceProperties{} })
}
