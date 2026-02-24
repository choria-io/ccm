// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"fmt"
	"time"

	"github.com/goccy/go-yaml"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/templates"
)

const (
	// EnsurePresent indicates a resource should be present on the system
	EnsurePresent string = "present"
	// EnsureAbsent indicates a resource should be removed from the system
	EnsureAbsent string = "absent"
)

// Resource represents a system resource that can be managed
type Resource interface {
	Type() string
	Name() string
	Provider() string
	Properties() ResourceProperties
	Apply(context.Context) (*TransactionEvent, error)
	Info(context.Context) (any, error)
	Healthcheck(ctx context.Context) (*TransactionEvent, error)
}

type ResourceState interface {
	CommonState() *CommonResourceState
}

// ResourceProperties defines the interface for resource property validation and template resolution
type ResourceProperties interface {
	CommonProperties() *CommonResourceProperties
	Validate() error
	ResolveTemplates(*templates.Env) error
	ToYamlManifest() (yaml.RawMessage, error)
}

// CommonResourceProperties contains properties shared by all resource types
type CommonResourceProperties struct {
	Type         string                 `json:"-" yaml:"-"`
	Name         string                 `json:"name" yaml:"name"`
	Alias        string                 `json:"alias,omitempty" yaml:"alias,omitempty"`
	Ensure       string                 `json:"ensure,omitempty" yaml:"ensure,omitempty"`
	Provider     string                 `json:"provider,omitempty" yaml:"provider,omitempty"`
	HealthChecks []CommonHealthCheck    `json:"health_checks,omitempty" yaml:"health_checks,omitempty"`
	Require      []string               `json:"require,omitempty" yaml:"require,omitempty"`
	Control      *CommonResourceControl `json:"control,omitempty" yaml:"control,omitempty"`
	SkipValidate bool                   `json:"-" yaml:"-"`
}

type CommonResourceControl struct {
	ManageIf     string `json:"if,omitempty" yaml:"if,omitempty"`
	ManageUnless string `json:"unless,omitempty" yaml:"unless,omitempty"`
}

// ResolveTemplates resolves template expressions in common resource properties
func (p *CommonResourceProperties) ResolveTemplates(env *templates.Env) error {
	val, err := templates.ResolveTemplateString(p.Ensure, env)
	if err != nil {
		return err
	}
	p.Ensure = val

	val, err = templates.ResolveTemplateString(p.Name, env)
	if err != nil {
		return err
	}
	p.Name = val

	val, err = templates.ResolveTemplateString(p.Provider, env)
	if err != nil {
		return err
	}
	p.Provider = val

	for _, hc := range p.HealthChecks {
		val, err = templates.ResolveTemplateString(hc.Command, env)
		if err != nil {
			return err
		}
		hc.Command = val
	}

	return nil
}

// Validate validates common resource properties
func (p *CommonResourceProperties) Validate() error {
	if p.SkipValidate {
		return nil
	}

	if p.Name == "" {
		return ErrResourceNameRequired
	}

	if p.Ensure == "" {
		return ErrResourceEnsureRequired
	}

	if len(p.Require) > 0 {
		if !iu.IsValidResourceRef(p.Require...) {
			return ErrInvalidRequires
		}
	}

	return nil
}

// NewCommonResourceState creates a new common resource state with the given properties
func NewCommonResourceState(protocol string, resourceType string, name string, ensure string) CommonResourceState {
	return CommonResourceState{
		TimeStamp:    time.Now().UTC(),
		Protocol:     protocol,
		ResourceType: resourceType,
		Name:         name,
		Ensure:       ensure,
	}
}

// TODO: seems redundant

// CommonResourceState contains state information shared by all resource types
type CommonResourceState struct {
	TimeStamp    time.Time          `json:"timestamp" yaml:"timestamp"`
	Protocol     string             `json:"protocol" yaml:"protocol"`
	ResourceType string             `json:"type" yaml:"type"`
	Name         string             `json:"name" yaml:"name"`
	Ensure       string             `json:"ensure" yaml:"ensure"`
	Changed      bool               `json:"changed" yaml:"changed"`
	Refreshed    bool               `json:"refreshed" yaml:"refreshed"`
	Stable       bool               `json:"stable" yaml:"stable"`
	Noop         bool               `json:"noop" yaml:"noop"`
	NoopMessage  string             `json:"noop_message,omitempty" yaml:"noop_message,omitempty"`
	HealthCheck  *HealthCheckResult `json:"health_check,omitempty" yaml:"health_check,omitempty"`
}

