// Copyright (c) 2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package choriascaffold

import (
	"fmt"

	"github.com/choria-io/ccm/model"
)

type logger struct {
	log model.Logger
}

func (l *logger) Debugf(format string, v ...any) {
	l.log.Debug(fmt.Sprintf(format, v...))
}

func (l *logger) Infof(format string, v ...any) {
	l.log.Info(fmt.Sprintf(format, v...))
}
