+++
title = "Exec Type"
toc = true
weight = 20
description = "Exec resource for command execution"
+++

This document describes the design of the exec resource type for executing commands.

## Overview

The exec resource executes commands with idempotency controls:
- **Creates**: Skip execution if a file exists
- **OnlyIf / Unless**: Guard commands that gate execution based on exit code
- **Refresh Only**: Only execute when triggered by a subscribed resource
- **Exit Codes**: Validate success via configurable return codes

## Provider Interface

Exec providers must implement the `ExecProvider` interface:

```go
type ExecProvider interface {
    model.Provider

    Execute(ctx context.Context, properties *model.ExecResourceProperties, log model.Logger) (int, error)
    EvaluateGuard(ctx context.Context, command string, properties *model.ExecResourceProperties) (bool, error)
    Status(ctx context.Context, properties *model.ExecResourceProperties) (*model.ExecState, error)
}
```

### Method Responsibilities

| Method          | Responsibility                                                            |
|-----------------|---------------------------------------------------------------------------|
| `Status`        | Check if `creates` file exists, return current state                      |
| `Execute`       | Run the command, return exit code                                         |
| `EvaluateGuard` | Run a guard command, return `true` if it exits 0, `false` if non-zero    |

### Status Response

The `Status` method returns an `ExecState` containing:

```go
type ExecState struct {
    CommonResourceState

    ExitCode         *int // Exit code from last execution (nil if not run)
    CreatesSatisfied bool // Whether creates file exists
    OnlyIfSatisfied  bool // Whether onlyif guard command exited 0
    UnlessSatisfied  bool // Whether unless guard command exited 0
}
```

The `Ensure` field in `CommonResourceState` is set to:
- `present` if the `creates` file exists
- `absent` if the `creates` file does not exist (or not specified)

## Available Providers

| Provider | Execution Method       | Documentation   |
|----------|------------------------|-----------------|
| `posix`  | Direct exec (no shell) | [Posix](posix/) |
| `shell`  | Via `/bin/sh -c`       | [Shell](shell/) |

## Properties

| Property       | Type     | Description                                    |
|----------------|----------|------------------------------------------------|
| `command`      | string   | Command to run (defaults to `name` if not set) |
| `cwd`          | string   | Working directory for command execution        |
| `environment`  | []string | Additional environment variables (`KEY=value`) |
| `path`         | string   | Search path for executables (colon-separated)  |
| `returns`      | []int    | Acceptable exit codes (default: `[0]`)         |
| `timeout`      | string   | Maximum execution time (e.g., `30s`, `5m`)     |
| `creates`      | string   | File path; skip execution if exists            |
| `onlyif`       | string   | Guard command; exec runs only if it exits 0    |
| `unless`       | string   | Guard command; exec runs only if it exits non-zero |
| `refresh_only` | bool     | Only execute via subscribe refresh             |
| `subscribe`    | []string | Resources to watch for changes (`type#name`)   |
| `logoutput`    | bool     | Log command output                             |

## Apply Logic

```
┌─────────────────────────────────────────┐
│ Get current state via Status()          │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│ Evaluate guard commands (onlyif/unless) │
│ via EvaluateGuard()                     │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│ Check for subscribe refresh             │
└─────────────────┬───────────────────────┘
                  │
    ┌─────────────┴─────────────┐
    │ Subscribed resource       │
    │ changed?                  │
    └─────────────┬─────────────┘
              Yes │         No
                  ▼         │
          ┌───────────┐     │
          │ Execute   │     │
          └───────────┘     │
                            ▼
              ┌─────────────────────────────┐
              │ Is desired state met?       │
              │ (creates file exists OR     │
              │  onlyif guard failed OR     │
              │  unless guard succeeded OR  │
              │  refresh_only is true)      │
              └─────────────┬───────────────┘
                        Yes │         No
                            ▼         │
                    ┌───────────┐     │
                    │ Skip      │     │
                    └───────────┘     │
                                      ▼
                        ┌─────────────────────────┐
                        │ Is refresh_only = true? │
                        └─────────────┬───────────┘
                                  Yes │         No
                                      ▼         │
                              ┌───────────┐     │
                              │ Skip      │     │
                              └───────────┘     │
                                                ▼
                                        ┌───────────┐
                                        │ Execute   │
                                        └───────────┘
```

## Idempotency

The exec resource provides idempotency through several mechanisms:

### Creates Property

The `creates` property specifies a file that indicates successful prior execution:

```yaml
- exec:
    - extract-archive:
        command: tar xzf /tmp/app.tar.gz -C /opt
        creates: /opt/app/bin/app
```

**Behavior:**
- If `/opt/app/bin/app` exists, skip execution
- Useful for one-time setup commands
- Provider checks file existence via `Status()`

### Guard Commands (OnlyIf / Unless)

The `onlyif` and `unless` properties specify guard commands that control whether the exec runs:

