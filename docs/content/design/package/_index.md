+++
title = "Package Type"
toc = true
weight = 40
+++

This document describes the design of the package resource type for managing software packages.

## Overview

The package resource manages software packages with two aspects:
- **Existence**: Whether the package is installed or absent
- **Version**: The specific version installed (when applicable)

## Provider Interface

Package providers must implement the `PackageProvider` interface:

```go
type PackageProvider interface {
    model.Provider

    Install(ctx context.Context, pkg string, version string) error
    Upgrade(ctx context.Context, pkg string, version string) error
    Downgrade(ctx context.Context, pkg string, version string) error
    Uninstall(ctx context.Context, pkg string) error
    Status(ctx context.Context, pkg string) (*model.PackageState, error)
    VersionCmp(versionA, versionB string, ignoreTrailingZeroes bool) (int, error)
}
```

### Method Responsibilities

| Method       | Responsibility                                               |
|--------------|--------------------------------------------------------------|
| `Status`     | Query current package state (installed version or absent)    |
| `Install`    | Install package at specified version (or latest if "latest") |
| `Upgrade`    | Upgrade package to specified version                         |
| `Downgrade`  | Downgrade package to specified version                       |
| `Uninstall`  | Remove the package                                           |
| `VersionCmp` | Compare two version strings (-1, 0, 1)                       |

### Status Response

The `Status` method returns a `PackageState` containing:

```go
type PackageState struct {
    CommonResourceState
    Metadata *PackageMetadata
}

type PackageMetadata struct {
    Name        string         // Package name
    Version     string         // Installed version
    Arch        string         // Architecture (e.g., "x86_64")
    License     string         // Package license
    URL         string         // Package URL
    Summary     string         // Short description
    Description string         // Full description
    Provider    string         // Provider name (e.g., "dnf")
    Extended    map[string]any // Provider-specific metadata
}
```

The `Ensure` field in `CommonResourceState` is set to:
- The installed version string if the package is installed
- `absent` if the package is not installed

## Available Providers

| Provider | Package Manager     | Documentation |
|----------|---------------------|---------------|
| `dnf`    | DNF (Fedora/RHEL)   | [DNF](dnf/)   |
| `apt`    | APT (Debian/Ubuntu) | [APT](apt/)   |

## Ensure States

| Value       | Description                                  |
|-------------|----------------------------------------------|
| `present`   | Package must be installed (any version)      |
| `absent`    | Package must not be installed                |
| `latest`    | Package must be upgraded to latest available |
| `<version>` | Package must be at specific version          |

```yaml
# Install any version
- package:
    - vim:
        ensure: present

# Install latest version
- package:
    - vim:
        ensure: latest

# Install specific version
- package:
    - nginx:
        ensure: "1.24.0-1.el9"

# Remove package
- package:
    - telnet:
        ensure: absent
```

## Apply Logic

### Phase 1: Handle Special Cases

```
┌─────────────────────────────────────────┐
│ Get current state via Status()          │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│ Is ensure = "latest"?                   │
└─────────────────┬───────────────────────┘
              Yes │         No
                  ▼         │
    ┌─────────────────────┐ │
    │ Is package absent?  │ │
    └─────────────┬───────┘ │
              Yes │     No  │
                  ▼     ▼   │
          ┌────────┐ ┌────────┐
          │Install │ │Upgrade │
          │latest  │ │latest  │
          └────────┘ └────────┘
                            │
                            ▼
              ┌─────────────────────────┐
              │ Is desired state met?   │
              └─────────────┬───────────┘
                        Yes │         No
                            ▼         │
                    ┌───────────┐     │
                    │ No change │     ▼
                    └───────────┘  (Phase 2)
```

### Phase 2: Handle Ensure Values

