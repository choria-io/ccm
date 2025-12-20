+++
title = "YAML Manifests"
toc = true
weight = 50
pre = "<b>5. </b>"
+++

A manifest is a YAML file that combines data, platform-specific configuration and resources all in one file.

There is no logic - other than what expressions can do, consider it to be no more complex than basic example multi-resource shell scripts.

The YAML file resolves the manifest using [Choria Hierarchical Data Resolver](https://github.com/choria-io/tinyhiera), our single-file implementation of Hiera.

### Full Example

##### Input Data

First, we define input data, think of this like the properties a module accepts:

```yaml
data:
  package_name: "httpd"
```

##### Resources

We will manage the Apache Server package here:

```yaml
ccm:
  resources:
    - package:
        name: "{{ lookup('data.package_name') }}"
        ensure: latest
```

##### Configure Hierarchy

We need to be able to configure the package name on a number of dimensions - like OS Platform - so we'll create a Hierarchy here:

```yaml
hierarchy:
  order:
    - os:{{ lookup('facts.host.info.platformFamily') }}
```

Here we will look for overrides in `os:rhel`, `os:debian` etc

##### Overrides

Finally, we configure a package name for Debian:

```yaml
overrides:
  os:debian:
    package_name: apache2
```

##### Applying the manifest

Let’s apply the manifest, this is how it should look now:

```
data:
  package_name: "httpd"

ccm:
  resources:
    - package:
        name: "{{ lookup('data.package_name') }}"
        ensure: latest

hierarchy:
  order:
    - os:{{ lookup('facts.host.info.platformFamily') }}

overrides:
  os:debian:
    package_name: apache2
```

We can apply this manifest like so, we can see first run makes a change 2nd run is stable:

```
$ ccm apply manifest.yaml
INFO  Creating new session record resources=1
WARN  package#httpd changed ensure=latest runtime=3.560699287s provider=dnf

$ ccm apply manifest.yaml
INFO  Creating new session record resources=1
INFO  package#httpd stable ensure=latest runtime=293.448824ms provider=dnf
```

One can also see the fully resolved manifest and data without applying it:

```
$ ccm apply manifest.yaml --render
data:
  package_name: apache2
resources:
- package:
    ensure: latest
    name: apache2
```

### Checking what would be done (Noop mode)

One can ask the system to operate in Noop mode, meaning it will attempt to detect what would happen without actually doing it.

This is achieved by using the `--noop` flag.

> [!info] Note
> Noop mode is not perfect, if a change in a resource affects a future resource, it cannot always be detected.

### Manifests in NATS Object Store

Like [Hierarchical Data](../hiera) data can be accessed via NATS Server to avoid needing all the manifests and hiera files locally.

[NATS](https://nats.io) is a lightweight messaging system that is straightforward to run and host; it supports being used as a Key-Value store.

We can't cover NATS here in detail here, but manifest and dependant files can be stored in NATS Object Stores and used in the `ccm apply` command.

Let's add a context called `ccm` for our needs:

```
$ nats context add ccm --user nats.example.org --user ccm --password s£cret --description "Choria CM Configuration Store" 
```

We create a Object Store stored replicated in a cluster and store the hierarchy from `hiera.yaml` in a Key called `data`:

```
$ nats obj add CCM --replicas 3 --context ccm
```

Let's create a manifest that copies a file onto the node:

```
$ mkdir /tmp/manifest
$ echo 'CCM Managed' > /tmp/manifest/mode
$ vi /tmp/manifest/manifest.yaml
...
ccm:
  resources:
        name: /etc/motd
        ensure: present
        source: motd
        owner: root
        group: root
        mode: "0644"
```

You can see here we read the file `motd` and copy it to `/etc/motd` on the node.

Let's create a tar file and place it in the Object Store: 

```
$ tar -C /tmp/manifest/ -cvzf /tmp/manifest.tgz .
$ cd /tmp
$ nats --server localhost:4222 obj put CCM manifest.tgz
```
We can now apply the manifest:

```
$ ccm apply obj://CCM/manifest.tgz --context local
 INFO  Using manifest from Object Store in temporary directory bucket=CCM file=manifest.tgz
 INFO  file#/etc/motd stable ensure=present runtime=0s provider=posix 
```
