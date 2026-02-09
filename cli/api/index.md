# JSON API

CCM provides a STDIN/STDOUT API for managing resources programmatically. This enables integration with external languages, allowing you to build DSLs in Ruby, Perl, Python, or any language that can execute processes and handle JSON or YAML.

## Overview

The API uses a simple request/response pattern:

1. Send a request to `ccm ensure api pipe` via STDIN
2. Receive a response on STDOUT

Both JSON and YAML formats are supported for requests. The response format is always JSON, but can be explicitly set to YAML using `--yaml`.

## Command

```nohighlight
ccm ensure api pipe [--yaml] [--noop] [--facts <file>] [--data <file>]
```

| Flag             | Description                                                   |
|------------------|---------------------------------------------------------------|
| `--yaml`         | Output response in YAML format instead of JSON                |
| `--noop`         | Dry-run mode; report what would change without making changes |
| `--facts <file>` | Load additional facts from a YAML file                        |
| `--data <file>`  | Load Hiera-style data from a YAML file                        |

## Request Format

Requests must include a protocol identifier, resource type, and properties.

> [!info] Note
> JSON Schemas for these requests and responses are available at [resource_ensure_request.json](https://choria-cm.dev/schemas/ccm/v1/resource_ensure_request.json) and [resource_ensure_response.json](https://choria-cm.dev/schemas/ccm/v1/resource_ensure_response.json).

### JSON Request

```json
{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "package",
  "properties": {
    "name": "nginx",
    "ensure": "present"
  }
}
```

### YAML Request

```yaml
protocol: io.choria.ccm.v1.resource.ensure.request
type: package
properties:
  name: nginx
  ensure: present
```

### Request Fields

| Field        | Required | Description                                        |
|--------------|----------|----------------------------------------------------|
| `protocol`   | Yes      | Must be `io.choria.ccm.v1.resource.ensure.request` |
| `type`       | Yes      | Resource type example `package`                    |
| `properties` | Yes      | Resource properties (varies by type)               |

## Response Format

Responses include a protocol identifier and either a state object (on success) or an error message.

For the full response structure, see [ResourceEnsureApiResponse](https://pkg.go.dev/github.com/choria-io/ccm@main/model#ResourceEnsureApiResponse) in the Go documentation. The `state` field contains a [TransactionEvent](https://pkg.go.dev/github.com/choria-io/ccm@main/model#TransactionEvent).

### Successful Response

```json
{
  "protocol": "io.choria.ccm.v1.resource.ensure.response",
  "state": {
    "protocol": "io.choria.ccm.v1.transaction.event",
    "event_id": "2abc123def456",
    "timestamp": "2026-01-28T10:30:00Z",
    "type": "package",
    "provider": "dnf",
    "name": "nginx",
    "requested_ensure": "present",
    "final_ensure": "1.24.0-1.el9",
    "duration": 1500000000,
    "changed": true,
    "failed": false
  }
}
```

### Error Response

```json
{
  "protocol": "io.choria.ccm.v1.resource.ensure.response",
  "error": "invalid protocol \"wrong.protocol\""
}
```

## Examples

### Install a Package

```nohighlight
echo '{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "package",
  "properties": {
    "name": "htop",
    "ensure": "present"
  }
}' | ccm ensure api pipe
```

### Manage a Service

```nohighlight
cat <<EOF | ccm ensure api pipe
protocol: io.choria.ccm.v1.resource.ensure.request
type: service
properties:
  name: nginx
  ensure: running
EOF
```

### Create a File

```nohighlight
echo '{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "file",
  "properties": {
    "name": "/etc/motd",
    "ensure": "present",
    "content": "Welcome to this server\n",
    "owner": "root",
    "group": "root",
    "mode": "0644"
  }
}' | ccm ensure api pipe
```

### Dry-Run Mode

```nohighlight
echo '{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "package",
  "properties": {
    "name": "vim",
    "ensure": "absent"
  }
}' | ccm ensure api pipe --noop
```

## Resource Types

For detailed information about each resource type and its properties, see the [Resource Documentation](/resources)
