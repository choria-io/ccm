# Scaffold

The scaffold resource renders files from a source template directory to a target directory. Templates have access to facts and Hiera data, enabling dynamic configuration generation from directory structures.

> [!info] Warning
> Target paths must be absolute and canonical (no `.` or `..` components).

{{< tabs >}}
{{% tab title="Manifest" %}}
```yaml
- scaffold:
    - /etc/app:
        ensure: present
        source: templates/app
        engine: jet
        purge: true
```
{{% /tab %}}
{{% tab title="CLI" %}}
```nohighlight
ccm ensure scaffold /etc/app templates/app --engine jet --purge
```
{{% /tab %}}
{{% tab title="API Request" %}}
```json
{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "scaffold",
  "properties": {
    "name": "/etc/app",
    "ensure": "present",
    "source": "templates/app",
    "engine": "jet",
    "purge": true
  }
}
```
{{% /tab %}}
{{< /tabs >}}

This renders templates from the `templates/app` directory into `/etc/app` using the Jet template engine, removing any files in the target not present in the source.

> [!tip] Tip
> This is implemented using the [github.com/choria-io/scaffold](https://github.com/choria-io/scaffold) Go library, you can use this in your own projects or use the included `scaffold` CLI tool.

## Ensure Values

| Value     | Description                                                      |
|-----------|------------------------------------------------------------------|
| `present` | Target directory must exist with rendered template files         |
| `absent`  | Managed files must be removed; target directory removed if empty |

## Properties

| Property          | Description                                                                |
|-------------------|----------------------------------------------------------------------------|
| `name`            | Absolute path to the target directory                                      |
| `source`          | Source template directory path (relative to working directory or absolute) |
| `engine`          | Template engine: `go` or `jet` (default: `jet`)                            |
| `skip_empty`      | Do not create empty files in rendered output                               |
| `left_delimiter`  | Custom left template delimiter                                             |
| `right_delimiter` | Custom right template delimiter                                            |
| `purge`           | Remove files in target not present in source                               |
| `post`            | Post-processing commands: glob pattern to command mapping                  |
| `provider`        | Force a specific provider (`choria` only)                                  |

## Template Engines

Two template engines are supported:

| Engine | Library            | Default Delimiters | Description           |
|--------|--------------------|--------------------|-----------------------|
| `go`   | Go `text/template` | `{{` / `}}`        | Standard Go templates |
| `jet`  | Jet templating     | `[[` / `]]`        | Jet template language |

The engine defaults to `jet` if not specified. Delimiters can be customized via `left_delimiter` and `right_delimiter`.

## Custom Delimiters

{{< tabs >}}
{{% tab title="Manifest" %}}
```yaml
- scaffold:
    - /etc/myservice:
        ensure: present
        source: templates/myservice
        engine: go
        left_delimiter: "<<"
        right_delimiter: ">>"
```
{{% /tab %}}
{{% tab title="CLI" %}}
```nohighlight
ccm ensure scaffold /etc/myservice templates/myservice \
    --engine go --left-delimiter "<<" --right-delimiter ">>"
```
{{% /tab %}}
{{% tab title="API Request" %}}
```json
{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "scaffold",
  "properties": {
    "name": "/etc/myservice",
    "ensure": "present",
    "source": "templates/myservice",
    "engine": "go",
    "left_delimiter": "<<",
    "right_delimiter": ">>"
  }
}
```
{{% /tab %}}
{{< /tabs >}}

## Post-Processing

The `post` property defines commands to run on rendered files. Each entry is a map where the key is a **glob pattern** matched against the file's basename and the value is a **command** to execute. Use `{}` in the command as a placeholder for the file's full path; if omitted, the path is appended as the last argument.

{{< tabs >}}
{{% tab title="Manifest" %}}
```yaml
- scaffold:
    - /opt/app:
        ensure: present
        source: templates/app
        post:
          - "*.go": "go fmt {}"
          - "*.sh": "chmod +x {}"
```
{{% /tab %}}
{{% tab title="CLI" %}}
```nohighlight
ccm ensure scaffold /opt/app templates/app \
    --post "*.go"="go fmt {}" --post "*.sh"="chmod +x {}"
```
{{% /tab %}}
{{% tab title="API Request" %}}
```json
{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "scaffold",
  "properties": {
    "name": "/opt/app",
    "ensure": "present",
    "source": "templates/app",
    "post": [
      {"*.go": "go fmt {}"},
      {"*.sh": "chmod +x {}"}
    ]
  }
}
```
{{% /tab %}}
{{< /tabs >}}

Post-processing runs immediately after each file is rendered. Files skipped due to `skip_empty` are not post-processed.

## Purge Behavior

When `purge: true` is set, files in the target directory that are not present in the source template directory are deleted during rendering. In noop mode, these deletions are logged but not performed.

When `purge` is disabled (the default), files not present in the source are tracked but not removed. They do not affect idempotency checks for `ensure: present`, meaning the resource is considered stable even if extra files exist in the target.

## Removal Behavior

When `ensure: absent`, only managed files (changed and stable) are removed. Files not belonging to the scaffold (purged files) are left untouched. After removing managed files and empty subdirectories, the target directory itself is removed on a best-effort basis — it is only deleted if empty. If unrelated files remain, the directory is preserved and no error is raised.

## Idempotency

The scaffold resource determines idempotency by rendering templates in noop mode and comparing results against the target directory.

For `ensure: present`:
- **Changed files**: Files that would be created or modified. Any changed files make the resource unstable.
- **Stable files**: Files whose content matches the rendered output. At least one stable file must exist for the resource to be considered stable.
- **Purged files**: Files in the target not present in the source. These only affect stability when `purge` is enabled.

For `ensure: absent`, the status check filters `Changed` and `Stable` lists to only include files that actually exist on disk. This means after a successful removal, the scaffold is considered absent even if the target directory still exists with unrelated files. Purged files never affect the absent stability check.

## Source Resolution

The `source` property is resolved relative to the manager's working directory when it is a relative path. URL sources (with a scheme) are passed through unchanged. This allows manifests bundled with template directories to use relative paths.

## Template Environment

Templates receive the full template environment, which provides access to:
- `facts` - System facts for the managed node
- `data` - Hiera-resolved configuration data
- Template helper functions

## Creating Scaffolds

A scaffold source is a directory tree where every file is a template. The directory structure is mirrored directly into the target, so the source layout becomes the output layout.

### Source Directory Structure

```
templates/app/
├── _partials/
│   └── header.conf
├── config.yaml
├── scripts/
│   └── setup.sh
└── README.md
```

This renders into:

```
/etc/app/
├── config.yaml
├── scripts/
│   └── setup.sh
└── README.md
```

The `_partials` directory is special — its contents are available to templates but are never copied to the target.

### Template Syntax

Every file in the source directory is processed as a template. The syntax depends on the engine selected.

**Jet engine** (default, `[[` / `]]` delimiters):

```
# config.yaml
hostname: [[ facts.hostname ]]
environment: [[ data.environment ]]
workers: [[ data.worker_count ]]
```

**Go engine** (`{{` / `}}` delimiters):

```
# config.yaml
hostname: {{ .facts.hostname }}
environment: {{ .data.environment }}
workers: {{ .data.worker_count }}
```

The Jet engine is the default because its `[[` / `]]` delimiters avoid conflicts with configuration files that use curly braces (YAML, JSON, systemd units). Use the Go engine when you need access to [Sprig functions](https://masterminds.github.io/sprig/).

### Partials

Files inside a `_partials` directory are reusable template fragments. They are rendered on demand using the `render` function but are excluded from the output.

This is useful for shared headers, repeated configuration blocks, or any content used across multiple files.

**Jet:**

```
[[ render("_partials/header.conf", .) ]]

server {
    listen [[ data.port ]];
}
```

**Go:**

```
{{ render "_partials/header.conf" . }}

server {
    listen {{ .data.port }};
}
```

### Built-in Functions

Two functions are available in both template engines:

**`render`** evaluates another template file from the source directory and returns its output as a string. The partial is rendered using the same engine and data as the calling template.

{{< tabs >}}
{{% tab title="Jet" %}}
```
[[ render("_partials/database.conf", .) ]]
```
{{% /tab %}}
{{% tab title="Go" %}}
```
{{ render "_partials/database.conf" . }}
```
{{% /tab %}}
{{< /tabs >}}

**`write`** creates an additional file in the target directory from within a template. This is useful for dynamically generating files based on data — for example, creating one configuration file per service.

{{< tabs >}}
{{% tab title="Jet" %}}
```
[[ write("extra.conf", "generated content") ]]
```
{{% /tab %}}
{{% tab title="Go" %}}
```
{{ write "extra.conf" "generated content" }}
```
{{% /tab %}}
{{< /tabs >}}

### Sprig Functions

When using the Go template engine, all [Sprig](https://masterminds.github.io/sprig/) template functions are available. These provide string manipulation, math, date formatting, list operations, and more:

```
# Go engine example with Sprig functions
hostname: {{ .facts.hostname | upper }}
packages: {{ join ", " .data.packages }}
generated: {{ now | date "2006-01-02" }}
```

### Example Scaffold

A complete scaffold for an application configuration:

**Source structure:**

```
templates/myapp/
├── _partials/
│   └── logging.conf
├── myapp.conf
└── scripts/
    └── healthcheck.sh
```

**`_partials/logging.conf`** (Jet):
```
log_level = [[ data.log_level ]]
log_file = /var/log/myapp/[[ facts.hostname ]].log
```

**`myapp.conf`** (Jet):
```
[[ render("_partials/logging.conf", .) ]]

[server]
bind = 0.0.0.0
port = [[ data.port ]]
workers = [[ data.workers ]]
```

**`scripts/healthcheck.sh`** (Jet):
```
#!/bin/bash
curl -sf http://localhost:[[ data.port ]]/health || exit 1
```

**Manifest using this scaffold:**

```yaml
- scaffold:
    - /etc/myapp:
        ensure: present
        source: templates/myapp
        purge: true
        post:
          - "*.sh": "chmod +x {}"
```

With facts `{"hostname": "web01"}` and data `{"port": 8080, "workers": 4, "log_level": "info"}`, this renders:

```
/etc/myapp/
├── myapp.conf
└── scripts/
    └── healthcheck.sh
```

Where `myapp.conf` contains:

```
log_level = info
log_file = /var/log/myapp/web01.log

[server]
bind = 0.0.0.0
port = 8080
workers = 4
```