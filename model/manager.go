// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"encoding/json"

	"github.com/choria-io/ccm/templates"
)

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(args ...any) Logger
}

type Manager interface {
	FactsRaw(ctx context.Context) (json.RawMessage, error)
	Facts(ctx context.Context) (map[string]any, error)
	Data() map[string]any
	SetData(data map[string]any) map[string]any
	Logger(args ...any) (Logger, error)
	NewRunner() (CommandRunner, error)
	RecordEvent(event *TransactionEvent) error
	ShouldRefresh(resourceType string, resourceName string) (bool, error)
	TemplateEnvironment(ctx context.Context) (*templates.Env, error)
	WorkingDirectory() string
	StartSession(Apply) (SessionStore, error)
	ResourceInfo(ctx context.Context, typeName, name string) (any, error)
	SessionSummary() (*SessionSummary, error)
	NoopMode() bool
}
