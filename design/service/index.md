# Service Type

This document describes the design of the service resource type for managing system services.

## Overview

The service resource manages system services with two independent dimensions:
- **Running state**: Whether the service is currently running or stopped
- **Enabled state**: Whether the service starts automatically at boot

These are managed independently, allowing combinations like "running but disabled" or "stopped but enabled".

## Provider Interface

Service providers must implement the `ServiceProvider` interface:

```go
type ServiceProvider interface {
    model.Provider

    Enable(ctx context.Context, service string) error
    Disable(ctx context.Context, service string) error
    Start(ctx context.Context, service string) error
    Stop(ctx context.Context, service string) error
    Restart(ctx context.Context, service string) error
    Status(ctx context.Context, service string) (*model.ServiceState, error)
}
```

### Method Responsibilities

| Method    | Responsibility                           |
|-----------|------------------------------------------|
| `Status`  | Query current running and enabled state  |
| `Start`   | Start the service if not running         |
| `Stop`    | Stop the service if running              |
| `Restart` | Stop and start the service (for refresh) |
| `Enable`  | Configure service to start at boot       |
| `Disable` | Configure service to not start at boot   |

### Status Response

The `Status` method returns a `ServiceState` containing:

```go
type ServiceState struct {
    CommonResourceState
    Metadata *ServiceMetadata
}

type ServiceMetadata struct {
    Name     string  // Service name
    Provider string  // Provider name (e.g., "systemd")
    Enabled  bool    // Whether service starts at boot
    Running  bool    // Whether service is currently running
}
```

The `Ensure` field in `CommonResourceState` is set to:
- `running` if the service is active
- `stopped` if the service is inactive

## Available Providers

| Provider  | Init System | Documentation                            |
|-----------|-------------|------------------------------------------|
| `systemd` | systemd     | [Systemd](systemd/) |

## Ensure States

| Value     | Description                       |
|-----------|-----------------------------------|
| `running` | Service must be running (default) |
| `stopped` | Service must be stopped           |

If `ensure` is not specified, it defaults to `running`.

## Enable Property

The `enable` property is a boolean pointer (`*bool`) with three possible states:

| Value           | Behavior                              |
|-----------------|---------------------------------------|
| `true`          | Enable service to start at boot       |
| `false`         | Disable service from starting at boot |
| `nil` (not set) | Leave boot configuration unchanged    |

This allows managing running state without affecting boot configuration:

```yaml
# Start service but don't change boot config
- service:
    - myapp:
        ensure: running

# Start and enable at boot
- service:
    - myapp:
        ensure: running
        enable: true

# Stop and disable at boot
- service:
    - myapp:
        ensure: stopped
        enable: false
```

## Apply Logic

The service type applies changes in two phases:

### Phase 1: Running State

```
┌─────────────────────────────────────────┐
│ Check for subscribe refresh             │
└─────────────────┬───────────────────────┘
                  │
    ┌─────────────┴─────────────┐
    │ Subscribed resource       │
    │ changed?                  │
    └─────────────┬─────────────┘
              Yes │         │ No
                  ▼         │
    ┌─────────────────────┐ │
    │ ensure=running?     │ │
    │ already running?    │ │
    └─────────────┬───────┘ │
          Yes+Yes │         │
                  ▼         │
          ┌───────────┐     │
          │ Restart   │     │
          └───────────┘     │
                            ▼
              ┌─────────────────────────┐
              │ Compare ensure vs state │
              └─────────────┬───────────┘
                            │
    ┌───────────────────────┼───────────────────────┐
    │ ensure=stopped        │ ensure=running        │
    │ state=running         │ state=stopped         │
    ▼                       ▼                       │
┌────────┐            ┌────────┐                    │
│ Stop   │            │ Start  │                    │
└────────┘            └────────┘                    │
                                                    ▼
                                          ┌───────────────┐
                                          │ No change     │
                                          └───────────────┘
```

### Phase 2: Enabled State

After running state is handled, enabled state is processed:

```
┌─────────────────────────────────────────┐
│ enable property set?                    │
└─────────────────┬───────────────────────┘
                  │
          nil     │     true/false
          ▼       │
  ┌───────────┐   │
  │ No change │   │
  └───────────┘   │
                  ▼
    ┌─────────────────────────────┐
    │ Compare enable vs enabled   │
    └─────────────┬───────────────┘
                  │
    ┌─────────────┼─────────────┐
    │ enable=true │ enable=false│
    │ !enabled    │ enabled     │
    ▼             ▼             │
┌────────┐  ┌─────────┐         │
│ Enable │  │ Disable │         │
└────────┘  └─────────┘         │
                                ▼
                      ┌───────────────┐
                      │ No change     │
                      └───────────────┘
```

## Subscribe Behavior

Services can subscribe to other resources and restart when they change:

```yaml
- service:
    - httpd:
        ensure: running
        subscribe:
          - file#/etc/httpd/conf/httpd.conf
          - package#httpd
```

**Special Cases:**

| Condition                               | Behavior                       |
|-----------------------------------------|--------------------------------|
| `ensure: stopped`                       | Subscribe ignored (no restart) |
| Service not running + `ensure: running` | Start (not restart)            |
| Service running + `ensure: running`     | Restart                        |

This prevents restarting stopped services and ensures a clean start when the service should be running but isn't.

## Idempotency

The service resource is idempotent through state comparison:

| Desired           | Current  | Action  |
|-------------------|----------|---------|
| `ensure: running` | running  | None    |
| `ensure: running` | stopped  | Start   |
| `ensure: stopped` | stopped  | None    |
| `ensure: stopped` | running  | Stop    |
| `enable: true`    | enabled  | None    |
| `enable: true`    | disabled | Enable  |
| `enable: false`   | enabled  | Disable |
| `enable: false`   | disabled | None    |
| `enable: nil`     | any      | None    |

## Desired State Validation

After applying changes, the type verifies the service reached the desired state:

```go
func (t *Type) isDesiredState(properties, state) bool {
    // Check running state
    if properties.Ensure != state.Ensure {
        return false
    }

    // Check enabled state (only if explicitly set)
    if properties.Enable != nil {
        if *properties.Enable != state.Metadata.Enabled {
            return false
        }
    }

    return true
}
```

If the desired state is not reached, an `ErrDesiredStateFailed` error is returned.

## Service Name Validation

Service names are validated to prevent injection attacks:

**Allowed Characters:**
- Alphanumeric (`a-z`, `A-Z`, `0-9`)
- Period (`.`), underscore (`_`), plus (`+`)
- Colon (`:`), tilde (`~`), hyphen (`-`)

**Rejected:**
- Shell metacharacters (`;`, `|`, `&`, etc.)
- Whitespace
- Path separators

## Noop Mode

In noop mode, the service type:
1. Queries current state normally
2. Logs what actions would be taken
3. Sets appropriate `NoopMessage` (e.g., "Would have started", "Would have enabled")
4. Reports `Changed: true` if changes would occur
5. Does not call provider Start/Stop/Restart/Enable/Disable methods