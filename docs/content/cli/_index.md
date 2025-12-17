+++
title = "CLI Usage"
toc = true
weight = 40
pre = "<b>4. </b>"
+++

We are designing CCM to be used as a CLI tool as its first class method of interaction.

Each resource type has its own subcommand under `ccm ensure` that maps to the resource type, with required inputs being
arguments and optionals being flags.

The CLI commands are designed to be idempotent; this means that they can be used to write shell scripts that can safely be run multiple times.

## Managing a single resource

Managing a single resource is really easy, however, some features will not function without preparing a session.

```
$ ccm ensure package zsh 5.8
```

Here we create a resource that ensures the package `zsh` is installed with version `5.8`.

We can also get the current state of a resource using the `status` subcommand.

```
$ ccm status package zsh
```

## Managing multiple resources

When managing multiple resources, in a script it is worth creating a session and then running the commands. The purpose of the session is to record the outcome of earlier resource so that features like refreshing a Service when a File is updated will function correctly.

```bash
#!/bin/bash

eval $(ccm session new)
ccm ensure package httpd
ccm ensure file /etc/httpd/conf/httpd.conf ....
ccm ensure service httpd --subscribe file#/etc/httpd/conf/httpd.conf
ccm session report
```

When this script is run it will create a temporary directory where session progress is stored, run the commands and should the file change the service will be restarted.

If evaluating random output from commands does not appeal then you can do this instead `export CCM_SESSION_STORE=$(mktemp -d)`

## Data in the CLI

As covered in the [data section of the documentation](../data) the commands will read `./.env` and `./.hiera` files and merge that data into the session.

This behavior can be configured using the `--hiera` flag or `CCM_HIERA_DATA` environment variable to point to a specific file.

With this enabled you can access `{{ lookup("data.my_data_key") }}` for Hiera data in your templates, `{{ lookup("env.MY_ENV_VAR") }}` for environment variables and `{{ lookup("facts.host.info.platformFamily") }}` for fact data.

Below we load the package name and version from Hiera and use them in a template to ensure the package is installed.

```bash
$ ccm ensure package '{{ lookup("data.package") }}' '{{ lookup("data.version") }}'
```

As pointed out these expressions are [Expr Language](https://expr-lang.org/docs/language-definition) so these expressions can be quite complex for example:

```bash
$ ccm ensure package '{{ lookup("facts.host.info.platformFamily") == "rhel" ? "httpd" : "apache2" }}'
WARN  package#httpd changed ensure=present runtime=14.509s provider=dnf
```

Though of course we would recommend using Hiera data for this.

## Checking what would be done (Noop mode)

One can ask the system to operate in Noop mode, meaning it will attempt to detect what would happen without actually doing it.

This is achieved by using the `--noop` flag.

> [!info] Note
> Noop mode is not perfect, if a change in a resource affects a future resource, it cannot always be detected.
