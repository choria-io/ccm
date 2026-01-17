+++
title = "CLI Plugins"
toc = true
weight = 10
+++

CCM supports extending the CLI with custom commands using [App Builder](https://choria-io.github.io/appbuilder/index.html). This allows you to create organization-specific workflows that integrate with the `ccm` command.

## Plugin Locations

CCM searches for plugins in two directories:

| Location | Purpose |
|----------|---------|
| `/etc/choria/ccm/plugins/` | System-wide plugins |
| `$XDG_CONFIG_HOME/choria/ccm/plugins/` | User plugins (typically `~/.config/choria/ccm/plugins/`) |

Plugins in the user directory override system plugins with the same name.

## Plugin File Format

Plugin files must be named `<command>-plugin.yaml`. The filename determines the command name:

```
deploy-plugin.yaml    → ccm deploy
backup-plugin.yaml    → ccm backup
myapp-plugin.yaml     → ccm myapp
```

## Basic Plugin Structure

Plugins use App Builder's YAML definition format. Here's a minimal example:

```yaml
# deploy-plugin.yaml
name: deploy
description: Deploy application configuration
commands:
  - name: web
    description: Deploy web server configuration
    type: exec
    command: |
      ccm apply /etc/ccm/manifests/webserver.yaml
```

This creates `ccm deploy web` which applies a manifest.

## Passing Data to Manifests

Plugins can pass data to manifests through environment variables or facts. Both are accessible in manifest templates.

### Using Environment Variables

Environment variables set in the plugin are available in manifests via `{{ lookup("env.VAR_NAME") }}`:

```yaml
# deploy-plugin.yaml
name: deploy
description: Deploy with environment
commands:
  - name: production
    description: Deploy to production
    type: exec
    environment:
      - "DEPLOY_ENV=production"
      - "LOG_LEVEL=warn"
    command: |
      ccm apply /etc/ccm/manifests/app.yaml
```

In the manifest:

```yaml
resources:
  - type: file
    name: /etc/app/config.yaml
    content: |
      environment: {{ lookup("env.DEPLOY_ENV") }}
      log_level: {{ lookup("env.LOG_LEVEL") }}
```

### Using Facts

Pass additional facts using the `--fact` flag. Facts are available via `{{ lookup("facts.KEY") }}`:

```yaml
# deploy-plugin.yaml
name: deploy
description: Deploy with custom facts
commands:
  - name: app
    description: Deploy application
    type: exec
    arguments:
      - name: version
        description: Application version to deploy
        required: true
    command: |
      ccm apply /etc/ccm/manifests/app.yaml --fact app_version={{ .Arguments.version }}
```

In the manifest:

```yaml
resources:
  - type: package
    name: myapp
    ensure: present
    version: "{{ lookup('facts.app_version') }}"
```

## Further Reading

For complete App Builder documentation including all command types, templating features, and advanced options, see the [App Builder documentation](https://choria-io.github.io/appbuilder/index.html).

Since version `0.12.0` of App Builder it has a transform that can invoke CCM Manifests, this combines well with flags, arguments and form wizards to create custom UI's that manage your infrastructure. 