// NewResourcePropertiesFromYaml creates a new resource properties object from a yaml document, it validates the properties and expands any templates
func NewResourcePropertiesFromYaml(typeName string, rawProperties yaml.RawMessage, env *templates.Env) ([]ResourceProperties, error) {
	var props []ResourceProperties
	var err error

	// TODO: extend the registry to include these so its automatic and doesnt need maintenance

	switch typeName {
	case PackageTypeName:
		props, err = NewPackageResourcePropertiesFromYaml(rawProperties)
	case ServiceTypeName:
		props, err = NewServiceResourcePropertiesFromYaml(rawProperties)
	case FileTypeName:
		props, err = NewFileResourcePropertiesFromYaml(rawProperties)
	case ExecTypeName:
		props, err = NewExecResourcePropertiesFromYaml(rawProperties)
	case ArchiveTypeName:
		props, err = NewArchiveResourcePropertiesFromYaml(rawProperties)
	case ScaffoldTypeName:
		props, err = NewScaffoldResourcePropertiesFromYaml(rawProperties)
	default:
		return nil, fmt.Errorf("%w: %s %s", ErrResourceInvalid, ErrUnknownType, typeName)
	}
	if err != nil {
		return nil, err
	}

	for _, prop := range props {
		err = prop.ResolveTemplates(env)
		if err != nil {
			return nil, err
		}
	}

	return props, nil
}

// NewValidatedResourcePropertiesFromYaml creates and validates a new resource properties object from a yaml document, it validates the properties and expands any templates
func NewValidatedResourcePropertiesFromYaml(typeName string, rawProperties yaml.RawMessage, env *templates.Env) ([]ResourceProperties, error) {
	props, err := NewResourcePropertiesFromYaml(typeName, rawProperties, env)
	if err != nil {
		return nil, err
	}

	for _, prop := range props {
		err = prop.Validate()
		if err != nil {
			return nil, err
		}
	}

	return props, nil
}

func findDefaultProperties(props []map[string]yaml.RawMessage) (yaml.RawMessage, error) {
	var defaultProps yaml.RawMessage

	for _, v := range props {
		if len(v) != 1 {
			return nil, fmt.Errorf("multiple resource names found in package resource")
		}
		_, dflt := v["defaults"]
		if dflt {
			if defaultProps != nil {
				return nil, fmt.Errorf("multiple defaults found in package resource")
			}
			defaultProps = v["defaults"]
		}
	}

	return defaultProps, nil
}

func parseProperties(raw yaml.RawMessage, typeName string, target func() ResourceProperties) ([]ResourceProperties, error) {
	var props []map[string]yaml.RawMessage

	yaml.Unmarshal(raw, &props) // failure is expected cos this detects 2 formats

	switch len(props) {
	case 0:
		prop := target()
		err := yaml.Unmarshal(raw, prop)
		if err != nil {
			return nil, err
		}

		cp := prop.CommonProperties()
		cp.Type = typeName
		return []ResourceProperties{prop}, nil

	default:
		return parseMultipleProperties(props, typeName, target)
	}
}

func parseMultipleProperties(props []map[string]yaml.RawMessage, typeName string, target func() ResourceProperties) ([]ResourceProperties, error) {
	var res []ResourceProperties

	dflt, err := findDefaultProperties(props)
	if err != nil {
		return nil, err
	}

	for _, v := range props {
		for name, vprop := range v {
			if name == "defaults" {
				continue
			}

			prop := target()

			if len(dflt) > 0 {
				err := yaml.Unmarshal(dflt, prop)
				if err != nil {
					return nil, err
				}
			}

			err := yaml.Unmarshal(vprop, prop)
			if err != nil {
				return nil, err
			}

			cp := prop.CommonProperties()
			cp.Name = name
			cp.Type = typeName
			res = append(res, prop)
		}
	}

	return res, nil
}
