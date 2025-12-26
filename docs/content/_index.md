+++
weight = 5
+++

## Overview

Mainstream Configuration Management systems focus on full-system management, optimized for 100s or 1000s of managed resources per node. This makes them complex to use and with a lot of dependencies.

Systems like Puppet do not really do well with Application management - they focus on on systems management.

Choria has [Autonomous Agents](https://choria.io/docs/autoagents/) that can be used for application lifecycle management but thus far lacked any kind of Configuration Management.

This then is a new system designed for small-scale Configuration Management designed to meet users where they are:

 * Focused on the needs of a single application - think of it as a single module.
 * Supports Hierarchical data similar to Hiera but with a focus on single file and single data structure
 * No dependencies for the binaries to operate other than your OS
 * Designed to work at a first-class level in many environments:
   * Command Line.
   * Shell scripts.
   * Single-file manifests in YAML format.
   * Choria Autonomous Agents.
   * Choria RPC.
   * Embedded in Go applications.
 
## Status

This is an experimental work in progress, there are only `exec`, `file` (very basic), `service` and `package` resources implemented so far, they support only `dnf` and `systemd`.

We've got the CLI/shell interaction to a quite mature state, next we're exploring network related features and deeper monitoring integration.

At this point we think the idea has legs and will keep working on it. Keeping in mind the minimal focus of this is to deliver something that can do package-config-service style deployments and as such will have minimal resource types (`file`, `package`, `service` and `exec` are the current targets).

We're also only likely to support only the key Linux distros in common use.

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

A major problem with doing configuration management with shell scripts is that they cannot be run multiple times to repair issues. This is why idempotence was invented in Configuration Management tools. They work not just by being told exactly what to do but rather by understanding the desired state and determining how to achieve it.

Idempotence is key to making rerunnable scripts used for Configuration Management; by making the `ccm ensure` commands idempotent we enable shell scripts to be used in this space as one would use Puppet or Chef.

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
        name: "{{ Data.package_name }}"
        ensure: present
    - file: 
        name: "/etc/httpd/conf.d/listen.conf"
        ensure: present
        content: |
          Listen 8080
    - service:
        name: "{{ Data.service_name }}"
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

### Autonomous Agent Support

As the Autonomous Agent is designed to own one single component and manage that component's lifecycle and is written as a YAML file, one can easily see how this could be used.

As the state machine goes about its normal lifecycle, it would simply manage individual CCM resources autonomously and forever, augmenting that with data from key-value stores, parsed as Hiera data, monitoring and remediation components.

```yaml
name: ccm
version: 0.0.1
initial_state: MANAGE

transitions:
  - name: maintenance
    description: Maintenance mode where no runs will be scheduled
    destination: MAINTENANCE
    from: [MANAGE]

  - name: resume
    description: Exit maintenance mode and start scheduling ccm runs
    destination: MANAGE
    from: [MAINTENANCE]

watchers:
  - name: ccm
    type: ccmmanifest
    state_match: [MANAGE]
    interval: 1m
    properties:
      governor: CCM
      manifest_file: manifest.yaml
```

Here we have a basic `ccm apply` scheduler, but by combining other parts like `kv` data you can imagine a `kv` update could create data that would be accessible in the manifest.

This way we have a managed manifest, with external data and a scheduler that runs it periodically. 

Together with Governors we can create rolling rollouts where any remediation or configuration change can be globally restricted to single nodes in a cluster concurrently. 

### Choria RPC Support

Choria has various commands like `choria req package install package=zsh`, this is currently invoking Puppet to do the work - and as such is very slow. We'd use these capabilities instead to deliver the same feature set.