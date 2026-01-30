# Resources

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

{{< tabs >}}
{{% tab title="Manifest" %}}
```yaml
package:
  name: zsh
  ensure: 5.9
  control:
    if: lookup("facts.host.info.os") == "linux"
    unless: lookup("facts.host.info.virtualizationSystem") == "docker"
```
{{% /tab %}}
{{% tab title="CLI" %}}
```nohighlight
ccm ensure package zsh 5.9 \
    --if "lookup('facts.host.info.os') == 'linux'"
    --unless "lookup('facts.host.info.virtualizationSystem') == 'docker'"
```
{{% /tab %}}
{{% tab title="API Request" %}}
```json
{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "package",
  "properties": {
    "name": "zsh",
    "ensure": "5.9",
    "control": {
      "if": "lookup(\"facts.host.info.os\") == \"linux\"",
      "unless": "lookup(\"facts.host.info.virtualizationSystem\") == \"docker\""
    }
  }
}
```
{{% /tab %}}
{{< /tabs >}}


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

## Archive

The archive resource downloads and extracts archives from HTTP/HTTPS URLs. It supports tar.gz, tgz, tar, and zip formats.

> [!info] Note
> The archive file path (`name`) must have the same archive type extension as the URL. For example, if the URL ends in `.tar.gz`, the name must also end in `.tar.gz`.

{{< tabs >}}
{{% tab title="Manifest" %}}
```yaml
- archive:
    - /opt/downloads/app-v1.2.3.tar.gz:
        url: https://releases.example.com/app/v1.2.3/app-v1.2.3.tar.gz
        checksum: "a1b2c3d4e5f6..."
        extract_parent: /opt/app
        creates: /opt/app/bin/app
        owner: app
        group: app
        cleanup: true
```
{{% /tab %}}
{{% tab title="CLI" %}}
```nohighlight
ccm ensure archive /opt/downloads/app.tar.gz \
    --url https://releases.example.com/app.tar.gz \
    --extract-parent /opt/app \
    --creates /opt/app/bin/app \
    --owner root --group root
```
{{% /tab %}}
{{% tab title="API Request" %}}
```json
{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "archive",
  "properties": {
    "name": "/opt/downloads/app-v1.2.3.tar.gz",
    "url": "https://releases.example.com/app/v1.2.3/app-v1.2.3.tar.gz",
    "checksum": "a1b2c3d4e5f6...",
    "extract_parent": "/opt/app",
    "creates": "/opt/app/bin/app",
    "owner": "app",
    "group": "app",
    "cleanup": true
  }
}
```
{{% /tab %}}
{{< /tabs >}}


This downloads the archive, extracts it to `/opt/app`, and removes the archive file after extraction. Future runs skip the download if `/opt/app/bin/app` exists.

### Ensure Values

| Value     | Description                       |
|-----------|-----------------------------------|
| `present` | The archive must be downloaded    |
| `absent`  | The archive file must not exist   |

### Properties

| Property         | Description                                                                                   |
|------------------|-----------------------------------------------------------------------------------------------|
| `name`           | Absolute path where the archive will be saved                                                 |
| `url`            | HTTP/HTTPS URL to download the archive from                                                   |
| `checksum`       | Expected SHA256 checksum of the downloaded file                                               |
| `extract_parent` | Directory to extract the archive contents into                                                |
| `creates`        | File path; if this file exists, the archive is not downloaded or extracted                    |
| `cleanup`        | Remove the archive file after successful extraction (requires `extract_parent` and `creates`) |
| `owner`          | Owner of the downloaded archive file (username)                                               |
| `group`          | Group of the downloaded archive file (group name)                                             |
| `username`       | Username for HTTP Basic Authentication                                                        |
| `password`       | Password for HTTP Basic Authentication                                                        |
| `headers`        | Additional HTTP headers to send with the request (map of header name to value)                |
| `provider`       | Force a specific provider (`http` only)                                                       |

### Authentication

The archive resource supports two authentication methods:

**Basic Authentication:**

