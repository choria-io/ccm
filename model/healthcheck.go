// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/choria-io/fisk"
)

type HealthCheckFormat string
type HealthCheckStatus int

const (
	HealthCheckNagiosFormat HealthCheckFormat = "nagios"
	HealthCheckGossFormat   HealthCheckFormat = "goss"

	HealthCheckOK       HealthCheckStatus = 0
	HealthCheckWarning  HealthCheckStatus = 1
	HealthCheckCritical HealthCheckStatus = 2
	HealthCheckUnknown  HealthCheckStatus = 3
)

func (s HealthCheckStatus) String() string {
	switch s {
	case HealthCheckOK:
		return "OK"
	case HealthCheckWarning:
		return "WARNING"
	case HealthCheckCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// CommonHealthCheck defines the configuration for resource health checks
type CommonHealthCheck struct {
	Command       string            `json:"command,omitempty" yaml:"command,omitempty"`       // Command is the shell command to execute for nagios-format health checks
	GossRules     yaml.RawMessage   `json:"goss_rules,omitempty" yaml:"goss_rules,omitempty"` // GossRules contains YAML rules for goss-format health checks
	Name          string            `json:"name,omitempty" yaml:"name,omitempty"`             // Name is the human-readable identifier for this health check
	Timeout       string            `json:"timeout,omitempty" yaml:"timeout,omitempty"`       // Timeout is the maximum duration to wait for health check completion (parsed into ParsedTimeout)
	Tries         int               `json:"tries,omitempty" yaml:"tries,omitempty"`           // Tries is the number of retry attempts before marking the health check as failed
	TrySleep      string            `json:"try_sleep,omitempty" yaml:"try_sleep,omitempty"`   // TrySleep is the duration to wait between retry attempts (parsed into ParseTrySleep)
	Format        HealthCheckFormat `json:"format,omitempty" yaml:"format,omitempty"`         // Format specifies the health check output format (nagios or goss)
	ParsedTimeout time.Duration     `json:"-" yaml:"-"`                                       // ParsedTimeout is the parsed duration from the Timeout field
	ParseTrySleep time.Duration     `json:"-" yaml:"-"`                                       // ParseTrySleep is the parsed duration from the TrySleep field
	TypeName      string            `json:"-" yaml:"-"`                                       // TypeName is the resource type this health check belongs to (e.g., "service", "package")
	ResourceName  string            `json:"-" yaml:"-"`                                       // ResourceName is the specific resource instance this health check belongs to
}

// commonHealthCheckAlias is used to prevent infinite recursion in custom unmarshallers
type commonHealthCheckAlias CommonHealthCheck

// UnmarshalJSON implements json.Unmarshaler to parse Timeout string into ParsedTimeout duration
func (c *CommonHealthCheck) UnmarshalJSON(data []byte) error {
	var alias commonHealthCheckAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}

	*c = CommonHealthCheck(alias)

	return c.normalizeAfterUnmarshal()
}

func (c *CommonHealthCheck) normalizeAfterUnmarshal() error {
	c.Command = strings.TrimSpace(c.Command)

	if c.Timeout != "" {
		d, err := fisk.ParseDuration(c.Timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout duration %q: %w", c.Timeout, err)
		}
		c.ParsedTimeout = d
	}

	if c.TrySleep != "" {
		d, err := fisk.ParseDuration(c.TrySleep)
		if err != nil {
			return fmt.Errorf("invalid try sleep duration %q: %w", c.TrySleep, err)
		}
		c.ParseTrySleep = d
	}

	switch {
	case len(c.Command) > 0 && len(c.GossRules) > 0:
		return fmt.Errorf("'command' or 'goss_rules' are mutually exclusive")
	case len(c.GossRules) > 0 && c.Command != "":
		return fmt.Errorf("'command' and 'goss_rules' are mutually exclusive")
	case len(c.GossRules) > 0 && c.Format == HealthCheckGossFormat:
		// its good
	case len(c.Command) > 0 && c.Format == HealthCheckNagiosFormat:
		// its good
	case len(c.GossRules) > 0 && c.Format == "":
		c.Format = HealthCheckGossFormat
	case len(c.Command) > 0 && c.Format == "":
		c.Format = HealthCheckNagiosFormat
	case c.Command != "" && c.Format == "":
		c.Format = HealthCheckNagiosFormat
	case len(c.GossRules) > 0 && c.Format != HealthCheckGossFormat:
		return fmt.Errorf("'format' must be 'goss' when using 'goss_check'")
	case len(c.Command) > 0 && c.Format != HealthCheckNagiosFormat:
		return fmt.Errorf("'format' must be 'nagios' when using 'command'")
	default:
		return fmt.Errorf("'format' flag is required")
	}

	// TODO: once builtins come make this work
	if c.Name == "" {
		c.Name = filepath.Base(c.Command)
	}

	return nil
}

// UnmarshalYAML implements yaml.BytesUnmarshaler to parse Timeout string into ParsedTimeout duration
func (c *CommonHealthCheck) UnmarshalYAML(data []byte) error {
	var alias commonHealthCheckAlias
	if err := yaml.Unmarshal(data, &alias); err != nil {
		return err
	}

	*c = CommonHealthCheck(alias)

	return c.normalizeAfterUnmarshal()
}

// HealthCheckResult represents the outcome of a health check execution
type HealthCheckResult struct {
	Status HealthCheckStatus `json:"status" yaml:"status"`
	Tries  int               `json:"tries" yaml:"tries"`
	Output string
}
