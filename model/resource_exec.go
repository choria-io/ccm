// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"time"

	"github.com/choria-io/fisk"
	"github.com/goccy/go-yaml"
)

const (
	// ResourceStatusExecProtocol is the protocol identifier for exec resource state
	ResourceStatusExecProtocol = "io.choria.ccm.v1.resource.exec.state"

	// ExecTypeName is the type name for file resources
	ExecTypeName = "exec"
)

type ExecResourceProperties struct {
	CommonResourceProperties `yaml:",inline"`
	Cwd                      string   `json:"cwd,omitempty" yaml:"cwd,omitempty"`
	Environment              []string `json:"environment,omitempty" yaml:"environment,omitempty"`
	Path                     string   `json:"path,omitempty" yaml:"path,omitempty"`
	Returns                  int      `json:"returns,omitempty" yaml:"returns,omitempty"`
	User                     string   `json:"user,omitempty" yaml:"user,omitempty"`
	Timeout                  string   `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Creates                  string   `json:"creates,omitempty" yaml:"creates,omitempty"`
	RefreshOnly              bool     `json:"refreshonly,omitempty" yaml:"refreshonly,omitempty"`
	Subscribe                []string `json:"subscribe,omitempty" yaml:"subscribe,omitempty"`

	ParsedTimeout time.Duration `json:"-" yaml:"-"`
}

// ExecMetadata contains detailed metadata about an execution
type ExecMetadata struct {
	Name     string         `json:"name" yaml:"name"`
	Provider string         `json:"provider,omitempty" yaml:"provider,omitempty"`
	Runtime  time.Duration  `json:"runtime,omitempty" yaml:"runtime,omitempty"`
	Extended map[string]any `json:"extended,omitempty" yaml:"extended,omitempty"`
}

// ExecState represents the current state of an execution
type ExecState struct {
	CommonResourceState

	Metadata any `json:"metadata,omitempty"`
}

func (f *ExecState) CommonState() CommonResourceState {
	return f.CommonResourceState
}

// Validate validates the package resource properties
func (p *ExecResourceProperties) Validate() error {
	if p.SkipValidate {
		return nil
	}

	// First run common validation
	err := p.CommonResourceProperties.Validate()
	if err != nil {
		return err
	}

	p.ParsedTimeout, err = fisk.ParseDuration(p.Timeout)
	if err != nil {
		return err
	}

	return nil
}

func (p *ExecResourceProperties) CommonProperties() *CommonResourceProperties {
	return &p.CommonResourceProperties
}

// ToYamlManifest returns the package resource properties as a yaml document
func (p *ExecResourceProperties) ToYamlManifest() (yaml.RawMessage, error) {
	return yaml.Marshal(p)
}

// NewExecResourcePropertiesFromYaml creates a new exec resource properties object from a yaml document, does not validate or expand templates
func NewExecResourcePropertiesFromYaml(raw yaml.RawMessage) (*ExecResourceProperties, error) {
	prop := &ExecResourceProperties{}
	err := yaml.Unmarshal(raw, prop)
	if err != nil {
		return nil, err
	}

	prop.Type = FileTypeName

	return prop, nil
}
