# Event schema

## Payload

`POST /api/v1/events` accepts a single event or a batch, in the same shape.

```jsonc
{
  "site_id": "site_abc",              // required, must match the API key
  "events": [
    {
      "event_id": "0192f8a1-...",     // UUIDv7, client-generated, used for de-duplication
      "event": "page_view",           // required, snake_case, <= 64 chars
      "user_id": "u_123",             // anonymous id when the user is not logged in
      "session_id": "s_456",          // derived server-side when omitted
      "timestamp": "2026-08-11T14:20:00.123Z",  // ISO 8601 UTC
      "page": "/products/123",
      "referrer": "https://google.com/",
      "utm": { "source": "google", "medium": "cpc", "campaign": "summer" },
      "device": "desktop",            // desktop | mobile | tablet | bot | unknown
      "os": "macOS",
      "browser": "Chrome",
      "screen": "1920x1080",
      "country": "VN",                // server-enriched from IP when omitted
      "city": "Ho Chi Minh City",
      "revenue": 199000,              // purchase events only
      "currency": "VND",
      "properties": { "product_id": "123", "category": "shoes" }
    }
  ]
}
```

Only `site_id` and `event` are required. Everything else is either optional or filled in by
the server.

## Validation rules

Validation is per event, never per batch.

Two things happen under that one word. Some faults cost the event; others cost only the
value. The split is deliberate: rejecting the second group throws away real traffic over
circumstances the visitor did not choose, and accepting the first group silently corrupts the
numbers the dashboard reports.

### Faults that reject the event

The event comes back in `rejected: [{index, reason}]` and the rest of the batch is stored.

| Field | Rule | `reason` |
|---|---|---|
| the element itself | Must decode as a JSON object | `malformed_event` |
| `event` | Required | `missing_event_name` |
| `event` | `^[a-z0-9_]{1,64}$` | `invalid_event_name` |
| `event_id` | A UUID when present | `invalid_event_id` |
| `timestamp` | Parses as ISO 8601 when present | `invalid_timestamp` |
| `properties` | A JSON **object**, not an array or a scalar | `invalid_properties` |
| `properties` | ≤ 8 KB serialised | `properties_too_large` |
| `revenue` | Fits `Decimal(18, 4)`: ≤ 14 integer digits, ≤ 4 decimal places, no exponent | `invalid_revenue` |

### Faults that repair the value

The event is stored. Each repair increments `pulse_events_field_repaired_total{field,repair}`,
so nothing here is silent.

| Field | Rule | Repair |
|---|---|---|
| `event_id` | Missing | A server-side UUIDv7 is generated |
| `timestamp` | Missing | Set to `now()` |
| `timestamp` | More than 24h ahead or 30 days behind | Replaced with `now()`, `pulse_events_clock_skew_total{direction}` incremented |
| `user_id`, `session_id` | ≤ 128 characters | Truncated, counted in characters rather than bytes |
| `page`, `referrer` | ≤ 2048 characters; fragment and sensitive query parameters removed | Sanitised |
| `country` | Not an ISO 3166-1 alpha-2 code | Cleared, so GeoIP enrichment can fill it |
| `city` | ≤ 128 characters | Truncated |
| `device` | Not one of `desktop`, `mobile`, `tablet`, `bot`, `unknown` | Normalised to `unknown` |
| `os`, `browser`, `utm_*` | ≤ 64 characters | Truncated — these are `LowCardinality` columns and each one keeps a dictionary per part |
| `currency` | Not an ISO 4217 code | Falls back to `VND`, the column default |

### Faults that reject the whole request

| Rule | Status |
|---|---|
| `site_id` present in the body and different from the one the API key authorises | `401` |
| No events | `400` |
| More than 500 events | `413` |
| Body over 1 MiB | `413` |

`site_id` is taken from the API key, never from the body. Every analytics query is filtered by
it, so a body that could set it would be a cross-tenant write.

**Clock skew is corrected, not rejected.** A device with a wrong clock is common, and dropping
its events would silently lose real traffic. The counter makes the correction visible. An
unparseable timestamp is a different thing — a bug in the client rather than a wrong clock —
and is reported back so it can be fixed.

**Unknown fields are accepted.** A client running an SDK newer than the server is a normal
state during a rollout, and refusing its events would turn a forward-compatible payload into
an outage.

**`screen` is accepted but not stored.** The payload documents it and `analytics.events` has
no column for it. Send it inside `properties` until one exists.

## Server-side enrichment

1. **IP → country and city** via MaxMind GeoLite2. The raw IP is then **discarded** — it is
   never written to storage. A hash is stored only if a specific requirement demands one.
2. **User-Agent → device, OS, browser, bot flag.** Skipped when the client already sent them.
3. **Session stitching.** An empty `session_id` becomes `hash(user_id + date + 30-minute
   window)`.
4. **`ingested_at = now()`** so end-to-end lag can be measured and a backlog detected.

## Privacy

Two rules are structural, not configurable:

- **Raw IP addresses are never stored.** They exist in memory for the duration of the GeoIP
  lookup and are then dropped.
- **Sensitive query parameters are stripped from `page` and `referrer` before storage.** The
  built-in denylist covers `token`, `email`, `password`, `secret`, `otp`, `api_key`,
  `apikey`, `access_token`, `refresh_token`, `id_token`, `authorization`, `passwd` and `pwd`.
  An analytics URL is a common accidental leak of a password-reset token, and a visitor who
  clicks a link *out of* that page sends the whole thing as the next event's referrer — so
  both fields get the same treatment.

  The list is additive through `ingest.sensitive_query_params`
  ([configuration](/guide/configuration)) for names that are a credential in one application
  and ordinary analytics data in another, such as `code` or `ref`. Nothing can remove an entry
  from the built-in set: turning off password stripping must not be one typo away.

- **The URL fragment is always dropped.** It never reaches a server in a real page load
  anyway, so storing whatever a client put there buys nothing.

Bot traffic is classified during enrichment and filtered from user counts by default,
otherwise daily active users inflate quietly.

## De-duplication

Every event carries a client-generated `event_id` (UUIDv7). Because the Kafka pipeline is
at-least-once, a retry can deliver the same event twice; de-duplication happens at query time
on `event_id` rather than by trying to make the write path exactly-once. See ADR-0005 on the
[decisions page](/adr/).

UUIDv7 is chosen over v4 because it is time-ordered, which keeps inserts into a sorted column
store cheap and makes the id useful as a tiebreaker in cursor pagination.

## Tracking snippet

<Badge type="warning" text="Level 5" />

```html
<script defer src="https://cdn.example.com/pulse.js" data-site="site_abc"></script>
```

Under 2 KB gzipped. Automatic page views including SPA route changes via a `history.pushState`
patch, `pulse('event_name', {props})` for custom events, batching of 10 events or 3 seconds,
`navigator.sendBeacon` on `visibilitychange`, respect for `navigator.doNotTrack`, and a
localStorage queue for retrying while offline.

## Full specification

The complete rules, including the exact denylist and the DDL the payload maps onto, are in
[`PLAN.md` §5](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/PLAN.md).

How the pipeline is put together, and the checklist for adding a rule to it, is the
[validation pipeline guide](/guide/validation). Why the rules are hand-written rather than
struct tags is [ADR-0011](/adr/0011-hand-written-event-validation).
