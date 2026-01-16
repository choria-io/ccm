+++
title = "Agent"
toc = true
weight = 60
pre = "<b>6. </b>"
+++

For certain use cases it would be useful to run [YAML Manifests](../yamlmanifests/) in an continuous nature, you might not want to manage your `dotfiles` automatically, allowing for local modifications, but do want to keep your Docker up to date and current.

The `CCM Agent` allows you to run manifests, loaded from Object Storage, in a continuous fashion with Key-Value data overlayed.

In time the Agent will become a key part in a service registry system, allowing for continous monitoring of managed resources and sharing that state into a registry: enabling other nodes to use `file` resources to create configurations that combine knowledge of all nodes.

## Run Modes

The agent supports two major modes of operation that combine into being very efficient and fast reacting:

 * Full manifest apply managing the complete state of every resource
 * Health check mode, where only [Monitoring](../monitoring) checks are run which would trigger full manifest apply as remediation

By enabling both modes at the same time you can run monitoring very frequently - even 10 or 20 seconds intervals - and keep your full CM as something to run every few hours.

While enabling the combination is optional, it is recommended, and so we also recommend using health checks in your key resources.

## Supported Manifest snd Data Sources

For Hiera data we support Key-Value stores as a local file, Key-Value values and HTTP(s) URLs.

For the manifest bundles you can point to a local file, a Object Storage URL or a HTTP(s) URL.

## Logical Flow

The agent continuously run and manage manifests in the following way:

 1. At start the Agent fetches data and gathers facts
 2. Starts workers for each manifest bundle
    1. Each worker starts watchers to download and manage the manifest tarball
 3. Triggers the workers at the configured interval to do a full apply
    1. Each run updates facts and data
    2. Triggers each manifest in a serial manner
 4. Triggers the workers at the configured health check interval to do health checks
    1. Each run of the health checks do not update facts or data
    2. Triggers each manifest in a serial manner
    3. Should any health checks be critical (not warning) the agent will trigger a full apply of the specific worker

In the background object stores and KV as watched and updates to these will trigger apply runs.

## Prometheus Metrics

When the HTTP port is configured the agent will expose Prometheus metrics on `/metrics` that can be used to monitor the health of the agent, get metrics of resource states and event, health check statusses and later we will expose health check metrics.

## Configuration

The agent is included in the same `ccm` binary so nothing new is needed but you should configure it enable the `systemd` service to use it:

The configuration file is located at `/etc/choria/ccm/config.yaml` and looks like:

```yaml
# CCM Agent Configuration Example
#
# This file configures the CCM agent that manages manifest apply cycles.

# Time between scheduled manifest apply runs.
# Must be at least 30s. Defaults to 5m.
# Supports duration strings like "30s", "5m", "1h".
interval: 5m

# Time between health check runs.
# When set, health checks run independently of apply runs and can trigger
# remediation applies when critical issues are detected.
# Leave empty or omit to disable periodic health checks.
health_check_interval: 1m

# List of manifest sources to apply. Each source creates a separate worker
# that manages its own apply cycle.
# Supported formats:
#   - File paths: /path/to/manifest.yaml
#   - Object store URLs: obj://bucket/key.tar.gz
manifests:
  - /etc/choria/ccm/manifests/base.yaml
  - obj://ccm-manifests/app.tar.gz


# Configures the logging level to use, one of debug, info, warn, error
log_level: info

# Sets the NATS context to use for authentication, defaults to 'CCM' when not set
nats_context: CCM

# Optional URL to fetch external data from using hiera resolution.
# The resolved data is merged into the manifest data context.
# Supports file paths, kv://bucket/key, obj://bucket/key, and http(s):// URLs.
# Leave empty or omit if not using external data.
external_data_url: obj://ccm-data/common

# Directory used to cache manifest sources fetched from remote locations.
# Defaults to /etc/choria/ccm/source.
cache_dir: /etc/choria/ccm/source

# Optional broker authentication using Choria JWT credentials.
# When set, nats_servers is required and nats_context is ignored.
# nats_servers: "tls://broker.choria.local:4222"
# choria_token_file: /root/.config/choria/client.jwt
# choria_seed_file: /root/.config/choria/client.key
# choria_collective: choria
# nats_tls_ca: /etc/choria/credentials/broker/public.crt
# nats_tls_insecure: false

# If set, a HTTP listener will be started that expose Prometheus metrics on /metrics
monitor_port: 0
```

Follow the comments in the file and then start your service using `ccm ensure service ccm-agent running --enable`
