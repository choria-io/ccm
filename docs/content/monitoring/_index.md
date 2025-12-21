+++
title = "Monitoring"
toc = true
weight = 60
pre = "<b>6. </b>"
+++

Resources will check their health in the basic way that resources support - if you say a service should be running, if `systemd` says it's running then we assume it's healthy.

In some cases, though, it makes sense to go deeper than that, for example check the web server is actually serving correct content.

All resources support health checks.

```yaml
service:
  name: httpd
  ensure: running
  enable: true
  subscribe: package#zsh
  health_checks:
    - tries: 5
      try_sleep: 1s
      command: |
        /usr/lib64/nagios/plugins/check_http -H localhost:80 --expect "Acme Inc"
```

Here we check that the web server is serving the correct content - the output must include `Acme Inc`. If at first it is not doing that we will try again every 1 second for five times.

These options are also available on the `ccm ensure` commands via the `--check`, `--check-tries`, `--check-sleep` flags. While resources support multiple health checks the CLI `ensure` command supports adding only one.