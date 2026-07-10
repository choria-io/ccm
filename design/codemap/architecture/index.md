# Architecture

CCM is built around a strict dependency inversion. The `model` package defines contracts
and almost no behavior. Concrete resource types and providers depend on those contracts and
on a global registry, never on the orchestrator. The `manager` package is the orchestrator
that satisfies the central contract and wires in facts, data, sessions, NATS, and logging.
That inversion is why a provider can self-register without the manager knowing it exists,
and why the same resource logic runs from the CLI, a manifest, or the agent.

{{% notice style="note" title="Where it lives" %}}
`model`: the contracts and shared structs. `internal/registry`: the global provider
directory. `manager`: the concrete orchestrator, type `CCM`. Key files: `model/resource.go`,
`model/provider.go`, `model/manager.go`, `model/transaction.go`, `internal/registry/registry.go`,
`manager/manager.go`, `manager/opts.go`.
{{% /notice %}}

## The layers

Read the stack from the outside in. Entry points build a manager and hand it work. The apply
engine turns a manifest into resources. Each resource embeds a shared base that owns the
apply flow. The base calls into the concrete type, which decides what to change and calls a
provider. Providers do the platform work. Every layer below the manager programs against
`model` interfaces only.

<figure class="cm-diagram">
  <svg viewBox="0 0 760 340" role="img" aria-label="Layered architecture from entry points down through the manager, apply engine, resources, base, and providers to the model contracts">
    <defs>
      <marker id="ah" markerWidth="9" markerHeight="9" refX="7" refY="3" orient="auto">
        <path d="M0,0 L7,3 L0,6 Z" fill="var(--cm-accent)"/>
      </marker>
    </defs>
    <rect class="cm-svg-box" x="40" y="20" width="680" height="40" rx="8"/>
    <text class="cm-svg-label" x="380" y="44" text-anchor="middle">Entry points · cmd/ · agent/</text>
    <rect class="cm-svg-box" x="40" y="72" width="680" height="40" rx="8"/>
    <text class="cm-svg-label" x="380" y="96" text-anchor="middle">Apply engine · resources/apply</text>
    <rect class="cm-svg-box" x="40" y="124" width="680" height="40" rx="8"/>
    <text class="cm-svg-label" x="380" y="148" text-anchor="middle">Resource types · resources/file · package · service · exec · archive · scaffold</text>
    <rect class="cm-svg-box" x="40" y="176" width="680" height="40" rx="8"/>
    <text class="cm-svg-label" x="380" y="200" text-anchor="middle">Shared base · resources/base</text>
    <rect x="40" y="228" width="680" height="40" rx="8"
          fill="color-mix(in srgb, var(--cm-accent2) 14%, transparent)" stroke="var(--cm-accent2)"/>
    <text class="cm-svg-label" x="380" y="252" text-anchor="middle" style="fill:var(--cm-accent2)">Providers · posix · apt · dnf · systemd · http</text>
    <rect x="40" y="284" width="500" height="40" rx="8"
          fill="color-mix(in srgb, var(--cm-accent) 12%, transparent)" stroke="var(--cm-accent)"/>
    <text class="cm-svg-label" x="290" y="308" text-anchor="middle" style="fill:var(--cm-accent)">model contracts · every layer depends on these</text>
    <rect class="cm-svg-box" x="556" y="284" width="164" height="40" rx="8"/>
    <text class="cm-svg-label" x="638" y="304" text-anchor="middle">internal/registry</text>
    <text class="cm-svg-sub"   x="638" y="318" text-anchor="middle">provider directory</text>
    <line x1="380" y1="60"  x2="380" y2="72"  stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="380" y1="112" x2="380" y2="124" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="380" y1="164" x2="380" y2="176" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="380" y1="216" x2="380" y2="228" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
  </svg>
  <figcaption>Every layer below the manager depends downward only, and only on the model contracts.</figcaption>
</figure>

## The contracts

The `model` package is deliberately thin. It holds interfaces and shared value structs, and
defers behavior to the packages that implement them.

