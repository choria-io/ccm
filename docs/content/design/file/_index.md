+++
title = "File Type"
toc = true
weight = 30
+++

This document describes the design of the file resource type for managing files and directories.

## Overview

The file resource manages files and directories with three aspects:
- **Existence**: Whether the file/directory exists or is absent
- **Content**: The contents of a file (from inline content or source file)
- **Attributes**: Owner, group, and permissions

## Provider Interface

File providers must implement the `FileProvider` interface:

```go
type FileProvider interface {
    model.Provider

    CreateDirectory(ctx context.Context, dir string, owner string, group string, mode string) error
    Store(ctx context.Context, file string, contents []byte, source string, owner string, group string, mode string) error
    Status(ctx context.Context, file string) (*model.FileState, error)
}
```

### Method Responsibilities

| Method            | Responsibility                                                       |
|-------------------|----------------------------------------------------------------------|
| `Status`          | Query current file state (existence, type, content hash, attributes) |
| `Store`           | Create or update a file with content and attributes                  |
| `CreateDirectory` | Create a directory with attributes                                   |

### Status Response

The `Status` method returns a `FileState` containing:

```go
type FileState struct {
    CommonResourceState
    Metadata *FileMetadata
}

type FileMetadata struct {
    Name     string         // File path
    Checksum string         // SHA256 hash of contents (files only)
    Owner    string         // Owner username
    Group    string         // Group name
    Mode     string         // Permissions in octal (e.g., "0644")
    Provider string         // Provider name (e.g., "posix")
    MTime    time.Time      // Modification time
    Size     int64          // File size in bytes
    Extended map[string]any // Provider-specific metadata
}
```

The `Ensure` field in `CommonResourceState` is set to:
- `present` if a regular file exists
- `directory` if a directory exists
- `absent` if the path does not exist

## Available Providers

| Provider | Platform   | Documentation   |
|----------|------------|-----------------|
| `posix`  | Unix/Linux | [Posix](posix/) |

## Ensure States

| Value       | Description                                        |
|-------------|----------------------------------------------------|
| `present`   | Path must be a regular file with specified content |
| `absent`    | Path must not exist                                |
| `directory` | Path must be a directory                           |

## Content Sources

Files can receive content from two mutually exclusive sources:

| Property   | Description                               |
|------------|-------------------------------------------|
| `contents` | Inline string content (template-resolved) |
| `source`   | Path to local file to copy from           |

```yaml
# Inline content with template
- file:
    - /etc/motd:
        ensure: present
        content: |
          Welcome to {{ lookup('facts.hostname') }}
          Managed by CCM
        owner: root
        group: root
        mode: "0644"

# Copy from source file
- file:
    - /etc/app/config.yaml:
        ensure: present
        source: files/config.yaml
        owner: app
        group: app
        mode: "0640"
```

When using `source`, the path is relative to the manifest's working directory if one is set.

## Required Properties

Unlike some resources, file resources require explicit attributes:

| Property | Required | Description                   |
|----------|----------|-------------------------------|
| `owner`  | Yes      | Username that owns the file   |
| `group`  | Yes      | Group that owns the file      |
| `mode`   | Yes      | Permissions in octal notation |

This prevents accidental creation of files with default or inherited permissions.

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
    ┌───────────────────────┼───────────────────────┐
    │ absent                │ directory             │ present
    ▼                       ▼                       ▼
┌────────────┐      ┌───────────────┐      ┌───────────────┐
│ Remove     │      │ CreateDir     │      │ Store         │
│ (os.Remove)│      │               │      │               │
└────────────┘      └───────────────┘      └───────────────┘
```

## Idempotency

The file resource checks multiple attributes for idempotency:

### State Checks (in order)

1. **Ensure match**: Current type matches desired (`present`/`absent`/`directory`)
2. **Content match**: SHA256 checksum of contents matches (for `ensure: present`)
3. **Owner match**: Current owner matches desired
4. **Group match**: Current group matches desired
5. **Mode match**: Current permissions match desired

### Decision Table

| Desired     | Current State              | Action                          |
|-------------|----------------------------|---------------------------------|
| `absent`    | absent                     | None                            |
| `absent`    | present/directory          | Remove                          |
| `directory` | directory + matching attrs | None                            |
| `directory` | absent/present             | CreateDirectory                 |
| `directory` | directory + wrong attrs    | CreateDirectory (updates attrs) |
| `present`   | present + matching all     | None                            |
| `present`   | absent                     | Store                           |
| `present`   | present + wrong content    | Store                           |
| `present`   | present + wrong attrs      | Store                           |

### Content Comparison

Content is compared using SHA256 checksums:

| Source              | Checksum Method                     |
|---------------------|-------------------------------------|
| `contents` property | `Sha256HashBytes([]byte(contents))` |
| `source` property   | `Sha256HashFile(adjustedPath)`      |
| Existing file       | `Sha256HashFile(filePath)`          |

## Mode Validation

File modes are validated during resource creation:

**Valid Formats:**
- `"0644"` - Standard octal
- `"644"` - Without leading zero
- `"0o755"` - With `0o` prefix
- `"0O700"` - With `0O` prefix

**Validation Rules:**
- Must be valid octal number (digits 0-7)
- Must be ≤ `0777` (no setuid/setgid/sticky via mode)

## Path Validation

File paths must be:
- Absolute (start with `/`)
- Clean (no `.` or `..` components, `filepath.Clean(path) == path`)

```go
if filepath.Clean(p.Name) != p.Name {
    return fmt.Errorf("file path must be absolute")
}
```

## Working Directory

When a manifest has a working directory (e.g., extracted from an archive), the `source` property is resolved relative to it:

```go
if properties.Source != "" && mgr.WorkingDirectory() != "" {
    source = filepath.Join(mgr.WorkingDirectory(), properties.Source)
}
```

This allows manifests bundled with their source files to use relative paths.

## Noop Mode

In noop mode, the file type:
1. Queries current state normally
2. Computes content checksums
3. Logs what actions would be taken
4. Sets appropriate `NoopMessage`:
   - "Would have created the file"
   - "Would have created directory"
   - "Would have removed the file"
5. Reports `Changed: true` if changes would occur
6. Does not call provider Store/CreateDirectory methods
7. Does not remove files

## Desired State Validation

After applying changes (in non-noop mode), the type verifies the file reached the desired state by calling `Status()` again and checking all attributes match. If validation fails, `ErrDesiredStateFailed` is returned.