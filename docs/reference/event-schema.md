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

| Field | Rule | Behaviour when violated |
|---|---|---|
| `site_id` | Required, must match the API key | `401` for the whole request |
| `event` | Required, `^[a-z0-9_]{1,64}$` | Reject that event only |
| `timestamp` | ISO 8601; more than 24h in the future or 30 days in the past | Overridden with `now()`, counter `events_clock_skew_total` incremented |
| `user_id` | ≤ 128 characters | Truncated |
| `page` | ≤ 2048 characters; sensitive query parameters stripped | Sanitised |
| `properties` | JSON object, ≤ 8 KB serialised | Reject that event |
| Batch size | ≤ 500 events, body ≤ 1 MiB | `413` for the whole request |

**Clock skew is corrected, not rejected.** A device with a wrong clock is common, and dropping
its events would silently lose real traffic. The counter makes the correction visible.

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
- **Sensitive query parameters are stripped from `page` before storage** — `token`, `email`,
  `password` and anything else on the denylist. An analytics URL is a common accidental leak
  of a password-reset token.

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
