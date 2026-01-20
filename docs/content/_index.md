+++
weight = 5
+++

## Overview

Mainstream Configuration Management systems focus on full-system management, optimized for hundreds or thousands of managed resources per node. This makes them complex and dependency-heavy.

Traditional Configuration Management tools are not well suited for application management—they focus on systems management. They also struggle with ad hoc systems where each node is a unique snowflake.

CCM is a small-scale Configuration Management system designed to meet users where they are:

 * Focused on the needs of a single application (think of it as a single module)
 * Excel at bringing Configuration Management to ad-hoc and snowflake use cases like personal virtual machines and laptops
 * Supports hierarchical data similar to Hiera, with a focus on single-file manifests and simple data structures
 * No dependencies for the binaries to operate other than your OS
 * Designed to work at a first-class level in many environments:
   * Command line
   * Shell scripts
   * Single-file manifests in YAML format, run manually or continuously
   * Choria Autonomous Agents
   * Choria RPC
   * Embedded in Go applications

## Status

This is experimental and a work in progress. Currently, only `exec`, `file`, `service`, and `package` resources are implemented, with support limited to `dnf`, `apt` and `systemd`.

The CLI and shell interaction have reached a mature state. Next, we're exploring network-related features and deeper monitoring integration.

Manifests are supported in YAML format and can be run manually or continuously using an agent. Manifests and data can be stored in key-value and object stores.

## Examples

### Shell Example

Here we do a package-config-service style deployment using a shell script.  The script is safe to run multiple times as the CCM commands are all idempotent.

```bash
#!/bin/bash

eval $(ccm session new)
ccm ensure package httpd
ccm ensure file /etc/httpd/conf.d/listen.conf content="Listen 8080"
ccm ensure service httpd --subscribe file#/etc/httpd/conf.d/listen.conf
ccm session report
```

When run, this will create a session in a temporary directory and manage the resources. If the file resource changes after initial deployment, the service will restart.

A major problem with shell-based Configuration Management is that scripts often cannot be run multiple times to repair issues. This is why idempotence was invented in Configuration Management tools. They work not by being told exactly what to do, but rather by understanding the desired state and determining how to achieve it.

Idempotence is key to making rerunnable scripts for Configuration Management. By making the `ccm ensure` commands idempotent, we enable shell scripts to be used in this space as one would use traditional Configuration Management tools.

We support dynamic data on the CLI, `ccm` will read `.env` files and `.hiera` files and feed that into the runtime data. Using this even shell scripts can easily gain access to rich data.

```bash
$ cat .env
package_name="httpd"
$ ccm ensure package '{{ Data.package_name }}'
 INFO  package#httpd stable ensure=present runtime=8ms provider=dnf
```

### Manifest Example

Taking the example above, here is what it looks like in a manifest, complete with multi-OS support:

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
