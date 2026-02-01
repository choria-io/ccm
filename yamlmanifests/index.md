# YAML Manifests

A manifest is a YAML file that combines data, hierarchy configuration, and resources in a single file.

Manifests support template expressions but not procedural logic. Think of them as declarative configuration similar to multi-resource shell scripts.

> [!tip] CCM Studio
> An experimental visual editor for manifests is available at [Choria Studio](https://studio.choria-cm.dev/).

## Manifest Structure

A manifest contains these top-level sections:

| Section     | Description                                              |
|-------------|----------------------------------------------------------|
| `data`      | Input data (like module parameters)                      |
| `hierarchy` | Lookup order and merge strategy for overrides            |
| `overrides` | Data overrides keyed by hierarchy entries                |
| `ccm`       | Resource definitions and execution options               |

> [!info] Note
> A JSON Schema for manifests is available at [https://choria.io/schemas/ccm/v1/manifest.json](https://choria.io/schemas/ccm/v1/manifest.json). Configure your editor to use this schema for completion and validation.

The manifest is resolved using the [Choria Hierarchical Data Resolver](../hiera/).

## Full Example

### Input Data

Define input data like parameters for a module:

```yaml
data:
  package_name: "httpd"
```

### Resources

Define resources to manage in the `ccm` section:

```yaml
ccm:
  resources:
    - package:
        - "{{ lookup('data.package_name') }}":
            ensure: latest
```

### Configure Hierarchy

Configure a hierarchy to vary data by dimensions like OS platform:

```yaml
hierarchy:
  order:
    - os:{{ lookup('facts.host.info.platformFamily') }}
```

This looks for overrides in `os:rhel`, `os:debian`, etc.

### Overrides

Provide platform-specific data overrides:

```yaml
overrides:
  os:debian:
    package_name: apache2
```

## Applying Manifests

The complete manifest:

```yaml
data:
  package_name: "httpd"

ccm:
  resources:
    - package:
        - "{{ lookup('data.package_name') }}":
             ensure: latest

hierarchy:
  order:
    - os:{{ lookup('facts.host.info.platformFamily') }}

overrides:
  os:debian:
    package_name: apache2
```

Apply the manifest with `ccm apply`. The first run makes changes; subsequent runs are stable:

```nohighlight
$ ccm apply manifest.yaml
INFO  Creating new session record resources=1
WARN  package#httpd changed ensure=latest runtime=3.560699287s provider=dnf

$ ccm apply manifest.yaml
INFO  Creating new session record resources=1
INFO  package#httpd stable ensure=latest runtime=293.448824ms provider=dnf
```

To preview the fully resolved manifest without applying it:

```nohighlight
$ ccm apply manifest.yaml --render
data:
  package_name: apache2
resources:
- package:
    - apache2:
        ensure: latest
```

## Pre and Post Messages

Display messages before and after manifest execution:

```yaml
ccm:
  pre_message: |
    Starting configuration update...
  post_message: |
    Configuration complete.
  resources:
    - ...
```

Both are optional.

## Overriding Data

Override or augment manifest data with an external Hiera source:

```nohighlight
ccm apply manifest.yaml --hiera kv://CCM/common
```

Supported sources include local files, KV stores (`kv://`), and HTTP(S) URLs.

## Setting Defaults

Reduce repetition by setting defaults for resources of the same type:

```yaml
ccm:
  resources:
    - file:
        - defaults:
            owner: app
            group: app
            mode: "0644"
            ensure: present
        - /app/config/file.conf:
            source: file.conf
        - /app/bin/app:
            source: app.bin
            mode: "0700"

    - file:
        - /etc/motd:
            ensure: present
            source: motd.txt
            owner: root
            group: root
            mode: "0644"
```

The first two files inherit the `defaults` values. The `/app/bin/app` file overrides just the mode. The `/etc/motd` file is a separate resource block, so defaults do not apply.

## Templating

Manifests support template expressions like `{{ lookup("key") }}` for adjusting values. These expressions cannot generate new resources; they only modify values in valid YAML.

### Available Variables

Templates have access to:

| Variable  | Description                          |
|-----------|--------------------------------------|
| `Facts`   | System facts                         |
| `Data`    | Resolved Hiera data                  |
| `Environ` | Environment variables                |

### Generating Resources with Jet Templates

To dynamically generate resources from data, use [Jet Templates](https://github.com/CloudyKit/jet/blob/master/docs/syntax.md).

Given this data:

```yaml
data:
  common_packages:
    - zsh
    - vim-enhanced
    - nagios-plugins-all
    - tar
    - nmap-ncat
```

Reference a Jet template file instead of inline resources:

```yaml
ccm:
  resources_jet_file: resources.jet
```

Create the template file:

```jet
{* resources.jet *}
[[ range Data.common_packages ]]
- package:
    - [[ . ]]:
        ensure: present
[[ end ]]
```

> [!info] Note
> The template file must be in the same directory as the manifest.

The template receives the fully resolved Hiera data, plus `Facts` and `Environ`.

## Failure Handling

By default, if a resource fails, the apply continues to the next resource.

### Fail on Error

To stop execution on the first failure, set `fail_on_error`:

```yaml
ccm:
  fail_on_error: true
  resources:
    - exec:
        - /usr/bin/false: {}
        - /usr/bin/true: {}
```

The second resource is never executed:

```nohighlight
$ ccm apply manifest.yaml
INFO  Executing manifest manifest=manifest.yaml resources=2
ERROR exec#/usr/bin/false failed ensure=present runtime=3ms provider=posix errors=failed to reach desired state exit code 1
WARN  Terminating manifest execution due to failed resource
```

### Resource Dependencies

Use the `require` property to ensure a resource only runs after its dependencies succeed:

```yaml
ccm:
  resources:
    - package:
        - httpd:
            ensure: present
    - file:
        - /etc/httpd/conf.d/custom.conf:
            ensure: present
            content: "Listen 8080"
            owner: root
            group: root
            mode: "0644"
            require:
              - package#httpd
```

If the required resource fails, the dependent resource is skipped.

## Dry Run (Noop Mode)

Preview changes without applying them:

```nohighlight
ccm apply manifest.yaml --noop
```

> [!info] Note
> Noop mode cannot always detect cascading effects. If one resource change would affect a later resource, that dependency may not be reflected in the dry run.

## Health Check Only Mode

Run only health checks without applying resources:

```nohighlight
ccm apply manifest.yaml --monitor-only
```

This is useful for verifying system state without making changes.

## Manifests in NATS Object Store

Manifests can be stored in [NATS](https://nats.io) Object Stores, avoiding the need to distribute files locally.

Configure a NATS context for authentication:

```nohighlight
nats context add ccm --server nats.example.org --user ccm --password s3cret \
  --description "CCM Configuration Store"
```

Create an Object Store:

```nohighlight
nats obj add CCM --replicas 3 --context ccm
```

Create a manifest with supporting files:

```nohighlight
$ mkdir /tmp/manifest
$ echo 'CCM Managed' > /tmp/manifest/motd
$ cat > /tmp/manifest/manifest.yaml << 'EOF'
ccm:
  resources:
    - file:
        - /etc/motd:
            ensure: present
            source: motd
            owner: root
            group: root
            mode: "0644"
EOF
```

Package and upload to the Object Store:

```nohighlight
$ tar -C /tmp/manifest/ -cvzf /tmp/manifest.tgz .
$ nats obj put CCM manifest.tgz --context ccm
```

Apply the manifest:

```nohighlight
$ ccm apply obj://CCM/manifest.tgz --context ccm
INFO  Using manifest from Object Store in temporary directory bucket=CCM file=manifest.tgz
INFO  file#/etc/motd stable ensure=present runtime=0s provider=posix
```

## Manifests on Web Servers

Store gzipped tar archives on a web server and apply them directly:

```nohighlight
$ ccm apply https://example.net/manifest.tar.gz
INFO  Executing manifest manifest=https://example.net/manifest.tar.gz resources=1
INFO  file#/etc/motd stable ensure=present runtime=0s provider=posix
```

HTTP Basic Auth is supported via URL credentials:

```nohighlight
ccm apply https://user:pass@example.net/manifest.tar.gz
```

## Additional Facts

Provide additional facts from the command line or a file:

```nohighlight
# Individual facts
$ ccm apply manifest.yaml --fact env=production --fact region=us-east

# Facts from a file
$ ccm apply manifest.yaml --facts custom-facts.yaml
```

Facts from these sources are merged with system facts, with command-line facts taking precedence.
