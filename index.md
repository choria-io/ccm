# Introduction

Configuration Management systems typically focus on full-system management - optimized for hundreds or thousands of resources per node. This makes them complex, dependency-heavy, and poorly suited to ad hoc systems where each node is a unique snowflake.

CCM is a small-scale Configuration Management system designed to meet users where they are - enabling experimentation, R&D, and exploration without the overhead of full-system management while still following sound Configuration Management principles.

We focus on great UX, immediate feedback, and interactive use with minimal friction.

{{< cards >}}
{{% card title="Small and Focused" %}}
Embraces the popular *package-config-service* style of Configuration Management.

Focused on the needs of a single application or unit of software.

Minimal design for easy adoption, experimentation, and integration with tools like LLMs.
{{% /card %}}
{{% card title="Loves Snowflakes" %}}
Brings true Configuration Management principles to ad-hoc systems, enabling use cases where traditional CM fails.

Use management bundles in an *à la carte* fashion to quickly bring experimental infrastructure to a known state.
{{% /card %}}
{{% card title="External Data" %}}
Rich, Hierarchical Data and Facts accessible in the command line, scripts, and manifests.

Democratizes and opens the data that drives systems management to the rest of your stack.
{{% /card %}}
{{% card title="No Dependencies" %}}
Zero-dependency binaries, statically linked and fast.

Easy deployment in any environment without the overhead-cost of traditional CM.

Embraces a *Just Works* philosophy with no, or minimal, configuration.
{{% /card %}}
{{% card title="Optional Networking" %}}
Optional Network infrastructure needed only when your needs expand.

Choose from simple webservers to clustered, reliable, Object and Key-Value stores using technology you already know.
{{% /card %}}
{{% card title="Everywhere" %}}
Great at the CLI, shell scripts, YAML manifests, Choria Agents, and Go applications.

Scales from a single IoT Raspberry Pi 3 to millions of nodes.

Integrates easily with other software via SDK or Unix-like APIs.
{{% /card %}}
{{< /cards >}}

## Shell Example

Here we do a package-config-service style deployment using a shell script.  The script is safe to run multiple times as the CCM commands are all idempotent.

```bash
#!/bin/bash

eval $(ccm session new)
ccm ensure package httpd
ccm ensure file /etc/httpd/conf.d/listen.conf content="Listen 8080"
ccm ensure service httpd --subscribe file#/etc/httpd/conf.d/listen.conf
ccm session report --remove
```

When run, this will create a session in a temporary directory and manage the resources. If the file resource changes after initial deployment, the service will restart.

We support dynamic data on the CLI, `ccm` will read `.env` files and `.hiera` files and feed that into the runtime data. Using this even shell scripts can easily gain access to rich data.

```bash
$ cat .env
package_name="httpd"
$ ccm ensure package '{{ Data.package_name }}'
 INFO  package#httpd stable ensure=present runtime=8ms provider=dnf
```

## Manifest Example

Taking the example above, here is what it looks like in a manifest, complete with multi-OS support using Hierarchical Data and System Facts:

```yaml
data:
  package_name: httpd
  service_name: httpd

ccm:
  resources:
    - package:
        - "{{ Data.package_name }}":
            ensure: present
    - file: 
        - "/etc/httpd/conf.d/listen.conf":
            ensure: present
            content: |
                Listen 8080
    - service:
        - "{{ Data.service_name }}":
            ensure: running
            enable: true
            subscribe: 
              - file#/etc/httpd/conf.d/listen.conf

hierarchy:
  order:
    - os:{{ lookup('facts.host.info.platformFamily') }}

overrides:
  os:debian:
    package_name: apache2
    service_name: apache2
```

Here we define the inputs in `data` and the Hiera hierarchy along with OS-specific overrides. The data is referenced in the manifest using the `{{ Data.package_name }}` syntax.

## Status

This is a new project seeking contributors and early adopters. 

Currently, only `archive`, `exec`, `file`, `service`, and `package` resources are implemented, with support limited to `dnf`, `apt` and `systemd`.

The CLI and shell interaction have reached a mature state. Next, we're exploring network-related features and deeper monitoring integration.
