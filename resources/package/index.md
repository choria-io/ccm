# Package

The package resource manages system packages. Specify whether the package should be present, absent, at the latest version, or at a specific version.

> [!info] Warning
> Use real package names, not virtual names, aliases, or group names.

{{< tabs >}}
{{% tab title="Manifest" %}}
```yaml
- package:
    - zsh:
        ensure: "5.9"
```
{{% /tab %}}
{{% tab title="CLI" %}}
```nohighlight
ccm ensure package zsh 5.9
```
{{% /tab %}}
{{% tab title="API Request" %}}
```json
{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "package",
  "properties": {
    "name": "zsh",
    "ensure": "5.9"
  }
}
```
{{% /tab %}}
{{< /tabs >}}

## Ensure Values

| Value       | Description                                     |
|-------------|-------------------------------------------------|
| `present`   | The package must be installed                   |
| `latest`    | The package must be installed at latest version |
| `absent`    | The package must not be installed               |
| `<version>` | The package must be installed at this version   |

## Properties

| Property   | Description                                |
|------------|--------------------------------------------|
| `name`     | Package name                               |
| `ensure`   | Desired state or version                   |
| `provider` | Force a specific provider (`dnf`, `apt`)   |

## Provider Notes

### APT (Debian/Ubuntu)

The APT provider preserves existing configuration files during package installation and upgrades. When a package is upgraded and the maintainer has provided a new version of a configuration file, the existing file is kept (`--force-confold` behavior).

Packages in a partially installed or `config-files` state (removed but configuration remains) are treated as absent. Reinstalling such packages will preserve the existing configuration files.

> [!info] Note
> The provider will not run `apt update` before installing a package. Use an `exec` resource to update the package index if necessary.

The provider runs non-interactively and suppresses prompts from `apt-listbugs` and `apt-listchanges`.