// Copyright (c) 2025-2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"fmt"
	"time"

	"github.com/segmentio/ksuid"
)

type Apply interface {
	Resources() []map[string]any
	Data() map[string]any
}

type SessionStore interface {
	RecordEvent(TransactionEvent)
	ResetSession(manifest Apply)
}

const TransactionEventProtocol = "io.choria.ccm.v1.transaction.event"

// TransactionEvent represents a single event for a resource session
type TransactionEvent struct {
	Protocol     string        `json:"protocol" yaml:"protocol"`
	EventID      string        `json:"event_id" yaml:"event_id"`
	TimeStamp    time.Time     `json:"timestamp" yaml:"timestamp"`
	ResourceType string        `json:"resourcetype" yaml:"resourcetype"`
	Provider     string        `json:"provider" yaml:"provider"`
	Name         string        `json:"name" yaml:"name"`
	Changed      bool          `json:"changed" yaml:"changed"`
	Failed       bool          `json:"failed" yaml:"failed"`
	Ensure       string        `json:"ensure" yaml:"ensure"`               // Ensure is the requested ensure value
	ActualEnsure string        `json:"actual_ensure" yaml:"actual_ensure"` // ActualEnsure is the actual `ensure` value after the session
	Error        string        `json:"error" yaml:"error"`
	Skipped      bool          `json:"skipped" yaml:"skipped"`
	Duration     time.Duration `json:"duration" yaml:"duration"`
	Properties   any           `json:"properties" yaml:"properties"`
	Status       any           `json:"status" yaml:"status"`
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

func (t *TransactionEvent) LogStatus(log Logger) {
	switch {
	case t.Failed:
		log.Error(fmt.Sprintf("%s#%s failed", t.ResourceType, t.Name), "ensure", t.Ensure, "runtime", t.Duration, "error", t.Error, "provider", t.Provider)
	case t.Skipped:
		log.Warn(fmt.Sprintf("%s#%s skipped", t.ResourceType, t.Name), "ensure", t.Ensure, "runtime", t.Duration, "provider", t.Provider)
	case t.Changed:
		log.Warn(fmt.Sprintf("%s#%s changed", t.ResourceType, t.Name), "ensure", t.Ensure, "runtime", t.Duration, "provider", t.Provider)
	default:
		log.Info(fmt.Sprintf("%s#%s stable", t.ResourceType, t.Name), "ensure", t.Ensure, "runtime", t.Duration, "provider", t.Provider)
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