<dl class="cm-kv">
  <dt>Resource</dt><dd>The runtime behavior of a managed thing: <code>Type</code>, <code>Name</code>, <code>ResourceId</code>, <code>Provider</code>, <code>Properties</code>, plus <code>Apply</code>, <code>Healthcheck</code>, and <code>Info</code>. Implemented by the per-type <code>Type</code> structs under <code>resources/*/type.go</code>. Defined at <code>model/resource.go:26</code>.</dd>
  <dt>ResourceProperties</dt><dd>The declarative desired state: <code>CommonProperties()</code>, <code>Validate()</code>, <code>ResolveTemplates()</code>, <code>ResolveDeferredTemplates()</code>, <code>ToYamlManifest()</code>. Every per-type properties struct embeds <code>CommonResourceProperties</code>. Defined at <code>model/resource.go:43</code>.</dd>
  <dt>Provider</dt><dd>Intentionally tiny: only <code>Name() string</code> (<code>model/provider.go:8</code>). Each resource package widens it into a capability interface, so the registry can stay type-agnostic while types get a rich contract.</dd>
  <dt>ProviderFactory</dt><dd>The registration unit: <code>TypeName()</code>, <code>Name()</code>, <code>IsManageable(facts, props)</code>, and <code>New(Logger, CommandRunner)</code>. Defined at <code>model/provider.go:12</code>.</dd>
  <dt>Manager</dt><dd>The large seam the whole engine programs against: facts, data, logging, execution, sessions and events, registration, templating, and NATS. Defined at <code>model/manager.go:25</code>.</dd>
</dl>

## The registry as a plugin bus

`internal/registry` is a global directory keyed by resource type then provider name. Each
provider package exposes a `Register()` that calls `registry.MustRegister(&factory{})` from
an `init()`. The manager triggers all of these through a single blank import,
`_ "github.com/choria-io/ccm/resources"` at `manager/manager.go:27`, so it never names a
concrete provider. Duplicate provider names within a type are rejected with
`ErrDuplicateProvider`.

Resolution happens when a resource applies. `FindSuitableProvider` (`internal/registry/registry.go:168`)
probes every factory for the type with `IsManageable(facts, props)`, keeps the ones that
report they can manage the resource, sorts them ascending by the returned priority integer,
and takes the first.

{{% notice style="warning" title="Load-bearing decision" %}}
Lower priority value wins. `selectProviders` sorts ascending and `FindSuitableProvider` takes
`provs[0]` (`internal/registry/registry.go:113`). A test comment calls this the "highest
priority" even though the numeric value is the lowest, so read the number, not the word.
{{% /notice %}}

## The manager

The `CCM` struct (`manager/manager.go:36`) is the concrete `model.Manager`. It is built with
`NewManager(log, userLogger, opts...)` and functional options from `manager/opts.go`
(`WithNatsContext`, `WithSessionDirectory`, `WithRegistrationDestination`, `WithNoop`,
`WithEnvironmentData`, and others). It defaults to an in-memory session store and a no-op
registration publisher, so a bare manager is safe to run offline.

<dl class="cm-kv">
  <dt>Facts</dt><dd><code>Facts()</code> caches gathered facts; <code>SystemFacts()</code> always re-gathers with a 2s default deadline; <code>SetFacts</code> and <code>MergeFacts</code> override or overlay the cache.</dd>
  <dt>Data</dt><dd><code>SetData</code> deep-merges resolved data with an external overlay that always wins; <code>Data()</code> returns a copy.</dd>
  <dt>Sessions</dt><dd><code>StartSession</code>, <code>RecordEvent</code>, <code>SessionSummary</code>, plus <code>ShouldRefresh</code> and <code>IsResourceFailed</code>, which read the last recorded event for a resource to drive subscribe and require.</dd>
  <dt>Templating</dt><dd><code>TemplateEnvironment(ctx)</code> assembles the render environment and injects the registration lookup and KV-get closures.</dd>
  <dt>Noop</dt><dd><code>NoopMode()</code> and <code>SetNoopMode</code> gate every mutating branch in the resource types.</dd>
</dl>

## Cross-manager safety

Some platform tooling is not safe to run in parallel. CCM guards it with process-wide
mutexes, `PackageGlobalLock` and `ServiceGlobalLock` (`model/global_locks.go:14`), that
serialize all package and service operations even across concurrent managers and manifests.
The stated reason is that concurrent `dpkg`/`apt` or `systemd` operations would corrupt their
databases.

{{% notice style="warning" title="Load-bearing decision" %}}
`TransactionEvent` carries the outcome flags (`Changed`, `Refreshed`, `Failed`, `Skipped`,
`Noop`) that are mirrored in metrics, the session summary, the CLI output, and
`CommonResourceState`. A comment at `model/transaction.go:45` warns that a change to this
struct must be propagated to all four. Treat it as a shared schema, not a local struct.
{{% /notice %}}

Two type-dispatch switches are hand-maintained and carry a matching `TODO`:
`NewResourcePropertiesFromYaml` (`model/resource.go:158`) and `ResourceInfo`
(`manager/manager.go:397`) both map type names to constructors by hand, and the intent is to
make the registry carry that mapping so new types register once. The `SelectProvider`
boilerplate is likewise duplicated across every `resources/*/type.go` with a
`// TODO: move to base`.

{{% notice style="tip" title="Next" %}}
Continue to [The Resource-Provider Model]({{% relref "resource-provider-model" %}}) to see how
a concrete type and its providers are built on these contracts.
{{% /notice %}}
