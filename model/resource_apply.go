package model

import (
	"fmt"
	"net/url"

	"github.com/choria-io/ccm/templates"
	"github.com/goccy/go-yaml"
)

const (
	// ResourceStatusApplyProtocol is the protocol identifier for apply resource state
	ResourceStatusApplyProtocol = "io.choria.ccm.v1.resource.apply.state"

	// ApplyTypeName is the type name for exec resources
	ApplyTypeName = "apply"
)

type ApplyResourceProperties struct {
	CommonResourceProperties `yaml:",inline"`
	Noop                     bool           `json:"noop,omitempty" yaml:"noop,omitempty"`
	HealthCheckOnly          bool           `json:"health_check_only,omitempty" yaml:"health_check_only,omitempty"`
	AllowApply               bool           `json:"allow_apply,omitempty" yaml:"allow_apply,omitempty"`
	Data                     map[string]any `json:"data,omitempty" yaml:"data,omitempty"`
}

type ApplyState struct {
	CommonResourceState
	ResourceCount int `json:"resource_count" yaml:"resource_count"`
}

// Validate validates the exec resource properties
func (p *ApplyResourceProperties) Validate() error {
	if p.SkipValidate {
		return nil
	}

	err := p.CommonResourceProperties.Validate()
	if err != nil {
		return err
	}

	u, err := url.Parse(p.Name)
	if err == nil && u.Scheme != "" {
		return fmt.Errorf("name must be a file path, not a URL")
	}

	if p.Ensure != EnsurePresent && p.Ensure != "" {
		return fmt.Errorf("%w: must be %q", ErrInvalidEnsureValue, EnsurePresent)
	}

	return nil
}

// ResolveTemplates resolves template expressions in the apply resource properties
func (p *ApplyResourceProperties) ResolveTemplates(env *templates.Env) error {
	err := templates.ResolveStructTemplates(p, env, false)
	if err != nil {
		return err
	}

	return p.resolveRegistrations(env)
}

func (p *ApplyResourceProperties) CommonProperties() *CommonResourceProperties {
	return &p.CommonResourceProperties
}

func (p *ApplyResourceProperties) ToYamlManifest() (yaml.RawMessage, error) {
	return yaml.Marshal(p)
}

// NewApplyResourcePropertiesFromYaml creates a new apply resource properties object from a yaml document, does not validate or expand templates
func NewApplyResourcePropertiesFromYaml(raw yaml.RawMessage) ([]ResourceProperties, error) {
	res, err := parseProperties(raw, ApplyTypeName, func() ResourceProperties { return &ApplyResourceProperties{} })
	if err != nil {
		return nil, err
	}

	for _, prop := range res {
		p := prop.(*ApplyResourceProperties)
		if p.Ensure == "" {
			p.Ensure = EnsurePresent
		}
	}

	return res, nil
}
