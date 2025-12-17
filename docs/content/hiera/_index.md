+++
title = "Hierarchical Data"
toc = true
weight = 20
pre = "<b>2. </b>"
+++

Choria Hierarchical Data Resolver is a small data resolver inspired by Hiera. It evaluates a YAML or JSON document alongside a set of facts to produce a final data map. The resolver supports first and deep merge strategies and relwies on simple string interpolation for hierarchy entries.

It is optimized for single files that hold the hierarchy and data rather than the multi-file approach common in Hiera.

Major features:

 * Lookup expressions based on a full language
 * Types are supported, and lookups can return typed data
 * Command line tool that includes built-in system facts
 * Go library

### Usage

Here's an annotated example:

```yaml
hierarchy:
    # this is the lookup and override order, facts will be resolved here
    #
    # if your fact is nested, you can use gjson format queries like via the lookup function {{ lookup('networking.fqdn') }}
    order:
     - env:{{ lookup('facts.env') }}
     - role:{{ lookup('facts.role') }}
     - host:{{ lookup('facts.hostname') }}
    merge: deep # or first

# This is the resulting output and must be present, the hierarchy results will be merged in
data:
   log_level: INFO
   packages:
     - ca-certificates
   web:
     # we look up the number and convert its type to a int if the facts was not already an int
     listen_port: 80
     tls: false

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

The templating used here is identical to that used in the [Data documentation section](../data) except that only the `lookup()` function was added not ones for accessing files etc.

### CLI Example

A small utility is provided to resolve a hierarchy file and a set of facts. This utility has some different rules and behaviors for loading facts than the `ccm` suite in general since we want it to be a genericly usable tool even without system facts etc, so we make it easy to pass flags on the CLI etc.

Given the input file data.json:

```json
{
    "hierarchy": {
        "order": [
            "fqdn:{{ lookup('facts.fqdn'}} }}"
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

We can run the utility like this:

```bash
$ ccm hiera parse data.json fqdn=my.fqdn.com
{
  "test": "override"
}
$ ccm hiera parse data.json fqdn=other.fqdn.com
{
  "test": "value"
}
```

It can also produce YAML output:

```bash
$ ccm hiera parse test.json fqdn=other.fqdn.com --yaml
test: value
```

It can also produce Environment Variable output:

```bash
$ ccm hiera parse test.json fqdn=other.fqdn.com --env
HIERA_TEST=value
```

In these examples we provided facts from a file or on the CLI, we can also populate the facts from an internal fact provider, first we view the internal facts:

```bash
$ ccm hiera facts --system-facts
{
  ....
  "host": {
      "info": {
          "hostname": "example.net",
          "uptime": 3725832,
          "bootTime": 1760351572,
          "procs": 625,
          "os": "darwin",
          "platform": "darwin",
          "platformFamily": "Standalone Workstation",
          "platformVersion": "15.7.1",
          "kernelVersion": "24.6.0",
          "kernelArch": "arm64",
          "virtualizationSystem": "",
          "virtualizationRole": ""
      }
  }
....
```

Now we resolve the data using those facts:

```bash
$ ccm hiera parse test.json --system-facts
```

We can also populate the environment variables as facts, variables will be split on the = and the variable name becomes a fact name.

```bash
$ ccm hiera parse test.json --env-facts
```

These facts will be merged with ones from the command line and external files and all can be combined

### Go example

Supply a YAML document and a map of facts. The resolver will parse the hierarchy, replace `{{ lookup('facts.fact') }}` placeholders, and merge the matching sections.

Here the hierarchy key defines the lookup strategies and the data key defines what will be returned.

The rest is the hierarchy data.

```golang
package main

import (
        "fmt"

        "github.com/choria-io/ccm/hiera"
)

func main() {
        yamlDoc := []byte(`
 hierarchy:
   order:
     - env:{{ lookup('facts.env') }}
     - role:{{ lookup('facts.role') }}
     - host:{{ lookup('facts.hostname') }}
   merge: deep

 data:
   log_level: INFO
   packages:
     - ca-certificates
   web:
     listen_port: 80
     tls: false

 overrides:
     env:prod:
       log_level: WARN

     role:web:
       packages:
         - nginx
       web:
         tls: true

     host:web01:
       log_level: TRACE
`)

        facts := map[string]any{
                "env":      "prod",
                "role":     "web",
                "hostname": "web01",
        }

		logger := manager.NewSlogLogger(
			slog.New(
				slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
		
        resolved, err := hiera.ResolveYaml(yamlDoc, facts, hiera.DefaultOptions, logger)
        if err != nil {
                panic(err)
        }

    	jout, err := json.MarshalIndent(res, "", "  ")
    	if err != nil {
    		    panic(err)
    	}

	    fmt.Println(string(jout))
}
```