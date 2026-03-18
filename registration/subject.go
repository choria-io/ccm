// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"fmt"
	"strings"

	"github.com/choria-io/ccm/model"
)

const (
	natsSubjectPrefix = "choria.ccm.registration.v1"

	// Token positions within a split subject
	tokenCluster  = 4
	tokenProtocol = 5
	tokenService  = 6
	tokenAddress  = 7

	// Minimum number of tokens in a valid subject
	// prefix(3) + cluster + protocol + service + address + instance
	minSubjectTokens = 9
)

// publishSubject returns the full publish subject for an entry with the given instance ID.
// Format: choria.ccm.registration.v1.<cluster>.<protocol>.<service>.<address>.<instance>
func publishSubject(e *model.RegistrationEntry, instanceID string) string {
	return fmt.Sprintf("%s.%s.%s.%s.%s.%s", natsSubjectPrefix, e.Cluster, e.Protocol, e.Service, e.SubjectAddress(), instanceID)
}

// FilterSubject returns a NATS filter subject for querying registrations.
// Empty strings or "*" in any position become wildcards. The instance token
// is always wildcarded.
func filterSubject(cluster, protocol, service, ip string) string {
	wild := func(s string) string {
		if s == "" || s == "*" {
			return "*"
		}
		return s
	}

	addr := wild(ip)
	if addr != "*" {
		addr = strings.ReplaceAll(addr, ".", "_")
	}

	return fmt.Sprintf("%s.%s.%s.%s.%s.*", natsSubjectPrefix, wild(cluster), wild(protocol), wild(service), addr)
}

// ParseSubject extracts registration fields from a NATS subject.
// Returns a partially populated entry; only Cluster, Protocol, Service, and Address are set.
// Returns an empty entry if the subject does not have enough tokens.
func parseSubject(subject string) *model.RegistrationEntry {
	parts := strings.Split(subject, ".")

	if len(parts) < minSubjectTokens {
		return &model.RegistrationEntry{}
	}

	return &model.RegistrationEntry{
		Cluster:  parts[tokenCluster],
		Protocol: parts[tokenProtocol],
		Service:  parts[tokenService],
		Address:  strings.ReplaceAll(parts[tokenAddress], "_", "."),
	}
}
