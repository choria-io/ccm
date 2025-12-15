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

	cmd.Env = []string{
		"PATH=/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin:/usr/local/sbin",
		"LANG=C",
		"LC_ALL=C",
	}

	stdout := bytes.NewBuffer([]byte{})
	stderr := bytes.NewBuffer([]byte{})

	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Dir = "/"

	err := cmd.Run()
	exitCode := cmd.ProcessState.ExitCode()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		// we specifically dont want to error when exit codes are >0 but we do want to return the exit code instead
		if exitCode > 0 {
			return stdout.Bytes(), stderr.Bytes(), exitCode, nil
		}

		return stdout.Bytes(), stderr.Bytes(), exitCode, err
	}

	if err != nil {
		return stdout.Bytes(), stderr.Bytes(), exitCode, err
	}

	return stdout.Bytes(), stderr.Bytes(), exitCode, nil
}
