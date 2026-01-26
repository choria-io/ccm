+++
title = "Archive Type"
toc = true
weight = 10
+++

This document describes the design of the archive resource type for downloading and extracting archives.

## Overview

The archive resource manages remote archives with three phases:
- **Download**: Fetch archive from a URL to local filesystem
- **Extract**: Unpack archive contents to a target directory
- **Cleanup**: Optionally remove the archive file after extraction

These phases are conditional based on current state and configuration.

## Provider Interface

Archive providers must implement the `ArchiveProvider` interface:

```go
type ArchiveProvider interface {
    model.Provider

    Download(ctx context.Context, properties *model.ArchiveResourceProperties, log model.Logger) error
    Extract(ctx context.Context, properties *model.ArchiveResourceProperties, log model.Logger) error
    Status(ctx context.Context, properties *model.ArchiveResourceProperties) (*model.ArchiveState, error)
}
```

### Method Responsibilities

| Method     | Responsibility                                                       |
|------------|----------------------------------------------------------------------|
| `Status`   | Query archive file existence, checksum, attributes, and creates file |
| `Download` | Fetch archive from URL, verify checksum, set ownership               |
| `Extract`  | Unpack archive contents to extract parent directory                  |

### Status Response

The `Status` method returns an `ArchiveState` containing:

```go
type ArchiveState struct {
    CommonResourceState
    Metadata *ArchiveMetadata
}

type ArchiveMetadata struct {
    Name          string    // Archive file path
    Checksum      string    // SHA256 hash of archive
    ArchiveExists bool      // Whether archive file exists
    CreatesExists bool      // Whether creates marker file exists
    Owner         string    // Archive file owner
    Group         string    // Archive file group
    MTime         time.Time // Modification time
    Size          int64     // File size in bytes
    Provider      string    // Provider name (e.g., "http")
}
```

The `Ensure` field in `CommonResourceState` is set to:
- `present` if the archive file exists
- `absent` if the archive file does not exist

## Available Providers

| Provider | Source          | Documentation |
|----------|-----------------|---------------|
| `http`   | HTTP/HTTPS URLs | [HTTP](http/) |

## Ensure States

| Value     | Description                                           |
|-----------|-------------------------------------------------------|
| `present` | Archive must be downloaded (and optionally extracted) |
| `absent`  | Archive file must not exist                           |

## Supported Archive Formats

| Extension         | Description                 |
|-------------------|-----------------------------|
| `.tar.gz`, `.tgz` | Gzip-compressed tar archive |
| `.tar`            | Uncompressed tar archive    |
| `.zip`            | ZIP archive                 |

The URL and local file name must have matching archive type extensions.

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
    │ Remove archive│             │ Download needed?    │
    │ file          │             │ (checksum mismatch  │
    └───────────────┘             │  or file missing)   │
                                  └─────────┬───────────┘
                                        Yes │         No
                                            ▼         │
                                    ┌───────────┐     │
                                    │ Download  │     │
                                    └─────┬─────┘     │
                                          │           │
                                          ▼           ▼
                                  ┌─────────────────────────┐
                                  │ Extract needed?         │
                                  │ (extract_parent set AND │
                                  │  (download occurred OR  │
                                  │   creates file missing))│
                                  └─────────┬───────────────┘
                                        Yes │         No
                                            ▼         │
                                    ┌───────────┐     │
                                    │ Extract   │     │
                                    └─────┬─────┘     │
                                          │           │
                                          ▼           ▼
                                  ┌─────────────────────────┐
                                  │ Cleanup enabled?        │
                                  └─────────┬───────────────┘
                                        Yes │         No
                                            ▼         ▼
                                    ┌───────────┐ ┌───────┐
                                    │ Remove    │ │ Done  │
                                    │ archive   │ └───────┘
                                    └───────────┘
