+++
title = "Shell Usage"
toc = true
weight = 40
pre = "<b>4. </b>"
+++

CCM is designed as a CLI-first tool. Each resource type has its own subcommand under `ccm ensure`, with required inputs as arguments and optional settings as flags.

The `ccm ensure` commands are idempotent, making them safe to run multiple times in shell scripts.

Use `ccm --help` and `ccm <command> --help` to explore available commands and options.

## Managing a Single Resource

Managing a single resource is straightforward.

```nohighlight
ccm ensure package zsh 5.8
```

This ensures the package `zsh` is installed at version `5.8`.

To view the current state of a resource:

```nohighlight
ccm status package zsh
```

## Managing Multiple Resources

When managing multiple resources in a script, create a session first. The session records the outcome of each resource, enabling features like refreshing a service when a file changes.

```nohighlight
#!/bin/bash

eval "$(ccm session new)"

ccm ensure package httpd
ccm ensure file /etc/httpd/conf/httpd.conf .... --require package#httpd
ccm ensure service httpd --subscribe file#/etc/httpd/conf/httpd.conf
ccm session report --remove
```

This creates a temporary directory for session state. If the file resource changes, the service restarts automatically.

If you prefer not to eval command output, use: `export CCM_SESSION_STORE=$(mktemp -d)`

## Data in the CLI

As covered in the [data section](../data), commands automatically read `./.env` and `./.hiera` files, merging that data into the session.

Use the `--hiera` flag or `CCM_HIERA_DATA` environment variable to specify a different data file.

With data loaded, you can access:
- `{{ lookup("data.my_data_key") }}` for Hiera data
- `{{ lookup("env.MY_ENV_VAR") }}` for environment variables
- `{{ lookup("facts.host.info.platformFamily") }}` for system facts

Example using Hiera data:

```nohighlight
ccm ensure package '{{ lookup("data.package") }}' '{{ lookup("data.version") }}'
```

Expressions use the [Expr Language](https://expr-lang.org/docs/language-definition), enabling complex logic:

```nohighlight
$ ccm ensure package '{{ lookup("facts.host.info.platformFamily") == "rhel" ? "httpd" : "apache2" }}'
WARN  package#httpd changed ensure=present runtime=14.509s provider=dnf
```

For complex conditional logic, we recommend using Hiera data with hierarchy overrides instead.

## Applying Manifests

For non-shell script usage use YAML manifests with `ccm apply`:

See [YAML Manifests](../yamlmanifests/) for manifest format details.

## Viewing System Facts

CCM gathers system facts that can be used in templates and conditions:

```nohighlight
# Show all facts as JSON
$ ccm facts

# Show facts as YAML
$ ccm facts --yaml

# Query specific facts using gjson syntax
$ ccm facts host.info.platformFamily
```

## Resolving Hiera Data

The `ccm hiera` command helps debug and test Hiera data resolution:

```nohighlight
# Resolve a Hiera file with system facts
$ ccm hiera parse data.yaml -S

# Resolve with custom facts
$ ccm hiera parse data.yaml os=linux env=production

# Query a specific key from the result
$ ccm hiera parse data.yaml --query packages
```
