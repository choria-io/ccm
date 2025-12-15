// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"log/slog"

	"github.com/choria-io/ccm/model"
)

var _ model.Logger = (*SlogLogger)(nil)

type SlogLogger struct {
	log *slog.Logger
}

func (s *SlogLogger) Debug(msg string, args ...any) {
	s.log.Debug(msg, args...)
}

func (s *SlogLogger) Info(msg string, args ...any) {
	s.log.Info(msg, args...)
}

func (s *SlogLogger) Warn(msg string, args ...any) {
	s.log.Warn(msg, args...)
}

func (s *SlogLogger) Error(msg string, args ...any) {
	s.log.Error(msg, args...)
}

func (s *SlogLogger) With(args ...any) model.Logger {
	return NewSlogLogger(s.log.With(args...))
}

func NewSlogLogger(log *slog.Logger) *SlogLogger {
	return &SlogLogger{log: log}
}
