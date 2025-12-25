// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/kballard/go-shellquote"

	"github.com/choria-io/ccm/templates"
	"github.com/choria-io/fisk"
)

const (
	// ResourceStatusExecProtocol is the protocol identifier for exec resource state
	ResourceStatusExecProtocol = "io.choria.ccm.v1.resource.exec.state"

	// ExecTypeName is the type name for exec resources
	ExecTypeName = "exec"
)

// ExecResourceProperties defines the properties for an exec resource
type ExecResourceProperties struct {
	CommonResourceProperties `yaml:",inline"`
	Cwd                      string   `json:"cwd,omitempty" yaml:"cwd,omitempty"`                 // Cwd specifies the working directory from which to run the command
	Environment              []string `json:"environment,omitempty" yaml:"environment,omitempty"` // Environment specifies additional environment variables to set when running the command
	Path                     string   `json:"path,omitempty" yaml:"path,omitempty"`               // Path specifies the search path for executable commands, as an array of directories or a colon-separated list
	Returns                  []int    `json:"returns,omitempty" yaml:"returns,omitempty"`         // Returns specify the expected exit codes indicating success; defaults to 0 if not specified
	Timeout                  string   `json:"timeout,omitempty" yaml:"timeout,omitempty"`         // Timeout specifies the maximum time the command is allowed to run; if exceeded the command will be terminated, the timeout is a duration like 10s
	Creates                  string   `json:"creates,omitempty" yaml:"creates,omitempty"`         // Creates specifies a file that the command creates; if this file exists the command will not run
	RefreshOnly              bool     `json:"refreshonly,omitempty" yaml:"refreshonly,omitempty"` // RefreshOnly determines whether the command should only run when notified by a subscribed resource
	Subscribe                []string `json:"subscribe,omitempty" yaml:"subscribe,omitempty"`     // Subscribe specifies resources to subscribe to for refresh notifications in the format "type#name"
	LogOutput                bool     `json:"logoutput,omitempty" yaml:"logoutput,omitempty"`     // LogOutput determines whether to log the command's output

	ParsedTimeout time.Duration `json:"-" yaml:"-"` // ParsedTimeout is the parsed duration representation of Timeout, should not be set by callers
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

	ExitCode         *int `json:"exitcode,omitempty" yaml:"exitcode"`
	CreatesSatisfied bool `json:"creates_satisfied,omitempty" yaml:"creates_satisfied"`
}

func (f *ExecState) CommonState() *CommonResourceState {
	return &f.CommonResourceState
}

// Validate validates the package resource properties
func (p *ExecResourceProperties) Validate() error {
	if p.SkipValidate {
		return nil
	}

	err := p.CommonResourceProperties.Validate()
	if err != nil {
		return err
	}

	if p.Timeout != "" {
		p.ParsedTimeout, err = fisk.ParseDuration(p.Timeout)
		if err != nil {
			return err
		}
	}

	words, err := shellquote.Split(p.Name)
	if err != nil {
		return err
	}
	if len(words) == 0 {
		return fmt.Errorf("invalid command")
	}

	for _, sub := range p.Subscribe {
		parts := strings.Split(sub, "#")
		if len(parts) != 2 {
			return fmt.Errorf("invalid subscribe format %s", sub)
		}
	}

	if p.Path != "" {
		for _, dir := range strings.Split(p.Path, ":") {
			if dir == "" {
				return fmt.Errorf("invalid path: empty directory in path")
			}
			if !strings.HasPrefix(dir, "/") {
				return fmt.Errorf("invalid path: %q is not an absolute path", dir)
			}
		}
	}

	for _, env := range p.Environment {
		key, value, found := strings.Cut(env, "=")
		if !found {
			return fmt.Errorf("invalid environment variable %q: missing '='", env)
		}
		if key == "" {
			return fmt.Errorf("invalid environment variable %q: empty key", env)
		}
		if value == "" {
			return fmt.Errorf("invalid environment variable %q: empty value", env)
		}
	}

	return nil
}

// ResolveTemplates resolves template expressions in the package resource properties
func (p *ExecResourceProperties) ResolveTemplates(env *templates.Env) error {
	err := p.CommonResourceProperties.ResolveTemplates(env)
	if err != nil {
		return err
	}

	val, err := templates.ResolveTemplateString(p.Cwd, env)
	if err != nil {
		return err
	}
	p.Cwd = val

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

	prop.Type = ExecTypeName

	if prop.Ensure == "" {
		prop.Ensure = EnsurePresent
	}

	return prop, nil
}
