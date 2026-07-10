# Registration and Discovery

Registration is service discovery without a discovery daemon. When a managed resource reaches
a stable state, it publishes an entry into a NATS JetStream stream. Other nodes read those
entries, usually from a template, to build configuration such as a load-balancer upstream or a
Prometheus target list. Publishing is a side effect of `apply`; lookup is a template function
or a CLI query.

{{% notice style="note" title="Where it lives" %}}
`registration`: the transport, subject grammar, stream, lookup, and watch. `model/registration.go`
and `model/registration_ttl.go`: the entry and TTL data model. Key files:
`registration/subject.go`, `registration/stream.go`, `registration/nats.go`,
`registration/lookup.go`, `registration/watch.go`.
{{% /notice %}}

## The publish gate

A resource carries a `register_when_stable` list. After the resource applies,
`publishRegistration` (`resources/apply/apply.go:736`) decides whether to announce it. The gate
is conservative: it publishes only when the resource has entries, the manager is not in noop
mode, the event did not fail, and every health check returned OK. A resource with no health
checks passes that last test.

{{% notice style="warning" title="Load-bearing decision" %}}
A missing registration publisher is tolerated, not fatal. The manager defaults to a no-op
publisher, so a `register_when_stable` manifest runs safely on a node with no registration
backend. This is what lets the same manifest apply on nodes with and without NATS.
{{% /notice %}}

## A KV store built from a stream

<figure class="cm-diagram">
  <svg viewBox="0 0 760 240" role="img" aria-label="A node publishes a stable resource into the JetStream registration stream; other nodes look up the last entry per subject or watch for register and remove events">
    <defs>
      <marker id="ah" markerWidth="9" markerHeight="9" refX="7" refY="3" orient="auto">
        <path d="M0,0 L7,3 L0,6 Z" fill="var(--cm-accent)"/>
      </marker>
    </defs>
    <rect class="cm-svg-box" x="16" y="60" width="200" height="90" rx="8"/>
    <text class="cm-svg-label" x="116" y="98" text-anchor="middle">Node A · apply</text>
    <text class="cm-svg-sub"   x="116" y="116" text-anchor="middle">resource stable +</text>
    <text class="cm-svg-sub"   x="116" y="130" text-anchor="middle">health checks pass</text>
    <rect x="280" y="50" width="200" height="110" rx="10"
          fill="color-mix(in srgb, var(--cm-accent) 12%, transparent)" stroke="var(--cm-accent)"/>
    <text class="cm-svg-label" x="380" y="88" text-anchor="middle" style="fill:var(--cm-accent)">JetStream</text>
    <text class="cm-svg-sub"   x="380" y="106" text-anchor="middle">REGISTRATION</text>
    <text class="cm-svg-sub"   x="380" y="122" text-anchor="middle">1 msg / subject</text>
    <text class="cm-svg-sub"   x="380" y="138" text-anchor="middle">TTL · rollup · markers</text>
    <rect class="cm-svg-box" x="544" y="44" width="200" height="52" rx="8"/>
    <text class="cm-svg-label" x="644" y="66" text-anchor="middle">Node B · lookup()</text>
    <text class="cm-svg-sub"   x="644" y="82" text-anchor="middle">renders config</text>
    <rect class="cm-svg-box" x="544" y="118" width="200" height="52" rx="8"/>
    <text class="cm-svg-label" x="644" y="140" text-anchor="middle">Watcher</text>
    <text class="cm-svg-sub"   x="644" y="156" text-anchor="middle">register / remove</text>
    <line x1="216" y1="105" x2="280" y2="105" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <text class="cm-svg-sub" x="248" y="97" text-anchor="middle">publish</text>
    <line x1="480" y1="85"  x2="544" y2="70"  stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="480" y1="125" x2="544" y2="144" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <text class="cm-svg-sub" x="380" y="196" text-anchor="middle">choria.ccm.registration.v1.&lt;cluster&gt;.&lt;protocol&gt;.&lt;service&gt;.&lt;address&gt;.&lt;instance&gt;</text>
  </svg>
  <figcaption>The stream behaves like a KV store keyed on the entry tuple; readers take the last message per subject.</figcaption>
</figure>

Each entry maps to a single subject. The address contributes its IP with dots turned into
underscores so it is one token, and an `InstanceId` FNV hash of the full tuple forms the final
token, so every unique cluster, service, protocol, address, and port maps to one stable
subject. The stream captures the whole namespace with `MaxMsgsPerSubject: 1` and `AllowRollup`,
and reliable publishes carry a `Nats-Rollup: sub` header.

{{% notice style="warning" title="Load-bearing decision" %}}
One message per subject plus rollup plus a hashed per-instance subject makes the stream behave
like a KV store keyed on the entry tuple. Re-registering overwrites in place, and a read is
simply "last message per subject." Full KV semantics only hold with the JetStream destination;
the plain `nats` destination is best-effort and omits the TTL and rollup headers.
{{% /notice %}}

## Expiry, lookup, and watch

Expiry is two-tiered. A per-entry `Nats-TTL` header sits under the stream-wide `MaxAge`, and on
removal the server writes a subject delete marker so watchers see the removal rather than a
silent disappearance. The stream denies hard deletes but allows purge, so `JetStreamRemove` can
retire a single entry.

<dl class="cm-kv">
  <dt>Lookup</dt><dd><code>JetStreamLookup</code> builds a wildcard filter subject, fetches the last message per matching subject, drops server marker messages, and returns entries sorted by address then port.</dd>
  <dt>Watch</dt><dd><code>JetStreamWatch</code> streams <code>WatchEvent</code>s: normal messages become <code>Register</code> events, marker messages become <code>Remove</code> events with the entry reconstructed from the subject.</dd>
  <dt>From a template</dt><dd>The manager exposes lookup as <code>registrations(cluster, protocol, service, ip)</code>, so a rendered config file discovers its peers directly.</dd>
  <dt>Prometheus</dt><dd><code>RegistrationEntries.PrometheusFileSD()</code> converts entries into file_sd JSON, grouped by cluster, service, and protocol.</dd>
</dl>

{{% notice style="warning" title="Load-bearing decision" %}}
Both lookup and watch special-case the `Nats-Marker-Reason` header, per ADR-43. Lookup drops
markers; watch turns them into removals. A reader that ignored markers would surface expired or
purged entries as live services.
{{% /notice %}}

The `Priority` and `Annotations` fields travel on every entry but, within this subsystem, are
only consumed by the Prometheus conversion. Priority is validated and sorted around but not
otherwise acted on here, reserved for downstream consumers such as SRV-style weighting.

{{% notice style="tip" title="Next" %}}
Continue to [Observability]({{% relref "observability" %}}) to see how each apply is recorded,
measured, and health-checked.
{{% /notice %}}
