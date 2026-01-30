# Hierarchical Data

The Choria Hierarchical Data Resolver is a small data resolver inspired by Hiera. It evaluates a YAML or JSON document alongside a set of facts to produce a final data map.

The resolver supports `first` and `deep` merge strategies and relies on expression-based string interpolation for hierarchy entries. It is optimized for single files that hold both hierarchy and data, rather than the multi-file approach common in Hiera.

Key features:

 * Lookup expressions based on the [Expr Language](https://expr-lang.org/docs/language-definition)
 * Type-preserving lookups (returns typed data, not just strings)
 * Command-line tool with built-in system facts
 * Go library for embedding

### Usage

Here's an annotated example:

```yaml
hierarchy:
    # Lookup and override order - facts are resolved in expressions
    # Use GJSON path syntax for nested facts: {{ lookup('facts.host.info.hostname') }}
    order:
     - env:{{ lookup('facts.env') }}
     - role:{{ lookup('facts.role') }}
     - host:{{ lookup('facts.hostname') }}
    merge: deep  # "deep" merges all matches; "first" stops at first match

# Base data - hierarchy results are merged into this
data:
   log_level: INFO
   packages:
     - ca-certificates
   web:
     listen_port: 80
     tls: false

# Override sections keyed by hierarchy order entries
overrides:
    env:prod:
      log_level: WARN

    role:web:
      packages:
        - nginx
      web:
        listen_port: 443
        tls: true

    host:web01:
      log_level: TRACE
```

The templating here is identical to that in the [Data documentation](../data), except only the `lookup()` function is available (no file access functions).

> [!info] Default Hierarchy
> If no `hierarchy` section is provided, the resolver uses a default hierarchy of `["default"]`.

### CLI Example

The `ccm hiera` command resolves hierarchy files with facts. It is designed to be a generally usable tool, with flexible options for providing facts.

Given the input file `data.json`:

```json
{
    "hierarchy": {
        "order": [
            "fqdn:{{ lookup('facts.fqdn') }}"
        ]
    },
    "data": {
        "test": "value"
    },
    "overrides": {
        "fqdn:my.fqdn.com": {
            "test": "override"
        }
    }
}
```

Resolve with facts provided on the command line:

```nohighlight
$ ccm hiera parse data.json fqdn=my.fqdn.com
{
  "test": "override"
}

$ ccm hiera parse data.json fqdn=other.fqdn.com
{
  "test": "value"
}
```

Output formats:

```nohighlight
# YAML output
$ ccm hiera parse data.json fqdn=other.fqdn.com --yaml
test: value

# Environment variable output
$ ccm hiera parse data.json fqdn=other.fqdn.com --env
HIERA_TEST=value
```

### Fact Sources

Facts can come from multiple sources, which are merged together:

**System facts** (`-S` or `--system-facts`):

```nohighlight
# View system facts
$ ccm hiera facts -S

# Resolve using system facts
$ ccm hiera parse data.json -S
```

**Environment variables as facts** (`-E` or `--env-facts`):

```banohighlightsh
ccm hiera parse data.json -E
```

**Facts file** (`--facts FILE`):

```nohighlight
ccm hiera parse data.json --facts facts.yaml
```

**Command-line facts** (key=value pairs):

```nohighlight
ccm hiera parse data.json env=prod role=web
```

All fact sources can be combined. Command-line facts take highest precedence.

### Data in NATS

[NATS](https://nats.io) is a lightweight messaging system that supports Key-Value stores. Hierarchy data can be stored in NATS and used with `ccm ensure` and `ccm hiera` commands.

To use NATS as a hierarchy store, configure a NATS context for authentication:

```nohighlight
nats context add ccm --server nats.example.org --user ccm --password s3cret \
  --description "CCM Configuration Store"
```

Create a KV store and add your hierarchy data:

```nohighlight
$ nats kv add CCM --replicas 3 --context ccm
$ nats kv put CCM data "$(cat hiera.yaml)"
```

Resolve the hierarchy using the KV store:

```nohighlight
ccm hiera parse kv://CCM/data --context ccm -S
```

### Data on Web Servers

Hierarchy data can also be stored on a web server and fetched via HTTP or HTTPS.

```nohighlight
ccm hiera parse https://example.net/site.yaml -S
```

HTTP Basic Auth is supported via URL credentials:

```nohighlight
ccm hiera parse https://user:pass@example.net/site.yaml -S
```
