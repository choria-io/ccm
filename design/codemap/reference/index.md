# Reference and Map

This page is the index to the codebase: the commands a user can run, the packages that back
them, and the types that recur across the subsystem pages.

{{% notice style="note" title="Where it lives" %}}
`cmd`: the entire CLI, one file per command, built on `choria-io/fisk`. `main` is in
`cmd/ccm.go`. Shared plumbing, including manager construction and the `.env` reader, is in
`cmd/util.go`.
{{% /notice %}}

## Command surface

Every `ccm` command builds a manager and drives a subsystem covered elsewhere in this map. The
global flags `--debug` and `--info` set log verbosity.

| Command | Purpose | Drives |
|---------|---------|--------|
| `ccm ensure <type>` | Manage one resource imperatively: `archive`, `exec`, `file`, `package`, `scaffold`, `service` | [Resource-Provider Model]({{% relref "resource-provider-model" %}}) |
| `ccm ensure api piped` | Apply a resource sent as JSON or YAML on stdin | [Resource-Provider Model]({{% relref "resource-provider-model" %}}) |
| `ccm apply <manifest>` | Apply a manifest from a file, `obj://`, or `https://` tarball | [Apply Engine]({{% relref "apply-engine" %}}) |
| `ccm agent --config <file>` | Run the continuous manifest daemon | [The Agent]({{% relref "agent" %}}) |
| `ccm facts [query]` | Show system facts, with an optional gjson query | [Data, Facts, and Templates]({{% relref "data-and-templates" %}}) |
| `ccm hiera parse <input>` | Resolve a Hiera input against facts | [Data, Facts, and Templates]({{% relref "data-and-templates" %}}) |
| `ccm registration create / query / watch / rm / init` | Publish, read, watch, remove entries, or provision the stream | [Registration and Discovery]({{% relref "registration" %}}) |
| `ccm session new / report` | Create a session store or summarize one | [Observability]({{% relref "observability" %}}) |
| `ccm status <type> <name>` | Read the current state of a resource | [Resource-Provider Model]({{% relref "resource-provider-model" %}}) |

Every `ensure` subcommand and the piped API funnel through one factory,
`resources.NewResourceFromProperties`, so the CLI, manifests, and the wire API run identical
resource logic. The `status` command's valid types are generated at run time from the registry,
so it tracks whatever providers are registered.

## Source map

| Package | Contents |
|---------|----------|
| `cmd/` | The fisk CLI surface, `main`, one file per command, shared helpers in `util.go`. |
| `agent/` | The continuous runner: config, worker loop, and NATS, HTTP, and object-store sources. |
| `facts/` | System fact collectors backed by gopsutil, plus file-based facts. |
| `hiera/` | The hierarchical data resolver and comment-driven validation. |
| `templates/` | The render environment and the expr, Jet, and Go template engines. |
| `manager/` | The concrete `model.Manager` (type `CCM`), its options, and logger adapters. |
| `model/` | Core interfaces and shared structs; per-resource property types; `modelmocks/`. |
| `registration/` | Service registration over JetStream: stream, subjects, publish, lookup, watch. |
| `resources/` | Resource implementations, the shared `base`, the apply engine, and provider subpackages. |
| `internal/registry/` | The global provider directory and `FindSuitableProvider`. |
| `internal/session/` | The directory and memory session stores. |
| `internal/metrics/` | Prometheus collectors and the `/metrics` server. |
| `internal/healthcheck/` | The goss and nagios health-check runners. |
| `internal/` (other) | `cmdrunner`, `backoff`, `fs` (embedded schemas), and `util` helpers. |

## Key types

| Type | Role | Explained in |
|------|------|--------------|
| `model.Manager` (`CCM`) | The orchestrator every layer programs against | [Architecture]({{% relref "architecture" %}}) |
| `model.Provider` / `ProviderFactory` | The provider seam and its registration unit | [Architecture]({{% relref "architecture" %}}) |
| `model.Resource` / `ResourceProperties` | A managed thing and its declarative desired state | [Resource-Provider Model]({{% relref "resource-provider-model" %}}) |
| `base.Base` / `EmbeddedResource` | The shared apply flow and its callback interface | [Resource-Provider Model]({{% relref "resource-provider-model" %}}) |
| `model.Apply` / `TransactionEvent` | A parsed manifest and the per-resource result record | [Apply Engine]({{% relref "apply-engine" %}}) |
| `hiera.Resolve` / `templates.Env` | Data resolution and the render environment | [Data, Facts, and Templates]({{% relref "data-and-templates" %}}) |
| `model.RegistrationEntry` | A published service-discovery entry | [Registration and Discovery]({{% relref "registration" %}}) |
| `model.SessionStore` / `SessionSummary` | Event storage and run aggregation | [Observability]({{% relref "observability" %}}) |
| `agent.Agent` | The continuous loop and its workers | [The Agent]({{% relref "agent" %}}) |

## Glossary

<dl class="cm-kv">
  <dt>Resource</dt><dd>A system component to manage: a package, file, service, exec, archive, or scaffold. Declares a desired state.</dd>
  <dt>Provider</dt><dd>The platform-specific implementation for a resource type, such as apt, dnf, systemd, or posix. Selected at run time by facts.</dd>
  <dt>Ensure</dt><dd>The desired state of a resource, such as present, absent, running, or a package version.</dd>
  <dt>Manifest</dt><dd>A YAML document of data, a hierarchy, and a list of resources, applied as a unit.</dd>
  <dt>Facts</dt><dd>Gathered truths about the host, used to select providers and drive the data hierarchy.</dd>
  <dt>Hiera</dt><dd>The hierarchical data resolver that merges layered data blocks, chosen by facts, into one resolved map.</dd>
  <dt>Session</dt><dd>The record of a run: a sequence of events, stored in memory or on disk, that drives require and subscribe.</dd>
  <dt>Noop</dt><dd>Dry-run mode. No change is made, but the event still reports what would have changed.</dd>
  <dt>Require</dt><dd>A fail-gate: a resource is skipped if a named prior resource failed. Not a scheduler.</dd>
  <dt>Subscribe / refresh</dt><dd>A resource, in practice a service, restarts when a named prior resource changed.</dd>
  <dt>register_when_stable</dt><dd>A resource property that publishes a discovery entry once the resource is stable and healthy.</dd>
</dl>

{{% notice style="tip" title="Back to the start" %}}
Return to the [Code Map overview]({{% relref "/design/codemap" %}}) for the mental model and the full
page list.
{{% /notice %}}
