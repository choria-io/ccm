# Systemd Provider

This document describes the implementation details of the Systemd service provider for managing system services via `systemctl`.

## Provider Selection

The Systemd provider is selected when `systemctl` is found in the system PATH. The provider checks for the executable using `util.ExecutableInPath("systemctl")`.

**Availability Check:**
- Searches PATH for `systemctl`
- Returns priority 1 if found
- Returns unavailable if not found

## Concurrency

A global service lock (`model.ServiceGlobalLock`) is held during all `systemctl` command executions to prevent concurrent systemd operations within the same process. This prevents race conditions when multiple service resources are managed simultaneously.

```go
func (p *Provider) execute(ctx context.Context, cmd string, args ...string) (...) {
    model.ServiceGlobalLock.Lock()
    defer model.ServiceGlobalLock.Unlock()
    return p.runner.Execute(ctx, cmd, args...)
}
```

## Daemon Reload

The provider performs a `systemctl daemon-reload` once per provider instance before any service operations. This ensures systemd picks up any unit file changes made by other resources (e.g., file resources managing unit files).

```go
func (p *Provider) maybeReload(ctx context.Context) error {
    p.mu.Lock()
    defer p.mu.Unlock()

    if p.didReload {
        return nil
    }

    _, _, _, err := p.execute(ctx, "systemctl", "daemon-reload")
    return err
}
```

The reload is performed only once, tracked by the `didReload` flag.

## Operations

### Status

**Commands:**
```
systemctl is-active --system <service>
systemctl is-enabled --system <service>
```

**Active State Detection:**

| `is-active` Output | Interpreted As |
|--------------------|----------------|
| `active`           | Running        |
| `inactive`         | Stopped        |
| `failed`           | Stopped        |
| `activating`       | Stopped        |
| Other              | Error          |

**Enabled State Detection:**

| `is-enabled` Output | Interpreted As           |
|---------------------|--------------------------|
| `enabled`           | Enabled                  |
| `enabled-runtime`   | Enabled                  |
| `alias`             | Enabled                  |
| `static`            | Enabled                  |
| `indirect`          | Enabled                  |
| `generated`         | Enabled                  |
| `transient`         | Enabled                  |
| `linked`            | Disabled                 |
| `linked-runtime`    | Disabled                 |
| `masked`            | Disabled                 |
| `masked-runtime`    | Disabled                 |
| `disabled`          | Disabled                 |
| `not-found`         | Error: service not found |

**Returned State:**

| Field               | Value                                     |
|---------------------|-------------------------------------------|
| `Ensure`            | `running` or `stopped` based on is-active |
| `Metadata.Enabled`  | Boolean from is-enabled                   |
| `Metadata.Running`  | Boolean from is-active                    |
| `Metadata.Provider` | "systemd"                                 |

### Start

**Command:**
```
systemctl start --system <service>
```

Called when `ensure: running` and service is currently stopped.

### Stop

**Command:**
```
systemctl stop --system <service>
```

Called when `ensure: stopped` and service is currently running.

### Restart

**Command:**
```
systemctl restart --system <service>
```

Called when a subscribed resource has changed and the service should be refreshed.

### Enable

**Command:**
```
systemctl enable --system <service>
```

Called when `enable: true` and service is currently disabled.

### Disable

**Command:**
```
systemctl disable --system <service>
```

Called when `enable: false` and service is currently enabled.

## Command Flags

All commands use the `--system` flag to explicitly target system-level units (as opposed to user-level units managed with `--user`).

## Decision Flow

### Ensure State (Running/Stopped)

```
┌─────────────────────────────────────────┐
│ Subscribe triggered?                     │
│ (subscribed resource changed)            │
└─────────────┬───────────────┬───────────┘
              │ Yes           │ No
              │               │
              │               ▼
              │       ┌───────────────────┐
              │       │ Ensure = stopped? │
              │       └─────────┬─────────┘
              │           Yes   │   No
              │           ▼     ▼
              │   ┌─────────┐ ┌───────────────────┐
              │   │ Running?│ │ Ensure = running? │
              │   └────┬────┘ └─────────┬─────────┘
              │    Yes │ No         Yes │   No
              │        ▼               ▼
              │   ┌────────┐    ┌─────────┐
              │   │ Stop   │    │ Running?│
              │   └────────┘    └────┬────┘
              │                  No  │ Yes
              │                      ▼
              │                 ┌────────┐
              │                 │ Start  │
              │                 └────────┘
              │
              ▼
      ┌───────────────────────────────┐
      │ Ensure = running?             │
      │ (only restart running services)│
      └───────────────┬───────────────┘
                  Yes │   No
                      ▼
              ┌─────────────┐
              │ Restart     │
              └─────────────┘
```

