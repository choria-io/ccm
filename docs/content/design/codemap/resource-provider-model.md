+++
title = "The Resource-Provider Model"
weight = 30
description = "How a concrete resource type is built, and how one resource is driven to its desired state: decide, act, and verify."
+++

Every resource type in CCM is built from three parts. A `Type` decides whether the system
already matches the desired state. A `Provider` makes the platform-specific change. A shared
`base.Base` owns the flow that connects them: gates, requirements, health checks, and turning
the outcome into an event. The split is deliberate. The type stays portable and unit-testable
against a mock provider, and the flow is written once for all seven resource types.

{{% notice style="note" title="Where it lives" %}}
`resources/base`: the shared apply flow. `resources/file`, `resources/package`,
`resources/service` and their `posix`, `apt`, `dnf`, `systemd` provider subpackages: the
concrete types and their platform work. `model/resource_*.go`: the properties and state
structs. Key files: `resources/base/base.go`, `resources/file/type.go`,
`resources/file/posix/posix.go`.
{{% /notice %}}

## The inversion

`base.Base` owns the apply flow but does not know any resource type. It calls back into the
concrete type through a small interface, `base.EmbeddedResource` (`resources/base/base.go:27`):
`ApplyResource`, `SelectProvider`, `NewTransactionEvent`, and `Type`. Each type embeds
`*base.Base` and, in its `New` constructor, sets `Base.Resource = t` so the base can call back
into it. That back-pointer is what lets one copy of the control-gating, require-checking, and
event-mapping code serve every type.

<dl class="cm-kv">
  <dt>Type</dt><dd>Embeds <code>*base.Base</code>; holds the typed properties, the manager, and the resolved provider. Its <code>ApplyResource</code> reads status, compares against the desired state, calls a provider verb, and re-verifies.</dd>
  <dt>Provider interface</dt><dd>Widens <code>model.Provider</code> per type. <code>FileProvider</code> adds <code>CreateDirectory</code>, <code>Store</code>, <code>SetAttributes</code>, <code>Remove</code>, <code>Status</code> (<code>resources/file/file.go:18</code>). <code>PackageProvider</code> and <code>ServiceProvider</code> add their own verbs.</dd>
  <dt>Concrete provider</dt><dd>The only place OS mutation lives. The posix file provider is the sole caller of <code>os.MkdirAll</code>, <code>os.Rename</code>, <code>os.Chown</code>, and <code>os.RemoveAll</code> (<code>resources/file/posix/posix.go</code>).</dd>
</dl>

{{% notice style="warning" title="Load-bearing decision" %}}
Real work must live in the provider, never in the type. The `Type.ApplyResource` only decides
and verifies; all platform mutation sits behind the capability interface. This keeps a type
portable across providers and keeps its decision logic OS-agnostic and testable with a mocked
provider. A type that stubbed work inline would break both properties.
{{% /notice %}}

## Applying one resource

The flow below is `base.applyOrHealthCheck` (`resources/base/base.go:74`). The type-specific
decisions happen inside the type's `ApplyResource`.

<ol class="cm-steps">
  <li><b>Select the provider</b> The type resolves through <code>registry.FindSuitableProvider</code>, which filters providers by <code>IsManageable(facts, props)</code> and picks the lowest priority number. The result is cached.</li>
  <li><b>Gate on control</b> <code>checkControl</code> evaluates <code>control.if</code> and <code>control.unless</code> expressions. If they say do not manage, the event is marked <code>Skipped</code> and returned.</li>
  <li><b>Resolve deferred templates</b> File overrides <code>ResolveDeferredTemplates</code> so <code>content</code> and <code>source</code> render only after the control gate, letting <code>unless</code> protect against template errors on a resource that will be skipped.</li>
  <li><b>Check requirements</b> For each <code>require</code> entry, <code>Manager.IsResourceFailed</code> reads the last recorded event. Any failed or unmet requirement marks this event <code>Skipped</code> with <code>UnmetRequirements</code> and returns.</li>
  <li><b>Read status and decide</b> <code>provider.Status</code> reads the actual state, and <code>isDesiredState</code> compares it against the properties, returning a human-readable mismatch reason.</li>
  <li><b>Act, or describe</b> A switch calls the provider verb. Every mutating branch is guarded by <code>if !noop</code>. In noop mode the verb is skipped and a <code>noopMessage</code> describes what would have happened.</li>
  <li><b>Verify convergence</b> After a real change, the type re-reads status and re-runs <code>isDesiredState</code>. If the system did not converge it returns <code>ErrDesiredStateFailed</code> with the reason.</li>
  <li><b>Finalize and record</b> <code>FinalizeState</code> writes the flags onto the state, base maps them onto the event, and the caller logs and records it.</li>
</ol>

