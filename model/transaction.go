// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
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
	FailOnError() bool
	Execute(ctx context.Context, mgr Manager, healthCheckOnly bool, userLog Logger) (SessionStore, error)
	Source() string
	String() string
	PreMessage() string
	PostMessage() string
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
	Protocol        string               `json:"protocol" yaml:"protocol"`
	EventID         string               `json:"event_id" yaml:"event_id"`
	TimeStamp       time.Time            `json:"timestamp" yaml:"timestamp"`
	ResourceType    string               `json:"type" yaml:"type"`
	Provider        string               `json:"provider" yaml:"provider"`
	Name            string               `json:"name" yaml:"name"`
	Alias           string               `json:"alias,omitempty" yaml:"alias,omitempty"`
	RequestedEnsure string               `json:"requested_ensure" yaml:"requested_ensure"` // RequestedEnsure is the requested ensure value in the initial properties
	FinalEnsure     string               `json:"final_ensure" yaml:"final_ensure"`         // FinalEnsure is the actual `ensure` value after the session
	Duration        time.Duration        `json:"duration" yaml:"duration"`
	Properties      any                  `json:"properties" yaml:"properties"`
	Status          any                  `json:"status" yaml:"status"`
	NoopMessage     string               `json:"noop_message,omitempty" yaml:"noop_message,omitempty"`
	HealthChecks    []*HealthCheckResult `json:"health_check,omitempty" yaml:"health_check,omitempty"`
	HealthCheckOnly bool                 `json:"health_check_only,omitempty" yaml:"health_check_only,omitempty"`

	Errors            []string `json:"error" yaml:"error"`
	Changed           bool     `json:"changed" yaml:"changed"`
	Refreshed         bool     `json:"refreshed" yaml:"refreshed"` // Refreshed indicates the resource was restarted/reloaded via subscribe
	Failed            bool     `json:"failed" yaml:"failed"`
	Skipped           bool     `json:"skipped" yaml:"skipped"`
	Noop              bool     `json:"noop" yaml:"noop"`
	UnmetRequirements []string `json:"unmet_requirements" yaml:"unmet_requirements"`
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

func NewTransactionEvent(typeName string, name string, alias string) *TransactionEvent {
	return &TransactionEvent{
		Protocol:          TransactionEventProtocol,
		EventID:           ksuid.New().String(),
		TimeStamp:         time.Now().UTC(),
		ResourceType:      typeName,
		Name:              name,
		Alias:             alias,
		UnmetRequirements: make([]string, 0),
		Errors:            make([]string, 0),
	}
}

func (t *SessionStartEvent) SessionEventID() string { return t.EventID }
func (t *SessionStartEvent) String() string {
	return fmt.Sprintf("session %s started %s", t.EventID, t.TimeStamp.Format(time.RFC3339))
}

func (t *TransactionEvent) SessionEventID() string { return t.EventID }