```

## Idempotency

The archive resource uses multiple checks for idempotency:

### State Checks (in order)

1. **Ensure absent**: Archive file must not exist
2. **Creates file**: If `creates` is set, the marker file must exist
3. **Archive existence**: If `cleanup: false`, archive must exist
4. **Owner/Group**: Archive file attributes must match
5. **Checksum**: If specified, archive checksum must match

### Decision Table

| Condition                                       | Stable?                 |
|-------------------------------------------------|-------------------------|
| `ensure: absent` + archive missing              | Yes                     |
| `ensure: absent` + archive exists               | No (remove)             |
| `creates` file exists                           | Yes (skip all)          |
| `creates` file missing                          | No (extract needed)     |
| `cleanup: false` + archive missing              | No (download needed)    |
| Archive checksum mismatch                       | No (re-download needed) |
| Archive owner/group mismatch                    | No (re-download needed) |

## Creates Property

The `creates` property provides idempotency for extraction:

```yaml
- archive:
    - /tmp/app.tar.gz:
        url: https://example.com/app.tar.gz
        extract_parent: /opt/app
        creates: /opt/app/bin/app
        owner: root
        group: root
```

**Behavior:**
- If `/opt/app/bin/app` exists, skip download and extraction
- Useful when extracted files indicate successful prior extraction
- Prevents re-extraction on every run

## Cleanup Property

The `cleanup` property removes the archive after extraction:

```yaml
- archive:
    - /tmp/app.tar.gz:
        url: https://example.com/app.tar.gz
        extract_parent: /opt/app
        creates: /opt/app/bin/app
        cleanup: true
        owner: root
        group: root
```

**Requirements:**
- `extract_parent` must be set (cleanup only makes sense with extraction)
- `creates` must be set to track extraction state

**Behavior:**
- After successful extraction, remove the archive file
- On subsequent runs, `creates` file prevents re-download

## Checksum Verification

When `checksum` is specified:

```yaml
- archive:
    - /tmp/app.tar.gz:
        url: https://example.com/app.tar.gz
        checksum: "a1b2c3d4..."
        owner: root
        group: root
```

**Behavior:**
- Downloaded file is verified against SHA256 checksum
- Existing file checksum is compared to detect changes
- Checksum mismatch triggers re-download
- Download fails if fetched content doesn't match

## Authentication

Archives support two authentication methods:

### Basic Authentication

```yaml
- archive:
    - /tmp/app.tar.gz:
        url: https://private.example.com/app.tar.gz
        username: deploy
        password: "{{ lookup('data.password') }}"
        owner: root
        group: root
```

### Custom Headers

```yaml
- archive:
    - /tmp/app.tar.gz:
        url: https://api.example.com/releases/app.tar.gz
        headers:
          Authorization: "Bearer {{ lookup('data.token') }}"
        owner: root
        group: root
```

## Required Properties

| Property | Required | Description                         |
|----------|----------|-------------------------------------|
| `url`    | Yes      | Source URL for download             |
| `owner`  | Yes      | Username that owns the archive file |
| `group`  | Yes      | Group that owns the archive file    |

## URL Validation

URLs are validated during resource creation:

- Must be valid URL format
- Scheme must be `http` or `https`
- Path must end with supported archive extension
- Extension must match the `name` property extension

## Noop Mode

In noop mode, the archive type:
1. Queries current state normally
2. Computes what actions would be taken
3. Sets appropriate `NoopMessage`:
   - "Would have downloaded"
   - "Would have extracted"
   - "Would have cleaned up"
   - "Would have removed"
4. Reports `Changed: true` if changes would occur
5. Does not call provider Download/Extract methods
6. Does not remove files

Multiple actions are joined with ". " (e.g., "Would have downloaded. Would have extracted").

## Desired State Validation

After applying changes (in non-noop mode), the type verifies the archive reached the desired state by calling `Status()` again and checking all conditions. If validation fails, `ErrDesiredStateFailed` is returned.