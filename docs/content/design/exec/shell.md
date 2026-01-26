+++
title = "Shell Provider"
toc = true
weight = 20
+++

This document describes the implementation details of the Shell exec provider for executing commands via `/bin/sh`.

## Provider Selection

The Shell provider is selected when `provider: shell` is explicitly specified in the resource properties. It has a lower priority (99) than the Posix provider (1), so it is never automatically selected.

**Availability:** The provider checks for the existence of `/bin/sh` via `util.FileExists()`. If `/bin/sh` does not exist, the provider is not available.

## Comparison with Posix Provider

| Feature                                      | Shell                    | Posix                |
|----------------------------------------------|--------------------------|----------------------|
| Shell invocation                             | Yes (`/bin/sh -c`)       | No                   |
| Pipes (`\|`)                                 | Supported                | Not supported        |
| Redirections (`>`, `<`, `>>`)                | Supported                | Not supported        |
| Shell builtins (`cd`, `export`, `source`)    | Supported                | Not supported        |
| Glob expansion (`*.txt`, `?`)                | Supported                | Not supported        |
| Command substitution (`$(...)`, `` `...` ``) | Supported                | Not supported        |
| Variable expansion (`$VAR`, `${VAR}`)        | Supported                | Not supported        |
| Logical operators (`&&`, `\|\|`)             | Supported                | Not supported        |
| Argument parsing                             | Passed as single string  | `shellquote.Split()` |
| Security                                     | Shell injection possible | Lower attack surface |

**When to use Shell:**
- Commands with pipes: `cat file.txt | grep pattern | sort`
- Commands with redirections: `echo "data" > /tmp/file`
- Commands with shell builtins: `cd /tmp && pwd`
- Commands with variable expansion: `echo $HOME`
- Complex one-liners with logical operators

**When to use Posix (default):**
- Simple commands with arguments
- When shell features are not needed
- For better security (no shell injection risk)

## Operations

### Execute

**Process:**

1. Determine command source (`Command` property or `Name` if `Command` is empty)
2. Validate command is not empty
3. Execute via `CommandRunner.ExecuteWithOptions()` with `/bin/sh -c "<command>"`
4. Optionally log output line-by-line if `LogOutput` is enabled

**Execution Method:**

The entire command string is passed to the shell as a single argument:

```
/bin/sh -c "<entire command string>"
```

This allows the shell to interpret all shell syntax, including:
- Pipes and redirections
- Variable expansion
- Glob patterns
- Command substitution
- Logical operators

**Execution Options:**

| Option        | Value                      | Description                              |
|---------------|----------------------------|------------------------------------------|
| `Command`     | `/bin/sh`                  | Shell executable path                    |
| `Args`        | `["-c", "<command>"]`      | Shell flag and command string            |
| `Cwd`         | `properties.Cwd`           | Working directory                        |
| `Environment` | `properties.Environment`   | Additional env vars (`KEY=VALUE` format) |
| `Path`        | `properties.Path`          | Search path for executables              |
| `Timeout`     | `properties.ParsedTimeout` | Maximum execution time                   |

**Output Logging:**

When `LogOutput: true` is set and a user logger is provided:

```go
scanner := bufio.NewScanner(bytes.NewReader(stdout))
for scanner.Scan() {
    log.Info(scanner.Text())
}
```

Each line of stdout is logged as a separate `Info` message.

**Error Handling:**

| Condition               | Behavior                                           |
|-------------------------|----------------------------------------------------|
| Empty command string    | Return error: "no command to execute"              |
| Runner not configured   | Return error: "no command runner configured"       |
| Shell not found         | Provider not available (checked at selection time) |
| Command execution fails | Return error from runner                           |
| Non-zero exit code      | Return exit code (not an error by itself)          |

### Status

**Process:**

1. Create state with `EnsurePresent` (exec resources are always "present")
2. Check if `Creates` file exists via `util.FileExists()`
3. Set `CreatesSatisfied` accordingly

**State Fields:**

