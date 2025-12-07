// Copyright (c) 2025, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmdrunner

import (
	"bytes"
	"context"
	"errors"
	"os/exec"

	"github.com/choria-io/ccm/model"
)

// CommandRunner executes system commands and captures their output
type CommandRunner struct {
	logger model.Logger
}

// NewCommandRunner creates a new CommandRunner instance with the provided logger
func NewCommandRunner(log model.Logger) (*CommandRunner, error) {
	return &CommandRunner{logger: log}, nil
}

// Execute runs a command with the given arguments and returns stdout, stderr, exit code, and any error
func (c *CommandRunner) Execute(ctx context.Context, command string, args ...string) ([]byte, []byte, int, error) {
	c.logger.Debug("Running command", "command", command, "args", args)
	cmd := exec.CommandContext(ctx, command, args...)

	stdout := bytes.NewBuffer([]byte{})
	stderr := bytes.NewBuffer([]byte{})

	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Dir = "/"

	var exitCode int

	err := cmd.Run()
	if errors.Is(err, &exec.ExitError{}) {
		return stdout.Bytes(), stderr.Bytes(), cmd.ProcessState.ExitCode(), err
	} else if err != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	return stdout.Bytes(), stderr.Bytes(), exitCode, nil
}
