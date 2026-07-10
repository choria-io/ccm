+++
title = "The Apply Engine"
weight = 40
description = "How a manifest of many resources is parsed, resolved against data, and executed in order, with require as a gate and events as the shared state."
+++

The apply engine turns a manifest into a sequence of applied resources. It resolves the
manifest from a file, an HTTP tarball, or a NATS object store; layers Hiera data and
overrides over it; builds each resource; and runs them in declaration order, recording an
event after each so later resources can see what earlier ones did.

{{% notice style="note" title="Where it lives" %}}
`resources/apply`: the manifest parser and executor. `resources/applyresource` and
`resources/applyresource/ccmmanifest`: the `apply` meta-resource that lets one manifest apply
another. Key files: `resources/apply/apply.go`, `resources/apply/jet.go`,
`resources/apply/validation.go`, `resources/applyresource/ccmmanifest/ccmmanifest.go`.
{{% /notice %}}

## Resolving a manifest

`ResolveManifestReader` (`resources/apply/apply.go:395`) is the core resolver. It runs before
any resource executes, and its ordering is precise: data is resolved and templates are
expanded before the schema is checked, so a runtime template does not fail a structural
validation.

<ol class="cm-steps">
  <li><b>Resolve the source</b> <code>ResolveManifestUrl</code> dispatches on scheme: <code>obj://</code> to the object store, <code>http(s)</code> to a tarball fetch, empty scheme to a local file. Archive paths untar, find <code>manifest.yaml</code>, and set the working directory.</li>
  <li><b>Parse the manifest</b> Unmarshal the top-level <code>data</code>, <code>hierarchy</code>, and <code>overrides</code>, plus the <code>ccm</code> block with <code>pre_message</code>, <code>post_message</code>, <code>fail_on_error</code>, <code>resources</code>, and <code>resources_jet_file</code>.</li>
  <li><b>Resolve Hiera</b> <code>hiera.ResolveYaml</code> consumes <code>hierarchy.order</code>, <code>merge</code>, and <code>overrides</code>, returning the resolved data and validation rules. Overriding data is deep-merged on top, then the rules are enforced.</li>
  <li><b>Publish data</b> <code>mgr.SetData</code> stores the resolved data and the template environment is built from it, so resource fields can reference <code>Data</code>.</li>
  <li><b>Produce the resource list</b> Either the inline <code>ccm.resources</code>, or, if <code>resources_jet_file</code> is set, the rendered output of a Jet template. Multi-name blocks are flattened in place, preserving order.</li>
  <li><b>Parse and template each resource</b> <code>NewValidatedResourcePropertiesFromYaml</code> builds typed properties per type, resolves templates, and validates.</li>
  <li><b>Validate against the schema</b> Last, the resolved payload is checked against <code>schemas/manifest.json</code>, substituting placeholders for still-deferred template fields. <code>NO_SCHEMA_VALIDATION=1</code> bypasses this.</li>
</ol>

