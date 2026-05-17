+++
title = "Posix Provider"
toc = true
weight = 10
+++

This document describes the implementation details of the Posix file provider for managing files and directories on Unix-like systems.

## Provider Selection

The Posix provider is the default and only file provider. It is always available and returns priority 1 for all file resources.

## Operations

### Store (Create/Update File)

**Process:**

1. Verify parent directory exists
2. Parse file mode from octal string
3. Open source file if `source` property is set
4. Create temporary file in the same directory as target
5. Set file permissions on temp file
6. Write content (from `source` file or `contents` property)
7. Set ownership (chown) on temp file
8. Close temp file
9. Atomic rename temp file to target path

**Atomic Write Pattern:**

```
[parent dir]/<basename>.* (temp file)
    ↓ chmod (set permissions)
    ↓ write content
    ↓ chown (set owner/group)
    ↓ close
    ↓ rename
[parent dir]/<basename> (final file)
```

The temp file is created in the same directory as the target to ensure `os.Rename()` is atomic (same filesystem).

**Content Sources:**

| Property   | Behavior                                                   |
|------------|------------------------------------------------------------|
| `contents` | Write string directly to file (template-resolved)          |
| `source`   | Copy from local file path (adjusted for working directory) |

If both are empty, an empty file is created.

**Error Handling:**

| Condition                      | Behavior                                        |
|--------------------------------|-------------------------------------------------|
| Parent directory doesn't exist | Return error: "is not a directory"              |
| Invalid mode format            | Return error from `strconv.ParseUint`           |
| Source file not found          | Return error from `os.Open`                     |
| Permission denied              | Return error from underlying syscall            |
| Rename failure                 | Return error: "could not rename temporary file" |

### CreateDirectory

**Process:**

1. Parse file mode from octal string
2. Create directory and parents via `os.MkdirAll()`
3. Lookup numeric UID/GID from owner/group names
4. Set permissions via `os.Chmod()` (ensures correct mode even if umask affected MkdirAll)
5. Set ownership via `os.Chown()`

**Command Sequence:**

```go
os.MkdirAll(dir, parsedMode)    // Create directory tree
os.Chmod(dir, parsedMode)        // Ensure correct permissions
os.Chown(dir, uid, gid)          // Set ownership
```

The explicit `Chmod` after `MkdirAll` is necessary because `MkdirAll` applies the process umask to the mode.

### Remove

**Process:**

- When `force` is `true`, the path is removed with `os.RemoveAll()`.
- When `force` is `false`, the path is removed with `os.Remove()`.
- A path that does not exist is treated as a no-op (no error).
- When `os.Remove()` fails with `syscall.ENOTEMPTY`, the error is wrapped with guidance pointing the user at `force: true`.

**Error Handling:**

| Condition                              | Behavior                                                                                       |
|----------------------------------------|------------------------------------------------------------------------------------------------|
| Path does not exist                    | Return `nil`                                                                                   |
| `force: false`, directory not empty    | Return wrapped error: `"cannot remove <path>: directory is not empty, set 'force: true' ..."` |
| Other syscall failure                  | Return error from underlying syscall                                                           |

**Symlink Behavior:**

`os.RemoveAll()` does not follow symlinks during traversal. If the target path is itself a symlink, only the symlink is removed and its target is left intact. A directory tree that contains symlinks to external locations is safe to remove with `force: true`: the symlink entries are unlinked, but the directories they point to are not deleted.

**Context Cancellation:**

`os.RemoveAll()` does not observe `ctx`. Removal of a very large tree cannot be interrupted mid-walk. This is acceptable for typical CCM workloads but should be considered when scheduling removal of large directories.

### Status

**Process:**

1. Initialize state with default metadata
2. Call `os.Stat()` on file path
3. Based on result, populate state accordingly

**State Detection:**

| `os.Stat()` Result | Ensure Value | Metadata                                  |
|--------------------|--------------|-------------------------------------------|
| File exists        | `present`    | Size, mtime, owner, group, mode, checksum |
| Directory exists   | `directory`  | Size, mtime, owner, group, mode           |
| `os.ErrNotExist`   | `absent`     | None                                      |
| `os.ErrPermission` | `absent`     | None (logged as warning)                  |
| Other error        | (unchanged)  | None (logged as warning)                  |

**Metadata Collection:**

| Field      | Source                                              |
|------------|-----------------------------------------------------|
| `Name`     | From properties                                     |
| `Provider` | "posix"                                             |
| `Size`     | `FileInfo.Size()`                                   |
| `MTime`    | `FileInfo.ModTime()`                                |
| `Owner`    | `util.GetFileOwner()` - resolves UID to username    |
| `Group`    | `util.GetFileOwner()` - resolves GID to group name  |
| `Mode`     | `util.GetFileOwner()` - octal string (e.g., "0644") |
| `Checksum` | `util.Sha256HashFile()` - SHA256 hash (files only)  |

**Note:** Checksum is only calculated for regular files, not directories.

## Idempotency

The file resource achieves idempotency by comparing current state against desired state:

### State Checks

The `isDesiredState()` function checks (in order):

1. **Ensure value matches** - `present`, `absent`, or `directory`
2. **Content checksum matches** - SHA256 of contents vs existing file (for files only)
3. **Owner matches** - Owner comparison, normalized through `util.UserIDMatches` so a manifest written with a numeric UID compares equal to a named user with the same UID, and vice versa
4. **Group matches** - Group comparison, normalized through `util.GroupIDMatches` using the same rules
5. **Mode matches** - Octal permission string comparison

