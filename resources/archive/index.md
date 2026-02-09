# Archive

The archive resource downloads and extracts archives from HTTP/HTTPS URLs. It supports tar.gz, tgz, tar, and zip formats.

> [!info] Note
> The archive file path (`name`) must have the same archive type extension as the URL. For example, if the URL ends in `.tar.gz`, the name must also end in `.tar.gz`.

{{< tabs >}}
{{% tab title="Manifest" %}}
```yaml
- archive:
    - /opt/downloads/app-v1.2.3.tar.gz:
        url: https://releases.example.com/app/v1.2.3/app-v1.2.3.tar.gz
        checksum: "a1b2c3d4e5f6..."
        extract_parent: /opt/app
        creates: /opt/app/bin/app
        owner: app
        group: app
        cleanup: true
```
{{% /tab %}}
{{% tab title="CLI" %}}
```nohighlight
ccm ensure archive /opt/downloads/app.tar.gz \
    --url https://releases.example.com/app.tar.gz \
    --extract-parent /opt/app \
    --creates /opt/app/bin/app \
    --owner root --group root
```
{{% /tab %}}
{{% tab title="API Request" %}}
```json
{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "archive",
  "properties": {
    "name": "/opt/downloads/app-v1.2.3.tar.gz",
    "url": "https://releases.example.com/app/v1.2.3/app-v1.2.3.tar.gz",
    "checksum": "a1b2c3d4e5f6...",
    "extract_parent": "/opt/app",
    "creates": "/opt/app/bin/app",
    "owner": "app",
    "group": "app",
    "cleanup": true
  }
}
```
{{% /tab %}}
{{< /tabs >}}


This downloads the archive, extracts it to `/opt/app`, and removes the archive file after extraction. Future runs skip the download if `/opt/app/bin/app` exists.

## Ensure Values

| Value     | Description                       |
|-----------|-----------------------------------|
| `present` | The archive must be downloaded    |
| `absent`  | The archive file must not exist   |

## Properties

| Property         | Description                                                                                   |
|------------------|-----------------------------------------------------------------------------------------------|
| `name`           | Absolute path where the archive will be saved                                                 |
| `url`            | HTTP/HTTPS URL to download the archive from                                                   |
| `checksum`       | Expected SHA256 checksum of the downloaded file                                               |
| `extract_parent` | Directory to extract the archive contents into                                                |
| `creates`        | File path; if this file exists, the archive is not downloaded or extracted                    |
| `cleanup`        | Remove the archive file after successful extraction (requires `extract_parent` and `creates`) |
| `owner`          | Owner of the downloaded archive file (username)                                               |
| `group`          | Group of the downloaded archive file (group name)                                             |
| `username`       | Username for HTTP Basic Authentication                                                        |
| `password`       | Password for HTTP Basic Authentication                                                        |
| `headers`        | Additional HTTP headers to send with the request (map of header name to value)                |
| `provider`       | Force a specific provider (`http` only)                                                       |

## Authentication

The archive resource supports two authentication methods:

**Basic Authentication:**

{{< tabs >}}
{{% tab title="Manifest" %}}
```yaml
- archive:
    - /opt/downloads/private-app.tar.gz:
        url: https://private.example.com/app.tar.gz
        username: deploy
        password: "{{ lookup('data.deploy_password') }}"
        extract_parent: /opt/app
        owner: root
        group: root
```
{{% /tab %}}
{{% tab title="CLI" %}}
```nohighlight
ccm ensure archive /opt/downloads/private-app.tar.gz \
    --url https://private.example.com/app.tar.gz \
    --username deploy \
    --password "{{ lookup('data.deploy_password') }}"
```
{{% /tab %}}
{{% tab title="API Request" %}}
```json
{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "archive",
  "properties": {
    "name": "/opt/downloads/private-app.tar.gz",
    "url": "https://private.example.com/app.tar.gz",
    "username": "deploy",
    "password": "secret",
    "extract_parent": "/opt/app",
    "owner": "root",
    "group": "root"
  }
}
```
{{% /tab %}}
{{< /tabs >}}

**Custom Headers:**

{{< tabs >}}
{{% tab title="Manifest" %}}
```yaml
- archive:
    - /opt/downloads/app.tar.gz:
        url: https://api.example.com/releases/app.tar.gz
        headers:
          Authorization: "Bearer {{ lookup('data.api_token') }}"
          X-Custom-Header: custom-value
        extract_parent: /opt/app
        owner: root
        group: root
```
{{% /tab %}}
{{% tab title="CLI" %}}
```nohighlight
ccm ensure archive /opt/downloads/app.tar.gz \
    --url https://api.example.com/releases/app.tar.gz \
    --headers "Authorization:{{ lookup('data.api_token') }}"
```
{{% /tab %}}
{{% tab title="API Request" %}}
```json
{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "archive",
  "properties": {
    "name": "/opt/downloads/app.tar.gz",
    "url": "https://api.example.com/releases/app.tar.gz",
    "headers": {
      "Authorization": "Bearer mytoken",
      "X-Custom-Header": "custom-value"
    },
    "extract_parent": "/opt/app",
    "owner": "root",
    "group": "root"
  }
}
```
{{% /tab %}}
{{< /tabs >}}

## Idempotency

The archive resource is idempotent through multiple mechanisms:

1. **Checksum verification**: If a `checksum` is provided and the existing file matches, no download occurs.
2. **Creates file**: If `creates` is specified and that file exists, neither download nor extraction occurs.
3. **File existence**: If the archive file exists with matching checksum and owner/group, no changes are made.

For best idempotency, always specify either `checksum` or `creates` (or both).

## Cleanup Behavior

When `cleanup: true` is set:

- The archive file is deleted after successful extraction
- The `extract_parent` property is required
- The `creates` property is required to track extraction state across runs

## Supported Archive Formats

| Extension         | Extraction Tool |
|-------------------|-----------------|
| `.tar.gz`, `.tgz` | `tar -xzf`      |
| `.tar`            | `tar -xf`       |
| `.zip`            | `unzip`         |

> [!info] Note
> The extraction tools (`tar`, `unzip`) must be available in the system PATH.