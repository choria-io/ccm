# File

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

This creates `/etc/motd` with the given content, parsed through the template engine, and sets ownership and permissions.

## Ensure values

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
| `mode`     | File permissions in octal notation (e.g., `"0644"`). For directories, the execute bit is added automatically to any permission triad that has read or write bits (e.g., `"0644"` becomes `"0755"`) |
| `provider` | Force a specific provider (`posix` only)                       |