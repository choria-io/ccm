# Choria Configuration Management

This is a small Configuration Management system inspired by Puppet.

## Background

Mainstream Configuration Management systems focus on full-system management, optimized for 100s or 1000s of managed resources per node. This makes them complex to use and with a lot of dependencies.

Additionally, systems like Puppet do not really do well with Application management - much better at systems management.

Choria has Autonomous Agents that can be used for application lifecycle management but thus far lacked any kind of Configuration Management.

This then is a new system designed for small-scale Configuration Management designed to meet users where they are:

 * Focussed on the needs of a single application - think of it as a single module.
 * Designed to work in shell scripts.
 * Single-file manifests in YAML format.
 * No dependencies.
 * Ready to integrate into other Go-based software like Choria Autonomous Agents.
 * Integrates with [Choria Hierarchical Data Resolver](https://github.com/choria-io/tinyhiera).

## Status

It's very early days, at the moment there is only a `package` resource that supports `dnf`. We have been focussed on the interaction model, manifests and general plumbing.

Additionally the `dnf` provider is very limited and probably not fully robust yet.

Most commands just dump the internal transaction events as output, once we have a bit more providers and types we'll focus on improving those areas.

### TODO

We are specifically keeping this small and not adding 10s of providers and types, we essentially want to support what is needed for the basic package-config-service style module.

At a broad level this is what we hope to achieve

 * `apt` provider for the `package` type, improve the `dnf` provider
 * `file` type
 * `service` type that supports `systemd
 * `exec` type
 * Relationships allowing service to refresh if a file changes
 * Support relationships in a shell script
 * Choria Autonomous Agent integration

## Facts

`ccm` includes native Facts that can be used in manifests, command line and more places.

Today these are just a standard set, but we plan to add reading in other sources of information like JSON or YAML files.

See `ccm facts` for the facts of your system.

## Usage

### Expressions

In these examples you'll see expressions like `{{ lookup('facts.host.info.platformFamily') }}`, these are written using Expr Language:

 * [Expr Language](https://expr-lang.org/#documentation)
 * [Expr Language Documentation](https://expr-lang.org/docs/language-definition)

We've added just the `lookup(key, default)` function to expr at this time but it would be easy to add more as needs arise.

### Shell Scripts

The system can be used in shell scripts, traditionally using shell scripts for Configuration Management meant they cannot be run multiple times because many system commands are not idempotent.

`ccm` creates idempotent shell commands that act like resources and are safe to run repeatedly.

```bash
$ ccm package ensure zsh 5.8
```

This is equivalent to:

```puppet
package {"zsh":
  ensure => "5.8"
}
```

Lookups can be done in facts even on the CLI, here's an example that adjusts the Apache Server package name based on the platform, later we'll see examples using Hiera:

```bash
$ ccm package ensure "{{ lookup('facts.host.info.platformFamily') == 'rhel' ? 'httpd' : 'apache2' }}"
```

There are some other helper commands also like `ccm package info` and `ccm package uninstall`.

### YAML Manifests

`ccm` has a YAML based manifest language that resolves the manifest using [Choria Hierarchical Data Resolver](https://github.com/choria-io/tinyhiera), our single-file implementation of Hiera.

The below sections are all added to a single YAML file but I show them seperate for clarity:

#### Input Data

First we define input data, think of this like the properties a module accepts:

```yaml
data:
  package_name: "httpd"
```

#### Resources

We will manage the Apache Server package here:

```yaml
ccm:
  resources:
    - package:
        name: "{{ Data.package_name }}"
        ensure: latest
```

#### Configure Hierarchy

We need to be able to configure the package name on a number of dimensions - like OS Platform - so we'll create a Hierarchy here:

```yaml
hierarchy:
  order:
    - os:{{ lookup('facts.host.info.platformFamily') }}
```

Here we will look for overrides in `os:rhel`, `os:debian` etc

#### Overrides

Finally we configure a package name for Debian:

```yaml
overrides:
  os:debian:
    package_name: apache2
```

#### Applying the manifest

Lets apply the manifest, this is how it should look now:

```
data:
  package_name: "httpd"

ccm:
  resources:
    - package:
        name: "{{ Data.package_name }}"
        ensure: latest

hierarchy:
  order:
    - os:{{ lookup('facts.host.info.platformFamily') }}

overrides:
  os:debian:
    package_name: apache2
```

We can apply this manifest like so, we can see first run makes a change 2nd run is stable:

```
$ ccm apply manifest.yaml
INFO  Creating new session record resources=1
WARN  package#httpd changed ensure=latest runtime=3.560699287s provider=dnf

$ ccm apply manifest.yaml
INFO  Creating new session record resources=1
INFO  package#httpd stable ensure=latest runtime=293.448824ms provider=dnf
```

One can also see the fully resolved manifest and data without applying it:

```
$ ccm apply manifest.yaml --render
data:
  package_name: apache2
resources:
- package:
    ensure: latest
    name: apache2
```

### Go Programs

Using from Go is pretty simple, first we need a manager:

```golang
appLogger:=slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
userLogger:=slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

mgr, _ := manager.NewManager(appLogger, userLogger)
```

Now we can use a type directly:

```golang
	pkg, _ := packageresource.New(ctx, mgr, model.PackageResourceProperties{
		CommonResourceProperties: model.CommonResourceProperties{
			Name:     "apache2",
			Ensure:   model.EnsurePresent,
		},
	})

    status, _ := pkg.Apply(ctx)
```

Or apply a manifest:

```golang
    manifest, _ := os.Open("manifest.yaml")

    _, apply, err := mgr.ResolveManifestReader(ctx, manifest)

    _, err = mgr.ApplyManifest(ctx, apply)
```