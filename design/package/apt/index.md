# APT Provider

This document describes the implementation details of the APT package provider for Debian-based systems.

## Environment

All commands are executed with the following environment variables to ensure non-interactive operation:

| Variable                   | Value            | Purpose                                     |
|----------------------------|------------------|---------------------------------------------|
| `DEBIAN_FRONTEND`          | `noninteractive` | Prevents dpkg from prompting for user input |
| `APT_LISTBUGS_FRONTEND`    | `none`           | Suppresses apt-listbugs prompts             |
| `APT_LISTCHANGES_FRONTEND` | `none`           | Suppresses apt-listchanges prompts          |

## Concurrency

A global package lock (`model.PackageGlobalLock`) is held during all command executions to prevent concurrent apt/dpkg operations within the same process. This prevents lock contention on `/var/lib/dpkg/lock`.

## Operations

### Status Check

**Command:**
```
dpkg-query -W -f='${Package} ${Version} ${Architecture} ${db:Status-Status}' <package>
```

**Behavior:**
- Exit code 0 with `installed` status → Package is present, returns version info
- Exit code non-zero OR status not `installed` → Package is absent

**Package States:**
The `db:Status-Status` field can return various values. Only `installed` is treated as present:

| Status            | Treated As | Description                     |
|-------------------|------------|---------------------------------|
| `installed`       | Present    | Package fully installed         |
| `config-files`    | Absent     | Removed but config files remain |
| `half-installed`  | Absent     | Installation started but failed |
| `half-configured` | Absent     | Configuration failed            |
| `unpacked`        | Absent     | Unpacked but not configured     |
| `not-installed`   | Absent     | Not installed                   |

Treating non-`installed` states as absent allows `apt-get install` to repair broken installations.

### Install

**Ensure Present:**
```
apt-get install -y -q -o DPkg::Options::=--force-confold <package>
```

**Ensure Latest:**
```
apt-cache policy <package>                    # Get candidate version
apt-get install -y -q -o DPkg::Options::=--force-confold <package>=<version>
```

**Specific Version:**
```
apt-get install -y -q -o DPkg::Options::=--force-confold --allow-downgrades <package>=<version>
```

**Flags:**

| Flag                                 | Purpose                                                 |
|--------------------------------------|---------------------------------------------------------|
| `-y`                                 | Assume yes to prompts                                   |
| `-q`                                 | Quiet output                                            |
| `-o DPkg::Options::=--force-confold` | Keep existing config files on upgrade                   |
| `--allow-downgrades`                 | Allow installing older versions (specific version only) |

### Upgrade

Delegates to `Install()` with the target version. The `--allow-downgrades` flag is only added for specific version requests, not for `latest`.

### Downgrade

Delegates to `Install()` with the target version. The `--allow-downgrades` flag enables this operation.

### Uninstall

**Command:**
```
apt-get -q -y remove <package>
```

**Note:** Uses `remove` not `purge`, so configuration files are preserved. A subsequent install will find existing config files.

### Latest Available Version

**Command:**
```
apt-cache policy <package>
```

**Parsing:** Extracts the `Candidate:` line from output.

**Example output:**
```
zsh:
  Installed: 5.9-8+b18
  Candidate: 5.9-8+b18
  Version table:
 *** 5.9-8+b18 500
        500 http://deb.debian.org/debian trixie/main amd64 Packages
        100 /var/lib/dpkg/status
```

## Version Comparison

Version comparison follows the [Debian Policy Manual](https://www.debian.org/doc/debian-policy/ch-controlfields.html#version) algorithm.

### Version Format

```
[epoch:]upstream_version[-debian_revision]
```

| Component          | Required | Description                                   |
|--------------------|----------|-----------------------------------------------|
| `epoch`            | No       | Integer, default 0. Higher epoch always wins. |
| `upstream_version` | Yes      | The main version from upstream                |
| `debian_revision`  | No       | Debian-specific revision                      |

**Examples:**
- `1.0` → epoch=0, upstream=1.0, revision=""
- `1:2.0-3` → epoch=1, upstream=2.0, revision=3
- `2:1.0.0+git-20190109-0ubuntu2` → epoch=2, upstream=1.0.0+git-20190109, revision=0ubuntu2

### Comparison Algorithm

1. **Compare epochs numerically** - Higher epoch wins regardless of other components

2. **Compare upstream_version and debian_revision** using the Debian string comparison:

   The string is processed left-to-right in segments:

   a. **Tildes (`~`)** - Compared first. More tildes = earlier version. Tilde sorts before everything, even empty string.
      - `1.0~alpha` < `1.0` (tilde before empty)
      - `1.0~~` < `1.0~` (more tildes = earlier)

   b. **Letters (`A-Za-z`)** - Compared lexically (ASCII order)
      - Letters sort before non-letters

   c. **Non-letters (`.`, `+`, `-`)** - Compared lexically

   d. **Digits** - Compared numerically (not lexically)
      - `9` < `13` (numeric comparison)

   These steps repeat until a difference is found or both strings are exhausted.

### Comparison Examples

| A           | B          | Result | Reason                          |
|-------------|------------|--------|---------------------------------|
| `1.0`       | `2.0`      | A < B  | Numeric comparison              |
| `1:1.0`     | `2.0`      | A > B  | Epoch 1 > epoch 0               |
| `1.0~alpha` | `1.0`      | A < B  | Tilde sorts before empty        |
| `1.0~alpha` | `1.0~beta` | A < B  | Lexical: alpha < beta           |
| `1.0.1`     | `1.0.2`    | A < B  | Numeric: 1 < 2                  |
| `1.0-1`     | `1.0-2`    | A < B  | Revision comparison             |
| `1.0a`      | `1.0-`     | A < B  | Letters sort before non-letters |

### Implementation

The version comparison is implemented in `version.go`, ported from Puppet's `Puppet::Util::Package::Version::Debian` module. It provides:

- `ParseVersion(string)` - Parse a version string into components
- `CompareVersionStrings(a, b)` - Compare two version strings directly
- `Version.Compare(other)` - Compare parsed versions (-1, 0, 1)
- Helper methods: `LessThan`, `GreaterThan`, `Equal`, etc.