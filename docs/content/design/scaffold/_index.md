+++
title = "Scaffold Type"
toc = true
weight = 45
+++

This document describes the design of the scaffold resource type for rendering template directories into target directories.

## Overview

The scaffold resource renders files from a source template directory to a target directory:
- **Rendering**: Process templates using Go `text/template` or Jet engine
- **Synchronization**: Detect changed, stable, and purgeable files
- **Cleanup**: Optionally remove files in the target not present in the source

Templates have access to facts and data from Hiera, enabling dynamic configuration generation from directory structures.

## Provider Interface

Scaffold providers must implement the `ScaffoldProvider` interface:

```go
type ScaffoldProvider interface {
    model.Provider

    Remove(ctx context.Context, prop *model.ScaffoldResourceProperties, state *model.ScaffoldState) error
    Scaffold(ctx context.Context, env *templates.Env, prop *model.ScaffoldResourceProperties, noop bool) (*model.ScaffoldState, error)
    Status(ctx context.Context, env *templates.Env, prop *model.ScaffoldResourceProperties) (*model.ScaffoldState, error)
}
```

### Method Responsibilities

| Method     | Responsibility                                                        |
|------------|-----------------------------------------------------------------------|
| `Status`   | Render in noop mode to determine current state of managed files       |
| `Scaffold` | Render templates to target directory (or noop to preview changes)     |
| `Remove`   | Delete all managed files and empty directories from the target        |

### Status Response

The `Status` method returns a `ScaffoldState` containing:

```go
type ScaffoldState struct {
    CommonResourceState
    Metadata *ScaffoldMetadata
}

type ScaffoldMetadata struct {
    Name         string                 // Target directory
    Provider     string                 // Provider name (e.g., "choria")
    TargetExists bool                   // Whether target directory exists
    Changed      []string               // Files created or modified
    Purged       []string               // Files removed (not in source)
    Stable       []string               // Files unchanged
    Engine       ScaffoldResourceEngine // Template engine used
}
```

The `Ensure` field in `CommonResourceState` is set to:
- `present` if the target directory exists
- `absent` if the target directory does not exist

## Available Providers

| Provider  | Engine Support   | Documentation      |
|-----------|------------------|--------------------|
| `choria`  | Go, Jet          | [Choria](choria/)  |

## Ensure States

| Value     | Description                                             |
|-----------|---------------------------------------------------------|
| `present` | Target directory must exist with rendered template files |
| `absent`  | Managed files must be removed from the target           |

## Template Engines

Two template engines are supported:

| Engine | Library             | Default Delimiters | Description                |
|--------|---------------------|--------------------|----------------------------|
| `go`   | Go `text/template`  | `{{` / `}}`       | Standard Go templates      |
| `jet`  | Jet templating      | `[[` / `]]`       | Jet template language      |

The engine defaults to `jet` if not specified. Delimiters can be customized via `left_delimiter` and `right_delimiter` properties.

## Properties

| Property          | Type              | Required | Description                                                |
|-------------------|-------------------|----------|------------------------------------------------------------|
| `source`          | string            | Yes      | Source template directory path or URL                      |
| `engine`          | string            | No       | Template engine: `go` or `jet` (default: `jet`)            |
| `skip_empty`      | bool              | No       | Skip empty files in rendered output                        |
| `left_delimiter`  | string            | No       | Custom left template delimiter                             |
| `right_delimiter` | string            | No       | Custom right template delimiter                            |
| `purge`           | bool              | No       | Remove files in target not present in source               |
| `post`            | []map[string]string | No     | Post-processing commands to run after rendering            |

```yaml
# Render configuration templates using Jet engine
- scaffold:
    - /etc/app:
        ensure: present
        source: templates/app
        engine: jet
        purge: true

# Render with Go templates and custom delimiters
- scaffold:
    - /etc/myservice:
        ensure: present
        source: templates/myservice
        engine: go
        left_delimiter: "<<"
        right_delimiter: ">>"

# With post-processing commands
- scaffold:
    - /etc/nginx/sites:
        ensure: present
        source: templates/nginx
        post:
          - nginx: "-t"
```

## Apply Logic

```
┌─────────────────────────────────────────┐
│ Get current state via Status()          │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│ Is current state desired state?         │
└─────────────────┬───────────────────────┘
              Yes │         No
                  ▼         │
          ┌───────────┐     │
          │ No change │     │
          └───────────┘     │
                            ▼
              ┌─────────────────────────┐
              │ What is desired ensure? │
              └─────────────┬───────────┘
                            │
            ┌───────────────┴───────────────┐
            │ absent                        │ present
            ▼                               ▼
    ┌───────────────┐             ┌─────────────────────┐
    │ Remove all    │             │ Scaffold            │
    │ managed files │             │ (render templates)  │
    │ and empty dirs│             │                     │
    └───────────────┘             └─────────────────────┘
```

## Idempotency

The scaffold resource determines idempotency by rendering templates in noop mode and comparing results against the target directory.

### State Checks

1. **Ensure absent**: Target must not exist or must have no managed files (changed, stable, or purged lists all empty)
2. **Ensure present**: The `Changed` list and `Purged` list must both be empty (all files are stable)

### Decision Table

| Desired   | Target Exists | Changed Files | Purged Files | Stable?                  |
|-----------|---------------|---------------|--------------|--------------------------|
| `absent`  | No            | N/A           | N/A          | Yes                      |
| `absent`  | Yes           | None          | None         | Yes (no managed files)   |
| `absent`  | Yes           | Some          | Any          | No (remove needed)       |
| `present` | Yes           | None          | None         | Yes                      |
| `present` | Yes           | Some          | Any          | No (render needed)       |
| `present` | No            | N/A           | N/A          | No (target missing)      |

## Source Resolution

The `source` property is resolved relative to the manager's working directory when it is a relative path:

```go
parsed, _ := url.Parse(properties.Source)
if parsed == nil || parsed.Scheme == "" {
    if !filepath.IsAbs(properties.Source) {
        t.prop.Source = filepath.Join(mgr.WorkingDirectory(), properties.Source)
    }
}
```

This allows manifests bundled with template directories to use relative paths. URL sources (with a scheme) are passed through unchanged.

## Path Validation

Target paths (the resource name) must be:
- Absolute (start with `/`)
- Canonical (no `.` or `..` components, `filepath.Clean(path) == path`)

## Post-Processing

The `post` property defines commands to run after template rendering:

```yaml
post:
  - nginx: "-t"
  - systemctl: "reload nginx"
```

Each entry is a map with a single key-value pair where the key is the command and the value is its arguments. Validation ensures neither keys nor values are empty.

## Noop Mode

In noop mode, the scaffold type:
1. Queries current state by rendering templates in noop mode
2. Determines what changes would occur
3. For `ensure: present`: calls `Scaffold()` with noop flag, which renders without writing
4. Reports `Changed: true` if changes would occur
5. Does not modify any files on disk

## Desired State Validation

After applying changes (in non-noop mode), the type verifies the scaffold reached the desired state by checking the changed and purged file lists. If validation fails, `ErrDesiredStateFailed` is returned.