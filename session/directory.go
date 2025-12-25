// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/segmentio/ksuid"

	iu "github.com/choria-io/ccm/internal/util"
	"github.com/choria-io/ccm/model"
)

// DirectorySessionStore stores transaction events in a directory of files
type DirectorySessionStore struct {
	directory string
	log       model.Logger
	out       model.Logger
	mu        sync.Mutex
}

// NewDirectorySessionStore creates a new directory of files based session store with the provided loggers
func NewDirectorySessionStore(directory string, logger model.Logger, writer model.Logger) (*DirectorySessionStore, error) {
	// Reject empty directory path early
	if directory == "" {
		return nil, fmt.Errorf("session directory path cannot be empty")
	}

	// Clean and make the directory path absolute to prevent path traversal
	absDir, err := filepath.Abs(filepath.Clean(directory))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve directory path: %w", err)
	}

	// Validate the directory path for safety
	err = validateSessionDirectory(absDir)
	if err != nil {
		return nil, err
	}

	logger.Info("Creating new session store")

	return &DirectorySessionStore{
		out:       writer,
		log:       logger,
		directory: absDir,
	}, nil
}

// validateSessionDirectory checks if a directory path is safe to use as a session store
func validateSessionDirectory(absPath string) error {
	// Reject empty paths
	if absPath == "" {
		return fmt.Errorf("session directory path cannot be empty")
	}

	return nil
}

func (s *DirectorySessionStore) StartSession(manifest model.Apply) error {
	s.log.Info("Creating new session record", "resources", len(manifest.Resources()), "store", "directory")

	s.mu.Lock()
	err := os.MkdirAll(s.directory, 0755)
	s.mu.Unlock()
	if err != nil {
		return err
	}

	start := model.NewSessionStartEvent()

	return s.RecordEvent(start)
}

// EventsForResource returns all events for a given resource, the events are sorted in time order with latest event at the end
func (s *DirectorySessionStore) EventsForResource(resourceType string, resourceName string) ([]model.TransactionEvent, error) {
	// Get all events from the session
	allEvents, err := s.AllEvents()
	if err != nil {
		return nil, err
	}

	return fileterEvents(allEvents, resourceType, resourceName)
}

func (s *DirectorySessionStore) RecordEvent(event model.SessionEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	updateMetrics(event)

	// Validate EventID is a valid ksuid to prevent directory traversal
	// Valid ksuids contain only safe characters (base62) and no path separators
	_, err := ksuid.Parse(event.SessionEventID())
	if err != nil {
		return fmt.Errorf("invalid event ID: %w", err)
	}

	if !iu.IsDirectory(s.directory) {
		return fmt.Errorf("session store %s does not exist", s.directory)
	}

	// Marshal event to JSON
	data, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return err
	}

	// Write to file named <eventid>.event
	// Safe to use EventID directly since it's validated as a ksuid
	filename := filepath.Join(s.directory, event.SessionEventID()+".event")
	s.log.Info("Recording event", "filename", filename)

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return err
	}

	return nil
}
func (s *DirectorySessionStore) StopSession(destroy bool) (*model.SessionSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	events, err := s.allEventsUnlocked()
	if err != nil {
		return nil, err
	}

	summary := model.BuildSessionSummary(events)

	if destroy && iu.IsDirectory(s.directory) {
		err = os.RemoveAll(s.directory)
		if err != nil {
			s.log.Error("Failed to remove session directory", "error", err)
		}
	}

	return summary, nil
}

func (s *DirectorySessionStore) AllEvents() ([]model.SessionEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.allEventsUnlocked()
}

// AllEvents returns all events in the session sorted by time order (oldest first)
func (s *DirectorySessionStore) allEventsUnlocked() ([]model.SessionEvent, error) {
	var events []model.SessionEvent

	// Read all files in the directory
	entries, err := os.ReadDir(s.directory)
	if err != nil {
		if os.IsNotExist(err) {
			// Directory doesn't exist yet, return empty slice
			return events, nil
		}
		return nil, fmt.Errorf("failed to read session directory: %w", err)
	}

	// Process each .event file
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".event") {
			continue
		}

		// Read the event file
		filename := filepath.Join(s.directory, entry.Name())
		data, err := os.ReadFile(filename)
		if err != nil {
			s.log.Error("Failed to read event file", "filename", filename, "error", err)
			continue
		}

		// Try to determine event type by examining the protocol field
		var eventType struct {
			Protocol string `json:"protocol"`
		}
		err = json.Unmarshal(data, &eventType)
		if err != nil {
			s.log.Error("Failed to parse event type", "filename", filename, "error", err)
			continue
		}

		// Parse based on protocol
		var event model.SessionEvent
		switch eventType.Protocol {
		case model.SessionStartEventProtocol:
			var startEvent model.SessionStartEvent
			err = json.Unmarshal(data, &startEvent)
			if err != nil {
				s.log.Error("Failed to parse session start event", "filename", filename, "error", err)
				continue
			}
			event = &startEvent

		case model.TransactionEventProtocol:
			var txEvent model.TransactionEvent
			err = json.Unmarshal(data, &txEvent)
			if err != nil {
				s.log.Error("Failed to parse transaction event", "filename", filename, "error", err)
				continue
			}
			event = &txEvent

		default:
			s.log.Warn("Unknown event protocol", "filename", filename, "protocol", eventType.Protocol)
			continue
		}

		events = append(events, event)
	}

	// Sort by EventID (ksuids are k-sortable, so this gives us time order)
	sort.Slice(events, func(i, j int) bool {
		return events[i].SessionEventID() < events[j].SessionEventID()
	})

	return events, nil
}