{{< tabs >}}
{{% tab title="Manifest" %}}
```yaml
- archive:
    - /opt/downloads/private-app.tar.gz:
        url: https://private.example.com/app.tar.gz
        username: deploy
        password: "{{ lookup('data.deploy_password') }}"
        extract_parent: /opt/app
        owner: root
        group: root
```
{{% /tab %}}
{{% tab title="CLI" %}}
```nohighlight
ccm ensure archive /opt/downloads/private-app.tar.gz \
    --url https://private.example.com/app.tar.gz \
    --username deploy \
    --password "{{ lookup('data.deploy_password') }}"
```
{{% /tab %}}
{{% tab title="API Request" %}}
```json
{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "archive",
  "properties": {
    "name": "/opt/downloads/private-app.tar.gz",
    "url": "https://private.example.com/app.tar.gz",
    "username": "deploy",
    "password": "secret",
    "extract_parent": "/opt/app",
    "owner": "root",
    "group": "root"
  }
}
```
{{% /tab %}}
{{< /tabs >}}

**Custom Headers:**

{{< tabs >}}
{{% tab title="Manifest" %}}
```yaml
- archive:
    - /opt/downloads/app.tar.gz:
        url: https://api.example.com/releases/app.tar.gz
        headers:
          Authorization: "Bearer {{ lookup('data.api_token') }}"
          X-Custom-Header: custom-value
        extract_parent: /opt/app
        owner: root
        group: root
```
{{% /tab %}}
{{% tab title="CLI" %}}
```nohighlight
ccm ensure archive /opt/downloads/app.tar.gz \
    --url https://api.example.com/releases/app.tar.gz \
    --headers "Authorization:{{ lookup('data.api_token') }}"
```
{{% /tab %}}
{{% tab title="API Request" %}}
```json
{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "archive",
  "properties": {
    "name": "/opt/downloads/app.tar.gz",
    "url": "https://api.example.com/releases/app.tar.gz",
    "headers": {
      "Authorization": "Bearer mytoken",
      "X-Custom-Header": "custom-value"
    },
    "extract_parent": "/opt/app",
    "owner": "root",
    "group": "root"
  }
}
```
{{% /tab %}}
{{< /tabs >}}

### Idempotency

The archive resource is idempotent through multiple mechanisms:

1. **Checksum verification**: If a `checksum` is provided and the existing file matches, no download occurs.
2. **Creates file**: If `creates` is specified and that file exists, neither download nor extraction occurs.
3. **File existence**: If the archive file exists with matching checksum and owner/group, no changes are made.

For best idempotency, always specify either `checksum` or `creates` (or both).

### Cleanup Behavior

When `cleanup: true` is set:

- The archive file is deleted after successful extraction
- The `extract_parent` property is required
- The `creates` property is required to track extraction state across runs

### Supported Archive Formats

| Extension         | Extraction Tool |
|-------------------|-----------------|
| `.tar.gz`, `.tgz` | `tar -xzf`      |
| `.tar`            | `tar -xf`       |
| `.zip`            | `unzip`         |

> [!info] Note
> The extraction tools (`tar`, `unzip`) must be available in the system PATH.

## Exec

The exec resource executes commands to bring the system into the desired state. It is idempotent when used with the `creates` property or `refreshonly` mode.

> [!info] Warning
> Specify commands with their full path, or use the `path` property to set the search path.

{{< tabs >}}
{{% tab title="Manifest" %}}
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
{{% /tab %}}
{{% tab title="CLI" %}}
```nohighlight
ccm ensure exec "/usr/bin/touch /tmp/hello" --creates /tmp/hello --timeout 30s
```
{{% /tab %}}
{{% tab title="API Request" %}}
```json
{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "exec",
  "properties": {
    "name": "/usr/bin/touch /tmp/hello",
    "creates": "/tmp/hello",
    "timeout": "30s",
    "cwd": "/tmp"
  }
}
```
{{% /tab %}}
{{< /tabs >}}

The command runs only if `/tmp/hello` does not exist.

### Providers

The exec resource supports two providers:

| Provider | Description                                                                                                   |
|----------|---------------------------------------------------------------------------------------------------------------|
| `posix`  | Default. Executes commands directly without a shell. Arguments are parsed and passed to the executable.       |
| `shell`  | Executes commands via `/bin/sh -c "..."`. Use this for shell features like pipes, redirections, and builtins. |

The `posix` provider is the default and is suitable for most commands. Use the `shell` provider when you need shell features:

