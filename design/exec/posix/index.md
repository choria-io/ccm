# Posix Provider

This document describes the implementation details of the Posix exec provider for executing commands without a shell.

## Provider Selection

The Posix provider is the default exec provider. It is always available and returns priority 1 for all exec resources unless a different provider is explicitly requested via the `provider` property.

To use the shell provider instead, specify `provider: shell` in the resource properties.

## Comparison with Shell Provider

| Feature                         | Posix                | Shell                    |
|---------------------------------|----------------------|--------------------------|
| Shell invocation                | No                   | Yes (`/bin/sh -c`)       |
| Pipes (`\|`)                    | Not supported        | Supported                |
| Redirections (`>`, `<`)         | Not supported        | Supported                |
| Shell builtins (`cd`, `export`) | Not supported        | Supported                |
| Glob expansion                  | Not supported        | Supported                |
| Command substitution (`$(...)`) | Not supported        | Supported                |
| Argument parsing                | `shellquote.Split()` | Passed as single string  |
| Security                        | Lower attack surface | Shell injection possible |

**When to use Posix (default):**
- Simple commands with arguments
- When shell features are not needed
- For better security (no shell injection risk)

**When to use Shell:**
- Commands with pipes, redirections, or shell builtins
- Complex command strings
- When shell expansion is required

## Operations

### Execute

**Process:**

1. Determine command source (`Command` property or `Name` if `Command` is empty)
2. Parse command string into words using `shellquote.Split()`
3. Extract command (first word) and arguments (remaining words)
4. Execute via `CommandRunner.ExecuteWithOptions()`
5. Optionally log output line-by-line if `LogOutput` is enabled

**Command Parsing:**

The command string is parsed using `github.com/kballard/go-shellquote`, which handles:

| Syntax         | Example              | Result                       |
|----------------|----------------------|------------------------------|
| Simple words   | `echo hello world`   | `["echo", "hello", "world"]` |
| Single quotes  | `echo 'hello world'` | `["echo", "hello world"]`    |
| Double quotes  | `echo "hello world"` | `["echo", "hello world"]`    |
| Escaped spaces | `echo hello\ world`  | `["echo", "hello world"]`    |
| Mixed quoting  | `echo "it's a test"` | `["echo", "it's a test"]`    |

**Execution Options:**

| Option        | Source                     | Description                              |
|---------------|----------------------------|------------------------------------------|
| `Command`     | First word after parsing   | Executable path or name                  |
| `Args`        | Remaining words            | Command arguments                        |
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

| Condition               | Behavior                                                 |
|-------------------------|----------------------------------------------------------|
| Empty command string    | Return error: "no command specified"                     |
| Invalid shell quoting   | Return parsing error (e.g., "Unterminated single quote") |
| Runner not configured   | Return error: "no command runner configured"             |
| Command execution fails | Return error from runner                                 |
| Non-zero exit code      | Return exit code (not an error by itself)                |

### EvaluateGuard

**Process:**

1. Parse guard command string into words using `shellquote.Split()`
2. Extract command (first word) and arguments (remaining words)
3. Execute via `CommandRunner.ExecuteWithOptions()` using the same `cwd`, `environment`, `path`, and `timeout` from the exec properties
4. Return `true` if exit code is 0, `false` if non-zero

**Error Handling:**

| Condition               | Behavior                                         |
|-------------------------|--------------------------------------------------|
| Empty command string    | Return error: "empty guard command"              |
| Invalid shell quoting   | Return error: "invalid guard command: ..."       |
| Runner not configured   | Return error: "no command runner configured"     |
| Command execution fails | Return error from runner                         |
| Non-zero exit code      | Return `false` (not an error)                    |

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

## Idempotency

The exec resource achieves idempotency through multiple mechanisms:

### Creates File

If `creates` is specified and the file exists, the command does not run:

