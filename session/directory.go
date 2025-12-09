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

	"github.com/choria-io/ccm/model"
	"github.com/segmentio/ksuid"
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
	// Clean and make the directory path absolute to prevent path traversal
	absDir, err := filepath.Abs(filepath.Clean(directory))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve directory path: %w", err)
	}

	logger.Info("Creating new session store")

	return &DirectorySessionStore{
		out:       writer,
		log:       logger,
		directory: absDir,
	}, nil
}

func (s *DirectorySessionStore) StartSession(manifest model.Apply) error { return nil }

// EventsForResource returns all events for a given resource, the events are sorted in time order with latest event at the end
func (d *DirectorySessionStore) EventsForResource(resourceType string, name string) ([]model.TransactionEvent, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var events []model.TransactionEvent

	// Read all files in the directory
	entries, err := os.ReadDir(d.directory)
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

		// Read and parse the event file
		filename := filepath.Join(d.directory, entry.Name())
		data, err := os.ReadFile(filename)
		if err != nil {
			d.log.Error("Failed to read event file", "filename", filename, "error", err)
			continue
		}

		var event model.TransactionEvent
		err = json.Unmarshal(data, &event)
		if err != nil {
			d.log.Error("Failed to parse event file", "filename", filename, "error", err)
			continue
		}

		// Filter by resourceType and name
		if event.ResourceType == resourceType && event.Name == name {
			events = append(events, event)
		}
	}

	// Sort by EventID (ksuids are k-sortable, so this gives us time order)
	sort.Slice(events, func(i, j int) bool {
		return events[i].EventID < events[j].EventID
	})

	return events, nil
}

func (d *DirectorySessionStore) RecordEvent(event model.TransactionEvent) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Validate EventID is a valid ksuid to prevent directory traversal
	// Valid ksuids contain only safe characters (base62) and no path separators
	_, err := ksuid.Parse(event.EventID)
	if err != nil {
		return fmt.Errorf("invalid event ID: %w", err)
	}

	// Create directory if it doesn't exist
	err = os.MkdirAll(d.directory, 0755)
	if err != nil {
		return err
	}

	// Marshal event to JSON
	data, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return err
	}

	// Write to file named <eventid>.event
	// Safe to use EventID directly since it's validated as a ksuid
	filename := filepath.Join(d.directory, event.EventID+".event")
	d.log.Info("Recording event", "filename", filename)

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (d *DirectorySessionStore) ResetSession(manifest model.Apply) {
}
