# Observability

## What exists today

Both binaries expose `/metrics` on their main port with the Go runtime collector, the process
collector, and a build-info gauge:

```
pulse_build_info{service="ingest-api",tag="v0.1.0",commit="abc1234",go_version="go1.26.5"} 1
```

`pulse_build_info` lets a dashboard annotate a latency graph with the exact build that
produced it — which is how you find out that the p99 doubled at 14:32 because of a deploy and
not because of traffic.

There is one registry for the whole process, `metrics.Registry`. It is deliberately not
`prometheus.DefaultRegisterer`, so a third-party library cannot add metrics without someone
noticing.

ClickHouse exposes its own Prometheus endpoint on port 9363, enabled in
`deploy/clickhouse/config.d/pulse.xml`.

## Application metrics

<Badge type="warning" text="Level 6" />

| Metric | Type | Labels |
|---|---|---|
| `pulse_events_received_total` | counter | site, event_name |
| `pulse_events_rejected_total` | counter | site, reason |
| `pulse_events_dropped_total` | counter | site |
| `pulse_buffer_size` | gauge | worker |
| `pulse_batch_size` | histogram | |
| `pulse_batch_flush_duration_seconds` | histogram | result |
| `pulse_clickhouse_query_duration_seconds` | histogram | endpoint |
| `pulse_kafka_consumer_lag` | gauge | topic, partition |
| `pulse_http_request_duration_seconds` | histogram | method, route, status |
| `pulse_end_to_end_lag_seconds` | histogram | |

`pulse_end_to_end_lag_seconds` is `ingested_at − event_time`. It is the one number that says
whether the pipeline is keeping up; consumer lag says whether Kafka is backing up, but
end-to-end lag says whether the dashboard is telling the truth.

::: danger Cardinality
Labels use the **route pattern** — `/api/v1/sites/:id` — never the raw path. A raw path turns
every unique URL into a new time series and will take down Prometheus before it takes down the
application. The same rule applies to `site` labels once there are many tenants.
:::

## Dashboards

Four, provisioned from files in `deploy/grafana/dashboards/` so they are code, not clicks:

1. **Ingest health** — events received and rejected, buffer depth, drops, batch size, flush
   duration.
2. **ClickHouse internals** — parts per table, merge activity, memory, disk, query duration
   from `system.query_log`.
3. **Kafka** — consumer lag per partition, records processed, DLQ rate, rebalances.
4. **API RED** — rate, errors and duration per route.

## Alerts

Four rules that map to real failure modes rather than to arbitrary thresholds:

| Alert | Why it matters |
|---|---|
| Consumer lag climbing for 10 minutes | The pipeline is falling behind; the dashboard is stale |
| Disk above 75% | ClickHouse stops accepting writes when it fills up, and recovering is slow |
| Ingest error rate above 1% | Events are being lost right now |
| No events received for 5 minutes | Either traffic stopped or the pipeline broke — both need a human |

## Logging

`log/slog` with the JSON handler in production, text locally. Every line carries `service`,
`env` and `request_id`; request logs add `method`, `route`, `status`, `bytes` and `duration`,
plus `site_id` once authentication is in place.

`/healthz`, `/readyz` and `/metrics` are excluded from the request log — they are polled every
few seconds and would bury real traffic.

Status codes map to levels: 5xx logs at error, 4xx at warn, everything else at info. A
recovered panic logs at error with a truncated stack trace and returns the standard `500`
envelope.

**Never log an event payload.** Level 6 adds a test that asserts no payload appears in
info-level output (task L6-10) — a rule nobody checks is a rule that decays.

## Tracing

<Badge type="info" text="Optional, Level 5" />

OpenTelemetry spanning HTTP → service → ClickHouse query, exported to Jaeger or Tempo. Marked
optional: with structured logs carrying a request id and per-endpoint histograms, tracing adds
less here than it would in a system with many services.

## Health endpoints

| Endpoint | Question it answers | Behaviour when ClickHouse is down |
|---|---|---|
| `/healthz` | Should this process be restarted? | Still `200` |
| `/readyz` | Should this process receive traffic? | `503`, with the reason per dependency |

Getting this backwards — probing storage from the liveness endpoint — turns a brief ClickHouse
hiccup into a restart loop across every replica.

Adding a dependency to readiness means implementing two methods:

```go
type Prober interface {
	Name() string
	Check(ctx context.Context) error
}
```

and passing it to the router constructor. Each check is bounded by a two-second timeout.
