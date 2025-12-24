// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"time"
)

type ExtendedExecOptions struct {
	Command     string
	Args        []string
	Cwd         string
	Environment []string
	Path        string
	Timeout     time.Duration
}

type CommandRunner interface {
	Execute(ctx context.Context, cmd string, args ...string) (stdout []byte, stderr []byte, exitCode int, err error)
	ExecuteWithOptions(ctx context.Context, opts ExtendedExecOptions) ([]byte, []byte, int, error)
}
