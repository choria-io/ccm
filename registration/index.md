# Registration

The registration system publishes service discovery entries to NATS when managed resources reach a stable state. Resources that pass all health checks and are not in a failed state announce themselves to a shared registry. Other nodes discover these services dynamically through template lookups.

> [!note] Supported Version
> Added in version `0.0.20`

## Use Cases

* **Dynamic load balancer configuration** - Web server resources register themselves on successful deploy, and a file resource on the load balancer node uses template lookups to generate an upstream configuration
* **Service mesh discovery** - Database and cache resources register their addresses and ports, allowing application nodes to build connection strings from live registry data
* **Canary deployments** - New service instances register with lower priority values, allowing consumers to prefer established instances while gradually shifting traffic

## Resource Configuration

Any resource type can publish registration entries by adding `register_when_stable` to its properties. Each entry describes a service endpoint to advertise.

```yaml
resources:
  - service:
      nginx:
        ensure: running
        enable: true
        health_checks:
          - check: exec
            command: curl -sf http://localhost:80/health
        register_when_stable:
          - cluster: production
            service: web
            protocol: http
            address: "{{ Facts.network.primaryIP }}"
            port: 80
            priority: 10
            ttl: 10m
            annotations:
              hostname: "{{ Facts.host.info.hostname }}"
              version: "1.2.3"
```

This manifest ensures Nginx is running, verifies it responds to health checks, and then registers the node as a `web` service in the `production` cluster.

### Entry Properties

| Property              | Template Key  | Description                                                                                                         |
|-----------------------|---------------|---------------------------------------------------------------------------------------------------------------------|
| `cluster` (required)  | `Cluster`     | Logical cluster name, must match `[a-zA-Z][a-zA-Z\d_-]*`                                                            |
| `service` (required)  | `Service`     | Service name, must match `[a-zA-Z][a-zA-Z\d_-]*`                                                                    |
| `protocol` (required) | `Protocol`    | Protocol identifier, must match `[a-zA-Z][a-zA-Z\d_-]*`                                                             |
| `address` (required)  | `Address`     | IP address or hostname of the service endpoint                                                                      |
| `port` (integer)      | `Port`        | Port number, between 1 and 65535                                                                                    |
| `priority` (required) | `Priority`    | Priority value between 1 and 255, lower values indicate higher priority                                             |
| `ttl`                 | `TTL`         | Time-to-live for the entry: a duration (e.g., `10m`, `1h`) or `never` to disable expiry. Sets the `Nats-TTL` header |
| `annotations` (map)   | `Annotations` | Arbitrary key-value metadata published with the entry                                                               |

The `cluster`, `address`, `port`, and `annotations` fields support template expressions for dynamic values resolved at apply time.

## How It Works

Registration entries are published after a resource is applied. The following conditions must all be met:

1. The resource has one or more `register_when_stable` entries configured
2. The resource is not in noop mode
3. The resource apply did not fail
4. All health checks (if any) passed with OK status

Each entry is published to a NATS subject with the structure and may be persisted in a NATS stream.

## Prerequisites

### NATS Stream

When using the `jetstream` destination, a JetStream stream must exist before registration entries can be published. Use `ccm registration init` to create or update the stream:

```nohighlight
ccm registration init --replicas 3 --max-age 5m
```

This creates a stream named `REGISTRATION` (or updates it if it already exists) with the correct subject filter, rollup, and TTL settings. Use `--registration` (`-R`) to specify a different stream name and `--context` to select a NATS context.

The `--max-age` value should exceed the agent's apply or health check interval to prevent entries from expiring between runs. It also controls how long deletion markers for expired entries are retained in the stream.

### Agent Configuration

Add the `registration` field to the agent configuration file at `/etc/choria/ccm/config.yaml`:

```yaml
registration: jetstream

nats_context: CCM
interval: 5m
manifests:
  - /etc/choria/ccm/manifests/base.yaml
```

Valid values for `registration` are `nats` (core NATS, fire-and-forget) and `jetstream` (reliable delivery). Omitting the field disables registration.

## Template Lookups

Other resources can query the registration registry using the `registrations()` function in templates. This enables dynamic configuration based on what services are currently registered.

