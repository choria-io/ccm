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

| Property          | Description                                                                                                                                                                                                                          |
|-------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `name`            | Absolute path to the file                                                                                                                                                                                                            |
| `ensure`          | Desired state (`present`, `absent`, `directory`)                                                                                                                                                                                     |
| `content`         | File contents, parsed through the template engine                                                                                                                                                                                    |
| `source`          | Copy contents from another local file                                                                                                                                                                                                |
| `owner`           | File owner as a username, or a numeric UID (a purely-numeric value is always interpreted as a UID). Required unless `ensure: absent`                                                                                                 |
| `group`           | File group as a group name, or a numeric GID (a purely-numeric value is always interpreted as a GID). Required unless `ensure: absent`                                                                                               |
| `mode`            | File permissions in octal notation (e.g., `"0644"`). For directories, the execute bit is added automatically to any permission triad that has read or write bits (e.g., `"0644"` becomes `"0755"`). Required unless `ensure: absent` |
| `force` (boolean) | Allow `ensure: absent` to remove non-empty directories. Has no effect on regular files. Only valid with `ensure: absent`                                                                                                             |
| `provider`        | Force a specific provider (`posix` only)                                                                                                                                                                                             |

## Manage attributes only

Omitting both `content` and `source` puts the resource in attribute-only mode. The file's contents are left untouched and only `owner`, `group`, and `mode` are enforced. This is useful when another resource produces the file and CCM is responsible for its permissions.

```yaml
- file:
    - /etc/sysconfig/myapp:
        ensure: present
        owner: root
        group: root
        mode: "0640"
```

> [!info] Note
> If the file does not exist, an empty file is created with the requested attributes. To create an explicit empty file in any other context, set `content: ""`. A symlink at `name` is rejected to avoid mutating the target through the link.

## Removal

When `ensure: absent`, the file or directory at `name` is removed. The `owner`, `group`, and `mode` properties describe a desired on-disk state and are not consulted during removal, so they may be omitted.

```yaml
- file:
    - /tmp/leftover.lock:
        ensure: absent
```

By default, removing a directory fails if it is not empty. Set `force: true` to remove it anyway:

```yaml
- file:
    - /var/lib/myapp:
        ensure: absent
        force: true
```

* `force` is only valid with `ensure: absent`. Any other ensure value is rejected at validation time
* `force: true` cannot be used with `name: /`
* `force` has no effect on regular files or symlinks
* When the target is a symlink to a directory, only the symlink is removed; the target directory is left untouched
* Without `force`, removing a non-empty directory fails with a hint that `force: true` is required. Every apply that removes a non-empty directory must opt in

## Numeric owner and group

`owner` and `group` accept either a name or a numeric ID. A value composed entirely of digits is always treated as a UID or GID, even when a user or group with that literal name exists. The kernel does not require a matching entry in `/etc/passwd` or `/etc/group`, which is useful for container images or for files owned by accounts that only exist in another namespace.

```yaml
- file:
    - /var/lib/app/data:
        ensure: directory
        owner: "1000"
        group: "1000"
        mode: "0750"
```

State comparison normalizes both sides before comparing, so a manifest written with `"1000"` is stable against a file owned by a named user whose UID is 1000.