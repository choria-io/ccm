// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/choria-io/ccm/model"
	"github.com/kballard/go-shellquote"
)

var (
	ErrCommandNotSpecified = errors.New("command not specified")
)

// ParseNagiosExitCode converts a Nagios exit code to a HealthCheckResult with appropriate status
func ParseNagiosExitCode(exitCode int, output string) *model.HealthCheckResult {
	result := &model.HealthCheckResult{
		Output: strings.TrimSpace(output),
	}

	switch exitCode {
	case 0:
		result.Status = model.HealthCheckOK
	case 1:
		result.Status = model.HealthCheckWarning
	case 2:
		result.Status = model.HealthCheckCritical
	default:
		result.Status = model.HealthCheckUnknown
	}

	return result
}

// Execute runs a health check command and returns an error if the check fails.
// If Tries > 1, the check will be retried up to Tries times with ParseTrySleep
// delay between attempts until the check passes (returns OK status).
func Execute(ctx context.Context, mgr model.Manager, hc *model.CommonHealthCheck, log model.Logger) (*model.HealthCheckResult, error) {
	if hc.Command == "" {
		return nil, ErrCommandNotSpecified
	}

	runner, err := mgr.NewRunner()
	if err != nil {
		return nil, err
	}

	cmd, err := shellquote.Split(hc.Command)
	if err != nil {
		return nil, err
	}
	var args []string
	if len(cmd) > 1 {
		args = cmd[1:]
	}

	tries := hc.Tries
	if tries < 1 {
		tries = 1
	}

	var result *model.HealthCheckResult

	for attempt := 1; attempt <= tries; attempt++ {
		// Check if context is canceled before attempting
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		log.Info("Executing health check command", "command", cmd[0], "args", strings.Join(args, " "), "try", attempt)

		execCtx := ctx
		var cancel context.CancelFunc
		if hc.ParsedTimeout > 0 {
			execCtx, cancel = context.WithTimeout(ctx, hc.ParsedTimeout)
		}

		out, _, exitCode, err := runner.Execute(execCtx, cmd[0], args...)
		if cancel != nil {
			cancel()
		}

		if err != nil {
			// On execution error, return immediately without retry
			return nil, err
		}

		result = ParseNagiosExitCode(exitCode, string(out))
		result.Tries = attempt

		// If check passed, return immediately
		if result.Status == model.HealthCheckOK {
			return result, nil
		}

		// If this was the last attempt, return the result
		if attempt >= tries {
			break
		}

		sleep := hc.ParseTrySleep
		if sleep == 0 {
			sleep = time.Second
		}

		// Sleep before next retry if configured
		if hc.ParseTrySleep > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(hc.ParseTrySleep):
			}
		}
	}

	return result, nil
}
