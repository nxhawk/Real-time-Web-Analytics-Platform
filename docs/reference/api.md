# HTTP API

Base path `/api/v1`. Every response is JSON, `Content-Type: application/json; charset=utf-8`.

The machine-readable contract is `docs/api/openapi.yaml`, which is the single source of truth
— frontend types are generated from it and never hand-edited. This page is the human summary.

::: info Availability
Only the operational endpoints exist today. Ingest and analytics arrive in Level 1, the
advanced analytics endpoints in Level 5. Each row below is marked.
:::

## Operational

Served by both binaries.

| Method | Path | Description | Status |
|---|---|---|---|
| `GET` | `/healthz` | Liveness — `200` for as long as the process is alive. Never touches a dependency | ✅ |
| `GET` | `/readyz` | Readiness — pings every registered dependency; `503` with a per-dependency reason if any fails | ✅ |
| `GET` | `/metrics` | Prometheus exposition | ✅ |
| `GET` | `/version` | Tag, commit sha, build time, Go version | ✅ |

```bash
$ curl -s localhost:8080/readyz | jq
{
  "status": "ok",
  "checks": {}
}

$ curl -s localhost:8080/version | jq
{
  "tag": "v0.1.0",
  "commit": "abc1234",
  "build_time": "2026-08-12T10:00:00Z",
  "go_version": "go1.26.5"
}
```

`healthz` and `readyz` answer different questions. Liveness asks "should I be restarted";
readiness asks "should I receive traffic". A ClickHouse outage must not restart a healthy
process, so only `/readyz` degrades.

## Ingest

| Method | Path | Auth | Description | Status |
|---|---|---|---|---|
| `POST` | `/events` | `X-API-Key` | Accept 1–500 events, partial success | <Badge type="warning" text="L1" /> |
| `GET` | `/pixel.gif` | `?k=` | No-JavaScript fallback pixel | <Badge type="warning" text="L1" /> |

**Partial success.** A batch of 100 events containing 3 invalid ones accepts 97 and returns
`202` with a `rejected` array. One malformed event never discards a batch.

```json
{
  "accepted": 97,
  "rejected": [
    { "index": 12, "reason": "event name must match ^[a-z0-9_]{1,64}$" },
    { "index": 40, "reason": "properties exceed 8KB" },
    { "index": 71, "reason": "page exceeds 2048 characters" }
  ]
}
```

The payload shape is on the [event schema](/reference/event-schema) page.

## Analytics

Every analytics endpoint requires `X-API-Key` or the dashboard session cookie. The API key
determines `site_id`, and `site_id` is enforced in every query — there are no cross-tenant
reads.

**Shared query parameters:** `from`, `to` (ISO date or datetime), `tz` (default
`Asia/Ho_Chi_Minh`; data is stored in UTC), `filter[device]`, `filter[country]`,
`filter[page]`, `filter[event]`.

| Method | Path | Returns | Status |
|---|---|---|---|
| `GET` | `/analytics/overview` | users, sessions, events, pageviews, revenue, bounce rate, average session, deltas | <Badge type="warning" text="L1" /> |
| `GET` | `/analytics/timeseries` | `{series:[{ts,value}], interval}`; `metric=users\|events\|sessions\|revenue`, `interval=hour\|day\|week` | <Badge type="warning" text="L1" /> |
| `GET` | `/analytics/pages` | `{items:[{page,views,users,avg_time_sec}], total}` | <Badge type="warning" text="L1" /> |
| `GET` | `/analytics/devices` | `{items:[{name,users,events,pct}]}` | <Badge type="warning" text="L1" /> |
| `GET` | `/analytics/countries` | as above | <Badge type="warning" text="L1" /> |
| `GET` | `/analytics/browsers` | as above | <Badge type="warning" text="L5" /> |
| `GET` | `/analytics/os` | as above | <Badge type="warning" text="L5" /> |
| `GET` | `/analytics/sources` | referrer domain and UTM breakdown | <Badge type="warning" text="L5" /> |
| `GET` | `/analytics/funnel` | `{steps:[{name,users,conv_from_prev,conv_from_first}]}`; 2–8 steps, window 60s–7d | <Badge type="warning" text="L5" /> |
| `GET` | `/analytics/retention` | `{cohorts:[{date,size,values:[…]}]}`; `cohort=day\|week` | <Badge type="warning" text="L5" /> |
| `GET` | `/analytics/realtime` | `{active_users, events_last_5m, top_pages, by_country}` | <Badge type="warning" text="L5" /> |
| `GET` | `/analytics/events` | raw event stream, cursor-paginated on `(event_time, event_id)` | <Badge type="warning" text="L5" /> |
| `GET` | `/analytics/export?format=csv` | streamed CSV via ClickHouse `FORMAT CSVWithNames` | <Badge type="warning" text="L5" /> |

## Errors

One envelope, everywhere:

```json
{
  "error": {
    "code": "invalid_range",
    "message": "from must be before to",
    "details": {}
  },
  "request_id": "0192f8a1-0000-7000-8000-000000000001"
}
```

`request_id` is a UUIDv7 generated per request, echoed in the `X-Request-ID` response header,
and present in every log line for that request. Quote it in a bug report.

| Code | HTTP | Meaning |
|---|---|---|
| `invalid_json` | 400 | The body is not valid JSON |
| `validation_failed` | 400 | The payload is well-formed but violates a rule |
| `unauthorized` | 401 | Missing or unknown API key |
| `not_found` | 404 | Unknown route, or method not allowed on a known one |
| `payload_too_large` | 413 | Body above `HTTP_MAX_BODY_BYTES` |
| `rate_limited` | 429 | Above the per-key or per-IP limit |
| `invalid_range` | 400 | `from` is not before `to` |
| `range_too_large` | 400 | Range above 400 days |
| `upstream_unavailable` | 503 | A dependency is down and the request cannot be served |
| `internal` | 500 | Anything unexpected; the details are in the logs, not the response |

## Limits

| Limit | Value |
|---|---|
| Events per request | 1–500 |
| Request body | 1 MiB |
| `properties` per event | 8 KB serialised |
| Query range | 400 days |
| `limit` parameter | 1000 |
| Ingest rate | 1000 requests/min per API key |
| Analytics rate | 120 requests/min per IP |
| Server-side query guards | `max_execution_time = 15`, `max_memory_usage = 4GB` |

The query guards are set on the ClickHouse user profile rather than per query, so they apply
even to a query typed by hand in `clickhouse-client`.
