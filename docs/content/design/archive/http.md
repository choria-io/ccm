+++
title = "HTTP Provider"
toc = true
weight = 10
+++

This document describes the implementation details of the HTTP archive provider for downloading and extracting archives from HTTP/HTTPS URLs.

## Provider Selection

The HTTP provider is selected when:

1. The URL scheme is `http` or `https`
2. The archive file extension is supported (`.tar.gz`, `.tgz`, `.tar`, `.zip`)
3. The required extraction tool (`tar` or `unzip`) is available in PATH

The `IsManageable()` function checks these conditions and returns a priority of 1 if all are met.

## Operations

### Download

**Process:**

1. Parse the URL and add Basic Auth credentials if username/password provided
2. Create HTTP request with custom headers (if specified)
3. Execute GET request via `util.HttpGetResponse()`
4. Verify HTTP 200 status code
5. Create temporary file in the same directory as the target
6. Set ownership on temp file before writing content
7. Copy response body to temp file
8. Verify checksum if provided
9. Atomic rename temp file to target path

**Atomic Write Pattern:**

```
[parent dir]/archive-name-* (temp file)
    ↓ write content
    ↓ set owner/group
    ↓ verify checksum
    ↓ rename
[parent dir]/archive-name (final file)
```

The temp file is created in the same directory as the target to ensure `os.Rename()` is atomic (same filesystem).

**Error Handling:**

| Condition         | Behavior                                                 |
|-------------------|----------------------------------------------------------|
| HTTP non-200      | Return error with status code                            |
| Write failure     | Clean up temp file, return error                         |
| Checksum mismatch | Clean up temp file, return error with expected vs actual |
| Rename failure    | Temp file cleaned up by defer                            |

**Authentication:**

| Method                       | Implementation                                                                  |
|------------------------------|---------------------------------------------------------------------------------|
| Basic Auth                   | URL userinfo is passed to `HttpGetResponse()` which sets `Authorization` header |
| Username/Password properties | Embedded in URL before request: `url.UserPassword(username, password)`          |
| Custom Headers               | Added to request via `http.Header.Add()`                                        |

### Extract

**Process:**

1. Validate `ExtractParent` is set
2. Create `ExtractParent` directory if it doesn't exist (mode 0755)
3. Determine archive type from file extension
4. Execute appropriate extraction command

**Extraction Commands:**

| Extension         | Command                                  |
|-------------------|------------------------------------------|
| `.tar.gz`, `.tgz` | `tar -xzf <archive> -C <extract_parent>` |
| `.tar`            | `tar -xf <archive> -C <extract_parent>`  |
| `.zip`            | `unzip -d <extract_parent> <archive>`    |

**Command Execution:**

Commands are executed via `model.CommandRunner.ExecuteWithOptions()` with:

| Option    | Value                      |
|-----------|----------------------------|
| `Command` | `tar` or `unzip`           |
| `Args`    | Extraction flags and paths |
| `Cwd`     | `ExtractParent` directory  |
| `Timeout` | 1 minute                   |

**Error Handling:**

| Condition             | Behavior                                  |
|-----------------------|-------------------------------------------|
| Unsupported extension | Return "archive type not supported" error |
| Command not found     | Runner returns error                      |
| Non-zero exit code    | Return error with exit code and stderr    |

### Status

**Process:**

1. Initialize state with `EnsureAbsent` default
2. Check if archive file exists via `os.Stat()`
3. If exists: set `EnsurePresent`, populate metadata (size, mtime, owner, group, checksum)
4. If `Creates` property set: check if creates file exists

**Metadata Collected:**

| Field           | Source                                             |
|-----------------|----------------------------------------------------|
| `Name`          | From properties                                    |
| `Provider`      | "http"                                             |
| `ArchiveExists` | `os.Stat()` success                                |
| `Size`          | `FileInfo.Size()`                                  |
| `MTime`         | `FileInfo.ModTime()`                               |
| `Owner`         | `util.GetFileOwner()` - resolves UID to username   |
| `Group`         | `util.GetFileOwner()` - resolves GID to group name |
| `Checksum`      | `util.Sha256HashFile()`                            |
| `CreatesExists` | `os.Stat()` on `Creates` path                      |

## Idempotency

The provider supports idempotency through the type's `isDesiredState()` function:

### State Checks (in order)

1. **Ensure Absent**: If `ensure: absent`, archive must not exist
2. **Creates File**: If `Creates` set and file doesn't exist → not stable
3. **Archive Existence**: If `cleanup: false`, archive must exist
4. **Owner/Group**: Must match properties
5. **Checksum**: If specified in properties, must match

Note: When `cleanup: true`, the `creates` property is required (enforced at validation time).

### Decision Matrix

| Archive Exists | Creates Exists | Checksum Match | Cleanup | Stable?              |
|----------------|----------------|----------------|---------|----------------------|
| No             | No             | N/A            | false   | No (download needed) |
| Yes            | No             | Yes            | false   | No (extract needed)  |
| Yes            | Yes            | Yes            | false   | Yes                  |
| No             | Yes            | N/A            | true    | Yes                  |
| Yes            | Yes            | Yes            | true    | No (cleanup needed)  |

## Checksum Verification

**Algorithm:** SHA-256

**Implementation:**

```go
sum, err := util.Sha256HashFile(tempFile)
if sum != properties.Checksum {
    return fmt.Errorf("checksum mismatch, expected %q got %q", properties.Checksum, sum)
}
```

**Timing:** Checksum is verified after download completes but before the atomic rename. This ensures:
- Corrupted downloads are never placed at the target path
- Temp file is cleaned up on mismatch
- Clear error message with both expected and actual checksums

## Security Considerations

### Credential Handling

- Credentials in URL are redacted in log messages via `util.RedactUrlCredentials()`
- Basic Auth header is set by Go's `http.Request.SetBasicAuth()`, not manually constructed

### Archive Extraction

- Extraction uses system `tar`/`unzip` commands
- No path traversal protection beyond what the tools provide
- `ExtractParent` must be an absolute path (validated in model)

### Temporary Files

- Created with `os.CreateTemp()` using pattern `<archive-name>-*`
- Deferred removal ensures cleanup on all exit paths
- Ownership set before content written

## Platform Support

The provider is Unix-only due to:

- Dependency on `util.GetFileOwner()` which uses syscall for UID/GID resolution
- Dependency on `util.ChownFile()` for ownership management

## Timeouts

| Operation          | Timeout                                 | Configurable |
|--------------------|-----------------------------------------|--------------|
| HTTP Download      | 1 minute (default in `HttpGetResponse`) | No           |
| Archive Extraction | 1 minute                                | No           |

Large archives may require increased timeouts in future versions.