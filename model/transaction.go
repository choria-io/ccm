// Copyright (c) 2025-2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/ksuid"
)

type SessionEvent interface {
	SessionEventID() string
	String() string
}

type Apply interface {
	Resources() []map[string]ResourceProperties
	Data() map[string]any
	Execute(ctx context.Context, mgr Manager, healthCheckOnly bool, userLog Logger) (SessionStore, error)
}

type SessionStore interface {
	StartSession(Apply) error
	StopSession(destroy bool) (*SessionSummary, error)
	RecordEvent(SessionEvent) error
	EventsForResource(resourceType string, resourceName string) ([]TransactionEvent, error)
	AllEvents() ([]SessionEvent, error)
}

const TransactionEventProtocol = "io.choria.ccm.v1.transaction.event"
const SessionStartEventProtocol = "io.choria.ccm.v1.session.start"

// **NOTE** If this change also update metrics, event summary, cmd and CommonResourceState

// TransactionEvent represents a single event for a resource session
type TransactionEvent struct {
	Protocol     string             `json:"protocol" yaml:"protocol"`
	EventID      string             `json:"event_id" yaml:"event_id"`
	TimeStamp    time.Time          `json:"timestamp" yaml:"timestamp"`
	ResourceType string             `json:"type" yaml:"type"`
	Provider     string             `json:"provider" yaml:"provider"`
	Name         string             `json:"name" yaml:"name"`
	Ensure       string             `json:"ensure" yaml:"ensure"`               // Ensure is the requested ensure value
	ActualEnsure string             `json:"actual_ensure" yaml:"actual_ensure"` // ActualEnsure is the actual `ensure` value after the session
	Duration     time.Duration      `json:"duration" yaml:"duration"`
	Properties   ResourceProperties `json:"properties" yaml:"properties"`
	Status       ResourceState      `json:"status" yaml:"status"`
	NoopMessage  string             `json:"noop_message,omitempty" yaml:"noop_message,omitempty"`
	HealthCheck  *HealthCheckResult `json:"health_check,omitempty" yaml:"health_check,omitempty"`

	Error     string `json:"error" yaml:"error"`
	Changed   bool   `json:"changed" yaml:"changed"`
	Refreshed bool   `json:"refreshed" yaml:"refreshed"` // Refreshed indicates the resource was restarted/reloaded via subscribe
	Failed    bool   `json:"failed" yaml:"failed"`
	Skipped   bool   `json:"skipped" yaml:"skipped"`
	Noop      bool   `json:"noop" yaml:"noop"`
}

type SessionStartEvent struct {
	Protocol  string    `json:"protocol" yaml:"protocol"`
	EventID   string    `json:"event_id" yaml:"event_id"`
	TimeStamp time.Time `json:"timestamp" yaml:"timestamp"`
}

func NewSessionStartEvent() *SessionStartEvent {
	return &SessionStartEvent{
		Protocol:  SessionStartEventProtocol,
		EventID:   ksuid.New().String(),
		TimeStamp: time.Now().UTC(),
	}
}

func NewTransactionEvent(typeName string, name string) *TransactionEvent {
	return &TransactionEvent{
		Protocol:     TransactionEventProtocol,
		EventID:      ksuid.New().String(),
		TimeStamp:    time.Now().UTC(),
		ResourceType: typeName,
		Name:         name,
	}
}

func (t *SessionStartEvent) SessionEventID() string { return t.EventID }
func (t *SessionStartEvent) String() string {
	return fmt.Sprintf("session %s started %s", t.EventID, t.TimeStamp.Format(time.RFC3339))
}

func (t *TransactionEvent) SessionEventID() string { return t.EventID }

func (t *TransactionEvent) LogStatus(log Logger) {
	args := []any{
		"ensure", t.Ensure,
		"runtime", t.Duration.Truncate(time.Millisecond),
		"provider", t.Provider,
	}

	if t.HealthCheck != nil {
		args = append(args, "checks", t.HealthCheck.Tries)
		args = append(args, "status", t.HealthCheck.Status.String())
	}

	if t.Noop {
		if t.NoopMessage != "" {
			args = append(args, "noop", t.NoopMessage)
		} else {
			args = append(args, "noop", true)
		}
	}

	switch {
	case t.Failed:
		log.Error(fmt.Sprintf("%s#%s failed", t.ResourceType, t.Name), append(args, "error", t.Error)...)
	case t.Skipped:
		log.Warn(fmt.Sprintf("%s#%s skipped", t.ResourceType, t.Name), args...)
	case t.Refreshed:
		log.Warn(fmt.Sprintf("%s#%s refreshed", t.ResourceType, t.Name), args...)
	case t.Changed:
		log.Warn(fmt.Sprintf("%s#%s changed", t.ResourceType, t.Name), args...)
	default:
		log.Info(fmt.Sprintf("%s#%s stable", t.ResourceType, t.Name), args...)
	}
}
func (t *TransactionEvent) String() string {
	switch {
	case t.Failed:
		return fmt.Sprintf("%s#%s failed ensure=%s runtime=%v error=%v provider=%s", t.ResourceType, t.Name, t.Ensure, t.Duration, t.Error, t.Provider)
	case t.Skipped:
		return fmt.Sprintf("%s#%s skipped ensure=%s runtime=%v provider=%s", t.ResourceType, t.Name, t.Ensure, t.Duration, t.Provider)
	case t.Changed:
		return fmt.Sprintf("%s#%s changed ensure=%s runtime=%v provider=%s", t.ResourceType, t.Name, t.Ensure, t.Duration, t.Provider)
	default:
		return fmt.Sprintf("%s#%s ensure=%s runtime=%v provider=%s", t.ResourceType, t.Name, t.Ensure, t.Duration, t.Provider)
	}
}

