// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/choria-io/fisk"
	"github.com/goccy/go-yaml"
)

type HealthCheckFormat string
type HealthCheckStatus int

const (
	HealthCheckNagiosFormat HealthCheckFormat = "nagios"
	HealthCheckOK           HealthCheckStatus = 0
	HealthCheckWarning      HealthCheckStatus = 1
	HealthCheckCritical     HealthCheckStatus = 2
	HealthCheckUnknown      HealthCheckStatus = 3
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

type CommonHealthCheck struct {
	Command       string            `json:"command,omitempty" yaml:"command,omitempty"`
	Timeout       string            `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Tries         int               `json:"tries,omitempty" yaml:"tries,omitempty"`
	TrySleep      string            `json:"try_sleep,omitempty" yaml:"try_sleep,omitempty"`
	Format        HealthCheckFormat `json:"format,omitempty" yaml:"format,omitempty"`
	ParsedTimeout time.Duration     `json:"-" yaml:"-"`
	ParseTrySleep time.Duration     `json:"-" yaml:"-"`
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

	if c.Format == "" {
		c.Format = HealthCheckNagiosFormat
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

	if c.Timeout != "" {
		d, err := fisk.ParseDuration(c.Timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout duration %q: %w", c.Timeout, err)
		}
		c.ParsedTimeout = d
	}

	if c.TrySleep == "" {
		c.TrySleep = "1s"
	}
	if c.TrySleep != "" {
		d, err := fisk.ParseDuration(c.TrySleep)
		if err != nil {
			return fmt.Errorf("invalid try sleep duration %q: %w", c.TrySleep, err)
		}
		c.ParseTrySleep = d
	}

	if c.Format == "" {
		c.Format = HealthCheckNagiosFormat
	}

	return nil
}

// HealthCheckResult represents the outcome of a health check execution
type HealthCheckResult struct {
	Status HealthCheckStatus `json:"status" yaml:"status"`
	Tries  int               `json:"tries" yaml:"tries"`
	Output string
}
