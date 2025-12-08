// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"sync"
	"time"

	"github.com/choria-io/ccm/model"
)

// MemorySessionStore stores transaction events in memory for a session
type MemorySessionStore struct {
	start  time.Time
	events []model.TransactionEvent
	log    model.Logger
	out    model.Logger
	mu     sync.Mutex
}

// NewMemorySessionStore creates a new in-memory session store with the provided loggers
func NewMemorySessionStore(logger model.Logger, writer model.Logger) (*MemorySessionStore, error) {
	return &MemorySessionStore{
		out:    writer,
		log:    logger,
		events: make([]model.TransactionEvent, 0),
	}, nil
}

// StartSession clears the event log and starts a new session for the given manifest
func (s *MemorySessionStore) StartSession(manifest model.Apply) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.out.Info("Creating new session record", "resources", len(manifest.Resources()))

	s.events = make([]model.TransactionEvent, 0)
	s.start = time.Now().UTC()

	return nil
}

// RecordEvent adds a transaction event to the session and logs its status
func (s *MemorySessionStore) RecordEvent(event model.TransactionEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = append(s.events, event)

	event.LogStatus(s.out)
}

// EventsForResource returns all events for a given resource, the events are in time order with latest event at the end
func (s *MemorySessionStore) EventsForResource(resourceType string, resourceName string) ([]model.TransactionEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var res []model.TransactionEvent
	for _, e := range s.events {
		if e.ResourceType == resourceType && e.Name == resourceName {
			res = append(res, e)
		}
	}

	return res, nil
}

// ResourceEvents returns all events for a given resource
func (s *MemorySessionStore) ResourceEvents(resourceType string, resourceName string) []model.TransactionEvent {
	var res []model.TransactionEvent
	for _, e := range s.events {
		if e.ResourceType == resourceType && e.Name == resourceName {
			res = append(res, e)
		}
	}

	return res
}
