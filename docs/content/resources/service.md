+++
title = "Service"
description = "Enable, disable, start, stop and restart services using OS Native service managers"
toc = true
weight = 50
+++

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

## Ensure Values

| Value     | Description                  |
|-----------|------------------------------|
| `running` | The service must be running  |
| `stopped` | The service must be stopped  |

If `ensure` is not specified, it defaults to `running`.

## Properties

| Property            | Description                                                                            |
|---------------------|----------------------------------------------------------------------------------------|
| `name`              | Service name                                                                           |
| `ensure`            | Desired state (`running` or `stopped`; default: `running`)                             |
| `enable` (boolean)  | Enable the service to start at boot                                                    |
| `subscribe` (array) | Resources to watch; restart the service when they change (`type#name` or `type#alias`) |
| `provider`          | Force a specific provider (`systemd` only)                                             |