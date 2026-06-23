# Templates

Applications need data to vary their behavior and configure their environments. Configuration management tools are no different.

Data can be used for:

 * Configuring resource names that differ between operating systems (e.g., `httpd` vs `apache2`)
 * Setting different configuration values depending on environment, role, or other dimensions
 * Deciding whether environments should have something installed (e.g., development vs production)

CCM supports various data sources:

 * **System Facts** - Operating system, networking, and disk configuration
 * **Custom Facts** - From `facts.{yaml,json}` and `facts.d/*.{yaml,json}` in system and user config directories
 * **Environment** - Variables from the shell environment and `./.env` files
 * **Hiera Data** - Hierarchical data with overrides based on facts

## Accessing data

Expressions like `{{ lookup('facts.host.info.platformFamily') }}` or `${ lookup('facts.host.info.platformFamily') }` use the [Expr Language](https://expr-lang.org/docs/language-definition).

### Available variables

In templates, you have direct access to:

| Variable  | Description                                           |
|-----------|-------------------------------------------------------|
| `Facts`   | System facts (e.g., `Facts.host.info.platformFamily`) |
| `Data`    | Resolved Hiera data (e.g., `Data.package_name`)       |
| `Environ` | Environment variables (e.g., `Environ.HOME`)          |

### Available functions

| Function                       | Description                                                                                                                                                                                    |
|--------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `lookup(key, default)`         | Lookup data using [GJSON Path Syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md). Example: `lookup("facts.host.info.os", "linux")`                                                |
| `readFile(path)`, `file(path)` | Read a file into a string. Relative paths are resolved from the working directory, absolute paths are used as-is                                                                               |
| `template(f)`                  | Parse `f` using templates. If `f` ends in `.templ`, reads the file first, if it ends in `.jet` calls the `jet()` function. Not available in Go templates (keyword clash)                       |
| `jet(f)`, `jet(f, "[[", "]]")` | Parse `f` using [Jet templates](https://github.com/CloudyKit/jet/blob/master/docs/syntax.md) with optional custom delimiters. If `f` ends in `.jet`, reads the file first                      |
| `kvGet(bucket, key)`           | Read a value from a NATS JetStream KV bucket. Requires a configured NATS context. Example: `kvGet("secrets", "db.password")` {{% badge style="primary" title="Version" %}}0.0.32{{% /badge %}} |

### GJSON path examples

The `lookup()` function uses GJSON path syntax for nested access:

```
lookup("facts.host.info.platformFamily")     # Simple nested path
lookup("facts.network.interfaces.0.name")    # Array index
lookup("data.packages.#")                    # Array length
```

### CLI usage

These expressions work on the CLI:

```nohighlight
$ ccm ensure package '${ lookup("data.package_name", "httpd") }'
```

It might be easier to avoid using `$` on the CLI so the alternative syntax is helpful:

```nohighlight
$ ccm ensure package '{{ lookup("data.package_name", "httpd") }}'
```

This fetches `package_name` from the data and defaults to `httpd` if not found.

## Facts

CCM includes a built-in fact resolver that gathers system information. To see available facts:

```nohighlight
$ ccm facts                              # All facts as JSON
$ ccm facts host                         # Query specific path
$ ccm facts --yaml                       # Output as YAML
```

Access facts in expressions using `${ Facts.host.info.platformFamily }` or `${ lookup('facts.host.info.
platformFamily') }`.

### Custom facts

Custom facts are loaded from the system (`/etc/choria/ccm/`) and user (`~/.config/choria/ccm/`) configuration directories. Within each directory, facts are loaded in this order:

 1. `facts.json`
 2. `facts.yaml`
 3. `facts.d/*.{json,yaml}` - files sorted by filename

Later sources override earlier ones, and user directory facts override system directory facts. This makes `facts.d/` useful for modular or drop-in facts from other tools.

Files in `facts.d/` are processed in lexicographic filename order, so `01-base.json` is loaded before `02-override.json`. Only files with `.json` or `.yaml` extensions are read; all other files (including `.yml`) are ignored.

### Security

Facts directories are subject to the following security constraints:

 * **Absolute paths only** - configuration directories must be absolute paths; relative paths are rejected
 * **Path cleaning** - paths are normalized to remove traversal components (e.g., `/../`)
 * **Symlinks ignored** - symlinked files, symlinked `facts.d/` directories, and symlinked entries within `facts.d/` are all skipped

## Hiera data for CLI

Hiera data is resolved using the Choria Hierarchical Data Resolver. By default, data is read from `./.hiera`, or you can specify a file with `--hiera`.

> [!info] Note
> This applies to `ccm ensure` commands. The `ccm apply` command uses manifests that contain their own Hiera data.

### Hiera data sources

Hiera data can be loaded from:

 * **Local file**: `./.hiera` or path specified with `--hiera`
 * **Key-Value store**: `--hiera kv://BUCKET/key` (requires `--context` for NATS)
 * **HTTP(S)**: `--hiera https://example.com/data.yaml` (supports Basic Auth via URL credentials)

### Merge strategies

The `hierarchy.merge` setting controls how overrides are applied:

 * **`first`** (default): Stops at the first matching override
 * **`deep`**: Deep merges all matching overrides in order

### Example

Given this file stored in `./.hiera`:

```yaml
hierarchy:
  order:
    - os:${ lookup('facts.host.info.platformFamily') }
  merge: first

data:
  package_name: ""

overrides:
  os:debian:
    package_name: apache2

  os:rhel:
    package_name: httpd
```

Running `ccm ensure package '${ lookup("data.package_name") }'` installs `httpd` on RHEL-based systems and `apache2` on Debian-based systems.

> [!info] Note
> See the [Hiera](../hiera) section for details on configuring Hiera data in NATS.

## Environment

The shell environment and variables defined in `./.env` can be accessed in two ways:

```yaml
# Direct access
home_dir: "${ Environ.HOME }"

# Via lookup (with default value)
my_var: "${ lookup('environ.MY_VAR', 'default') }"
```

The `.env` file uses standard `KEY=value` format, one variable per line.