# Monitoring

By default, CCM verifies resource health using the resource's native state. For example, if a service should be running and `systemd` reports it as running, CCM considers it healthy.

For deeper validation, all resources support custom health checks. These checks run after a resource is managed and can verify that the resource is functioning correctly, not just present.

## Health Check Properties

| Property    | Description                                              | Default           |
|-------------|----------------------------------------------------------|-------------------|
| `command`   | Command to execute (required)                            | -                 |
| `name`      | Name for logging and metrics                             | Command base name |
| `tries`     | Number of attempts before failing                        | 1                 |
| `try_sleep` | Duration to wait between retry attempts                  | 1s                |
| `timeout`   | Maximum time for command execution                       | No timeout        |
| `format`    | Output format interpretation                             | `nagios`          |

## Nagios Format

Health checks use Nagios plugin conventions for exit codes:

| Exit Code | Status     | Description                                    |
|-----------|------------|------------------------------------------------|
| 0         | OK         | Check passed                                   |
| 1         | WARNING    | Check passed with warnings (non-critical)      |
| 2         | CRITICAL   | Check failed                                   |
| 3+        | UNKNOWN    | Check could not determine status               |

## Example

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

## Agent Integration

When running the [Agent](../agent/) with `health_check_interval` configured, health checks run independently of full manifest applies. If any health check returns a CRITICAL status, the agent triggers a remediation apply for that manifest.