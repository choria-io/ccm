+++
title = "Resources"
toc = true
weight = 10
pre = "<b>1. </b>"
+++

Resources describe the desired state of your infrastructure. Each resource represents something to manage and is backed by a provider that implements platform-specific management logic.

Every resource has a type, a unique name, and resource-specific properties.

## Common Properties

All resources support the following common properties:

| Property        | Description                                                                 |
|-----------------|-----------------------------------------------------------------------------|
| `name`          | Unique identifier for the resource                                          |
| `ensure`        | Desired state (values vary by resource type)                                |
| `alias`         | Alternative name for use in `subscribe`, `require`, and logging             |
| `provider`      | Force a specific provider                                                   |
| `require`       | List of resources (`type#name` or `type#alias`) that must succeed first     |
| `health_checks` | Health checks to run after applying (see [Monitoring](../monitoring/))      |
| `control`       | Conditional execution rules (see below)                                     |

## Conditional Resource Execution

Resources can be conditionally executed using a `control` section and expressions that should resolve to boolean values.

```
package:
  name: zsh
  ensure: 5.9
  control:
    if: lookup("facts.host.info.os") == "linux"
    unless: lookup("facts.host.info.virtualizationSystem") == "docker"
```

Here we install `zsh` on all `linux` machines unless they are running inside a `docker` container.

The following table shows how the two conditions interact:

| `if`      | `unless`  | Resource Managed? |
|-----------|-----------|-------------------|
| (not set) | (not set) | Yes               |
| `true`    | (not set) | Yes               |
| `false`   | (not set) | No                |
| (not set) | `true`    | No                |
| (not set) | `false`   | Yes               |
| `true`    | `true`    | No                |
| `true`    | `false`   | Yes               |
| `false`   | `true`    | No                |
| `false`   | `false`   | No                |

## About Names

Resources can be specified like:

```yaml
/etc/motd:
  ensure: present
```

This sets `name` to `/etc/motd`, in the following paragraphs we will refer to this as `name`.

## Exec

The exec resource executes commands to bring the system into the desired state. It is idempotent when used with the `creates` property or `refreshonly` mode.

> [!info] Warning
> Specify commands with their full path, or use the `path` property to set the search path.

In a manifest:

```yaml
- exec:
    - /usr/bin/touch /tmp/hello:
        creates: /tmp/hello
        timeout: 30s
        cwd: /tmp
```

Alternatively for long commands or to improve UX for referencing execs in `require` or `subscribe`:

```yaml
- exec:
    - touch_hello:
        command: /usr/bin/touch /tmp/hello
        creates: /tmp/hello
        timeout: 30s
        cwd: /tmp
```

On the CLI:

```nohighlight
$ ccm ensure exec "/usr/bin/touch /tmp/hello" --creates /tmp/hello --timeout 30s
```

The command runs only if `/tmp/hello` does not exist.

### Properties

| Property                | Description                                                                                   |
|-------------------------|-----------------------------------------------------------------------------------------------|
| `name`                  | The command to execute (used as the resource identifier)                                      |
| `command`               | Alternative command to run instead of `name`                                                  |
| `cwd`                   | Working directory for command execution                                                       |
| `environment` (array)   | Environment variables in `KEY=VALUE` format                                                   |
| `path`                  | Search path for executables as a colon-separated list (e.g., `/usr/bin:/bin`)                 |
| `returns` (array)       | Exit codes indicating success (default: `[0]`)                                                |
| `timeout`               | Maximum execution time (e.g., `30s`, `5m`); command is killed if exceeded                     |
| `creates`               | File path; if this file exists, the command does not run                                      |
| `refreshonly` (boolean) | Only run when notified by a subscribed resource                                               |
| `subscribe` (array)     | Resources to subscribe to for refresh notifications (`type#name` or `type#alias`)             |
| `logoutput` (boolean)   | Log the command output                                                                        |
| `provider`              | Force a specific provider (`posix` only)                                                      |

## File

The file resource manages files and directories, including their content, ownership, and permissions.

> [!info] Warning
> Use absolute file paths and primary group names.

In a manifest:

```yaml
- file:
    - /etc/motd:
        ensure: present
        content: |
          Managed by CCM {{ now() }}
        owner: root
        group: root
        mode: "0644"
```

On the CLI:

```nohighlight
$ ccm ensure file /etc/motd --source /tmp/ccm/motd --owner root --group root --mode 0644
```

This copies the contents of `/tmp/ccm/motd` to `/etc/motd` verbatim and sets ownership.

Use `--content` or `--content-file` to parse content through the template engine before writing.

### Ensure Values

| Value       | Description                    |
|-------------|--------------------------------|
| `present`   | The file must exist            |
| `absent`    | The file must not exist        |
| `directory` | The path must be a directory   |

### Properties

| Property   | Description                                                    |
|------------|----------------------------------------------------------------|
| `name`     | Absolute path to the file                                      |
| `ensure`   | Desired state (`present`, `absent`, `directory`)               |
| `content`  | File contents, parsed through the template engine              |
| `source`   | Copy contents from another local file                          |
| `owner`    | File owner (username)                                          |
| `group`    | File group (group name)                                        |
| `mode`     | File permissions in octal notation (e.g., `"0644"`)            |
| `provider` | Force a specific provider (`posix` only)                       |

## Package

The package resource manages system packages. Specify whether the package should be present, absent, at the latest version, or at a specific version.

> [!info] Warning
> Use real package names, not virtual names, aliases, or group names.

In a manifest:

```yaml
- package:
    - zsh:
        ensure: "5.9"
```

On the CLI:

```nohighlight
$ ccm ensure package zsh 5.9
```

### Ensure Values

| Value       | Description                                     |
|-------------|-------------------------------------------------|
| `present`   | The package must be installed                   |
| `latest`    | The package must be installed at latest version |
| `absent`    | The package must not be installed               |
| `<version>` | The package must be installed at this version   |

### Properties

| Property   | Description                                |
|------------|--------------------------------------------|
| `name`     | Package name                               |
| `ensure`   | Desired state or version                   |
| `provider` | Force a specific provider (`dnf` only)     |

## Service

The service resource manages system services. Services have two independent properties: whether they are running and whether they are enabled to start at boot.

> [!info] Warning
> Use real service names, not virtual names or aliases.

Services can subscribe to other resources and restart when those resources change.

In a manifest:

```yaml
- service:
    - httpd:
        ensure: running
        enable: true
        subscribe:
          - package#httpd
```

On the CLI:

```nohighlight
$ ccm ensure service httpd running --enable --subscribe package#httpd
```

### Ensure Values

| Value     | Description                  |
|-----------|------------------------------|
| `running` | The service must be running  |
| `stopped` | The service must be stopped  |

If `ensure` is not specified, it defaults to `running`.

### Properties

| Property            | Description                                                                            |
|---------------------|----------------------------------------------------------------------------------------|
| `name`              | Service name                                                                           |
| `ensure`            | Desired state (`running` or `stopped`; default: `running`)                             |
| `enable` (boolean)  | Enable the service to start at boot                                                    |
| `subscribe` (array) | Resources to watch; restart the service when they change (`type#name` or `type#alias`) |
| `provider`          | Force a specific provider (`systemd` only)                                             |
