# The Agent

The agent runs CCM continuously. It resolves facts and external data, fetches manifests from
one or more sources, applies them on an interval, and runs health checks that can trigger a
remediating apply between intervals. It is a single scheduler driving many per-source workers.

{{% notice style="note" title="Where it lives" %}}
`agent`: the loop and its workers. Key files: `agent/agent.go` (the `Agent` type and the
control loop), `agent/worker.go` (the per-manifest worker), `agent/config.go`,
`agent/http.go` and `agent/object.go` (remote source watchers), `agent/nc.go` (a caching NATS
connection).
{{% /notice %}}

## One scheduler, many workers

The `Agent` (`agent/agent.go:30`) creates one `worker` per manifest. Each worker only watches
its own source and requests an apply; it never schedules one. A single `applyTicker` in `Run`
drives every scheduled apply, which serializes fact refreshes and prevents two manifests from
applying at once. Workers signal the loop through a buffered-size-one `applyTrigger` channel,
and the send is coalesced so a burst of source changes collapses into one priority apply.

<dl class="cm-kv">
  <dt>DefaultInterval</dt><dd>5 minutes. The apply cadence, floored at <code>MinInterval</code> of 30 seconds.</dd>
  <dt>MinFactUpdateInterval</dt><dd>2 minutes. Facts are not re-gathered more often than this, independent of the apply interval.</dd>
  <dt>applyTrigger</dt><dd>Buffered channel of size one. A worker whose source changed pushes a priority apply that runs even inside the interval window.</dd>
  <dt>Sources</dt><dd>Dispatched by scheme in <code>worker.cacheManifest</code>: <code>obj://</code> to a JetStream object store watcher, <code>http(s)</code> to a conditional-GET fetcher, empty scheme to a local file.</dd>
</dl>

## The loop

<figure class="cm-diagram">
  <svg viewBox="0 0 760 310" role="img" aria-label="The agent loop: an interval ticker refreshes facts and data then applies each manifest; a health-check ticker runs checks and a critical result triggers a remediating apply; workers watching sources also trigger priority applies">
    <defs>
      <marker id="ah" markerWidth="9" markerHeight="9" refX="7" refY="3" orient="auto">
        <path d="M0,0 L7,3 L0,6 Z" fill="var(--cm-accent)"/>
      </marker>
      <marker id="ahd" markerWidth="9" markerHeight="9" refX="7" refY="3" orient="auto">
        <path d="M0,0 L7,3 L0,6 Z" fill="var(--cm-faint)"/>
      </marker>
    </defs>
    <rect class="cm-svg-box" x="60" y="24" width="200" height="44" rx="8"/>
    <text class="cm-svg-label" x="160" y="45" text-anchor="middle">Interval tick · 5m</text>
    <text class="cm-svg-sub"   x="160" y="60" text-anchor="middle">floor 30s</text>
    <rect class="cm-svg-box" x="60" y="100" width="200" height="44" rx="8"/>
    <text class="cm-svg-label" x="160" y="127" text-anchor="middle">Refresh facts + data</text>
    <rect x="60" y="176" width="200" height="44" rx="8"
          fill="color-mix(in srgb, var(--cm-accent) 12%, transparent)" stroke="var(--cm-accent)"/>
    <text class="cm-svg-label" x="160" y="203" text-anchor="middle" style="fill:var(--cm-accent)">Apply each manifest</text>
    <rect class="cm-svg-box" x="60" y="252" width="200" height="44" rx="8"/>
    <text class="cm-svg-label" x="160" y="273" text-anchor="middle">Workers watch sources</text>
    <text class="cm-svg-sub"   x="160" y="289" text-anchor="middle">file · http · obj://</text>
    <line x1="160" y1="68"  x2="160" y2="100" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="160" y1="144" x2="160" y2="176" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="160" y1="252" x2="160" y2="222" stroke="var(--cm-faint)" stroke-width="1.4" stroke-dasharray="5 4" marker-end="url(#ahd)"/>
    <path d="M60,198 L28,198 L28,46 L60,46" fill="none" stroke="var(--cm-faint)" stroke-width="1.4" stroke-dasharray="5 4" marker-end="url(#ahd)"/>
    <rect class="cm-svg-box" x="430" y="24" width="180" height="44" rx="8"/>
    <text class="cm-svg-label" x="520" y="51" text-anchor="middle">Health tick</text>
    <rect class="cm-svg-box" x="430" y="100" width="180" height="44" rx="8"/>
    <text class="cm-svg-label" x="520" y="121" text-anchor="middle">Run checks</text>
    <text class="cm-svg-sub"   x="520" y="137" text-anchor="middle">no fact refresh</text>
    <polygon class="cm-svg-box" points="520,174 596,198 520,222 444,198"/>
    <text class="cm-svg-label" x="520" y="202" text-anchor="middle">critical?</text>
    <line x1="520" y1="68"  x2="520" y2="100" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="520" y1="144" x2="520" y2="174" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <path d="M444,198 L260,198" fill="none" stroke="var(--cm-accent2)" stroke-width="1.5" marker-end="url(#ah)"/>
    <text class="cm-svg-sub" x="352" y="190" text-anchor="middle" style="fill:var(--cm-accent2)">remediate</text>
    <text class="cm-svg-sub" x="520" y="244" text-anchor="middle">no → ok</text>
  </svg>
  <figcaption>Scheduled applies on the left; health checks and remediation on the right feed the same apply step.</figcaption>
</figure>

## Facts, data, and resilience

Facts and data resolution sit behind a mutex and a retry policy. `getFacts` skips entirely
when the cached facts are younger than the 2-minute floor. Otherwise it retries under jittered
backoff, and after a configured number of failures it falls back to the last good facts rather
than blocking the loop. `getData` resolves external data through Hiera on each cycle and falls
back the same way. This is why a transient NATS or HTTP outage does not stall applies: the
agent keeps running on the last known-good inputs.

Health checks are deliberately independent of applies. `runHealthChecks` runs each worker in
health-check-only mode and does not refresh facts or data. A worker reporting a critical result
increments a remediation counter and queues a priority apply, but the queued applies fire only
after every check completes, so applies and checks never interleave.

## Metrics and shutdown

When a monitor port is configured, the agent registers Prometheus collectors and serves
`/metrics`. It exposes apply and health-check durations, remediation counts, manifest fetch
counts and errors, and facts and data resolve timing. Shutdown is graceful: `Run` returns on
context cancellation after waiting for the workers, and `Stop` closes the manager. Workers use
a cancel-with-cause context, so a manifest deleted from an object store propagates a readable
reason.

{{% notice style="warning" title="Load-bearing decision" %}}
All fact and data mutation happens under the agent mutex, and every scheduled or triggered
apply acquires it. The single ticker plus the size-one trigger channel is what keeps concurrent
manifests from applying over each other. Removing the shared lock or the single scheduler would
reintroduce apply races.
{{% /notice %}}

Two items are reserved rather than active. `AgentHealthCheckTime` is registered but never
observed, so the health-check duration series stays empty. A `TODO` in `agent.go` notes the
intent to watch the KV for external data and only re-fetch on change, rather than re-resolving
every cycle.

{{% notice style="tip" title="Next" %}}
Continue to [Data, Facts, and Templates]({{% relref "data-and-templates" %}}) to see how the
inputs the agent refreshes are produced.
{{% /notice %}}