// SessionSummary provides a statistical summary of a configuration management session
type SessionSummary struct {
	StartTime                time.Time     `json:"start_time" yaml:"start_time"`
	EndTime                  time.Time     `json:"end_time" yaml:"end_time"`
	TotalDuration            time.Duration `json:"total_duration" yaml:"total_duration"`
	TotalResources           int           `json:"total_resources" yaml:"total_resources"`
	UniqueResources          int           `json:"unique_resources" yaml:"unique_resources"`
	ChangedResources         int           `json:"changed_resources" yaml:"changed_resources"`
	FailedResources          int           `json:"failed_resources" yaml:"failed_resources"`
	SkippedResources         int           `json:"skipped_resources" yaml:"skipped_resources"`
	StableResources          int           `json:"stable_resources" yaml:"stable_resources"`
	RefreshedCount           int           `json:"refreshed_count" yaml:"refreshed_count"`
	HealthCheckedCount       int           `json:"health_checked_count" yaml:"health_checked_count"`
	HealthCheckOKCount       int           `json:"health_check_ok_count" yaml:"health_check_ok_count"`
	HealthCheckWarningCount  int           `json:"health_check_warning_count" yaml:"health_check_warning_count"`
	HealthCheckCriticalCount int           `json:"health_check_critical_count" yaml:"health_check_critical_count"`
	HealthCheckUnknownCount  int           `json:"health_check_unknown_count" yaml:"health_check_unknown_count"`
	TotalErrors              int           `json:"total_errors" yaml:"total_errors"`
}

// BuildSessionSummary creates a summary report from all events in a session
func BuildSessionSummary(events []SessionEvent) *SessionSummary {
	summary := &SessionSummary{}
	var totalTime time.Duration
	var uniques = map[string]struct{}{}

	for _, event := range events {
		// Handle session start event
		if startEvent, ok := event.(*SessionStartEvent); ok {
			summary.StartTime = startEvent.TimeStamp
			continue
		}

		// Handle transaction events
		txEvent, ok := event.(*TransactionEvent)
		if !ok {
			continue
		}

		totalTime += txEvent.Duration
		summary.TotalResources++
		uniques[txEvent.ResourceType+"#"+txEvent.Name] = struct{}{}

		// Track the latest timestamp as end time
		if txEvent.TimeStamp.After(summary.EndTime) {
			summary.EndTime = txEvent.TimeStamp
		}

		// Categorize the resource by its outcome
		switch {
		case txEvent.Failed:
			summary.FailedResources++
			summary.TotalErrors++
		case txEvent.Skipped:
			summary.SkippedResources++
		case txEvent.Changed:
			summary.ChangedResources++
		default:
			summary.StableResources++
		}

		// Track refreshes separately (a resource can be both changed and refreshed)
		if txEvent.Refreshed {
			summary.RefreshedCount++
		}

		if txEvent.HealthCheck != nil {
			summary.HealthCheckedCount++
			switch txEvent.HealthCheck.Status {
			case HealthCheckOK:
				summary.HealthCheckOKCount++
			case HealthCheckWarning:
				summary.HealthCheckWarningCount++
			case HealthCheckCritical:
				summary.HealthCheckCriticalCount++
			default:
				summary.HealthCheckUnknownCount++
			}
		}
	}

	summary.UniqueResources = len(uniques)

	// Calculate total duration
	if !summary.StartTime.IsZero() && !summary.EndTime.IsZero() {
		summary.TotalDuration = summary.EndTime.Sub(summary.StartTime)
	} else {
		summary.TotalDuration = totalTime
	}

	return summary
}

// String returns a human-readable summary of the session
func (s *SessionSummary) String() string {
	return fmt.Sprintf("Session: %d resources, %d changed, %d failed, %d skipped, %d stable, %d refreshed, duration=%v",
		s.TotalResources, s.ChangedResources, s.FailedResources, s.SkippedResources, s.StableResources, s.RefreshedCount, s.TotalDuration)
}
