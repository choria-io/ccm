# Data, Facts, and Templates

Three cooperating packages feed every run. `facts` gathers what is true about the host.
`hiera` layers a hierarchy of data blocks into one resolved map, driven by those facts.
`templates` renders that data into resource fields. The manager wires them together and hands
the result to the apply engine.

{{% notice style="note" title="Where it lives" %}}
`facts`: system fact collectors backed by gopsutil, plus file-based facts. `hiera`: the
hierarchy resolver and its comment-driven validation. `templates`: the render environment and
the expression engines. Key files: `facts/facts.go`, `hiera/resolver.go`, `hiera/validate.go`,
`templates/templates.go`, `templates/expr.go`, `templates/jet.go`.
{{% /notice %}}

## Facts

`facts.Gather` (`facts/facts.go:17`) builds a map from the built-in families, `host`,
`network`, `partition`, `cpu`, and `memory`, each backed by gopsutil and each skippable with a
config flag. It then merges file-based facts on top: for the system config directory and then
the user directory, it reads `facts.json`, `facts.yaml`, and a sorted `facts.d/` directory,
deep-merging each in order so later sources win.

{{% notice style="warning" title="Load-bearing decision" %}}
File facts refuse symlinks and require absolute config directories. Symlinked fact files and
directories are rejected outright. This is a security invariant: fact data drives what gets
installed and where, so it must not be redirectable through a planted symlink.
{{% /notice %}}

Runtime caching lives in the manager, not the facts package. `Facts()` gathers once and
memoizes; `SystemFacts()` always re-gathers with a 2-second default deadline; `SetFacts` and
`MergeFacts` let callers override or overlay, which is how the agent reuses its last good facts
when a gather fails.

## Hiera

<figure class="cm-diagram">
  <svg viewBox="0 0 760 250" role="img" aria-label="Data resolution: facts drive a hierarchy whose levels select overrides, which merge into resolved data, which the template environment renders into resource fields">
    <defs>
      <marker id="ah" markerWidth="9" markerHeight="9" refX="7" refY="3" orient="auto">
        <path d="M0,0 L7,3 L0,6 Z" fill="var(--cm-accent)"/>
      </marker>
      <marker id="ahd" markerWidth="9" markerHeight="9" refX="7" refY="3" orient="auto">
        <path d="M0,0 L7,3 L0,6 Z" fill="var(--cm-faint)"/>
      </marker>
    </defs>
    <rect class="cm-svg-box" x="16" y="30" width="140" height="52" rx="8"/>
    <text class="cm-svg-label" x="86" y="51" text-anchor="middle">Facts</text>
    <text class="cm-svg-sub"   x="86" y="67" text-anchor="middle">gopsutil + facts.d</text>
    <rect class="cm-svg-box" x="190" y="30" width="170" height="52" rx="8"/>
    <text class="cm-svg-label" x="275" y="51" text-anchor="middle">Hierarchy + overrides</text>
    <text class="cm-svg-sub"   x="275" y="67" text-anchor="middle">levels templated by facts</text>
    <rect x="394" y="30" width="150" height="52" rx="8"
          fill="color-mix(in srgb, var(--cm-accent) 12%, transparent)" stroke="var(--cm-accent)"/>
    <text class="cm-svg-label" x="469" y="51" text-anchor="middle" style="fill:var(--cm-accent)">Merge</text>
    <text class="cm-svg-sub"   x="469" y="67" text-anchor="middle">first · deep</text>
    <rect class="cm-svg-box" x="578" y="30" width="166" height="52" rx="8"/>
    <text class="cm-svg-label" x="661" y="51" text-anchor="middle">Resolved data</text>
    <text class="cm-svg-sub"   x="661" y="67" text-anchor="middle">+ validate</text>
    <line x1="156" y1="56" x2="190" y2="56" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="360" y1="56" x2="394" y2="56" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="544" y1="56" x2="578" y2="56" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <rect x="190" y="176" width="200" height="56" rx="8"
          fill="color-mix(in srgb, var(--cm-accent2) 14%, transparent)" stroke="var(--cm-accent2)"/>
    <text class="cm-svg-label" x="290" y="199" text-anchor="middle" style="fill:var(--cm-accent2)">Template env</text>
    <text class="cm-svg-sub"   x="290" y="215" text-anchor="middle">lookup · kvGet · registrations</text>
    <rect class="cm-svg-box" x="470" y="176" width="220" height="56" rx="8"/>
    <text class="cm-svg-label" x="580" y="199" text-anchor="middle">Resource fields</text>
    <text class="cm-svg-sub"   x="580" y="215" text-anchor="middle">{{ }} · ${ } · jet</text>
    <path d="M661,82 L661,150 L290,150 L290,176" fill="none" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <path d="M86,82 L86,204 L190,204" fill="none" stroke="var(--cm-faint)" stroke-width="1.4" stroke-dasharray="5 4" marker-end="url(#ahd)"/>
    <line x1="390" y1="204" x2="470" y2="204" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
  </svg>
  <figcaption>Facts drive the hierarchy; the merged data and facts feed the template environment that renders resource fields.</figcaption>
