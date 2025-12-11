+++
title = "Resources"
toc = true
weight = 10
pre = "<b>1. </b>"
+++

A resource is how you describe the desired state of your infrastructure. They represent a thing you want to manage and they are backed by providers which implement the actual management for your Operating System.

Each resource has a type and a unique name followed by some resource-specific properties.

## Package

When you manage a package, you describe the stable state you desire. Should the package merely be present, or the latest version, or a specific version?

> [!info] Warning
> You should use real package names, not virtual names, aliases or group names

In a manifest:

```yaml
package:
  name: zsh
  ensure: 5.9
```

On the CLI:

```nohighlight
$ ccm ensure package zsh 5.9
```

### Ensure Values

| Ensure Values |                                                       |
|---------------|-------------------------------------------------------|
| `present`     | The package must be installed.                        |
| `latest`      | The package must be installed and the latest version. |
| `absent`      | The package must be not be installed.                 |
| `5.9`         | The package must be installed and version 5.9.        |    

### Properties

| Ensure Values |                                                            |
|---------------|------------------------------------------------------------|
| `name`        | The resource name match the package name exactly           |
| `ensure`      | The desired state                                          |
| `provider`    | Force a specific provider to be used, only `dnf` supported |

## Service

When you manage a service, you describe the stable state you desire. Unlike packages services have 2 properties that can very - are they enabled to start at boot or should they be running.

> [!info] Warning
> You should use real service names, not virtual names, aliases etc

Additionally, a service can listen to the state changes of another resource, and, should that resource change it can force a restart of the service.

In a manifest:

```yaml
service:
  name: httpd
  ensure: running
  enable: true
  subscribe: package#httpd
```

On the CLI:

```nohighlight
$ ccm ensure service httpd running --enable --subscribe package#httpd
```

### Ensure Values

| Ensure Values |                                                |
|---------------|------------------------------------------------|
| `running`     | The service must be running.                   |
| `stopped`     | The service must be stopped.                   |

### Properties

| Ensure Values      |                                                                                                                 |
|--------------------|-----------------------------------------------------------------------------------------------------------------|
| `name`             | The resource name match the service name exactly                                                                |
| `ensure`           | The desired state                                                                                               |
| `enable` (boolean) | Enables the service to start at boot time                                                                       |
| `subscribe`        | When the service is set to be running, and it's already running, restart it when th referenced resource changes |
| `provider`         | Force a specific provider to be used, only `systemd` supported                                                  |