| Field              | Value                                  |
|--------------------|----------------------------------------|
| `Protocol`         | `io.choria.ccm.v1.resource.exec.state` |
| `ResourceType`     | `exec`                                 |
| `Name`             | Resource name                          |
| `Ensure`           | `present` (always)                     |
| `CreatesSatisfied` | `true` if `Creates` file exists        |

## Use Cases

### Pipes and Filters

```yaml
- exec:
    - filter-logs:
        command: cat /var/log/app.log | grep ERROR | tail -100 > /tmp/errors.txt
        provider: shell
        creates: /tmp/errors.txt
```

### Conditional Execution

```yaml
- exec:
    - ensure-running:
        command: pgrep myapp || /opt/myapp/bin/start
        provider: shell
```

### Complex Scripts

```yaml
- exec:
    - deploy-app:
        command: |
          cd /opt/app &&
          git pull origin main &&
          npm install &&
          npm run build &&
          systemctl restart app
        provider: shell
        timeout: 5m
```

### Variable Expansion

```yaml
- exec:
    - backup-home:
        command: tar -czf /backup/home-$(date +%Y%m%d).tar.gz $HOME
        provider: shell
```

## Idempotency

The shell provider uses the same idempotency mechanisms as the posix provider:

### Creates File

If `creates` is specified and the file exists, the command does not run:

```yaml
- exec:
    - extract-archive:
        command: cd /opt && tar -xzf /tmp/app.tar.gz
        provider: shell
        creates: /opt/app/bin/app
```

### RefreshOnly Mode

When `refreshonly: true`, the command only runs when triggered by a subscribed resource:

```yaml
- exec:
    - reload-nginx:
        command: nginx -t && systemctl reload nginx
        provider: shell
        refreshonly: true
        subscribe:
          - file#/etc/nginx/nginx.conf
```

### Exit Code Validation

The `returns` property specifies acceptable exit codes (default: `[0]`):

```yaml
- exec:
    - check-service:
        command: systemctl is-active myapp || true
        provider: shell
        returns: [0]
```

## Security Considerations

### Shell Injection Risk

The shell provider passes the command string directly to `/bin/sh`, making it vulnerable to shell injection if user input is incorporated:

```yaml
# DANGEROUS if filename comes from untrusted input
- exec:
    - process-file:
        command: cat {{ user_provided_filename }} | process
        provider: shell
```

**Mitigations:**
- Validate and sanitize any templated values
- Use the posix provider when shell features aren't needed
- Prefer explicit file paths over user-provided values

### Environment Variable Exposure

Shell commands can access environment variables, including sensitive ones:

```yaml
# $SECRET_KEY will be expanded by the shell
- exec:
    - use-secret:
        command: myapp --key=$SECRET_KEY
        provider: shell
```

Consider using the `environment` property to explicitly pass required variables rather than relying on inherited environment.

### Command Logging

The full command string (including any expanded variables) may appear in logs. Avoid embedding secrets directly in commands:

```yaml
# BAD - password visible in logs
- exec:
    - bad-example:
        command: mysql -p'secret123' -e 'SELECT 1'
        provider: shell

# BETTER - use environment variable
- exec:
    - better-example:
        command: mysql -p"$MYSQL_PWD" -e 'SELECT 1'
        provider: shell
        environment:
          - MYSQL_PWD={{ lookup('data.mysql_password') }}
```

## Platform Support

The shell provider requires `/bin/sh` to be available. This is standard on:
- Linux distributions
- macOS
- BSD variants
- Most Unix-like systems

On Windows, the provider will not be available unless `/bin/sh` exists (e.g., via WSL or Cygwin).

## Shell Compatibility

The provider uses `/bin/sh`, which is typically:
- **Linux:** Often a symlink to `bash`, `dash`, or another POSIX-compliant shell
- **macOS:** `/bin/sh` is `bash` (older) or `zsh` (newer) in POSIX mode
- **BSD:** Usually `ash` or similar

For maximum portability, use POSIX shell syntax and avoid bash-specific features like:
- Arrays (`arr=(1 2 3)`)
- `[[` conditionals (use `[` instead)
- `source` (use `.` instead)
- Process substitution (`<(command)`)