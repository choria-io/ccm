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

func (c *CommandRunner) ExecuteWithOptions(ctx context.Context, opts model.ExtendedExecOptions) ([]byte, []byte, int, error) {
	if opts.Command == "" {
		return nil, nil, 0, errors.New("command not specified")
	}

	logOpts := []any{
		"command", opts.Command, "args", opts.Args,
	}
	if opts.Cwd != "" {
		logOpts = append(logOpts, "cwd", opts.Cwd)
	}

	c.logger.Debug("Running command", logOpts...)

	toCtx := ctx
	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		toCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(toCtx, opts.Command, opts.Args...)
	if cancel != nil {
		cmd.Cancel = func() error { cancel(); return nil }
	}

	cmd.Env = []string{
		"PATH=/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin:/usr/local/sbin",
		"LANG=C",
		"LC_ALL=C",
	}
	cmd.Env = append(cmd.Env, opts.Environment...)

	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	} else {
		cmd.Dir = "/"
	}

	if opts.Path != "" {
		cmd.Path = opts.Path
	}

	stdout := bytes.NewBuffer([]byte{})
	stderr := bytes.NewBuffer([]byte{})

	cmd.Stdout = stdout
	cmd.Stderr = stderr

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

// Execute runs a command with the given arguments and returns stdout, stderr, exit code, and any error
func (c *CommandRunner) Execute(ctx context.Context, command string, args ...string) ([]byte, []byte, int, error) {
	return c.ExecuteWithOptions(ctx, model.ExtendedExecOptions{Command: command, Args: args})
}