```yaml
- exec:
    - cleanup-logs:
        command: find /var/log -name '*.log' -mtime +30 -delete && echo "Done"
        provider: shell

    - check-service:
        command: systemctl is-active httpd || systemctl start httpd
        provider: shell

    - process-data:
        command: cat /tmp/input.txt | grep -v '^#' | sort | uniq > /tmp/output.txt
        provider: shell
```

> [!info] Note
> The `shell` provider passes the entire command string to `/bin/sh -c`, so shell quoting rules apply. The `posix` provider parses arguments using shell-like quoting but does not invoke a shell.

### Properties

| Property                | Description                                                                       |
|-------------------------|-----------------------------------------------------------------------------------|
| `name`                  | The command to execute (used as the resource identifier)                          |
| `command`               | Alternative command to run instead of `name`                                      |
| `cwd`                   | Working directory for command execution                                           |
| `environment` (array)   | Environment variables in `KEY=VALUE` format                                       |
| `path`                  | Search path for executables as a colon-separated list (e.g., `/usr/bin:/bin`)     |
| `returns` (array)       | Exit codes indicating success (default: `[0]`)                                    |
| `timeout`               | Maximum execution time (e.g., `30s`, `5m`); command is killed if exceeded         |
| `creates`               | File path; if this file exists, the command does not run                          |
| `refreshonly` (boolean) | Only run when notified by a subscribed resource                                   |
| `subscribe` (array)     | Resources to subscribe to for refresh notifications (`type#name` or `type#alias`) |
| `logoutput` (boolean)   | Log the command output                                                            |
| `provider`              | Force a specific provider (`posix` or `shell`)                                    |

## File

The file resource manages files and directories, including their content, ownership, and permissions.

> [!info] Warning
> Use absolute file paths and primary group names.

{{< tabs >}}
{{% tab title="Manifest" %}}
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
{{% /tab %}}
{{% tab title="CLI" %}}
```nohighlight
ccm ensure file /etc/motd --source /tmp/ccm/motd --owner root --group root --mode 0644
```
{{% /tab %}}
{{% tab title="API Request" %}}
```json
{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "file",
  "properties": {
    "name": "/etc/motd",
    "ensure": "present",
    "content": "Managed by CCM\n",
    "owner": "root",
    "group": "root",
    "mode": "0644"
  }
}
```
{{% /tab %}}
{{< /tabs >}}

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

{{< tabs >}}
{{% tab title="Manifest" %}}
```yaml
- package:
    - zsh:
        ensure: "5.9"
```
{{% /tab %}}
{{% tab title="CLI" %}}
```nohighlight
ccm ensure package zsh 5.9
```
{{% /tab %}}
{{% tab title="API Request" %}}
```json
{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "package",
  "properties": {
    "name": "zsh",
    "ensure": "5.9"
  }
}
```
{{% /tab %}}
{{< /tabs >}}

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
| `provider` | Force a specific provider (`dnf`, `apt`)   |

### Provider Notes

#### APT (Debian/Ubuntu)

The APT provider preserves existing configuration files during package installation and upgrades. When a package is upgraded and the maintainer has provided a new version of a configuration file, the existing file is kept (`--force-confold` behavior).

Packages in a partially installed or `config-files` state (removed but configuration remains) are treated as absent. Reinstalling such packages will preserve the existing configuration files.

> [!info] Note
> The provider will not run `apt update` before installing a package. Use an `exec` resource to update the package index if necessary.

The provider runs non-interactively and suppresses prompts from `apt-listbugs` and `apt-listchanges`.

## Service

The service resource manages system services. Services have two independent properties: whether they are running and whether they are enabled to start at boot.

> [!info] Warning
> Use real service names, not virtual names or aliases.

Services can subscribe to other resources and restart when those resources change.

{{< tabs >}}
{{% tab title="Manifest" %}}
```yaml
- service:
    - httpd:
        ensure: running
        enable: true
        subscribe:
          - package#httpd
```
{{% /tab %}}
{{% tab title="CLI" %}}
```nohighlight
ccm ensure service httpd running --enable --subscribe package#httpd
```
{{% /tab %}}
{{% tab title="API Request" %}}
```json
{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "service",
  "properties": {
    "name": "httpd",
    "ensure": "running",
    "enable": true,
    "subscribe": ["package#httpd"]
  }
}
```
{{% /tab %}}
{{< /tabs >}}

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
