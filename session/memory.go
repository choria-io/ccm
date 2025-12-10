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
	events []model.SessionEvent
	log    model.Logger
	out    model.Logger
	mu     sync.Mutex
}

// NewMemorySessionStore creates a new in-memory session store with the provided loggers
func NewMemorySessionStore(logger model.Logger, writer model.Logger) (*MemorySessionStore, error) {
	logger.Info("Creating new session store")
	return &MemorySessionStore{
		out:    writer,
		log:    logger,
		events: make([]model.SessionEvent, 0),
	}, nil
}

// StartSession clears the event log and starts a new session for the given manifest
func (s *MemorySessionStore) StartSession(manifest model.Apply) error {
	s.mu.Lock()
	s.out.Info("Creating new session record", "resources", len(manifest.Resources()))
	s.events = make([]model.SessionEvent, 0)
	s.mu.Unlock()

	start := model.NewSessionStartEvent()
	s.start = start.TimeStamp

	return s.RecordEvent(start)
}

// RecordEvent adds a transaction event to the session and logs its status
func (s *MemorySessionStore) RecordEvent(event model.SessionEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = append(s.events, event)

	te, ok := event.(*model.TransactionEvent)
	if ok {
		te.LogStatus(s.out)
	}

	return nil
}

// EventsForResource returns all events for a given resource, the events are in time order with latest event at the end
func (s *MemorySessionStore) EventsForResource(resourceType string, resourceName string) ([]model.TransactionEvent, error) {
	// Get all events from the session
	allEvents, err := s.AllEvents()
	if err != nil {
		return nil, err
	}

	// Filter for the specific resource
	var filtered []model.TransactionEvent
	for _, event := range allEvents {
		// Only include TransactionEvents (skip SessionStartEvent)
		txEvent, ok := event.(*model.TransactionEvent)
		if !ok {
			continue
		}

		// Filter by resourceType and name
		if txEvent.ResourceType == resourceType && txEvent.Name == resourceName {
			filtered = append(filtered, *txEvent)
		}
	}

	return filtered, nil
}

// AllEvents returns all events in the session in time order
func (s *MemorySessionStore) AllEvents() ([]model.SessionEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Return a copy of the events slice to avoid external modifications
	eventsCopy := make([]model.SessionEvent, len(s.events))
	copy(eventsCopy, s.events)

	return eventsCopy, nil
}