### Decision Flow

```
┌─────────────────────────────────────────┐
│ What is the desired ensure state?       │
└─────────────────┬───────────────────────┘
                  │
    ┌─────────────┼─────────────┬─────────────┐
    │ absent      │ directory   │ present     │
    ▼             ▼             ▼             │
┌───────────┐ ┌───────────┐ ┌───────────────┐ │
│ File      │ │ Directory │ │ File exists?  │ │
│ exists?   │ │ exists?   │ │ Content match?│ │
└─────┬─────┘ └─────┬─────┘ │ Owner match?  │ │
      │             │       │ Group match?  │ │
  Yes │ No      Yes │ No    │ Mode match?   │ │
      ▼             ▼       └───────┬───────┘ │
┌─────────┐ ┌───────────┐           │         │
│ Remove  │ │ Stable    │     All Yes│    No   │
│ file    │ │           │           ▼         ▼
└─────────┘ └───────────┘   ┌───────────┐ ┌───────────┐
                            │ Stable    │ │ Store     │
                            └───────────┘ └───────────┘
```

### Content Comparison

For files with `ensure: present`:

| Content Source      | Checksum Calculation                 |
|---------------------|--------------------------------------|
| `contents` property | `Sha256HashBytes([]byte(contents))`  |
| `source` property   | `Sha256HashFile(adjustedSourcePath)` |

The source path is adjusted based on the manager's working directory when set.

## Mode Validation

File modes are validated during resource creation:

1. Strip optional `0o` or `0O` prefix
2. Parse as octal number (base 8)
3. Validate range: must be ≤ `0777`

**Valid Mode Examples:**

| Input     | Parsed Value |
|-----------|--------------|
| `"0644"`  | 0o644        |
| `"644"`   | 0o644        |
| `"0o755"` | 0o755        |
| `"0O700"` | 0o700        |

**Invalid Mode Examples:**

| Input         | Error                                                  |
|---------------|--------------------------------------------------------|
| `"0888"`      | Invalid octal digit                                    |
| `"1777"`      | Exceeds maximum (setuid/setgid not supported via mode) |
| `"rw-r--r--"` | Not octal format                                       |

## Ownership Resolution

Owner and group values are resolved to numeric UID/GID via:

```go
uid, gid, err := util.LookupOwnerGroup(owner, group)
```

Resolution depends on the value's form:

- A purely-numeric value (digits only) is parsed directly as a UID or GID. The system user database is not consulted, matching the semantics of `chown(1)` when given a numeric argument.
- Any other value is resolved through the system user database (`/etc/passwd`, `/etc/group`, or equivalent) using `user.Lookup(owner)` and `user.LookupGroup(group)`.

**Error Handling:**

| Condition                       | Behavior                               |
|---------------------------------|----------------------------------------|
| Empty value                     | Return error: "user name cannot be empty" or "group name cannot be empty" |
| Named user not in database      | Return error: "could not lookup user"  |
| Named group not in database     | Return error: "could not lookup group" |
| Numeric value out of int range  | Return error from `strconv.Atoi`       |

Numeric values are not validated against the user database. A file may be chowned to a UID or GID that has no matching entry, which is intentional for namespaced or container scenarios.

### State Comparison

State comparison reads ownership from the filesystem as the on-disk numeric UID/GID, reverse-resolved to a name when possible by `util.GetFileOwner`. Because the manifest may use either form, comparison is delegated to `util.UserIDMatches` and `util.GroupIDMatches`, which normalize both sides to numeric IDs before comparing. This keeps a manifest stable regardless of which form was chosen.

## Working Directory Support

When a manager has a working directory set (e.g., from extracted manifest), the `source` property path is adjusted:

```go
func (t *Type) adjustedSource(properties *model.FileResourceProperties) string {
    source := properties.Source
    if properties.Source != "" && t.mgr.WorkingDirectory() != "" {
        source = filepath.Join(t.mgr.WorkingDirectory(), properties.Source)
    }
    return source
}
```

This allows manifests to use relative paths for source files bundled with the manifest.

## Platform Support

The Posix provider uses Unix-specific system calls:

| Operation            | System Call                          |
|----------------------|--------------------------------------|
| Get file owner/group | `syscall.Stat_t` (UID/GID from stat) |
| Set ownership        | `os.Chown()` → `chown(2)`            |
| Set permissions      | `os.Chmod()` → `chmod(2)`            |

The provider has separate implementations for Unix and Windows (`file_unix.go`, `file_windows.go` in `internal/util`), with Windows returning errors for ownership operations.

## Security Considerations

### Atomic Writes

Files are written atomically via temp file + rename. This prevents:
- Partial file reads during write
- Corruption if process is interrupted
- Race conditions with concurrent readers

### Permission Ordering

Permissions and ownership are set on the temp file before rename:
1. `Chmod` - Set permissions
2. Write content
3. `Chown` - Set ownership
4. Rename to target

This ensures the file never exists at the target path with incorrect permissions.

### Path Validation

File paths must be absolute and clean (no `.` or `..` components):

```go
if filepath.Clean(p.Name) != p.Name {
    return fmt.Errorf("file path must be absolute")
}
```

### Required Properties

Owner, group, and mode are required properties and cannot be empty, preventing accidental creation of files with default/inherited permissions.