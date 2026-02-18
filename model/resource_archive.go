// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"fmt"
	"net/url"
	"path/filepath"
	"time"

	"github.com/goccy/go-yaml"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/templates"
)

const (
	// ResourceStatusArchiveProtocol is the protocol identifier for archive resource state
	ResourceStatusArchiveProtocol = "io.choria.ccm.v1.resource.archive.state"

	// ArchiveTypeName is the type name for archive resources
	ArchiveTypeName = "archive"
)

// ArchiveResourceProperties defines the properties for a archive resource
type ArchiveResourceProperties struct {
	CommonResourceProperties `yaml:",inline"`
	Url                      string            `json:"url" yaml:"url"`                                           // URL specifies the URL to download the archive from
	Headers                  map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`               // Headers specify any HTTP headers to include in the request
	Username                 string            `json:"username,omitempty" yaml:"username,omitempty"`             // Username specifies the username to use for basic auth
	Password                 string            `json:"password,omitempty" yaml:"password,omitempty"`             // Password specifies the password to use for basic auth
	Checksum                 string            `json:"checksum,omitempty" yaml:"checksum,omitempty"`             // Checksum specifies the expected sha256 checksum of the archive
	ExtractParent            string            `json:"extract_parent,omitempty" yaml:"extract_parent,omitempty"` // ExtractParent specifies the parent directory to extract the archive into
	Cleanup                  bool              `json:"cleanup,omitempty" yaml:"cleanup,omitempty"`               // Cleanup specifies whether to remove the archive file after extraction
	Creates                  string            `json:"creates,omitempty" yaml:"creates,omitempty"`               // Creates specifies a file that the archive creates; if this file exists, the archive will not be extracted on future runs
	Owner                    string            `json:"owner" yaml:"owner"`                                       // Owner specifies the user that should own the file
	Group                    string            `json:"group" yaml:"group"`                                       // Group specifies the group that should own the file
}

// ArchiveMetadata contains detailed metadata about an archive
type ArchiveMetadata struct {
	Name          string    `json:"name" yaml:"name"`
	Checksum      string    `json:"checksum,omitempty" yaml:"checksum,omitempty"`
	ArchiveExists bool      `json:"archive_exists,omitempty" yaml:"archive_exists,omitempty"`
	CreatesExists bool      `json:"creates_exists,omitempty" yaml:"creates_exists,omitempty"`
	Owner         string    `json:"owner" yaml:"owner"`
	Group         string    `json:"group" yaml:"group"`
	MTime         time.Time `json:"mtime,omitempty" yaml:"mtime,omitempty"`
	Size          int64     `json:"size,omitempty" yaml:"size,omitempty"`
	Provider      string    `json:"provider,omitempty" yaml:"provider,omitempty"`
}

// ArchiveState represents the current state of an archive on the system
type ArchiveState struct {
	CommonResourceState

	Metadata *ArchiveMetadata `json:"metadata,omitempty"`
}

func (f *ArchiveState) CommonState() *CommonResourceState {
	return &f.CommonResourceState
}

func (p *ArchiveResourceProperties) CommonProperties() *CommonResourceProperties {
	return &p.CommonResourceProperties
}

// Validate validates the package resource properties
func (p *ArchiveResourceProperties) Validate() error {
	if p.SkipValidate {
		return nil
	}

	// First run common validation
	err := p.CommonResourceProperties.Validate()
	if err != nil {
		return err
	}

	// Validate URL
	if p.Url == "" {
		return fmt.Errorf("url cannot be empty")
	}

	parsedURL, err := url.Parse(p.Url)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}

	if !parsedURL.IsAbs() {
		return fmt.Errorf("url must be absolute (include scheme like http:// or https://)")
	}

	filename := filepath.Base(parsedURL.Path)
	if filename == "" || filename == "." || filename == "/" {
		return fmt.Errorf("url must have a filename in the path")
	}

	if !iu.FileHasSuffix(filename, ".zip", ".tar.gz", ".tgz", ".tar") {
		return fmt.Errorf("url filename must end in .zip, .tar.gz, .tgz, or .tar")
	}

	// Validate that Name has a matching archive extension
	if !iu.FileHasSuffix(p.Name, ".zip", ".tar.gz", ".tgz", ".tar") {
		return fmt.Errorf("name must end in .zip, .tar.gz, .tgz, or .tar")
	}

	// Ensure URL and Name have compatible archive types
	urlType := archiveTypeFromFilename(filename)
	nameType := archiveTypeFromFilename(p.Name)
	if urlType != nameType {
		return fmt.Errorf("url and name must have the same archive type: url is %s, name is %s", urlType, nameType)
	}

	// Validate ensure value
	if p.Ensure != EnsurePresent && p.Ensure != EnsureAbsent {
		return fmt.Errorf("%w: must be one of %q or %q", ErrInvalidEnsureValue, EnsurePresent, EnsureAbsent)
	}

	if p.Cleanup && p.ExtractParent == "" {
		return fmt.Errorf("cleanup requires extract_parent to be set")
	}

	if p.Cleanup && p.Creates == "" {
		return fmt.Errorf("cleanup requires creates to be set")
	}

	if filepath.Clean(p.Name) != p.Name {
		return fmt.Errorf("file path must be canonical")
	}

	if !filepath.IsAbs(p.Name) {
		return fmt.Errorf("file path must be absolute")
	}

	if len(p.Creates) > 0 {
		if filepath.Clean(p.Creates) != p.Creates {
			return fmt.Errorf("creates path must be absolute")
		}
	}
	if len(p.ExtractParent) > 0 {
		if filepath.Clean(p.ExtractParent) != p.ExtractParent {
			return fmt.Errorf("extract_parent path must be absolute")
		}
	}

	if p.Owner == "" {
		return fmt.Errorf("owner cannot be empty")
	}
	if p.Group == "" {
		return fmt.Errorf("group cannot be empty")
	}

	return nil
}

// archiveTypeFromFilename returns a normalized archive type string based on the file extension.
// Returns "tar.gz" for .tar.gz and .tgz, "tar" for .tar, "zip" for .zip, or "unknown".
func archiveTypeFromFilename(filename string) string {
	if iu.FileHasSuffix(filename, ".tar.gz", ".tgz") {
		return "tar.gz"
	}
	if iu.FileHasSuffix(filename, ".tar") {
		return "tar"
	}
	if iu.FileHasSuffix(filename, ".zip") {
		return "zip"
	}
	return "unknown"
}

// ResolveTemplates resolves template expressions in the package resource properties
func (p *ArchiveResourceProperties) ResolveTemplates(env *templates.Env) error {
	err := p.CommonResourceProperties.ResolveTemplates(env)
	if err != nil {
		return err
	}

	val, err := templates.ResolveTemplateString(p.Url, env)
	if err != nil {
		return err
	}
	p.Url = val

	val, err = templates.ResolveTemplateString(p.Username, env)
	if err != nil {
		return err
	}
	p.Username = val

	val, err = templates.ResolveTemplateString(p.Password, env)
	if err != nil {
		return err
	}
	p.Password = val

	val, err = templates.ResolveTemplateString(p.Owner, env)
	if err != nil {
		return err
	}
	p.Owner = val

	val, err = templates.ResolveTemplateString(p.Group, env)
	if err != nil {
		return err
	}
	p.Group = val

	if len(p.Checksum) > 0 {
		val, err = templates.ResolveTemplateString(p.Checksum, env)
		if err != nil {
			return err
		}
		p.Checksum = val
	}

	if len(p.Creates) > 0 {
		val, err = templates.ResolveTemplateString(p.Creates, env)
		if err != nil {
			return err
		}
		p.Creates = val
	}
	return nil
}

// ToYamlManifest returns the package resource properties as a yaml document
func (p *ArchiveResourceProperties) ToYamlManifest() (yaml.RawMessage, error) {
	return yaml.Marshal(p)
}

// NewArchiveResourcePropertiesFromYaml creates a new archive resource properties object from a yaml document, does not validate or expand templates
func NewArchiveResourcePropertiesFromYaml(raw yaml.RawMessage) ([]ResourceProperties, error) {
	return parseProperties(raw, ArchiveTypeName, func() ResourceProperties { return &ArchiveResourceProperties{} })
}
