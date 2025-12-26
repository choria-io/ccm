![Choria Configuration Manager](https://raw.githubusercontent.com/choria-io/ccm/refs/heads/main/docs/static/logo.png)

## Overview

This is an experimental Configuration Management System designed to manage software from CLI, Shell Script, Manifests, Choria Autonomous Agents and more

 * [Documentation](https://choria-io.github.io/ccm/)
 * [Community](https://github.com/choria-io/ccm/discussions)
 * [Code of Conduct](https://github.com/choria-io/.github/blob/master/CODE_OF_CONDUCT.md)
 * [Contribution Guide](https://github.com/choria-io/.github/blob/master/CONTRIBUTING.md)

## Status

This is an experimental work in progress, there are only `exec`, `service`, `file` and `package` resources implemented so far, they support only dnf and systemd.

It also includes a new implementation of Hiera that is focused on a single file.

We've got the CLI/shell interaction to a quite mature state, next we're exploring network related features and deeper monitoring integration.

At this point we think the idea has legs and will keep working on it. Keeping in mind the minimal focus of this is to deliver something that can do package-config-service style deployments and as such will have minimal resource types (file, package, service and exec are the current targets).

We’re also only likely to support only the key Linux distros in common use.