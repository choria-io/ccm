// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/choria-io/ccm/templates"
)

const (
	validBasicName = `[a-zA-Z][a-zA-Z\d_-]*`
)

var (
	validServiceRegex      = regexp.MustCompile(`^` + validBasicName + `$`)
	ErrRegistrationInvalid = errors.New("registration is invalid")
)

type RegistrationDestination string

const (
	NatsRegistrationDestination      RegistrationDestination = "nats"
	JetStreamRegistrationDestination RegistrationDestination = "jetstream"
)

type RegistrationPublisher interface {
	Publish(ctx context.Context, entry *RegistrationEntry) error
}

type RegistrationEntry struct {
	Cluster     string            `json:"cluster" yaml:"cluster"`
	Service     string            `json:"service" yaml:"service"`
	Protocol    string            `json:"protocol" yaml:"protocol"`
	Address     string            `json:"address" yaml:"address"`
	Port        any               `json:"port" yaml:"port"`
	Priority    int64             `json:"priority" yaml:"priority"`
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	TTL         time.Duration     `json:"ttl,omitempty" yaml:"ttl,omitempty"`
}

// SubjectAddress returns the Address address with dots replaced by underscores,
// making it safe to use as a single NATS subject token
func (e *RegistrationEntry) SubjectAddress() string {
	return strings.ReplaceAll(e.Address, ".", "_")
}

func (e *RegistrationEntry) InstanceId() string {
	h := fnv.New64a()
	h.Write([]byte(e.Cluster))
	h.Write([]byte(e.Service))
	h.Write([]byte(e.Protocol))
	h.Write([]byte(e.Address))
	h.Write([]byte(fmt.Sprintf("%d", e.Port)))

	return hex.EncodeToString(h.Sum(nil))
}

func NewRegistrationEntry(cluster string, service string, protocol string, address string, port int64, priority int, ttl time.Duration) (*RegistrationEntry, error) {
	return &RegistrationEntry{
		Cluster:  cluster,
		Service:  service,
		Protocol: protocol,
		Address:  address,
		Port:     port,
		Priority: int64(priority),
		TTL:      ttl,
	}, nil
}

func (e *RegistrationEntry) ResolveTemplates(env *templates.Env) error {
	var err error

	e.Address, err = templates.ResolveTemplateString(e.Address, env)
	if err != nil {
		return err
	}

	e.Cluster, err = templates.ResolveTemplateString(e.Cluster, env)
	if err != nil {
		return err
	}

	for k, v := range e.Annotations {
		resolved, err := templates.ResolveTemplateString(v, env)
		if err != nil {
			return err
		}
		e.Annotations[k] = resolved
	}

	if e.Port != nil {
		switch p := e.Port.(type) {
		case string:
			val, err := templates.ResolveTemplateTyped(p, env)
			if err != nil {
				return err
			}
			if _, ok := val.(int64); !ok {
				return fmt.Errorf("%w: port must be an integer number not %T", ErrRegistrationInvalid, val)
			}

			e.Port = val
		case int64, uint64:
		default:
			return fmt.Errorf("%w: port must be an integer number not %T", ErrRegistrationInvalid, e.Port)
		}
	}
	return nil
}

func (e *RegistrationEntry) Validate() error {
	if e.Cluster == "" {
		return fmt.Errorf("%w: cluster is required", ErrRegistrationInvalid)
	}
	if !validServiceRegex.MatchString(e.Cluster) {
		return fmt.Errorf("%w: cluster %q is not a valid name", ErrRegistrationInvalid, e.Cluster)
	}

	if e.Protocol == "" {
		return fmt.Errorf("%w: protocol is required", ErrRegistrationInvalid)
	}
	if !validServiceRegex.MatchString(e.Protocol) {
		return fmt.Errorf("%w: protocol %q is not a valid name", ErrRegistrationInvalid, e.Protocol)
	}

	if e.Service == "" {
		return fmt.Errorf("%w: service is required", ErrRegistrationInvalid)
	}
	if !validServiceRegex.MatchString(e.Service) {
		return fmt.Errorf("%w: service %q is not a valid name", ErrRegistrationInvalid, e.Service)
	}

	if e.Address == "" {
		return fmt.Errorf("%w: address is required", ErrRegistrationInvalid)
	}

	ip := net.ParseIP(e.Address)
	if ip == nil {
		return fmt.Errorf("%w: address %q is not a valid Address address", ErrRegistrationInvalid, e.Address)
	}

	switch port := e.Port.(type) {
	case nil:
	case uint64:
		err := e.validatePort(int64(port))
		if err != nil {
			return err
		}

	case int64:
		err := e.validatePort(port)
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("%w: port must be an integer number, not %T", ErrRegistrationInvalid, e.Port)
	}

	if e.Priority <= 0 || e.Priority > math.MaxUint8 {
		return fmt.Errorf("%w: priority '%d' must be between 1 and 255", ErrRegistrationInvalid, e.Priority)
	}

	return nil
}

func (e *RegistrationEntry) validatePort(p int64) error {
	if p <= 0 || p > math.MaxUint16 {
		return fmt.Errorf("%w: port '%d' must be between 1 and %d", ErrRegistrationInvalid, p, math.MaxUint16)
	}

	return nil
}
