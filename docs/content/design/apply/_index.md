+++
title = "Apply Type"
toc = true
weight = 15
description = "Apply resource for composing manifests from reusable parts"
+++

This document describes the design of the apply resource type for composing manifests from smaller reusable manifests.

## Overview

The apply resource resolves and executes a child manifest within the parent manifest's execution context. The child manifest shares the parent's manager and session, allowing resource ordering and subscribe relationships across manifest boundaries.

Key behaviors:
- **Noop strengthening**: A parent in noop mode forces all children into noop mode, regardless of the child's `noop` property
- **Health check strengthening**: Same semantics as noop; health check mode can only be strengthened, never weakened
- **Recursion depth limiting**: Nested apply resources are capped at a configurable maximum depth (default 10) to prevent infinite loops
- **Transitive trust control**: The `allow_apply` property prevents a child manifest from containing its own apply resources

## Provider Interface

Apply providers must implement the `ApplyProvider` interface:

```go
type ApplyProvider interface {
    model.Provider

    ApplyManifest(ctx context.Context, mgr model.Manager, properties *model.ApplyResourceProperties, currentDepth int, healthCheckOnly bool, log model.Logger) (*model.ApplyState, error)
}
```

### Method Responsibilities

| Method          | Responsibility                                                                  |
|-----------------|---------------------------------------------------------------------------------|
| `ApplyManifest` | Resolve and execute a child manifest, handling state save/restore and overrides  |

### State Response

The `ApplyManifest` method returns an `ApplyState` containing:

```go
type ApplyState struct {
    CommonResourceState
    ResourceCount int // Number of resources in the child manifest
}
```

## Available Providers

| Provider      | Source              | Documentation          |
|---------------|---------------------|------------------------|
| `ccmmanifest` | Local manifest file | [CCM Manifest](ccmmanifest/) |

## Ensure States

| Value     | Description                          |
|-----------|--------------------------------------|
| `present` | Resolve and execute the child manifest |

Only `present` is valid. The `ensure` property defaults to `present` if not specified.

## Properties

| Property            | Type            | Description                                              |
|---------------------|-----------------|----------------------------------------------------------|
| `name`              | string          | File path to the child manifest                          |
| `noop`              | bool            | Execute child in noop mode (can only strengthen)         |
| `health_check_only` | bool            | Execute child in health check mode (can only strengthen) |
| `allow_apply`       | bool            | Allow the child manifest to contain apply resources (default `true`) |
| `data`              | map[string]any  | Data to pass to the child manifest, merged with external data |

## Apply Logic

```
┌─────────────────────────────────────────┐
│ Save parent state (noop, data, wd)      │
│ (restored via defer on all paths)       │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│ Strengthen noop and health_check_only   │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│ Resolve child manifest                  │
│ (applies data overrides, sets wd)       │
└─────────────────┬───────────────────────┘
                  │
                  ▼
    ┌─────────────┴─────────────┐
    │ Recursion depth within    │
    │ limit?                    │
    └─────────────┬─────────────┘
              Yes │         No
                  │         │
                  ▼         ▼
    ┌─────────────┴───┐ ┌───────┐
    │ allow_apply     │ │ Error │
    │ satisfied?      │ └───────┘
    └─────────────┬───┘
              Yes │         No
                  │         │
                  ▼         ▼
    ┌─────────────┴───┐ ┌───────┐
    │ Execute child   │ │ Error │
    │ resources       │ └───────┘
    └─────────┬───────┘
              │
              ▼
┌─────────────────────────────────────────┐
│ Report resource count, changed, failed  │
└─────────────────────────────────────────┘
```

## Noop Strengthening

Noop mode follows a strict strengthening rule: a child manifest can run in noop mode when the parent does not, but a child can never weaken noop mode.

| Parent noop | Child `noop` property | Effective child noop | Behavior                            |
|-------------|-----------------------|----------------------|-------------------------------------|
| `true`      | `false`               | `true`               | Warning logged, parent noop applies |
| `true`      | `true`                | `true`               | Both agree                          |
| `false`     | `true`                | `true`               | Child strengthens to noop           |
| `false`     | `false`               | `false`              | Normal execution                    |

Health check mode follows the same strengthening pattern.

## State Save and Restore

The provider saves three pieces of manager state before manifest resolution and restores them after execution:

| State             | Save method             | Restore method            | Reason                                        |
|-------------------|-------------------------|---------------------------|-----------------------------------------------|
| Noop mode         | `NoopMode()`            | `SetNoopMode(saved)`      | Child noop must not leak to subsequent resources |
| Working directory | `WorkingDirectory()`    | `SetWorkingDirectory(wd)` | `ResolveManifestUrl` changes working directory |
| Data              | `Data()`                | `SetData(saved)`          | Child data overrides must not persist          |

State is saved before calling any resolve functions because `ResolveManifestUrl` mutates the manager's working directory and data during resolution.

## Recursion Depth Limiting

Nested apply resources increment a depth counter passed through `Execute()` options. The default maximum depth is 10.

```
parent.yaml (depth 0)
  +-- child.yaml (depth 1)
        +-- grandchild.yaml (depth 2)
              +-- ... (up to depth 10)
```

Exceeding the maximum depth returns an error before iterating any child resources.

## Transitive Trust

The `allow_apply` property controls whether a child manifest may contain its own apply resources. When `allow_apply` is `false`, the child manifest is scanned for apply resources after resolution but before execution. If any are found, an error is returned.

This provides a mechanism to limit the trust boundary when including manifests authored by others.

| `allow_apply` value | Child contains apply resources | Result       |
|---------------------|-------------------------------|--------------|
| `true` (default)    | Yes                           | Allowed      |
| `true` (default)    | No                            | Allowed      |
| `false`             | Yes                           | Error        |
| `false`             | No                            | Allowed      |

## Data Handling

The `data` property provides key-value data to the child manifest. This data is passed through the `WithOverridingResolvedData` option and merged into the resolved data after the child manifest's own data resolution.

External data (CLI overrides) always persists through the merge. The parent's original data is restored after child execution via the state save/restore mechanism.

## Subscribe Behavior

Apply resources support the standard `subscribe` property. Subscribe targets use the `apply#name` format:

```yaml
- apply:
    - child.yaml:

- exec:
    - post-apply:
        command: /usr/local/bin/notify.sh
        refresh_only: true
        subscribe:
          - apply#child.yaml
```

## Child Manifest Failures

After child manifest execution, the provider inspects the result to determine the outcome:

| Child result              | Parent behavior                       |
|---------------------------|---------------------------------------|
| All resources succeeded   | Log success, report unchanged         |
| Some resources changed    | Log warning, report changed           |
| Any resource failed       | Return error with failure count       |