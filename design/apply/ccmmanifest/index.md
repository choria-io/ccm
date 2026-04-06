# CCM Manifest Provider

This document describes the implementation details of the CCM Manifest provider for resolving and executing child manifests.

## Provider Selection

The CCM Manifest provider is the only apply provider. It is always available and returns priority 1 for all apply resources.

## Operations

### ApplyManifest

**Process:**

1. Capture parent state (noop mode, working directory, data)
2. Strengthen noop mode if the parent is in noop mode or the resource has `noop: true`
3. Build execution options for the child manifest
4. Resolve the child manifest via `apply.ResolveManifestUrl()`
5. Execute the resolved manifest via `resolvedApply.Execute()`
6. Inspect child resource outcomes (changed, failed, skipped)
7. Restore parent state via deferred restore

**Noop Strengthening:**

The provider only strengthens noop mode, never weakens it. If the parent manager is already in noop mode, the child inherits that regardless of its own `noop` property. If the parent is not in noop mode and the resource sets `noop: true`, the provider enables noop on the manager before resolution.

| Parent noop | Resource `noop` | Action                          |
|-------------|-----------------|-------------------------------------|
| `true`      | `false`         | No change, parent noop already active |
| `true`      | `true`          | No change, parent noop already active |
| `false`     | `true`          | Enable noop on manager              |
| `false`     | `false`         | No change                           |

Health check mode follows the same strengthening pattern. The effective health check mode is `true` if either the parent or the resource sets it.

**Execute Options:**

The provider builds these options to control child manifest behavior:

| Option                       | Condition              | Purpose                                            |
|------------------------------|------------------------|-----------------------------------------------------|
| `WithSkipSession()`          | Always                 | Reuse parent session instead of creating a new one  |
| `WithCurrentDepth(n)`        | Always                 | Track recursion depth for nested apply resources    |
| `WithOverridingResolvedData` | `data` property is set | Merge resource data into the child's resolved data  |
| `WithDenyApplyResources()`   | `allow_apply` is false | Prevent child from containing apply resources       |

## State Capture and Restore

The provider saves three pieces of manager state before manifest resolution and restores them after execution via `defer`. This ensures restoration runs even if resolution or execution fails.

| Field             | Capture                  | Restore                          |
|-------------------|--------------------------|----------------------------------|
| Noop mode         | `mgr.NoopMode()`         | `mgr.SetNoopMode(saved)`        |
| Working directory | `mgr.WorkingDirectory()` | `mgr.SetWorkingDirectory(saved)` |
| Data              | `mgr.Data()`             | `mgr.SetData(saved)`            |

State capture happens before any resolve or mutation calls. This ordering is critical because `ResolveManifestUrl` mutates the manager's working directory and data during resolution.

Restoration ensures that subsequent resources in the parent manifest see the original manager state. Without it, a child manifest's working directory and data changes would leak into sibling resources.

## Path Resolution

The resource `name` property specifies a file path relative to the parent manifest's directory. During resolution, `ResolveManifestFilePath` joins relative paths with the manager's current working directory before opening the file.

For nested apply resources, each level sets the working directory to its own manifest's parent directory. The state restore ensures the working directory returns to the correct value after each child completes.

```
/opt/ccm/manifest.yaml          WD = /opt/ccm/
  apply: sub/manifest.yaml       resolves to /opt/ccm/sub/manifest.yaml
                                  WD = /opt/ccm/sub/
    apply: lib/manifest.yaml     resolves to /opt/ccm/sub/lib/manifest.yaml
                                  WD = /opt/ccm/sub/lib/
                                  (restore WD to /opt/ccm/sub/)
                                (restore WD to /opt/ccm/)
```

## Child Resource Inspection

After execution, the provider iterates over child resources to count outcomes using the shared session:

| Outcome | Detection method       | Effect                 |
|---------|------------------------|------------------------|
| Failed  | `mgr.IsResourceFailed` | Increment fail count   |
| Changed | `mgr.ShouldRefresh`    | Increment change count |
| Skipped | Neither                | Remainder              |

The provider builds an `ApplyState` with the total resource count and reports the outcome:

| Child result            | Provider behavior                                    |
|-------------------------|------------------------------------------------------|
| All resources succeeded | Log informational message, return state              |
| Some resources changed  | Log warning with counts, return state                |
| Any resource failed     | Log error, return error with failure count           |

## Logging

The provider creates a child user logger with a `manifest` key set to the resource name. All child resource log output includes this key, providing attribution for which parent apply resource triggered the execution.