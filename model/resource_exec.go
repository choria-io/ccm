// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
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
	Command                  string   `json:"command" yaml:"command" template:"deferred"`                             // Command specifies the command to run, when not set will use the name property
	Cwd                      string   `json:"cwd,omitempty" yaml:"cwd,omitempty" template:"deferred"`                 // Cwd specifies the working directory from which to run the command
	Environment              []string `json:"environment,omitempty" yaml:"environment,omitempty" template:"deferred"` // Environment specifies additional environment variables to set when running the command
	Path                     string   `json:"path,omitempty" yaml:"path,omitempty"`                                   // Path specifies the search path for executable commands, as an array of directories or a colon-separated list
	Returns                  []int    `json:"returns,omitempty" yaml:"returns,omitempty"`                             // Returns specify the expected exit codes indicating success; defaults to 0 if not specified
	Timeout                  string   `json:"timeout,omitempty" yaml:"timeout,omitempty"`                             // Timeout specifies the maximum time the command is allowed to run; if exceeded the command will be terminated, the timeout is a duration like 10s
	Creates                  string   `json:"creates,omitempty" yaml:"creates,omitempty" template:"deferred"`         // Creates specifies a file that the command creates; if this file exists the command will not run
	OnlyIf                   string   `json:"onlyif,omitempty" yaml:"onlyif,omitempty" template:"deferred"`           // OnlyIf specifies a guard command; the exec runs only if this command exits 0
	Unless                   string   `json:"unless,omitempty" yaml:"unless,omitempty" template:"deferred"`           // Unless specifies a guard command; the exec runs only if this command exits non-zero
	RefreshOnly              bool     `json:"refreshonly,omitempty" yaml:"refreshonly,omitempty"`                     // RefreshOnly determines whether the command should only run when notified by a subscribed resource
	Subscribe                []string `json:"subscribe,omitempty" yaml:"subscribe,omitempty"`                         // Subscribe specifies resources to subscribe to for refresh notifications in the format "type#name"
	LogOutput                bool     `json:"logoutput,omitempty" yaml:"logoutput,omitempty"`                         // LogOutput determines whether to log the command's output

	ParsedTimeout time.Duration `json:"-" yaml:"-"` // ParsedTimeout is the parsed duration representation of Timeout, should not be set by callers
}

// ExecState represents the current state of an execution
type ExecState struct {
	CommonResourceState

	ExitCode         *int `json:"exitcode,omitempty" yaml:"exitcode"`
	CreatesSatisfied bool `json:"creates_satisfied,omitempty" yaml:"creates_satisfied"`
	OnlyIfSatisfied  bool `json:"onlyif_satisfied,omitempty" yaml:"onlyif_satisfied"`
	UnlessSatisfied  bool `json:"unless_satisfied,omitempty" yaml:"unless_satisfied"`
}

func (f *ExecState) CommonState() *CommonResourceState {
	return &f.CommonResourceState
}

// Validate validates the exec resource properties
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

// ResolveTemplates resolves template expressions in the exec resource properties
func (p *ExecResourceProperties) ResolveTemplates(env *templates.Env) error {
	err := templates.ResolveStructTemplates(p, env, false)
	if err != nil {
		return err
	}

	return p.resolveRegistrations(env)
}

// ResolveDeferredTemplates resolves command, guard, cwd, environment and creates
// templates after control evaluation. This lets these fields reference state
// (for example via ${ file(...) }) produced by earlier resources in the same
// manifest, which would not yet exist at parse time.
func (p *ExecResourceProperties) ResolveDeferredTemplates(env *templates.Env) error {
	return templates.ResolveStructTemplates(p, env, true)
}

func (p *ExecResourceProperties) CommonProperties() *CommonResourceProperties {
	return &p.CommonResourceProperties
}

// ToYamlManifest returns the package resource properties as a yaml document
func (p *ExecResourceProperties) ToYamlManifest() (yaml.RawMessage, error) {
	return yaml.Marshal(p)
}

// NewExecResourcePropertiesFromYaml creates a new exec resource properties object from a yaml document, does not validate or expand templates
func NewExecResourcePropertiesFromYaml(raw yaml.RawMessage) ([]ResourceProperties, error) {
	res, err := parseProperties(raw, ExecTypeName, func() ResourceProperties { return &ExecResourceProperties{} })
	if err != nil {
		return nil, err
	}

	for _, prop := range res {
		p := prop.(*ExecResourceProperties)
		p.ParsedTimeout = 0
		if p.Ensure == "" {
			p.Ensure = EnsurePresent
		}
	}

	return res, nil
}