func (t *TransactionEvent) LogStatus(log Logger) {
	args := []any{
		"ensure", t.RequestedEnsure,
		"runtime", t.Duration.Truncate(time.Millisecond),
		"provider", t.Provider,
	}

	if t.Noop {
		if t.NoopMessage != "" {
			args = append(args, "noop", t.NoopMessage)
		} else {
			args = append(args, "noop", true)
		}
	}

	name := t.Name
	if t.Alias != "" {
		name = fmt.Sprintf("%s (alias)", t.Alias)
	}
	rname := fmt.Sprintf("%s#%s", t.ResourceType, name)

	switch {
	case t.Failed:
		log.Error(fmt.Sprintf("%s failed", rname), append(args, "errors", strings.Join(t.Errors, ", "))...)
	case len(t.UnmetRequirements) > 0:
		for _, req := range t.UnmetRequirements {
			args = append(args, "unmet", req)
		}
		log.Error(fmt.Sprintf("%s skipped due to unmet requirement", rname), args...)
	case t.Skipped:
		log.Warn(fmt.Sprintf("%s skipped", rname), args...)
	case t.Refreshed:
		log.Warn(fmt.Sprintf("%s refreshed", rname), args...)
	case t.Changed:
		log.Warn(fmt.Sprintf("%s changed", rname), args...)
	default:
		log.Info(fmt.Sprintf("%s stable", rname), args...)
	}
}
func (t *TransactionEvent) String() string {
	name := t.Name
	if t.Alias != "" {
		name = fmt.Sprintf("%s (alias)", t.Alias)
	}
	rname := fmt.Sprintf("%s#%s", t.ResourceType, name)

	switch {
	case t.Failed:
		return fmt.Sprintf("%s failed ensure=%s runtime=%v errors=%v provider=%s", rname, t.RequestedEnsure, t.Duration, strings.Join(t.Errors, ","), t.Provider)
	case len(t.UnmetRequirements) > 0:
		return fmt.Sprintf("%s skipped unmet requirements ensure=%s runtime=%v provider=%s unmet:%s", rname, t.RequestedEnsure, t.Duration, t.Provider, strings.Join(t.UnmetRequirements, ", "))
	case t.Skipped:
		return fmt.Sprintf("%s skipped ensure=%s runtime=%v provider=%s", rname, t.RequestedEnsure, t.Duration, t.Provider)
	case t.Changed:
		return fmt.Sprintf("%s changed ensure=%s runtime=%v provider=%s", rname, t.RequestedEnsure, t.Duration, t.Provider)
	case t.Refreshed:
		return fmt.Sprintf("%s refreshed ensure=%s runtime=%v provider=%s", rname, t.RequestedEnsure, t.Duration, t.Provider)
	default:
		return fmt.Sprintf("%s ensure=%s runtime=%v provider=%s", rname, t.RequestedEnsure, t.Duration, t.Provider)
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
	RequirementsUnMetCount   int           `json:"requirements_unmet_count" yaml:"requirements_unmet_count"`
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

		// Track requirements aren't met separately as they are considered stable
		summary.RequirementsUnMetCount += len(txEvent.UnmetRequirements)

		// Track refreshes separately (a resource can be both changed and refreshed)
		if txEvent.Refreshed {
			summary.RefreshedCount++
		}

		for _, hc := range txEvent.HealthChecks {
			summary.HealthCheckedCount++
			switch hc.Status {
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
	parts := []string{
		"resources=" + strconv.Itoa(s.TotalResources),
		"changed=" + strconv.Itoa(s.ChangedResources),
		"failed=" + strconv.Itoa(s.FailedResources),
		"skipped=" + strconv.Itoa(s.SkippedResources),
		"stable=" + strconv.Itoa(s.StableResources),
		"refreshed=" + strconv.Itoa(s.RefreshedCount),
	}

	if s.HealthCheckedCount > 0 {
		if s.HealthCheckCriticalCount > 0 {
			parts = append(parts, "health_critical="+strconv.Itoa(s.HealthCheckCriticalCount))
		}
		if s.HealthCheckWarningCount > 0 {
			parts = append(parts, "health_warning="+strconv.Itoa(s.HealthCheckWarningCount))
		}
		if s.HealthCheckOKCount > 0 {
			parts = append(parts, "health_ok="+strconv.Itoa(s.HealthCheckOKCount))
		}
		if s.HealthCheckUnknownCount > 0 {
			parts = append(parts, "health_unknown="+strconv.Itoa(s.HealthCheckUnknownCount))
		}
	}

	parts = append(parts, "duration="+s.TotalDuration.Round(time.Millisecond).String())

	return fmt.Sprintf("Session: %s", strings.Join(parts, ", "))
}

func (s *SessionSummary) RenderText(w io.Writer) {
	fmt.Fprintln(w, "Manifest Run Summary")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "             Run Time: %v\n", s.TotalDuration.Round(time.Millisecond))
	fmt.Fprintf(w, "      Total Resources: %d\n", s.TotalResources)
	fmt.Fprintf(w, "     Stable Resources: %d\n", s.StableResources)
	fmt.Fprintf(w, "    Changed Resources: %d\n", s.ChangedResources)
	fmt.Fprintf(w, "     Failed Resources: %d\n", s.FailedResources)
	fmt.Fprintf(w, "    Skipped Resources: %d\n", s.SkippedResources)
	fmt.Fprintf(w, "  Refreshed Resources: %d\n", s.RefreshedCount)
	fmt.Fprintf(w, "   Unmet Requirements: %d\n", s.RequirementsUnMetCount)
	if s.HealthCheckOKCount > 0 || s.HealthCheckWarningCount > 0 || s.HealthCheckCriticalCount > 0 || s.HealthCheckUnknownCount > 0 {
		fmt.Fprintf(w, "    Checked Resources: %d (ok: %d, critical: %d, warning: %d unknown: %d)\n", s.HealthCheckedCount, s.HealthCheckOKCount, s.HealthCheckCriticalCount, s.HealthCheckWarningCount, s.HealthCheckUnknownCount)
	} else {
		fmt.Fprintf(w, "    Checked Resources: %d\n", s.HealthCheckedCount)
	}
	fmt.Fprintf(w, "         Total Errors: %d\n", s.TotalErrors)
}