</figure>

`Resolve` (`hiera/resolver.go:98`) starts from the `data:` block as the base, then walks
`hierarchy.order`. Each level is a template like `os:{{ lookup('facts.host.info.platformFamily') }}`.
A level counts as matched only when its embedded expression produces a non-empty value, so a
level whose fact is missing is skipped. The matched string is looked up in `overrides:` and
merged into the base.

<dl class="cm-kv">
  <dt>merge: first</dt><dd>The default. The first matching level wins; its top-level keys replace the base, and resolution returns immediately.</dd>
  <dt>merge: deep</dt><dd>Every matching level accumulates. Maps merge recursively and slices concatenate, in hierarchy order.</dd>
  <dt>Sources</dt><dd><code>ResolveUrl</code> dispatches by scheme: a local YAML or JSON file, an <code>http(s)</code> URL with basic auth, or a NATS JetStream KV document at <code>kv://Bucket/Key</code>.</dd>
  <dt>Validation</dt><dd>YAML comments <code>@require</code> and <code>@validate &lt;expr&gt;</code> become rules. Resolution returns the rules so a multi-source caller validates once after merging.</dd>
</dl>

{{% notice style="warning" title="Load-bearing decision" %}}
Hiera resolution runs in a restricted sandbox. During resolution only `facts` and `lookup()`
are available, with missing keys returning empty rather than erroring. File, KV, and
registration functions are excluded. The full function set only exists later, at render time, in
the manager-built environment. This keeps hierarchy selection from reaching into the network.
{{% /notice %}}

## Templates

`{{ expr }}` and `${ expr }` are aliases. Both route through the same expr-lang evaluator, so
`${ Data.package_name }` and `{{ Data.package_name }}` are identical. A separate `jet(...)`
renderer handles Jet templates with `[[ ]]` delimiters, used mainly to generate resource lists.

<dl class="cm-kv">
  <dt>Field access</dt><dd>Inside an expression, use Go field names: <code>Data.app_name</code>, <code>Facts.os</code>, <code>Environ.X</code>.</dd>
  <dt>lookup(key, default)</dt><dd>Takes a lowercased, dotted path rooted at the environment: <code>lookup('data.nested.key')</code>, <code>lookup('facts.env')</code>. It marshals the environment to JSON and queries it with gjson.</dd>
  <dt>Type preservation</dt><dd>When the whole string is a single expression, its native typed value is returned, so a templated port stays an integer. Mixed strings are stringified and concatenated.</dd>
  <dt>Deferred fields</dt><dd>Struct fields tagged <code>template:"deferred"</code>, such as file <code>content</code>, render in a second pass after the control gate, so an <code>unless</code> can protect a resource from a template error.</dd>
</dl>

{{% notice style="warning" title="Load-bearing decision" %}}
`kvGet(bucket, key)` and the `kv://Bucket/Key` Hiera source are different mechanisms and are
easy to conflate. `kvGet` is an in-template lookup of a single KV value at render time. `kv://`
is a whole-document Hiera data source resolved during data resolution.
{{% /notice %}}

One syntax is reserved but not implemented. The `Resolve` doc comment and a test reference
Puppet-style `env:%{env}` placeholders, but only `{{ }}` and `${ }` are recognized. A `%{...}`
entry is treated as a literal key with no matched expression, so it is skipped. Real
hierarchies use `env:{{ lookup('facts.env') }}`.

{{% notice style="tip" title="Next" %}}
Continue to [Registration and Discovery]({{% relref "registration" %}}) to see how resolved
resources announce themselves for others to find.
{{% /notice %}}