<figure class="cm-diagram">
  <svg viewBox="0 0 760 320" role="img" aria-label="The apply pipeline: resolve a source, parse, resolve Hiera and overrides, build the resource list, validate, then execute a per-resource loop in declaration order">
    <defs>
      <marker id="ah" markerWidth="9" markerHeight="9" refX="7" refY="3" orient="auto">
        <path d="M0,0 L7,3 L0,6 Z" fill="var(--cm-accent)"/>
      </marker>
      <marker id="ahd" markerWidth="9" markerHeight="9" refX="7" refY="3" orient="auto">
        <path d="M0,0 L7,3 L0,6 Z" fill="var(--cm-faint)"/>
      </marker>
    </defs>
    <rect class="cm-svg-box" x="16"  y="20" width="118" height="46" rx="8"/>
    <text class="cm-svg-label" x="75" y="41" text-anchor="middle">Source</text>
    <text class="cm-svg-sub"   x="75" y="56" text-anchor="middle">obj · http · file</text>
    <rect class="cm-svg-box" x="150" y="20" width="96" height="46" rx="8"/>
    <text class="cm-svg-label" x="198" y="47" text-anchor="middle">Parse</text>
    <rect class="cm-svg-box" x="262" y="20" width="140" height="46" rx="8"/>
    <text class="cm-svg-label" x="332" y="41" text-anchor="middle">Hiera</text>
    <text class="cm-svg-sub"   x="332" y="56" text-anchor="middle">+ overrides</text>
    <rect class="cm-svg-box" x="418" y="20" width="120" height="46" rx="8"/>
    <text class="cm-svg-label" x="478" y="47" text-anchor="middle">Resource list</text>
    <rect class="cm-svg-box" x="554" y="20" width="110" height="46" rx="8"/>
    <text class="cm-svg-label" x="609" y="47" text-anchor="middle">Validate</text>
    <line x1="134" y1="43" x2="150" y2="43" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="246" y1="43" x2="262" y2="43" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="402" y1="43" x2="418" y2="43" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="538" y1="43" x2="554" y2="43" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <path d="M609,66 L609,104 L380,104 L380,132" fill="none" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <rect x="70" y="132" width="620" height="168" rx="12" fill="var(--cm-box-bg)" stroke="var(--cm-border)"/>
    <text class="cm-svg-sub" x="86" y="126">Execute · session</text>
    <rect class="cm-svg-box" x="96"  y="196" width="160" height="48" rx="8"/>
    <text class="cm-svg-label" x="176" y="224" text-anchor="middle">Build resource</text>
    <rect x="300" y="196" width="160" height="48" rx="8"
          fill="color-mix(in srgb, var(--cm-accent2) 14%, transparent)" stroke="var(--cm-accent2)"/>
    <text class="cm-svg-label" x="380" y="224" text-anchor="middle" style="fill:var(--cm-accent2)">Apply / check</text>
    <rect class="cm-svg-box" x="504" y="196" width="160" height="48" rx="8"/>
    <text class="cm-svg-label" x="584" y="217" text-anchor="middle">Record event</text>
    <text class="cm-svg-sub"   x="584" y="233" text-anchor="middle">+ publish</text>
    <line x1="256" y1="220" x2="300" y2="220" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="460" y1="220" x2="504" y2="220" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <path d="M584,196 L584,170 L176,170 L176,196" fill="none" stroke="var(--cm-faint)" stroke-width="1.4" stroke-dasharray="5 4" marker-end="url(#ahd)"/>
    <text class="cm-svg-sub" x="380" y="164" text-anchor="middle">next resource · declaration order</text>
    <text class="cm-svg-sub" x="584" y="266" text-anchor="middle">fail_on_error: stop</text>
  </svg>
  <figcaption>Resolve once, then loop the resources in declaration order, recording each event before the next runs.</figcaption>
</figure>

## Execution and ordering

`Execute` (`resources/apply/apply.go:603`) opens a session, then iterates the resources. For
each it builds the concrete resource through the `ResourceFactory`, calls `Apply` or
`Healthcheck`, logs the result, records the event, and publishes any `register_when_stable`
entries. When `fail_on_error` is set, a failed resource stops the run after the current entry.

{{% notice style="warning" title="Load-bearing decision" %}}
There is no dependency graph and no topological sort. Resources run in manifest declaration
order. `require` is a fail-gate, not a scheduler: a resource whose required references failed
or were themselves skipped is skipped, not reordered. Authors must place producers before
consumers. This is the single most important invariant of the engine.
{{% /notice %}}

Cross-resource behavior is stateful through the session. Because `Execute` records each event
immediately, `IsResourceFailed` and `ShouldRefresh` inspect the last event for a reference, so
a later resource sees an earlier one's change. This is again why declaration order matters.

## Generating resources with Jet

When `resources_jet_file` is set instead of inline `resources`, `jetParseManifestResources`
(`resources/apply/jet.go:19`) renders a Jet template with the delimiters `[[` and `]]`, chosen
so they do not collide with the `{{ }}` templating used for scalar fields. The resolved Hiera
`Data` is in scope, so a template can loop over `Data.packages` to emit a resource per item.
The rendered YAML is then parsed exactly like inline resources. Jet generates the resource
list; ordinary templating fills in individual fields.

## Nested applies

The `apply` resource type lets one manifest apply another. Its sole provider,
`ccmmanifest.Provider` (`resources/applyresource/ccmmanifest/ccmmanifest.go:31`), snapshots the
manager's noop, data, and working directory, runs the child inside the parent's session so
events are shared, and restores the manager afterward.

{{% notice style="warning" title="Load-bearing decision" %}}
A nested apply can only strengthen execution, never relax it. Noop is turned on only if the
parent is not already noop and the child requests it; health-check-only is the OR of parent and
child. A child cannot escape the parent's noop or health-check state. A parent can also forbid
nested applies entirely with `WithDenyApplyResources` unless the child sets `AllowApply`.
{{% /notice %}}

Recursion is bounded by `DefaultMaxRecursionDepth = 10`. Note that the depth guard is not fully
threaded through the apply-resource boundary today: `Type.ApplyResource` calls `ApplyManifest`
with a hardcoded depth of `0`, so the guard is effectively aspirational for deeply nested
manifests. Two `TODO`s in the executor also flag that a resource-factory error currently aborts
the whole run rather than recording a failed event, and that resource dispatch should move into
the registry.

{{% notice style="tip" title="Next" %}}
Continue to [The Agent]({{% relref "agent" %}}) to see how the engine is driven continuously on
a timer.
{{% /notice %}}
