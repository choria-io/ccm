+++
title = "File"
description = "Manage files content, ownership and more"
toc = true
weight = 30
+++

The file resource manages files and directories, including their content, ownership, and permissions.

> [!info] Warning
> Use absolute file paths and primary group names.

{{< tabs >}}
{{% tab title="Manifest" %}}
```yaml
- file:
    - /etc/motd:
        ensure: present
        content: |
          Managed by CCM {{ now() }}
        owner: root
        group: root
        mode: "0644"
```
{{% /tab %}}
{{% tab title="CLI" %}}
```nohighlight
ccm ensure file /etc/motd --source /tmp/ccm/motd --owner root --group root --mode 0644
```
{{% /tab %}}
{{% tab title="API Request" %}}
```json
{
  "protocol": "io.choria.ccm.v1.resource.ensure.request",
  "type": "file",
  "properties": {
    "name": "/etc/motd",
    "ensure": "present",
    "content": "Managed by CCM\n",
    "owner": "root",
    "group": "root",
    "mode": "0644"
  }
}
```
{{% /tab %}}
{{< /tabs >}}

This copies the contents of `/tmp/ccm/motd` to `/etc/motd` verbatim and sets ownership.

Use `--content` or `--content-file` to parse content through the template engine before writing.

## Ensure Values

| Value       | Description                    |
|-------------|--------------------------------|
| `present`   | The file must exist            |
| `absent`    | The file must not exist        |
| `directory` | The path must be a directory   |

## Properties

| Property   | Description                                                    |
|------------|----------------------------------------------------------------|
| `name`     | Absolute path to the file                                      |
| `ensure`   | Desired state (`present`, `absent`, `directory`)               |
| `content`  | File contents, parsed through the template engine              |
| `source`   | Copy contents from another local file                          |
| `owner`    | File owner (username)                                          |
| `group`    | File group (group name)                                        |
| `mode`     | File permissions in octal notation (e.g., `"0644"`)            |
| `provider` | Force a specific provider (`posix` only)                       |