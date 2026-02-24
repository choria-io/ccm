+++
title = "Choria Provider"
toc = true
weight = 10
+++

This document describes the implementation details of the Choria scaffold provider for rendering template directories using the `choria-io/scaffold` library.

## Provider Selection

The Choria provider is the default and only scaffold provider. It is always available and returns priority 1 for all scaffold resources.

## Operations

### Scaffold (Render Templates)

**Process:**

1. Check if target directory exists
2. Configure scaffold with source, target, engine, delimiters, post-processing, and skip_empty settings
3. Create scaffold instance using the appropriate engine (`scaffold.New()` for Go, `scaffold.NewJet()` for Jet)
4. Call `Render()` (real mode) or `RenderNoop()` (noop mode)
5. Categorize results into changed, stable, and purged file lists

**Scaffold Configuration:**

| Config Field             | Source Property    | Description                              |
|--------------------------|--------------------|------------------------------------------|
| `TargetDirectory`        | `Name`             | Target directory for rendered files      |
| `SourceDirectory`        | `Source`            | Source template directory                |
| `MergeTargetDirectory`   | (always `true`)    | Merge into existing target directory     |
| `Post`                   | `Post`              | Post-processing commands                 |
| `SkipEmpty`              | `SkipEmpty`         | Skip empty rendered files                |
| `CustomLeftDelimiter`    | `LeftDelimiter`     | Custom template left delimiter           |
| `CustomRightDelimiter`   | `RightDelimiter`    | Custom template right delimiter          |

**Engine Selection:**

| Engine | Constructor           | Default Delimiters |
|--------|-----------------------|--------------------|
| `go`   | `scaffold.New()`      | `{{` / `}}`       |
| `jet`  | `scaffold.NewJet()`   | `[[` / `]]`       |

**Result Categorization:**

| Scaffold Action           | Metadata List | Description              |
|---------------------------|---------------|--------------------------|
| `FileActionEqual`         | `Stable`      | File content unchanged   |
| `FileActionAdd`           | `Changed`     | New file created         |
| `FileActionUpdate`        | `Changed`     | Existing file modified   |
| `FileActionRemove`        | `Purged`      | File removed from target |

File paths in the metadata lists are absolute paths, constructed by joining the target directory with the relative path from the scaffold result.

**Purge Behavior:**

When `purge` is enabled and a file has `FileActionRemove`, the provider deletes the file from disk during `Scaffold()`. In noop mode, the removal is logged but not performed. When `purge` is disabled, purged files are only tracked in metadata and not removed.

### Status

**Process:**

1. Perform a dry-run render (noop mode) to determine what the scaffold would do
2. When `ensure` is `absent`, filter `Changed` and `Stable` lists to only include files that actually exist on disk

The noop render reports what *would happen* if the scaffold were applied. For `ensure: present`, this is the desired output — it shows what needs to change. For `ensure: absent`, the raw render output is misleading after removal (it would show files to be *added*), so the lists are filtered to reflect what managed files actually remain on disk.

**State Detection:**

| Target Directory | Ensure Value | Metadata                                              |
|------------------|--------------|-------------------------------------------------------|
| Exists           | `present`    | Changed, stable, and purged file lists from render    |
| Exists           | `absent`     | Changed and stable filtered to files on disk, purged from render |
| Does not exist   | Any          | Empty metadata, `TargetExists: false`                 |

### Remove

**Process:**

1. Collect managed files from the state's `Changed` and `Stable` lists (purged files are not removed as they don't belong to the scaffold)
2. Remove each file individually
3. Track parent directories of removed files
4. Iteratively remove empty directories deepest-first
5. Stop when no more empty directories can be removed
6. Best-effort removal of the target directory (only succeeds if empty)

**File Removal Order:**

Files are collected from two metadata lists:
1. `Changed` - Files that were added or modified
2. `Stable` - Files that were unchanged

Purged files are not removed because they are unrelated to the scaffold and may belong to other processes.

**Directory Cleanup:**

```
For each removed file:
    Track its parent directory

Repeat:
    For each tracked directory:
        Skip if it is the target directory itself
        Skip if not empty
        Remove the directory
        Track its parent directory
Until no more directories removed

Best-effort: remove the target directory (fails silently if not empty)
```

The target directory is removed if empty after all managed files and subdirectories are cleaned up. If unrelated files remain (purged files), the directory is preserved.

**Error Handling:**

| Condition                      | Behavior                                      |
|--------------------------------|-----------------------------------------------|
| Non-absolute file path         | Return error immediately                      |
| File removal fails             | Log error, continue with remaining files      |
| Directory removal fails        | Log error, continue with remaining directories |
| File does not exist            | Silently skip (`os.IsNotExist` check)         |
| Target directory removal fails | Log at debug level, no error returned         |

## Template Environment

Templates receive the full `templates.Env` environment, which provides access to:
- `facts` - System facts for the managed node
- `data` - Hiera-resolved configuration data
- Template helper functions

This allows templates to generate host-specific configurations based on facts and hierarchical data.

## Logging

The provider wraps the CCM logger in a scaffold-compatible interface:

```go
type logger struct {
    log model.Logger
}

func (l *logger) Debugf(format string, v ...any)
func (l *logger) Infof(format string, v ...any)
```

This adapter translates the scaffold library's `Debugf`/`Infof` calls to CCM's structured logging.

## Platform Support

The Choria provider is platform-independent. It uses the `choria-io/scaffold` library for template rendering, which operates on standard filesystem operations. No platform-specific system calls are used.