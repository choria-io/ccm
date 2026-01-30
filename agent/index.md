# Agent

For certain use cases, it is useful to run [YAML Manifests](../yamlmanifests/) continuously. For example, you might not want to manage your `dotfiles` automatically (allowing for local modifications), but you do want to keep Docker up to date.

The CCM Agent runs manifests continuously, loading them from local files, Object Storage, or HTTP(S) URLs, with Key-Value data overlaid.

In time, the Agent will become a key part of a service registry system, allowing for continuous monitoring of managed resources and sharing that state into a registry—enabling other nodes to use `file` resources to create configurations that combine knowledge of all nodes.

## Run Modes

The agent supports two modes of operation that combine to be efficient and fast-reacting:

 * **Full manifest apply**: Manages the complete state of every resource
 * **Health check mode**: Runs only [Monitoring](../monitoring) checks, which can trigger a full manifest apply as remediation

By enabling both modes, you can run health checks very frequently (even at 10- or 20-second intervals) while keeping full Configuration Management runs less frequent (every few hours).

Enabling both modes is optional but recommended. We also recommend adding health checks to your key resources.

## Supported Manifest and Data Sources

### Manifest Sources

Manifests can be loaded from:

 * **Local file**: `/path/to/manifest.yaml`
 * **Object Storage**: `obj://bucket/key.tar.gz`
 * **HTTP(S)**: `https://example.com/manifest.tar.gz` (supports Basic Auth via URL credentials)

Remote sources (Object Storage and HTTP) must be `.tar.gz` archives containing a `manifest.yaml` file, templates and file sources.

### External Data Sources

For Hiera data resolution, the agent supports:

 * **Local file**: `file:///path/to/data.yaml`
 * **Key-Value store**: `kv://bucket/key`
 * **HTTP(S)**: `https://example.com/data.yaml`

## Logical Flow

The agent continuously runs and manages manifests as follows:

 1. At startup, the agent fetches data and gathers facts
 2. Starts a worker for each manifest source
    1. Each worker starts watchers to download and manage the manifest (polling every 30 seconds for remote sources)
 3. Triggers workers at the configured interval for a full apply
    1. Each run updates facts (minimum 2-minute interval) and data
    2. Applies each manifest serially
 4. Triggers workers at the configured health check interval
    1. Health check runs do not update facts or data
    2. Runs health checks for each manifest serially
    3. If any health checks are critical (not warning), the agent triggers a full apply for that worker

In the background, object stores and HTTP sources are watched for changes. Updates trigger immediate apply runs with exponential backoff retry on failures.

## Prometheus Metrics

When `monitor_port` is configured, the agent exposes Prometheus metrics on `/metrics`. These metrics can be used to monitor agent health, track resource states and events, and observe health check statuses.

### Agent Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `choria_ccm_agent_apply_duration_seconds` | Summary | manifest | Time taken to apply manifests |
| `choria_ccm_agent_healthcheck_duration_seconds` | Summary | manifests | Time taken for health check runs |
| `choria_ccm_agent_healthcheck_remediations_count` | Counter | manifest | Health checks that triggered remediation |
| `choria_ccm_agent_data_resolve_duration_seconds` | Summary | - | Time taken to resolve external data |
| `choria_ccm_agent_data_resolve_error_count` | Counter | url | Data resolution failures |
| `choria_ccm_agent_facts_resolve_duration_seconds` | Summary | - | Time taken to resolve facts |
| `choria_ccm_agent_facts_resolve_error_count` | Counter | - | Facts resolution failures |
| `choria_ccm_agent_manifest_fetch_count` | Counter | manifest | Remote manifest fetches |
| `choria_ccm_agent_manifest_fetch_error_count` | Counter | manifest | Remote manifest fetch failures |

### Resource Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `choria_ccm_manifest_apply_duration_seconds` | Summary | source | Time taken to apply an entire manifest |
| `choria_ccm_resource_apply_duration_seconds` | Summary | type, provider, name | Time taken to apply a resource |
| `choria_ccm_resource_state_total_count` | Counter | type, name | Total resources processed |
| `choria_ccm_resource_state_stable_count` | Counter | type, name | Resources in stable state |
| `choria_ccm_resource_state_changed_count` | Counter | type, name | Resources that changed |
| `choria_ccm_resource_state_refreshed_count` | Counter | type, name | Resources that were refreshed |
| `choria_ccm_resource_state_failed_count` | Counter | type, name | Resources that failed |
| `choria_ccm_resource_state_error_count` | Counter | type, name | Resources with errors |
| `choria_ccm_resource_state_skipped_count` | Counter | type, name | Resources that were skipped |
| `choria_ccm_resource_state_noop_count` | Counter | type, name | Resources in noop mode |

### Health Check Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `choria_ccm_healthcheck_duration_seconds` | Summary | type, name, check | Time taken for health checks |
| `choria_ccm_healthcheck_status_count` | Counter | type, name, status, check | Health check results by status |

### Facts Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `choria_ccm_facts_gather_duration_seconds` | Summary | - | Time taken to gather system facts |

## Configuration

The agent is included in the `ccm` binary. To use it, create a configuration file and enable the systemd service.

The configuration file is located at `/etc/choria/ccm/config.yaml`:

```yaml
# CCM Agent Configuration Example

# Time between scheduled manifest apply runs.
# Must be at least 30s. Defaults to 5m.
interval: 5m

# Time between health check runs.
# When set, health checks run independently of apply runs and can trigger
# remediation applies when critical issues are detected.
# Omit to disable periodic health checks.
health_check_interval: 1m

# List of manifest sources to apply. Each source creates a separate worker.
# Supported formats:
#   - Local file: /path/to/manifest.yaml
#   - Object store: obj://bucket/key.tar.gz
#   - HTTP(S): https://example.com/manifest.tar.gz
#   - HTTP with Basic Auth: https://user:pass@example.com/manifest.tar.gz
# Remote sources must be .tar.gz archives containing a manifest.yaml file.
manifests:
  - /etc/choria/ccm/manifests/base.yaml
  - obj://ccm-manifests/app.tar.gz
  - https://config.example.com/manifests/web.tar.gz

# Logging level: debug, info, warn, error
log_level: info

# NATS context for authentication. Defaults to 'CCM'.
nats_context: CCM

# Optional URL for external Hiera data resolution.
# Supported formats: file://, kv://, http(s)://
# The resolved data is merged into the manifest data context.
external_data_url: kv://ccm-data/common

# Directory for caching remote manifest sources.
# Defaults to /etc/choria/ccm/source.
cache_dir: /etc/choria/ccm/source

# Port for Prometheus metrics endpoint (/metrics).
# Set to 0 or omit to disable.
monitor_port: 9100
```

After configuring, start the service:

```nohighlight
ccm ensure service ccm-agent running --enable
```