**Subscribe Behavior Notes:**
- Restart only occurs if `ensure: running`
- If service is stopped and `ensure: running`, it starts instead of restarting
- Subscribe is ignored if `ensure: stopped`

### Enable State

Enable/disable is processed independently after ensure state:

```
┌─────────────────────────────────────────┐
│ Enable property set?                     │
└─────────────┬───────────────┬───────────┘
              │ Yes           │ No (nil)
              │               │
              │               ▼
              │       ┌───────────────┐
              │       │ No change     │
              │       └───────────────┘
              ▼
      ┌───────────────────────────────┐
      │ Enable = true?                 │
      └───────────────┬───────────────┘
                  Yes │   No
                      ▼
      ┌───────────────────────────────┐
      │ Currently enabled?             │
      └───────────────┬───────────────┘
                  No  │ Yes
                      ▼
              ┌─────────────┐
              │ Enable      │
              └─────────────┘

      (Similar flow for disable when enable=false)
```

## Idempotency

The service resource checks current state before making changes:

| Desired State   | Current State | Action  |
|-----------------|---------------|---------|
| `running`       | `running`     | None    |
| `running`       | `stopped`     | Start   |
| `stopped`       | `stopped`     | None    |
| `stopped`       | `running`     | Stop    |
| `enable: true`  | enabled       | None    |
| `enable: true`  | disabled      | Enable  |
| `enable: false` | enabled       | Disable |
| `enable: false` | disabled      | None    |
| `enable: nil`   | any           | None    |

## Service Name Validation

Service names are validated to prevent shell injection:

**Dangerous Characters Check:**
```go
if dangerousCharsRegex.MatchString(p.Name) {
    return fmt.Errorf("service name contains dangerous characters: %q", p.Name)
}
```

**Allowed Characters:**
- Alphanumeric (`a-z`, `A-Z`, `0-9`)
- Period (`.`)
- Underscore (`_`)
- Plus (`+`)
- Colon (`:`)
- Tilde (`~`)
- Hyphen (`-`)

**Examples:**

| Name            | Valid                     |
|-----------------|---------------------------|
| `httpd`         | Yes                       |
| `nginx.service` | Yes                       |
| `my-app_v2`     | Yes                       |
| `app@instance`  | No (@ not allowed)        |
| `app; rm -rf /` | No (shell metacharacters) |

## Subscribe and Refresh

Services can subscribe to other resources and restart when they change:

```yaml
- file:
    - /etc/myapp/config.yaml:
        ensure: present
        content: "..."
        owner: root
        group: root
        mode: "0644"

- service:
    - myapp:
        ensure: running
        enable: true
        subscribe:
          - file#/etc/myapp/config.yaml
```

**Behavior:**
- When the file resource changes, the service is restarted
- Restart only occurs if `ensure: running`
- If service was stopped and should be running, it starts (not restarts)

## Error Handling

| Condition                   | Behavior                                     |
|-----------------------------|----------------------------------------------|
| `systemctl` not in PATH     | Provider unavailable                         |
| Service not found           | Error from `is-enabled`: "service not found" |
| Unknown `is-active` output  | Error: "invalid systemctl is-active output"  |
| Unknown `is-enabled` output | Error: "invalid systemctl is-enabled output" |
| Command execution failure   | Error propagated from runner                 |

## Platform Support

The Systemd provider requires:
- Linux with systemd as init system
- `systemctl` command available in PATH

It does not support:
- Non-systemd init systems (SysVinit, Upstart, OpenRC)
- User-level units (uses `--system` flag)
- Windows, macOS, or BSD systems