The function takes four string arguments and returns an array of matching registration entries. Each entry exposes the struct fields listed in the Template Key column of the [Entry Properties](#entry-properties) table (e.g., `Address`, `Port`, `Priority`).

```nohighlight
registrations(cluster, protocol, service, address)
```

Any argument can be `"*"` to wildcard that position.

> [!info] Note
> The `registrations()` function queries JetStream and requires a NATS connection and a configured registration stream.

The `registrations()` function is available in all three template engines.

{{< tabs >}}
{{% tab title="Expr" %}}
```
registrations('production', 'http', 'web', '*')
```
{{% /tab %}}
{{% tab title="Jet" %}}
```
[[ range _, entry := registrations("production", "http", "web", "*") ]]
  server [[ entry.Address ]]:[[ entry.Port ]] weight [[ 256 - entry.Priority ]]
[[ end ]]
```
{{% /tab %}}
{{% tab title="Go Templates" %}}
```
{{ range $entry := registrations "production" "http" "web" "*" }}
  server {{ $entry.Address }}:{{ $entry.Port }}
{{ end }}
```
{{% /tab %}}
{{< /tabs >}}


### Format Transformers

The result of `registrations()` supports transformation methods that convert entries into formats consumed by other systems. These methods are callable from all three template engines.

#### Prometheus File Service Discovery

The `PrometheusFileSD()` method converts registration entries into the [Prometheus file-based service discovery](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#file_sd_config) JSON format. Entries are grouped by cluster, service, and protocol into target groups.

Entries without a port are excluded from the output.

For entries where the service or protocol is `prometheus`, entries are included by default and only excluded if the `prometheus.io/scrape` annotation is explicitly set to something other than `"true"`. For all other entries, the `prometheus.io/scrape` annotation must be explicitly set to `"true"` for the entry to be included.

Labels on each target group include `cluster`, `service`, `protocol`, and annotations that have non-empty values, do not start with `__`, and whose keys match `[a-zA-Z_][a-zA-Z0-9_]*`. Other annotations are silently skipped.

{{< tabs >}}
{{% tab title="Expr" %}}
```
registrations('production', 'http', 'web', '*').PrometheusFileSD()
```
{{% /tab %}}
{{% tab title="Jet" %}}
```
[[ registrations("production", "http", "web", "*").PrometheusFileSD() ]]
```
{{% /tab %}}
{{% tab title="Go Templates" %}}
```
{{ $r := registrations "production" "http" "web" "*" }}{{ $r.PrometheusFileSD }}
```
{{% /tab %}}
{{< /tabs >}}

##### Example Output

Given two registered web servers in the `production` cluster with a `prometheus.io/scrape: "true"` annotation:

```json
[
  {
    "labels": {
      "cluster": "production",
      "service": "web",
      "protocol": "http"
    },
    "targets": [
      "10.0.0.1:8080",
      "10.0.0.2:8080"
    ]
  }
]
```

## CLI Usage

The `ccm apply` and `ccm ensure file` commands can query the registration registry by passing the `--registration` flag with the name of the JetStream stream holding registration data.

Apply a manifest that uses `registrations()` in its templates:

```nohighlight
ccm apply manifest.yaml --registration REGISTRATIONS
```

Create a file whose content is generated from a template that queries registered services:

```nohighlight
ccm ensure file /etc/haproxy/backends.cfg \
  --content-file backends.cfg.templ \
  --registration REGISTRATIONS
```

A resource can delegate to a template file that calls `registrations()`:

```yaml
resources:
  - file:
      /etc/haproxy/backends.cfg:
        ensure: present
        content: |
          {{ template("backends.cfg.jet") }}
```

### Creating Entries

The `create` subcommand publishes a single registration entry to the JetStream stream. This is useful for manually registering services or for integration with external provisioning tools.

```nohighlight
ccm registration create \
  --cluster production \
  --service web \
  --protocol http \
  --address 10.0.0.5 \
  --port 8080
```

All entry properties are specified as flags:

| Flag                | Required | Default | Description                             |
|---------------------|----------|---------|-----------------------------------------|
| `--cluster`         | Yes      |         | Logical cluster name                    |
| `--service`         | Yes      |         | Service name                            |
| `--protocol`        | Yes      |         | Protocol identifier                     |
| `--address`         | Yes      |         | IP address of the service endpoint      |
| `--port`            | Yes      |         | Port number (1-65535)                   |
| `--priority`        | No       | `100`   | Priority value (1-255, lower is higher) |
| `--ttl`             | No       | `30s`   | Time-to-live for the entry              |
| `-A`/`--annotation` | No       |         | Annotations as `key=value`, repeatable  |

Add annotations by repeating the `-A` flag:

```nohighlight
ccm registration create \
  --cluster production \
  --service web \
  --protocol http \
  --address 10.0.0.5 \
  --port 8080 \
  --priority 10 \
  --ttl 5m \
  -A version=1.2.3 \
  -A hostname=web-05
```

### Removing Entries

The `rm` subcommand (alias `delete`) purges a registration entry from the JetStream stream. All identifying fields must be specified to match the entry to remove.

```nohighlight
ccm registration rm \
  --cluster production \
  --service web \
  --protocol http \
  --address 10.0.0.5 \
  --port 8080
```

The command prompts for confirmation before removing the entry. Use `--force` to skip the confirmation prompt:

```nohighlight
ccm registration rm \
  --cluster production \
  --service web \
  --protocol http \
  --address 10.0.0.5 \
  --port 8080 \
  --force
```

| Flag         | Required | Default | Description                        |
|--------------|----------|---------|------------------------------------|
| `--cluster`  | Yes      |         | Logical cluster name               |
| `--service`  | Yes      |         | Service name                       |
| `--protocol` | Yes      |         | Protocol identifier                |
| `--address`  | Yes      |         | IP address of the service endpoint |
| `--port`     | Yes      |         | Port number (1-65535)              |
| `--force`    | No       | `false` | Skip confirmation prompt           |

### Querying and Watching

The `ccm registration` command provides tools for inspecting and monitoring registration data from the command line.

Both subcommands accept the same positional arguments for filtering: `cluster`, `protocol`, `service`, and `address`. All default to `*` (wildcard). The `--context` flag (env: `NATS_CONTEXT`) controls NATS authentication and defaults to `CCM`. The `--registration` (`-R`) flag sets the JetStream stream name and defaults to `REGISTRATION`.

#### Query

The `query` subcommand performs a point-in-time lookup of registered entries.

```nohighlight
ccm registration query
```

This returns all entries across all clusters, protocols, services, and addresses. Filter by positional arguments:

```nohighlight
ccm registration query production http web
```

Output is a human-readable listing grouped by service and cluster. Machine-readable formats are available with `--json` or `--yaml`:

```nohighlight
ccm registration query production --json
ccm registration query production http web "*" --yaml
```

Results are sorted by cluster, then protocol, then service.

#### Watch

The `watch` subcommand subscribes to the registration stream and displays changes in real time. It runs until interrupted.

```nohighlight
ccm registration watch
```

New and updated entries are logged as info-level messages. Entries removed by TTL expiry, deletion, or purge are logged as warnings including the removal reason.

Filter the watch to specific entries using the same positional arguments:

```nohighlight
ccm registration watch production http web
```

The `--json` flag outputs each event as a JSON object for integration with other tools.

## Full Example

This example shows two nodes working together. A web server registers itself, and a load balancer discovers all web servers to generate its configuration.

### Web Server Node

```yaml
data:
  web_port: 8080

ccm:
  resources:
    - service:
        my-app:
          ensure: running
          enable: true
          health_checks:
            - check: exec
              command: curl -sf http://localhost:8080/health
          register_when_stable:
            - cluster: production
              service: web
              protocol: http
              address: "{{ Facts.network.primaryIP }}"
              port: "{{ Data.web_port }}"
              priority: 10
              ttl: 10m
```

### Load Balancer Node

```yaml
ccm:
  resources:
    - file:
        /etc/haproxy/backends.cfg:
          ensure: present
          content: |
            {{ template("backends.cfg.jet") }}

    - exec:
        reload-haproxy:
          command: systemctl reload haproxy
          refresh_only: true
          subscribe:
            - "file#/etc/haproxy/backends.cfg"
```

With a `backends.cfg.jet` template:

```
backend web_servers
  balance roundrobin
[[ range _, entry := registrations("production", "http", "web", "*") ]]
  server [[ entry.Address ]] [[ entry.Address ]]:[[ entry.Port ]] check weight [[ 256 - entry.Priority ]]
[[ end ]]
```

Each time the agent runs on the load balancer, it queries the registry for all `web` services and regenerates the HAProxy configuration. The `exec` resource reloads HAProxy when the configuration file changes.
