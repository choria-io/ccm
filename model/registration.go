// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"encoding/hex"
	"encoding/json"
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
	validPromLabelRegex    = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
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

// RegistrationEntries is a collection of registration entries with transformation helpers
type RegistrationEntries []*RegistrationEntry

type prometheusTargetGroup struct {
	Labels  map[string]string `json:"labels"`
	Targets []string          `json:"targets"`
}

// PrometheusFileSD converts registration entries to Prometheus file service discovery JSON format.
//
// Entries are grouped by cluster, service, and protocol. Each group becomes a target group
// with targets formatted as "address:port". Entries without a port are skipped. Entries with
// an annotation "prometheus.io/scrape" set to anything other than "true" are skipped.
//
// Labels include cluster, service, protocol, and valid annotations from the first entry in the group.
// Annotations are only included as labels if the key matches [a-zA-Z_][a-zA-Z0-9_]*, does not
// start with __ (reserved for Prometheus internals), and has a non-empty value.
func (entries RegistrationEntries) PrometheusFileSD() (string, error) {
	type groupKey struct {
		cluster  string
		service  string
		protocol string
	}

	groups := map[groupKey]*prometheusTargetGroup{}
	var order []groupKey

	for _, entry := range entries {
		port := registrationPortInt(entry.Port)
		if port == 0 {
			continue
		}

		if v, ok := entry.Annotations["prometheus.io/scrape"]; ok && v != "true" {
			continue
		}

		key := groupKey{
			cluster:  entry.Cluster,
			service:  entry.Service,
			protocol: entry.Protocol,
		}

		group, exists := groups[key]
		if !exists {
			labels := map[string]string{
				"cluster":  entry.Cluster,
				"service":  entry.Service,
				"protocol": entry.Protocol,
			}
			for k, v := range entry.Annotations {
				if v == "" {
					continue
				}
				if strings.HasPrefix(k, "__") {
					continue
				}
				if !validPromLabelRegex.MatchString(k) {
					continue
				}
				labels[k] = v
			}

			group = &prometheusTargetGroup{
				Labels: labels,
			}
			groups[key] = group
			order = append(order, key)
		}

		group.Targets = append(group.Targets, fmt.Sprintf("%s:%d", entry.Address, port))
	}

	result := make([]prometheusTargetGroup, 0, len(order))
	for _, key := range order {
		result = append(result, *groups[key])
	}

	j, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("could not marshal prometheus file SD: %w", err)
	}

	return string(j), nil
}

func registrationPortInt(v any) int64 {
	switch p := v.(type) {
	case int64:
		return p
	case uint64:
		return int64(p)
	case float64:
		return int64(p)
	case json.Number:
		n, _ := p.Int64()
		return n
	default:
		return 0
	}
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
