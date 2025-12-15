// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"github.com/choria-io/ccm/model"
	"github.com/sirupsen/logrus"
)

var _ model.Logger = (*LogrusLogger)(nil)

type LogrusLogger struct {
	log *logrus.Entry
}

func (s *LogrusLogger) genFields(args ...any) logrus.Fields {
	fields := logrus.Fields{}
	for i := 0; i < len(args); i += 2 {
		fields[args[i].(string)] = args[i+1]
	}
	return fields
}

func (s *LogrusLogger) Debug(msg string, args ...any) {
	s.log.WithFields(s.genFields(args...)).Debug(msg)
}

func (s *LogrusLogger) Info(msg string, args ...any) {
	s.log.WithFields(s.genFields(args...)).Info(msg)
}

func (s *LogrusLogger) Warn(msg string, args ...any) {
	s.log.WithFields(s.genFields(args...)).Warn(msg)
}

func (s *LogrusLogger) Error(msg string, args ...any) {
	s.log.WithFields(s.genFields(args...)).Error(msg)
}

func (s *LogrusLogger) With(args ...any) model.Logger {
	return NewLogrusLogger(s.log.WithFields(s.genFields(args...)))
}

func NewLogrusLogger(log *logrus.Entry) *LogrusLogger {
	return &LogrusLogger{log: log}
}