```yaml
- exec:
    - install-app:
        command: /usr/local/bin/install-app.sh
        onlyif: test -f /tmp/app-package.tar.gz

    - configure-firewall:
        command: /usr/sbin/iptables -A INPUT -p tcp --dport 8080 -j ACCEPT
        unless: /usr/sbin/iptables -C INPUT -p tcp --dport 8080 -j ACCEPT
```

**Behavior:**
- `onlyif`: Exec runs only if the guard command exits 0
- `unless`: Exec runs only if the guard command exits non-zero
- Guard commands are evaluated via `EvaluateGuard()`, not inside `Status()`
- Guards share the exec's `cwd`, `environment`, `path`, and `timeout`
- Guards run even in noop mode to accurately report what would happen
- `creates` takes precedence: if the creates file exists, guards are not checked
- Subscribe-triggered refreshes override guards

**Error handling:**
- A non-zero exit code from a guard is not an error; it simply means the condition is not met
- An actual execution failure (command not found, permission denied) is propagated as an error

### Refresh Only Property

The `refresh_only` property limits execution to subscribe refreshes:

```yaml
- exec:
    - reload-nginx:
        command: systemctl reload nginx
        refresh_only: true
        subscribe:
          - file#/etc/nginx/nginx.conf
```

**Behavior:**
- Command only runs when subscribed resource changes
- Without a subscribe trigger, command is skipped
- Useful for reload/restart commands

### Decision Table

| Condition                            | Action  |
|--------------------------------------|---------|
| Subscribe triggered                  | Execute |
| `creates` file exists                | Skip    |
| `onlyif` guard exits non-zero        | Skip    |
| `unless` guard exits 0               | Skip    |
| `refresh_only: true` + no trigger    | Skip    |
| `refresh_only: false` + no guards    | Execute |

## Subscribe Behavior

Exec resources can subscribe to other resources and execute when they change:

```yaml
- file:
    - /etc/app/config.yaml:
        ensure: present
        content: "..."

- exec:
    - reload-app:
        command: systemctl reload app
        refresh_only: true
        subscribe:
          - file#/etc/app/config.yaml
```

Subscribe takes precedence over all other idempotency checks - if a subscribed resource changed, the command executes regardless of `creates` file existence or guard command results.

## Exit Code Validation

By default, exit code `0` indicates success. The `returns` property customizes acceptable codes:

```yaml
- exec:
    - check-status:
        command: /usr/local/bin/check-health
        returns:
          - 0
          - 1
          - 2
```

**Behavior:**
- Command succeeds if exit code is in `returns` list
- Command fails if exit code is not in `returns` list
- Used for desired state validation after execution

## Noop Mode

In noop mode, the exec type:
1. Queries current state normally (checks `creates` file)
2. Evaluates guard commands (`onlyif`/`unless`) - these run even in noop mode
3. Evaluates subscribe triggers
4. Logs what actions would be taken
5. Sets appropriate `NoopMessage`:
   - "Would have executed"
   - "Would have executed via subscribe"
6. Reports `Changed: true` if execution would occur
7. Does not call provider `Execute` method

## Desired State Validation

After execution (in non-noop mode), the type verifies success:

```go
func (t *Type) isDesiredState(properties, status) bool {
    // Creates file check takes precedence
    if properties.Creates != "" && status.CreatesSatisfied {
        return true
    }

    // Guard checks only apply before execution (ExitCode is nil)
    if status.ExitCode == nil {
        if properties.OnlyIf != "" && !status.OnlyIfSatisfied {
            return true // onlyif guard failed, don't run
        }
        if properties.Unless != "" && status.UnlessSatisfied {
            return true // unless guard succeeded, don't run
        }
    }

    // Refresh-only without execution is stable
    if status.ExitCode == nil && properties.RefreshOnly {
        return true
    }

    // Check exit code against acceptable returns
    returns := []int{0}
    if len(properties.Returns) > 0 {
        returns = properties.Returns
    }

    if status.ExitCode != nil {
        return slices.Contains(returns, *status.ExitCode)
    }

    return false
}
```

Guard checks are gated on `ExitCode == nil` because after execution, the exit code determines success. The post-execution `isDesiredState()` call must not re-evaluate guards, which would produce incorrect results since guard state is only set on `initialStatus`.

If the exit code is not in the acceptable `returns` list, an `ErrDesiredStateFailed` error is returned.

## Command vs Name

The `command` property is optional. If not specified, the `name` is used as the command:

```yaml
# These are equivalent:
- exec:
    - /usr/bin/myapp --config /etc/myapp.conf:

- exec:
    - run-myapp:
        command: /usr/bin/myapp --config /etc/myapp.conf
```

Using a descriptive `name` with explicit `command` is recommended for clarity.

## Environment and Path

Commands can be configured with custom environment:

```yaml
- exec:
    - build-app:
        command: make build
        cwd: /opt/app
        environment:
          - CC=gcc
          - CFLAGS=-O2
        path: /usr/local/bin:/usr/bin:/bin
```

**Environment:**
- Added to the command's environment
- Format: `KEY=value`
- Does not replace existing environment

**Path:**
- Sets the `PATH` for executable lookup
- Must be absolute directories
- Colon-separated list