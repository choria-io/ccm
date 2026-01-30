# DNF Provider

This document describes the implementation details of the DNF package provider for RHEL/Fedora-based systems.

## Concurrency

A global package lock (`model.PackageGlobalLock`) is held during all command executions to prevent concurrent dnf/rpm operations within the same process. This prevents lock contention on the RPM database.

## Operations

### Status Check

**Command:**
```
rpm -q <package> --queryformat '%{NAME} %|EPOCH?{%{EPOCH}}:{0}| %{VERSION} %{RELEASE} %{ARCH}'
```

**Query Format (NEVRA):**
The query format extracts the full NEVRA (Name, Epoch, Version, Release, Architecture):

| Field                       | Description                         |
|-----------------------------|-------------------------------------|
| `%{NAME}`                   | Package name                        |
| `%\|EPOCH?{%{EPOCH}}:{0}\|` | Epoch (0 if not set)                |
| `%{VERSION}`                | Upstream version                    |
| `%{RELEASE}`                | Release/build number                |
| `%{ARCH}`                   | Architecture (x86_64, noarch, etc.) |

**Example output:**
```
zsh 0 5.8 9.el9 x86_64
```

**Behavior:**
- Exit code 0 → Package is present, parses NEVRA components
- Exit code non-zero → Package is absent

**Returned Version Format:**
The version returned combines VERSION and RELEASE: `5.8-9.el9`

The epoch and release are stored separately in the `Extended` metadata.

### Install

**Ensure Present or Latest:**
```
dnf install -y <package>
```

**Specific Version:**
```
dnf install -y <package>-<version>
```

**Flags:**

| Flag | Purpose                   |
|------|---------------------------|
| `-y` | Assume yes to all prompts |

**Note:** DNF uses `-` (hyphen) to separate package name from version, unlike APT which uses `=`.

### Upgrade

Delegates to `Install()`. DNF's install command handles upgrades automatically when a newer version is available or specified.

### Downgrade

**Command:**
```
dnf downgrade -y <package>-<version>
```

**Note:** Unlike upgrade, downgrade uses a dedicated DNF command rather than delegating to install.

### Uninstall

**Command:**
```
dnf remove -y <package>
```

## Version Format

RPM versions follow the EVR (Epoch:Version-Release) format:

```
[epoch:]version-release
```

| Component | Required | Description                                   |
|-----------|----------|-----------------------------------------------|
| `epoch`   | No       | Integer, default 0. Higher epoch always wins. |
| `version` | Yes      | Upstream version number                       |
| `release` | Yes      | Distribution-specific release/build number    |

**Examples:**
- `5.8-9.el9` → epoch=0, version=5.8, release=9.el9
- `1:2.0-3.fc39` → epoch=1, version=2.0, release=3.fc39
- `0:1.0.0-1.el9` → epoch=0 (explicit), version=1.0.0, release=1.el9

## Version Comparison

Version comparison uses a generic algorithm ported from Puppet, implemented in `internal/util.VersionCmp()`.

### Algorithm

The version string is tokenized into segments by splitting on `-`, `.`, digits, and non-digit sequences. Segments are compared left-to-right:

1. **Hyphens (`-`)** - A hyphen loses to any other character
2. **Dots (`.`)** - A dot loses to any non-hyphen character
3. **Digit sequences** - Compared numerically, except:
   - Leading zeros trigger lexical comparison (`01` vs `1` compared as strings)
4. **Non-digit sequences** - Compared lexically (case-insensitive)

If all segments match, falls back to whole-string comparison.

### Trailing Zero Normalization

When `ignoreTrailingZeroes` is enabled, trailing `.0` segments before the first `-` are removed:
- `1.0.0-rc1` → `1-rc1`
- `2.0.0` → `2`

This allows `1.0.0` to equal `1.0` when the flag is set.

### Comparison Examples

| A       | B       | Result | Reason                           |
|---------|---------|--------|----------------------------------|
| `1.0`   | `2.0`   | A < B  | Numeric: 1 < 2                   |
| `1.10`  | `1.9`   | A > B  | Numeric: 10 > 9                  |
| `1.0`   | `1.0.1` | A < B  | A exhausted first                |
| `1.0-1` | `1.0-2` | A < B  | Release comparison               |
| `1.0a`  | `1.0b`  | A < B  | Lexical: a < b                   |
| `01`    | `1`     | A < B  | Leading zero: lexical comparison |
| `1.0.0` | `1.0`   | A > B  | Without normalization            |
| `1.0.0` | `1.0`   | A = B  | With `ignoreTrailingZeroes=true` |

### Implementation

The version comparison is implemented in `internal/util/util.go` and provides:

- `VersionCmp(a, b, ignoreTrailingZeroes)` - Compare two version strings
- Returns -1 (a < b), 0 (a == b), or 1 (a > b)
