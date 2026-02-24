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
| `Remove`   | Delete managed files (changed and stable) and clean up directories    |

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
| `post`            | []map[string]string | No     | Post-processing: glob pattern to command mapping           |

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
    - /opt/app:
        ensure: present
        source: templates/app
        post:
          - "*.go": "go fmt {}"
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
      ┌───────────┐                   ┌───────────┐
      │ Noop?     │                   │ Noop?     │
      └─────┬─────┘                   └─────┬─────┘
        Yes │     No                    Yes │     No
            ▼     │                         ▼     │
    ┌────────────┐│                 ┌────────────┐│
    │ Set noop   ││                 │ Set noop   ││
    │ message    ││                 │ message    ││
    └────────────┘│                 └────────────┘│
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

1. **Ensure absent**: Target must not exist, or no managed files remain on disk (`Changed` and `Stable` lists empty). Purged files (files not belonging to the scaffold) do not affect this check.
2. **Ensure present**: The `Changed` list must be empty, and the `Purged` list must be empty when `purge` is enabled (all files are stable). When `purge` is disabled, purged files do not affect stability.

### Decision Table

For `ensure: absent`, purged files never affect stability since they don't belong to the scaffold. For `ensure: present`, purged files only affect stability when `purge` is enabled.

When `ensure: absent`, the `Status` method filters `Changed` and `Stable` lists to only include files that actually exist on disk, so the state reflects reality after removal rather than what the scaffold would create.

| Desired   | Target Exists | Changed Files | Purged Files | Purge Enabled | Stable?                        |
|-----------|---------------|---------------|--------------|---------------|--------------------------------|
| `absent`  | No            | N/A           | N/A          | N/A           | Yes                            |
| `absent`  | Yes           | None          | Any          | N/A           | Yes (no managed files on disk) |
| `absent`  | Yes           | Some          | Any          | N/A           | No (managed files remain)      |
| `present` | Yes           | None          | None         | Any           | Yes                            |
| `present` | Yes           | None          | Some         | No            | Yes (purged files ignored)     |
| `present` | Yes           | None          | Some         | Yes           | No (purge needed)              |
| `present` | Yes           | Some          | Any          | Any           | No (render needed)             |
| `present` | No            | N/A           | N/A          | Any           | No (target missing)            |

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

The `post` property defines commands to run on rendered files. Each entry is a map where the key is a glob pattern matched against the file's basename and the value is a command to execute. Use `{}` as a placeholder for the file's full path; if omitted, the path is appended as the last argument.

```yaml
post:
  - "*.go": "go fmt {}"
  - "*.sh": "chmod +x {}"
```

Post-processing runs immediately after each file is rendered. Validation ensures neither keys nor values are empty.

## Noop Mode

In noop mode, the scaffold type queries the current state via `Status()` and reports what would change without modifying the filesystem. Neither `Scaffold()` nor `Remove()` are called.

For `ensure: present`, the affected count is the number of changed files plus purged files (when `purge` is enabled). For `ensure: absent`, the affected count is the number of changed and stable files plus purged files (when `purge` is enabled).

| Desired   | Affected Count                                       | Message                                       |
|-----------|------------------------------------------------------|-----------------------------------------------|
| `present` | `Changed` + `Purged` (if purge enabled)              | `Would have changed N scaffold files`         |
| `absent`  | `Changed` + `Stable` + `Purged` (if purge enabled)   | `Would have removed N scaffold files`         |

`Changed` is set to `true` only when the affected count is greater than zero. When the resource is already in the desired state, `Changed` is `false` and `NoopMessage` is empty.

## Desired State Validation

After applying changes (in non-noop mode), the type verifies the scaffold reached the desired state by checking the changed and purged file lists. If validation fails, `ErrDesiredStateFailed` is returned.