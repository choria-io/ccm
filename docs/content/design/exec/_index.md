+++
title = "Exec Type"
toc = true
weight = 20
+++

This document describes the design of the exec resource type for executing commands.

## Overview

The exec resource executes commands with idempotency controls:
- **Creates**: Skip execution if a file exists
- **Refresh Only**: Only execute when triggered by a subscribed resource
- **Exit Codes**: Validate success via configurable return codes

## Provider Interface

Exec providers must implement the `ExecProvider` interface:

```go
type ExecProvider interface {
    model.Provider

    Execute(ctx context.Context, properties *model.ExecResourceProperties, log model.Logger) (int, error)
    Status(ctx context.Context, properties *model.ExecResourceProperties) (*model.ExecState, error)
}
```

### Method Responsibilities

| Method    | Responsibility                                       |
|-----------|------------------------------------------------------|
| `Status`  | Check if `creates` file exists, return current state |
| `Execute` | Run the command, return exit code                    |

### Status Response

The `Status` method returns an `ExecState` containing:

```go
type ExecState struct {
    CommonResourceState

    ExitCode         *int // Exit code from last execution (nil if not run)
    CreatesSatisfied bool // Whether creates file exists
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
              ┌─────────────────────────┐
              │ Is desired state met?   │
              │ (creates file exists OR │
              │  refresh_only is true)  │
              └─────────────┬───────────┘
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

The exec resource provides idempotency through two mechanisms:

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
| `refresh_only: true` + no trigger    | Skip    |
| `refresh_only: false` + no `creates` | Execute |

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

Subscribe takes precedence over other idempotency checks - if a subscribed resource changed, the command executes regardless of `creates` file existence.

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
2. Evaluates subscribe triggers
3. Logs what actions would be taken
4. Sets appropriate `NoopMessage`:
   - "Would have executed"
   - "Would have executed via subscribe"
5. Reports `Changed: true` if execution would occur
6. Does not call provider `Execute` method

## Desired State Validation

After execution (in non-noop mode), the type verifies success:

```go
func (t *Type) isDesiredState(properties, status) bool {
    // Creates file check takes precedence
    if properties.Creates != "" && status.CreatesSatisfied {
        return true
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