<figure class="cm-diagram">
  <svg viewBox="0 0 760 400" role="img" aria-label="Applying one resource: select provider, gate, read status, decide whether it matches the desired state, act via a provider or describe under noop, verify, and finalize">
    <defs>
      <marker id="ah" markerWidth="9" markerHeight="9" refX="7" refY="3" orient="auto">
        <path d="M0,0 L7,3 L0,6 Z" fill="var(--cm-accent)"/>
      </marker>
    </defs>
    <rect class="cm-svg-box" x="80" y="16" width="240" height="38" rx="8"/>
    <text class="cm-svg-label" x="200" y="40" text-anchor="middle">Select provider by facts</text>
    <rect class="cm-svg-box" x="80" y="68" width="240" height="38" rx="8"/>
    <text class="cm-svg-label" x="200" y="92" text-anchor="middle">Control + require gates</text>
    <rect class="cm-svg-box" x="80" y="120" width="240" height="38" rx="8"/>
    <text class="cm-svg-label" x="200" y="144" text-anchor="middle">provider.Status → actual</text>
    <polygon class="cm-svg-box" points="200,172 300,205 200,238 100,205"/>
    <text class="cm-svg-label" x="200" y="203" text-anchor="middle">matches</text>
    <text class="cm-svg-label" x="200" y="219" text-anchor="middle">desired?</text>
    <rect x="440" y="185" width="280" height="40" rx="8"
          fill="color-mix(in srgb, var(--cm-accent) 12%, transparent)" stroke="var(--cm-accent)"/>
    <text class="cm-svg-label" x="580" y="209" text-anchor="middle" style="fill:var(--cm-accent)">Stable · no change</text>
    <rect x="80" y="262" width="240" height="44" rx="8"
          fill="color-mix(in srgb, var(--cm-accent2) 14%, transparent)" stroke="var(--cm-accent2)"/>
    <text class="cm-svg-label" x="200" y="282" text-anchor="middle" style="fill:var(--cm-accent2)">Act via provider verb</text>
    <text class="cm-svg-sub"   x="200" y="298" text-anchor="middle">noop: describe, do not act</text>
    <rect class="cm-svg-box" x="80" y="326" width="240" height="44" rx="8"/>
    <text class="cm-svg-label" x="200" y="346" text-anchor="middle">Re-read + verify</text>
    <text class="cm-svg-sub"   x="200" y="362" text-anchor="middle">else ErrDesiredStateFailed</text>
    <rect x="440" y="326" width="280" height="44" rx="8"
          fill="color-mix(in srgb, var(--cm-accent) 12%, transparent)" stroke="var(--cm-accent)"/>
    <text class="cm-svg-label" x="580" y="352" text-anchor="middle" style="fill:var(--cm-accent)">FinalizeState → event</text>
    <line x1="200" y1="54"  x2="200" y2="68"  stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="200" y1="106" x2="200" y2="120" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="200" y1="158" x2="200" y2="172" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="300" y1="205" x2="440" y2="205" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <text class="cm-svg-sub" x="352" y="198" text-anchor="middle">yes</text>
    <line x1="200" y1="238" x2="200" y2="262" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <text class="cm-svg-sub" x="214" y="256">no</text>
    <line x1="200" y1="306" x2="200" y2="326" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="320" y1="348" x2="440" y2="348" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="580" y1="225" x2="580" y2="326" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
  </svg>
  <figcaption>Decide, act, verify. The provider does the work; the type only decides and confirms.</figcaption>
</figure>

## Ensure states

Ensure semantics are per type, and deliberately loose where the platform is unreliable.

| Type | States | Notes |
|------|--------|-------|
| `file` | `present`, `absent`, `directory` | Compares ensure, content checksum, owner, group, and mode. |
| `package` | `present`, `absent`, `latest`, `<version>` | `present` means "anything but absent"; `latest` is treated as "not absent" because the platform can lie about latest; an exact version uses `VersionCmp`. |
| `service` | `running`, `stopped` | Independent `enable` axis handled in a second switch; defaults to `running`. |

## Two invariants worth internalizing

{{% notice style="warning" title="Load-bearing decision" %}}
In noop mode, `Changed` stays true. The provider verbs are skipped, but the flow still records
that a change would have happened: `package` and `service` force `changed = true` when
`noop && refreshState`, and set a `NoopMessage`. A noop event therefore reports both
`Noop=true` and `Changed=true`. The convergence re-check is skipped under noop, since nothing
actually changed.
{{% /notice %}}

Convergence is verified, not assumed. After a non-noop change the type re-reads status and
re-checks the desired state, failing with the specific mismatch reason if the system did not
converge. This is why `isDesiredState` returns a reason string, not just a bool.

## Subscribe and refresh

Only the `service` type uses refresh. Inside its `ApplyResource` it calls
`ShouldRefresh(properties.Subscribe)`, which asks the manager whether the last recorded event
for each subscribed resource had `Changed == true`. A refresh is suppressed unless the service
is ensured `running`, and skipped when the service is currently stopped, since starting it
already covers the change. When it fires, the provider restarts the service and the event is
marked `Refreshed`.

{{% notice style="tip" title="Next" %}}
Continue to [The Apply Engine]({{% relref "apply-engine" %}}) to see how a manifest of many
resources is parsed, ordered, and executed.
{{% /notice %}}