```
              ┌─────────────────────────┐
              │ What is desired ensure? │
              └─────────────┬───────────┘
                            │
    ┌───────────────────────┼───────────────────────┐
    │ absent                │ present               │ <version>
    ▼                       ▼                       ▼
┌────────────┐      ┌───────────────┐      ┌───────────────┐
│ Uninstall  │      │ Is absent?    │      │ Is absent?    │
└────────────┘      └───────┬───────┘      └───────┬───────┘
                        Yes │     No           Yes │     No
                            ▼     ▼                ▼     ▼
                    ┌────────┐ ┌────────┐  ┌────────┐ ┌────────────┐
                    │Install │ │No      │  │Install │ │Compare     │
                    │        │ │change  │  │version │ │versions    │
                    └────────┘ └────────┘  └────────┘ └─────┬──────┘
                                                            │
                                           ┌────────────────┼────────────────┐
                                           │ current <      │ current =      │ current >
                                           ▼                ▼                ▼
                                   ┌───────────┐    ┌───────────┐    ┌───────────┐
                                   │ Upgrade   │    │ No change │    │ Downgrade │
                                   └───────────┘    └───────────┘    └───────────┘
```

## Version Comparison

The `VersionCmp` method compares two version strings:

| Return Value | Meaning                                |
|--------------|----------------------------------------|
| `-1`         | versionA < versionB (upgrade needed)   |
| `0`          | versionA == versionB (no change)       |
| `1`          | versionA > versionB (downgrade needed) |

Version comparison is delegated to the provider, allowing platform-specific version parsing (e.g., RPM epoch handling, Debian revision suffixes).

## Idempotency

The package resource is idempotent through state comparison:

### Decision Table

| Desired             | Current State           | Action                |
|---------------------|-------------------------|-----------------------|
| `ensure: present`   | installed (any version) | None                  |
| `ensure: present`   | absent                  | Install               |
| `ensure: absent`    | absent                  | None                  |
| `ensure: absent`    | installed               | Uninstall             |
| `ensure: latest`    | absent                  | Install latest        |
| `ensure: latest`    | installed               | Upgrade (always runs) |
| `ensure: <version>` | same version            | None                  |
| `ensure: <version>` | older version           | Upgrade               |
| `ensure: <version>` | newer version           | Downgrade             |
| `ensure: <version>` | absent                  | Install               |

### Special Case: `ensure: latest`

When `ensure: latest` is used:
- The package manager determines what "latest" means
- Upgrade is always called when the package exists (package manager is idempotent)
- The type cannot verify if "latest" was achieved (package managers may report stale data)
- Desired state validation only checks that the package is not absent

## Package Name Validation

Package names are validated to prevent injection attacks:

**Allowed Characters:**
- Alphanumeric (`a-z`, `A-Z`, `0-9`)
- Period (`.`), underscore (`_`), plus (`+`)
- Colon (`:`), tilde (`~`), hyphen (`-`)

**Rejected:**
- Shell metacharacters (`;`, `|`, `&`, `$`, etc.)
- Whitespace
- Quotes and backticks
- Path separators

Version strings (when ensure is a version) are also validated for dangerous characters.

## Noop Mode

In noop mode, the package type:
1. Queries current state normally
2. Computes version comparison
3. Logs what actions would be taken
4. Sets appropriate `NoopMessage`:
   - "Would have installed latest"
   - "Would have upgraded to latest"
   - "Would have installed version X"
   - "Would have upgraded to X"
   - "Would have downgraded to X"
   - "Would have uninstalled"
5. Reports `Changed: true` if changes would occur
6. Does not call provider Install/Upgrade/Downgrade/Uninstall methods

## Desired State Validation

After applying changes (in non-noop mode), the type verifies the package reached the desired state:

```go
func (t *Type) isDesiredState(properties, state) bool {
    switch properties.Ensure {
    case "present":
        // Any installed version is acceptable
        return state.Ensure != "absent"

    case "absent":
        return state.Ensure == "absent"

    case "latest":
        // Cannot verify "latest", just check not absent
        return state.Ensure != "absent"

    default:
        // Specific version must match
        return VersionCmp(state.Ensure, properties.Ensure, false) == 0
    }
}
```

If the desired state is not reached, an `ErrDesiredStateFailed` error is returned.