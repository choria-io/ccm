+++
title = "Apply"
description = "Compose manifests from smaller reusable manifests"
toc = true
weight = 5
+++

The apply resource resolves and executes a child manifest within the parent manifest's execution context. Child manifests share the parent's session, enabling resource ordering and subscribe relationships across manifest boundaries.

> [!info] Note
> The apply resource is manifest-only. It has no CLI or API equivalent. Only local file paths are supported; URL-based manifest sources may be added in future.

```yaml
- apply:
    - networking/manifest.yaml: {}

    - monitoring/manifest.yaml:
        data:
          alert_email: ops@example.com
```

This executes two child manifests in order. The second receives additional data that its templates can reference.

## Ensure values

| Value     | Description                            |
|-----------|----------------------------------------|
| `present` | Resolve and execute the child manifest |

Only `present` is valid. The `ensure` property defaults to `present` if not specified.

## Properties

| Property            | Description                                                              |
|---------------------|--------------------------------------------------------------------------|
| `name`              | File path to the child manifest (relative to parent manifest directory)  |
| `noop`              | Execute child in noop mode (can only strengthen, never weaken)           |
| `health_check_only` | Execute child in health check mode (can only strengthen, never weaken)   |
| `allow_apply`       | Allow the child manifest to contain its own apply resources (default: `true`) |
| `data`              | Data map passed to the child manifest, merged with resolved data         |

## Path resolution

The `name` property specifies a file path relative to the directory containing the parent manifest. Absolute paths are used as-is.

```yaml
# Given /opt/ccm/manifest.yaml contains:
- apply:
    - sub/manifest.yaml: {}
    # resolves to /opt/ccm/sub/manifest.yaml
```

For nested apply resources, each level resolves paths relative to its own manifest's directory. After a child manifest completes, the working directory reverts to the parent's directory.

```nohighlight
/opt/ccm/manifest.yaml
  apply: sub/manifest.yaml        -> /opt/ccm/sub/manifest.yaml
    apply: lib/manifest.yaml      -> /opt/ccm/sub/lib/manifest.yaml
```

## Noop and health check propagation

Noop and health check modes propagate downward through apply resources. A child can enter noop or health check mode when its parent has not, but a child can never weaken a mode that the parent has active.

| Parent mode | Resource property | Effective child mode |
|-------------|-------------------|----------------------|
| `noop`      | `noop: false`     | `noop`               |
| `noop`      | `noop: true`      | `noop`               |
| `normal`    | `noop: true`      | `noop`               |
| `normal`    | `noop: false`     | `normal`             |

Health check mode follows the same pattern. If either the parent or the resource enables health check mode, the child executes in health check mode.

Running `ccm apply manifest.yaml --noop` forces noop on all child manifests regardless of their `noop` property.

## Passing data to child manifests

The `data` property provides key-value data to the child manifest. This data merges into the child's resolved data after its own Hiera resolution completes.

```yaml
- apply:
    - app/manifest.yaml:
        data:
          port: 8080
          log_level: info
```

Templates in the child manifest can reference these values using standard template syntax, such as `{{ lookup("data.port") }}`. External data from CLI `--data` flags persists through the merge and takes precedence.

## Restricting nested apply resources

The `allow_apply` property controls whether a child manifest may contain its own apply resources. Setting `allow_apply` to `false` limits the trust boundary when including manifests authored by others.

```yaml
- apply:
    - vendor/manifest.yaml:
        allow_apply: false
```

If the child manifest contains apply resources and `allow_apply` is `false`, execution fails with an error before any child resources run.

Regardless of `allow_apply`, nested apply resources are limited to a maximum recursion depth of 10 levels.

## Subscribe behavior

Other resources can subscribe to apply resources using the `apply#name` format:

```yaml
- apply:
    - config/manifest.yaml: {}

- exec:
    - notify:
        command: /usr/local/bin/notify.sh
        refresh_only: true
        subscribe:
          - apply#config/manifest.yaml
```

The exec resource runs only if the child manifest made changes.

## Shared session

The child manifest executes within the parent's session. Resource events from child manifests are recorded in the same session as the parent, and subscribe relationships work across manifest boundaries.

If any child resource fails, the apply resource reports the failure. If the enclosing manifest sets `fail_on_error: true`, execution of subsequent resources in that manifest stops at that point.