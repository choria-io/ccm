// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"math"
	"net"
	"regexp"
	"sync"
	"time"
)

const (
	validBasicName         = `[a-zA-Z][a-zA-Z\d_-]*`
	ErrRegistrationInvalid = "registration is invalid"
)

var (
	validServiceRegex = regexp.MustCompile(`^` + validBasicName + `$`)
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
	Timestamp   time.Time         `json:"timestamp" yaml:"timestamp"`
	Cluster     string            `json:"cluster" yaml:"cluster"`
	Service     string            `json:"service" yaml:"service"`
	Protocol    string            `json:"protocol" yaml:"protocol"`
	IP          string            `json:"address" yaml:"address"`
	Port        uint              `json:"port" yaml:"port"`
	Priority    uint              `json:"priority" yaml:"priority"`
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	TTL         time.Duration     `json:"-" yaml:"-"`
	mu          sync.Mutex
}

func (e *RegistrationEntry) InstanceId() string {
	h := fnv.New64a()
	h.Write([]byte(e.Cluster))
	h.Write([]byte(e.Service))
	h.Write([]byte(e.Protocol))
	h.Write([]byte(e.IP))
	h.Write([]byte(fmt.Sprintf("%d", e.Port)))

	return hex.EncodeToString(h.Sum(nil))
}

func NewRegistrationEntry(cluster string, service string, protocol string, ip string, port uint, priority uint, ttl time.Duration) (*RegistrationEntry, error) {
	return &RegistrationEntry{
		Timestamp: time.Now().UTC(),
		Cluster:   cluster,
		Service:   service,
		Protocol:  protocol,
		IP:        ip,
		Port:      port,
		Priority:  priority,
		TTL:       ttl,
	}, nil
}

func (e *RegistrationEntry) Validate() error {
	if e.Cluster == "" {
		return fmt.Errorf("%s: cluster is required", ErrRegistrationInvalid)
	}

	if e.Protocol == "" {
		return fmt.Errorf("%s: protocol is required", ErrRegistrationInvalid)
	}

	if e.Service == "" {
		return fmt.Errorf("%s: service is required", ErrRegistrationInvalid)
	}

	if !validServiceRegex.MatchString(e.Service) {
		return fmt.Errorf("%s: service %q is not a valid name", ErrRegistrationInvalid, e.Service)
	}

	if e.IP == "" {
		return fmt.Errorf("%s: address is required", ErrRegistrationInvalid)
	}

	ip := net.ParseIP(e.IP)
	if ip == nil {
		return fmt.Errorf("%s: address %q is not a valid IP address", ErrRegistrationInvalid, e.IP)
	}

	if e.Port == 0 || e.Port > math.MaxUint16 {
		return fmt.Errorf("%s: port must be between 1 and %d", ErrRegistrationInvalid, math.MaxUint16)
	}

	if e.Priority == 0 {
		return fmt.Errorf("%s: priority is required", ErrRegistrationInvalid)
	}

	return nil
}
