# Code Map

CCM is a declarative configuration management engine written in Go. A manifest or a
command names resources and the desired state for each; the engine drives every resource
to that state and reports what changed. This code map is a deep-dive into how the Go code
is built, for contributors and for anyone who wants to understand the machine behind the
manifests.

{{% notice style="note" title="Snapshot" %}}
Generated 2026-07-10 against commit `d1b8674` on branch `main`. The working tree was clean
at capture time. Commits after this one may make parts of this map stale.
{{% /notice %}}

## The mental model

CCM has one core and many faces. The core is a small loop: a `Manager` holds the run's
facts, data, and session; a `Resource` decides whether the system already matches the
desired state; a `Provider` makes the platform-specific change when it does not. Every
entry point drives that same loop. The CLI applies one resource. The apply engine walks a
manifest of many. The background agent applies manifests on a timer. A piped wire API
applies a resource sent as JSON. The logic underneath is identical, so behavior does not
drift between how a resource is invoked.

Providers are selected at run time. A resource type declares a capability interface, and
each provider registers a factory that reports whether it can manage the resource on this
host. The registry picks the best match from the gathered facts, so the same manifest runs
`apt` on Debian and `dnf` on RHEL with no change to the resource.

<figure class="cm-diagram">
  <svg viewBox="0 0 760 300" role="img" aria-label="CCM system at a glance: entry points drive a manager, which drives resources and providers against the system, fed by facts and data">
    <defs>
      <marker id="ah" markerWidth="9" markerHeight="9" refX="7" refY="3" orient="auto">
        <path d="M0,0 L7,3 L0,6 Z" fill="var(--cm-accent)"/>
      </marker>
    </defs>
    <text class="cm-svg-sub" x="16" y="16">Entry points</text>
    <rect class="cm-svg-box" x="16"  y="24"  width="150" height="40" rx="8"/>
    <text class="cm-svg-label" x="91" y="48" text-anchor="middle">CLI · ensure</text>
    <rect class="cm-svg-box" x="16"  y="76"  width="150" height="40" rx="8"/>
    <text class="cm-svg-label" x="91" y="100" text-anchor="middle">Manifest · apply</text>
    <rect class="cm-svg-box" x="16"  y="128" width="150" height="40" rx="8"/>
    <text class="cm-svg-label" x="91" y="152" text-anchor="middle">Agent</text>
    <rect class="cm-svg-box" x="16"  y="180" width="150" height="40" rx="8"/>
    <text class="cm-svg-label" x="91" y="204" text-anchor="middle">Wire API</text>
    <line x1="166" y1="44"  x2="248" y2="112" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="166" y1="96"  x2="248" y2="118" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="166" y1="148" x2="248" y2="124" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="166" y1="200" x2="248" y2="130" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <rect x="250" y="64" width="150" height="116" rx="10"
          fill="color-mix(in srgb, var(--cm-accent) 12%, transparent)" stroke="var(--cm-accent)"/>
    <text class="cm-svg-label" x="325" y="116" text-anchor="middle" style="fill:var(--cm-accent)">Manager</text>
    <text class="cm-svg-sub"   x="325" y="134" text-anchor="middle">facts · data · session</text>
    <rect class="cm-svg-box" x="250" y="216" width="150" height="44" rx="8"/>
    <text class="cm-svg-label" x="325" y="236" text-anchor="middle">Facts · Data</text>
    <text class="cm-svg-sub"   x="325" y="252" text-anchor="middle">Hiera · Templates</text>
    <line x1="325" y1="216" x2="325" y2="182" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <text class="cm-svg-sub" x="470" y="16">Resource lifecycle</text>
    <rect class="cm-svg-box" x="470" y="24"  width="180" height="46" rx="8"/>
    <text class="cm-svg-label" x="560" y="44" text-anchor="middle">Resource</text>
    <text class="cm-svg-sub"   x="560" y="60" text-anchor="middle">decides desired vs actual</text>
    <rect x="470" y="112" width="180" height="46" rx="8"
          fill="color-mix(in srgb, var(--cm-accent2) 14%, transparent)" stroke="var(--cm-accent2)"/>
    <text class="cm-svg-label" x="560" y="132" text-anchor="middle" style="fill:var(--cm-accent2)">Provider</text>
    <text class="cm-svg-sub"   x="560" y="148" text-anchor="middle">acts on the platform</text>
    <rect class="cm-svg-box" x="470" y="200" width="180" height="46" rx="8"/>
    <text class="cm-svg-label" x="560" y="220" text-anchor="middle">System</text>
    <text class="cm-svg-sub"   x="560" y="236" text-anchor="middle">packages · files · services</text>
    <line x1="400" y1="100" x2="470" y2="47"  stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="560" y1="70"  x2="560" y2="112" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="560" y1="158" x2="560" y2="200" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <text class="cm-svg-sub" x="664" y="139">registry</text>
    <text class="cm-svg-sub" x="664" y="153">selects by</text>
    <text class="cm-svg-sub" x="664" y="167">facts</text>
  </svg>
  <figcaption>One core loop, many faces. Every entry point drives the same manager, resource, and provider machinery.</figcaption>
</figure>

## Explore

{{% children description="true" %}}

{{% notice style="tip" title="Next" %}}
Start with the [Architecture]({{% relref "architecture" %}}) page for the package layering
and the contracts every subsystem programs against.
{{% /notice %}}