```yaml
- exec:
    - /usr/bin/tar -xzf app.tar.gz:
        creates: /opt/app/bin/app
        cwd: /opt
```

### Guard Commands

If `onlyif` is specified, the command only runs when the guard exits 0. If `unless` is specified, the command only runs when the guard exits non-zero:

```yaml
- exec:
    - install-app:
        command: /usr/local/bin/install-app.sh
        onlyif: test -f /tmp/app-package.tar.gz

    - configure-firewall:
        command: /usr/sbin/iptables -A INPUT -p tcp --dport 8080 -j ACCEPT
        unless: /usr/sbin/iptables -C INPUT -p tcp --dport 8080 -j ACCEPT
```

### RefreshOnly Mode

When `refreshonly: true`, the command only runs when triggered by a subscribed resource:

```yaml
- exec:
    - systemctl reload httpd:
        refreshonly: true
        subscribe:
          - file#/etc/httpd/conf/httpd.conf
```

### Exit Code Validation

The `returns` property specifies acceptable exit codes (default: `[0]`):

```yaml
- exec:
    - /opt/app/healthcheck:
        returns: [0, 1, 2]  # 0=healthy, 1=degraded, 2=warning
```

## Decision Flow

```
┌─────────────────────────────────────────┐
│ Should resource be applied?             │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│ Subscribe triggered?                     │
│ (subscribed resource changed)            │
└─────────────┬───────────────┬───────────┘
              │ Yes           │ No
              ▼               ▼
┌─────────────────┐   ┌───────────────────┐
│ Execute command │   │ Creates satisfied? │
└─────────────────┘   └─────────┬─────────┘
                                │
                      ┌─────────┴─────────┐
                      │ Yes               │ No
                      ▼                   ▼
              ┌───────────────┐   ┌───────────────────────┐
              │ Skip (stable) │   │ Guard commands pass?  │
              └───────────────┘   │ (onlyif=0, unless≠0) │
                                  └─────────┬─────────────┘
                                            │
                                  ┌─────────┴─────────┐
                                  │ No                │ Yes
                                  ▼                   ▼
                          ┌───────────────┐   ┌───────────────────┐
                          │ Skip (stable) │   │ RefreshOnly mode? │
                          └───────────────┘   └─────────┬─────────┘
                                                        │
                                              ┌─────────┴─────────┐
                                              │ Yes               │ No
                                              ▼                   ▼
                                      ┌───────────────┐   ┌───────────────┐
                                      │ Skip (stable) │   │ Execute       │
                                      └───────────────┘   └───────────────┘
```

## Properties Validation

The model validates exec properties before execution:

| Property      | Validation                                                         |
|---------------|--------------------------------------------------------------------|
| `name`        | Must be parseable by shellquote (balanced quotes)                  |
| `timeout`     | Must be valid duration format (e.g., `30s`, `5m`)                  |
| `subscribe`   | Each entry must be `type#name` format                              |
| `path`        | Each directory must be absolute (start with `/`)                   |
| `environment` | Each entry must be `KEY=VALUE` format with non-empty key and value |

## Platform Support

The Posix provider works on all platforms supported by Go's `os/exec` package. It does not use any platform-specific system calls directly.

The command runner (`model.CommandRunner`) handles the actual process execution, which may have platform-specific implementations.

## Security Considerations

### No Shell Injection

Unlike the shell provider, the posix provider does not invoke a shell. Arguments are passed directly to the executable, preventing shell injection attacks:

```yaml
# Safe with posix provider - $USER is passed literally, not expanded
- exec:
    - /bin/echo $USER:
        provider: posix  # Default

# Potentially dangerous with shell provider - $USER is expanded
- exec:
    - /bin/echo $USER:
        provider: shell
```

### Path Validation

The `path` property only accepts absolute directory paths, preventing path traversal via relative paths.

### Environment Validation

Environment variables must have non-empty keys and values, preventing injection of empty or malformed entries.