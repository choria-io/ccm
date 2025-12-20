+++
title = "Data"
toc = true
weight = 30
pre = "<b>3. </b>"
+++

Just like applications need data to vary their behavior and configure their environments, so do Configuration Management tools.

For this reason I wrote `extlookup` and `hiera` years ago for the Puppet community. Despite the minimal focus of this tool we still need data to vary the behaviour.

Data can be used for:

 * Configuring resource names that differ between Operating Systems like `httpd` vs `apache2` package names
 * Set different configuration values depending on environment, role or other dimensions
 * Decide if some environments shold have something installed and others not - like development vs production environments

Choria CM supports various data sources:

 * **Facts** about the instance - the Operating System, Networking and Disk configuration
 * **Environment** data like variables read from the shell environment and files like `./.env`
 * **Hiera Data** data resolved using Hiera that supports hierarchical overrides based on facts

## Accessing Data

In these examples you'll see expressions like `{{ lookup('facts.host.info.platformFamily') }}`, these are written using Expr Language:

* [Expr Language](https://expr-lang.org/#documentation)
* [Expr Language Documentation](https://expr-lang.org/docs/language-definition)

We've added the following functions to Expr:

| Function               | Description                                                                                                                |
|------------------------|----------------------------------------------------------------------------------------------------------------------------|
| `lookup(key, default)` | Lookup data from the runtime environment using [GJSON Path Syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md) |
| `readFile(file)`       | Read a file into a string, can only read files in the working directory                                                    |
| `template(f)`          | Parses `f` using templates                                                                                                 |


These expressions can be used in Hiera Data and even on the CLI.

```bash
$ ccm ensure package '{{ lookup("data.package_name", "httpd") }}'
```

Above will fetch data from the runtime environment and default to `httpd` if not found.

## Facts

We have a built-in fact resolver that gathers system information, we intend to expand this a bit, but today just the basic system facts are available.

To see the available facts, run:

```nohighlight
$ ccm facts
$ ccm facts host
$ ccm facts --yaml
```

Accessing facts is via the expressions like `{{ lookup('facts.host.info.platformFamily') }}`.

## Hiera Data for CLI

Hiera Data is resolved using Choria Hierarchical Data Resolver and are ready by default from `./.hiera` or a file you choose.

> [!info] Note
> This only works for `ccm ensure`, since apply is a Hiera manifest on it's own it does not need this behavior

Given this example file stored in `./.hiera`:

```yaml
hierarchy:
  order:
    - os:{{ lookup('facts.host.info.PlatformFamily') }}

data:
  package_name: ""

overrides:
  os:debian:
    package_name: apache2

  os:rhel:
    package_name: httpd
```

Running `ccm ensure package '{{ lookup("data.package_name") }}'` will install `httpd` on RHEL based systems and `apache2` on Debian based systems. 

> [!info] Note
> See the [Hiera](../hiera) section for details on how to configure Hiera Data in NATS

The data can also be stored in a NATS Key-Value store and read from there instead of from `./hiera` by passing `--context` and `--hiera kv://BUCKET/key` options.

## Environment

The shell environment and any environment data defined in `./.env` can be accessed via expressions like `{{ lookup("env.MY_VAR") }}`