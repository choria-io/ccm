# Exec

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

## Providers

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

## Properties

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