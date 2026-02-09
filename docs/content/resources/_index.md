+++
title = "Resources"
description = "Introduction to resources and common properties"
toc = true
weight = 10
pre = "<b>1. </b>"
+++

Resources describe the desired state of your infrastructure. Each resource represents something to manage and is backed by a provider that implements platform-specific management logic.

Every resource has a type, a unique name, and resource-specific properties.

## Resource Types

{{< subpages >}}

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