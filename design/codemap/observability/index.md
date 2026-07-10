# Observability

Every applied resource produces one event. That event is recorded to a session store,
translated into Prometheus counters, and aggregated into a summary at the end of the run.
Health checks run alongside and feed the agent's remediation. Together these are how a CCM run
is observed after the fact and monitored while it runs.

{{% notice style="note" title="Where it lives" %}}
`internal/session`: the session stores. `internal/metrics`: the Prometheus collectors.
`internal/healthcheck`: the goss and nagios runners. `model/transaction.go` and
`model/healthcheck.go`: the event and health-check types. Key files: `internal/session/directory.go`,
`internal/session/util.go`, `internal/metrics/metrics.go`.
{{% /notice %}}

## The event flow

<figure class="cm-diagram">
  <svg viewBox="0 0 760 230" role="img" aria-label="An applied resource produces a transaction event that is recorded to a session store, then translated into Prometheus counters and aggregated into a session summary">
    <defs>
      <marker id="ah" markerWidth="9" markerHeight="9" refX="7" refY="3" orient="auto">
        <path d="M0,0 L7,3 L0,6 Z" fill="var(--cm-accent)"/>
      </marker>
    </defs>
    <rect class="cm-svg-box" x="16" y="96" width="150" height="56" rx="8"/>
    <text class="cm-svg-label" x="91" y="128" text-anchor="middle">Apply resource</text>
    <rect x="206" y="96" width="170" height="56" rx="8"
          fill="color-mix(in srgb, var(--cm-accent2) 14%, transparent)" stroke="var(--cm-accent2)"/>
    <text class="cm-svg-label" x="291" y="120" text-anchor="middle" style="fill:var(--cm-accent2)">TransactionEvent</text>
    <text class="cm-svg-sub"   x="291" y="136" text-anchor="middle">KSUID id · flags</text>
    <rect class="cm-svg-box" x="416" y="86" width="170" height="76" rx="8"/>
    <text class="cm-svg-label" x="501" y="112" text-anchor="middle">Session store</text>
    <text class="cm-svg-sub"   x="501" y="128" text-anchor="middle">memory (default)</text>
    <text class="cm-svg-sub"   x="501" y="144" text-anchor="middle">directory: JSON / event</text>
    <rect x="616" y="34" width="130" height="54" rx="8"
          fill="color-mix(in srgb, var(--cm-accent) 12%, transparent)" stroke="var(--cm-accent)"/>
    <text class="cm-svg-label" x="681" y="58" text-anchor="middle" style="fill:var(--cm-accent)">Metrics</text>
    <text class="cm-svg-sub"   x="681" y="74" text-anchor="middle">choria_ccm_*</text>
    <rect class="cm-svg-box" x="616" y="158" width="130" height="54" rx="8"/>
    <text class="cm-svg-label" x="681" y="182" text-anchor="middle">Summary</text>
    <text class="cm-svg-sub"   x="681" y="198" text-anchor="middle">on StopSession</text>
    <line x1="166" y1="124" x2="206" y2="124" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="376" y1="124" x2="416" y2="124" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <text class="cm-svg-sub" x="396" y="116" text-anchor="middle">record</text>
    <line x1="586" y1="110" x2="616" y2="62"  stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
    <line x1="586" y1="138" x2="616" y2="184" stroke="var(--cm-accent)" stroke-width="1.5" marker-end="url(#ah)"/>
  </svg>
  <figcaption>One event per resource, recorded once, then fanned out to counters and the run summary.</figcaption>
</figure>

## Sessions and events

`StartSession` records a start event, each applied resource records a `TransactionEvent`, and
`StopSession` builds a `SessionSummary` from all events. The memory store keeps events in a
slice and is the default. The directory store writes one JSON file per event, named by the
event's KSUID.

{{% notice style="warning" title="Load-bearing decision" %}}
Event IDs are KSUIDs, and that choice does double duty. KSUIDs are timestamp-prefixed and
lexicographically sortable, so sorting the `.event` filenames reconstructs chronological order
without reading timestamps. Parsing the ID also whitelists it to base62 with no path
separators, which is the directory store's defense against path traversal, since the ID becomes
a filename.
{{% /notice %}}

The directory store is append-only and crash-friendly. Each record is an independent file
write with no shared index to corrupt, and replay is a directory scan that reads each event's
`protocol` field to pick the concrete type before unmarshaling.

## Metrics

Collectors live under the `choria` namespace and `ccm` subsystem. `updateMetrics`
(`internal/session/util.go:12`) runs from both stores' `RecordEvent`. It increments a total and
exactly one outcome counter, chosen by a priority order: noop, then changed, skipped, refreshed,
failed, error, and stable.

{{% notice style="warning" title="Load-bearing decision" %}}
Noop is checked first in that priority order. A noop run has `Changed=true` on its event, but it
counts as noop, not changed, so the counters stay mutually consistent: each event increments the
total plus exactly one state counter.
{{% /notice %}}

Durations are recorded with timers around the work: manifest apply, resource apply, fact gather,
and per-check health-check time. `RegisterMetrics` registers the collectors and `ListenAndServe`
serves them at `/metrics` when a port is set.

## Health checks

Health checks run for both apply and health-check-only modes. Each check dispatches by format.

<dl class="cm-kv">
  <dt>nagios</dt><dd>Runs a command through the manager's runner and maps its exit code to a status: 0 is OK, 1 Warning, 2 Critical, anything else Unknown. Retries up to <code>tries</code>, sleeping between attempts, stopping on OK.</dd>
  <dt>goss</dt><dd>Renders the goss rules through the template engine, writes a temp spec, validates it, and reports Critical when any check failed, else OK. It never emits Warning or Unknown.</dd>
  <dt>Status</dt><dd><code>HealthCheckStatus</code> values equal the nagios exit codes, so the plugin format maps directly and goss reuses the same enum.</dd>
</dl>

A non-OK result or an execution error marks the event failed. In the agent, a critical result
increments a remediation counter and queues a priority apply, so a failing check drives a
corrective run. Several `Agent*` metrics are declared here but recorded at their call sites in
the agent, and `AgentHealthCheckTime` is currently registered without a call site.

{{% notice style="tip" title="Next" %}}
Continue to the [Reference and Map]({{% relref "reference" %}}) for the CLI surface, the source
map, and a glossary.
{{% /notice %}}
