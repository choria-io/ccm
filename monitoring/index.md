# Monitoring

By default, CCM verifies resource health using the resource's native state. For example, if a service should be running and `systemd` reports it as running, CCM considers it healthy.

For deeper validation, all resources support custom health checks. These checks run after a resource is managed and can verify that the resource is functioning correctly, not just present.

## Health Check Properties

| Property     | Description                                             | Default           |
|--------------|---------------------------------------------------------|-------------------|
| `command`    | Command to execute (Nagios-style check)                 | -                 |
| `goss_rules` | Inline [Goss](https://goss.readthedocs.io) validation rules | -            |
| `name`       | Name for logging and metrics                            | Command base name |
| `tries`      | Number of attempts before failing                       | 1                 |
| `try_sleep`  | Duration to wait between retry attempts                 | 1s                |
| `timeout`    | Maximum time for command execution                      | No timeout        |
| `format`     | Output format interpretation                            | Auto-detected     |

Each health check must specify either `command` or `goss_rules` -- they are mutually exclusive. The `format` is auto-detected based on which field is set (`nagios` for `command`, `goss` for `goss_rules`), but can be overridden explicitly.

## Nagios Format

Health checks using `command` follow Nagios plugin conventions for exit codes:

| Exit Code | Status     | Description                                    |
|-----------|------------|------------------------------------------------|
| 0         | OK         | Check passed                                   |
| 1         | WARNING    | Check passed with warnings (non-critical)      |
| 2         | CRITICAL   | Check failed                                   |
| 3+        | UNKNOWN    | Check could not determine status               |

### Example

{{< tabs >}}
{{% tab title="Manifest" %}}
```yaml
- service:
    - httpd:
        ensure: running
        enable: true
        health_checks:
          - name: check_http
            command: |
              /usr/lib64/nagios/plugins/check_http -H localhost -p 80 --expect "Acme Inc"
            tries: 5
            try_sleep: 1s
            timeout: 10s
```
{{% /tab %}}
{{% tab title="CLI" %}}
```nohighlight
ccm ensure service httpd running \
    --check '/usr/lib64/nagios/plugins/check_http -H localhost -p 80' \
    --check-tries 5 \
    --check-sleep 1s
```
The CLI supports a single health check per resource. For multiple health checks, use a manifest.
{{% /tab %}}
{{< /tabs >}}

This example verifies that the web server responds with content containing "Acme Inc". If the check fails, it retries up to 5 times with 1 second between attempts.

## Goss Format

Health checks using `goss_rules` embed [Goss](https://goss.readthedocs.io) validation rules directly in the manifest. This allows you to validate system state -- running services, listening ports, file contents, HTTP responses, and more -- without writing external check scripts.

The check result is OK when all Goss rules pass, or CRITICAL when any rule fails. See the [Goss documentation](https://goss.readthedocs.io) for the full list of supported resource types and matchers.

> [!tip]Supported Version 
> Added in version `0.1.0`

### Example

```yaml
- service:
    - httpd:
        ensure: running
        enable: true
        health_checks:
          - name: httpd_health
            goss_rules:
              service:
                httpd:
                  running: true
                  enabled: true
              port:
                tcp:80:
                  listening: true
              http:
                http://localhost:
                  status: 200
                  body:
                    - Acme Inc
            tries: 5
            try_sleep: 1s
```

This validates that the httpd service is running and enabled, port 80 is listening, and the web server responds with a 200 status containing "Acme Inc".

### Templates in Goss Rules

Goss rules are processed through CCM's own template engine before evaluation. This means you can use the standard `{{ }}` expression syntax with access to `Facts`, `Data`, and `Environ`, as well as the `lookup()`, `template()`, and `jet()` functions. See the [Data](../templates/) section for full details on template syntax.

```yaml
health_checks:
  - name: app_check
    goss_rules:
      http:
        "http://localhost:{{ Data.app_port }}/health":
          status: 200
          body:
            - "{{ lookup('data.expected_response', 'ok') }}"
      file:
        "{{ Data.config_path }}":
          exists: true
```

For more complex rules, you can use the `template()` function with Jet templates:

```yaml
health_checks:
  - name: app_check
    goss_rules:
      http:
        "http://localhost:{{ template('check_url.jet') }}":
          status: 200
```

> [!note]
> CCM resolves templates before passing rules to Goss. Goss's own template variables are not used.

## Agent Integration

When running the [Agent](../agent/) with `health_check_interval` configured, health checks run independently of full manifest applies. If any health check returns a CRITICAL status, the agent triggers a remediation apply for that manifest.