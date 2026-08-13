# Architecture

The system is built in two phases. Phase 1 is a monolith that writes straight to ClickHouse
and covers Levels 1 to 3. Phase 2 puts Kafka between ingest and storage and splits the
binaries; it starts at Level 4.

Both phases obey the same rule: **the ingest path never depends on ClickHouse being up.**

::: tip Reading the diagrams
Every diagram on this page is Mermaid source, rendered in the browser. Colour carries meaning:
violet is the client, blue is a Go HTTP service, teal is asynchronous processing, orange is
the message bus, amber is storage, and red is a failure path.
:::

## Phase 1 — monolith

```mermaid
flowchart TB
    SDK(["Web / SDK<br/>pulse.js · sendBeacon"])
    IH["Ingest HTTP handler<br/>validate · enrich"]

    %% The reply is declared before the request so the client keeps the top rank.
    IH -.->|"202 Accepted"| SDK
    SDK -->|"POST /api/v1/events"| IH

    subgraph API["Go / Gin — write path"]
        direction TB
        IH
        BW["Batch Writer<br/>worker pool"]
        IH -->|"buffered chan"| BW
    end

    WAL[["WAL fallback<br/>NDJSON segments on disk"]]
    CH[("ClickHouse<br/>events · events_hourly MV<br/>daily_users MV")]
    AA["Analytics API<br/>/api/v1/analytics/*"]
    DASH["Next.js Dashboard"]

    BW -->|"INSERT — BATCH_SIZE rows or FLUSH_INTERVAL_MS"| CH
    BW -.->|"3 retries exhausted"| WAL
    WAL -.->|"replay"| CH
    CH -->|"SELECT ... GROUP BY"| AA
    AA -->|"JSON envelope"| DASH

    classDef client fill:#7c3aed,stroke:#5b21b6,color:#ffffff
    classDef api fill:#2563eb,stroke:#1d4ed8,color:#ffffff
    classDef proc fill:#0d9488,stroke:#0f766e,color:#ffffff
    classDef store fill:#a16207,stroke:#854d0e,color:#ffffff
    classDef ui fill:#334155,stroke:#1e293b,color:#ffffff
    classDef fallback fill:#b91c1c,stroke:#991b1b,color:#ffffff

    class SDK client
    class IH,AA api
    class BW proc
    class CH store
    class WAL fallback
    class DASH ui
    style API fill:none,stroke:#94a3b8,stroke-width:1px,stroke-dasharray:5 4
```

The HTTP handler does not write to ClickHouse. It validates, enriches and pushes onto a
buffered channel, then returns `202`. A worker pool drains that channel and inserts in
batches of `BATCH_SIZE` rows or every `FLUSH_INTERVAL_MS`, whichever comes first.

When ClickHouse refuses the insert, the writer retries three times with exponential backoff
and jitter, and then writes the batch to a newline-delimited JSON write-ahead log on disk. A
replay process picks those files up later. That is what makes "kill ClickHouse for five
minutes and lose nothing" a test rather than a hope.

### One event, end to end

The dotted `202` in the diagram above is the whole point: the response leaves before the row
is stored.

```mermaid
sequenceDiagram
    autonumber
    actor U as Browser (pulse.js)
    participant M as Middleware
    participant H as Ingest handler
    participant S as Event service
    participant B as Batch writer
    participant CH as ClickHouse

    U->>M: POST /api/v1/events (batch of up to 500)

    rect rgba(37, 99, 235, 0.10)
        Note over M,H: request id · CORS · panic recovery · 1 MiB body cap
        M->>M: X-API-Key resolves to site_id
    end

    M->>H: decoded request, site_id in context
    H->>S: validate and enrich

    rect rgba(13, 148, 136, 0.10)
        S->>S: per-event validation, never per-batch
        S->>S: GeoIP lookup, then the IP is discarded
        S->>S: UA parse · session_id · ingested_at
    end

    S--)B: push accepted events onto the buffered chan
    H-->>U: 202 Accepted with accepted and rejected counts

    Note over S,CH: flush at BATCH_SIZE rows, or every FLUSH_INTERVAL_MS

    B->>CH: INSERT INTO events (batch)
    alt insert ok
        CH-->>B: ok
    else 3 retries exhausted
        rect rgba(185, 28, 28, 0.10)
            B->>B: append batch to the NDJSON WAL, replay later
        end
    end
```

Everything up to the `202` happens inside the request. Everything after it is the writer's
problem, and a slow or dead ClickHouse changes the response to the browser not at all.

## Phase 2 — event pipeline

```mermaid
flowchart TB
    SDK(["Web / SDK"])
    ING["Go Ingest API<br/>validate · enrich"]
    K{{"Kafka — events.raw<br/>6 partitions · retention 7d"}}
    C1["Consumer<br/>group ch-sink<br/>batch 10k / 500 ms"]
    C2["Consumer<br/>group alerting"]
    ML["Future<br/>ML / ETL"]
    DLQ{{"events.dlq"}}
    CH[("ClickHouse")]
    AN["Go Analytics API"]
    RD[("Redis cache<br/>optional")]
    DASH["Next.js Dashboard"]

    SDK -->|"POST /api/v1/events"| ING
    ING -.->|"202 Accepted immediately"| SDK
    ING -->|"produce — async, acks=1"| K
    K --> C1
    K --> C2
    K -.-> ML
    C1 -->|"INSERT batch, then commit offset"| CH
    C2 -->|"unprocessable"| DLQ
    CH --> AN
    AN -.->|"read-through cache"| RD
    AN -->|"JSON envelope"| DASH

    classDef client fill:#7c3aed,stroke:#5b21b6,color:#ffffff
    classDef api fill:#2563eb,stroke:#1d4ed8,color:#ffffff
    classDef proc fill:#0d9488,stroke:#0f766e,color:#ffffff
    classDef queue fill:#c2410c,stroke:#9a3412,color:#ffffff
    classDef store fill:#a16207,stroke:#854d0e,color:#ffffff
    classDef ui fill:#334155,stroke:#1e293b,color:#ffffff
    classDef fallback fill:#b91c1c,stroke:#991b1b,color:#ffffff
    classDef future fill:#64748b,stroke:#475569,color:#ffffff,stroke-dasharray:4 3

    class SDK client
    class ING,AN api
    class C1,C2 proc
    class K queue
    class DLQ fallback
    class CH store
    class DASH ui
    class ML,RD future
```

Kafka buys three things a direct write cannot: the ingest API stops caring whether ClickHouse
is reachable, events can be replayed after a schema fix, and a second consumer group can read
the same stream without touching the write path.

The cost is at-least-once delivery. The consumer commits its offset **after** ClickHouse
acknowledges the insert, never before, so a crash re-delivers rather than loses. Duplicates
are removed at query time using `event_id`. See [ADR-0005](/adr/).

### Where the offset is committed

```mermaid
sequenceDiagram
    autonumber
    actor U as Browser (pulse.js)
    participant I as Ingest API
    participant K as Kafka events.raw
    participant C as Consumer ch-sink
    participant CH as ClickHouse
    participant D as events.dlq

    U->>I: POST /api/v1/events
    I->>I: validate and enrich
    I--)K: produce (async, acks=1, key is site_id)
    I-->>U: 202 Accepted

    loop every 10k messages or 500 ms
        K->>C: fetch records
        C->>CH: INSERT batch
        alt insert ok
            rect rgba(13, 148, 136, 0.10)
                CH-->>C: ok
                C->>K: commit offset — after the ack, never before
            end
        else retryable failure
            C->>C: backoff and retry the same batch
            Note over C,CH: offset stays uncommitted, so a crash re-delivers
        else unprocessable
            rect rgba(185, 28, 28, 0.10)
                C->>D: publish to events.dlq with the reason
                C->>K: commit offset
            end
        end
    end
```

At-least-once is a deliberate trade: a duplicate row is cheap to remove at query time, a lost
event is not recoverable at all.

## Request lifecycle

What happens to one event, end to end:

1. **Middleware.** A UUIDv7 request id is attached (or an inbound `X-Request-ID` is reused),
   panics are converted into a `500` with the standard error envelope, CORS is applied, and
   the body size is capped at 1 MiB.
2. **Authentication.** The `X-API-Key` header resolves to a `site_id`, which is put in the
   request context. Every subsequent query is scoped by it — there are no cross-tenant reads.
3. **Validation.** Per-event, not per-batch. A batch of 100 events with 3 bad ones accepts 97
   and returns them in a `rejected` array. One malformed event never discards a batch.
4. **Enrichment.** IP address to country and city via MaxMind GeoLite2 — then the IP is
   **discarded**. User-Agent to device, OS and browser. A missing `session_id` is derived from
   a 30-minute window hash. An `ingested_at` timestamp is stamped for lag measurement.
5. **Sink.** Buffer, or Kafka producer, depending on `SINK`.
6. **Response.** `202 Accepted`, with the count of accepted events and the rejected ones.

## The read path

The write path is tuned for never losing an event. The read path is tuned for one number: a
dashboard panel that answers in under 300 ms at 100M rows.

```mermaid
sequenceDiagram
    autonumber
    actor A as Analyst
    participant N as Next.js dashboard
    participant Q as Analytics API
    participant R as Redis cache (optional)
    participant CH as ClickHouse

    A->>N: open the overview panel for the last 7 days
    N->>Q: GET /api/v1/analytics/overview with X-API-Key
    Q->>Q: rate limit 120 req/min per IP · site_id comes from the key

    opt cache enabled
        Q->>R: read the cached window
        R-->>Q: on a hit, answer without touching ClickHouse
    end

    rect rgba(161, 98, 7, 0.12)
        Q->>CH: SELECT from events_hourly filtered by site_id
        Note over Q,CH: max_execution_time and max_memory_usage guard every query
        CH-->>Q: pre-aggregated rows
    end

    Q-->>N: JSON envelope, p95 under 300 ms
    N-->>A: rendered charts
```

The materialized view is what makes that number reachable: `/analytics/overview` reads
`events_hourly`, which is orders of magnitude smaller than `events`, so the query touches
hours rather than raw rows. See the [ClickHouse schema](/reference/clickhouse).

## Why ClickHouse

The workload is append-only, read-mostly-by-aggregate, and the queries all look like "group
this time range by one low-cardinality dimension". That is the shape a column store is built
for: only the columns in the query are read from disk, the compression ratio on sorted
columnar data is an order of magnitude better than row storage, and execution is vectorised.

ClickHouse is also bad at things this project does not need: point lookups by primary key,
updates and deletes, large joins, and transactions. Level 3 measures both sides of that trade
against PostgreSQL and writes the numbers down — see
[benchmark results](/notes/benchmark-results).

## Performance targets

Every one of these is verified by a test or a measurement, not asserted:

| Target | Value | Verified in |
|---|---|---|
| Dashboard query p95 at 100M events | < 300 ms | Level 5 |
| `/analytics/overview` after materialized views | < 100 ms | Level 5 |
| `/analytics/realtime` | < 200 ms | Level 5 |
| Ingest throughput | 10,000 events/s for 10 minutes, zero drops | Level 3 |
| Ingest API p99 latency | < 50 ms | Level 3 |
| End-to-end lag p99 at 5k events/s | < 5 s | Level 4 |

## Editing these diagrams

Diagrams are Mermaid fenced blocks in this file, rendered by `vitepress-plugin-mermaid`. Two
rules keep them consistent:

- **Never pin a Mermaid `theme`.** The plugin switches to the dark theme together with the
  site. Node colour comes from the `classDef` palette declared at the bottom of each
  flowchart, and those fills carry white text, so they read on either background.
- **Reuse the palette.** `client`, `api`, `proc`, `queue`, `store`, `ui`, `fallback` and
  `future` mean the same thing on every diagram in these docs. A new box picks an existing
  class instead of a new colour.

## Read more

- [ClickHouse schema](/reference/clickhouse) — tables, materialized views, why each exists
- [Event schema](/reference/event-schema) — the payload contract and validation rules
- [Architecture decisions](/adr/) — the ten decisions and their trade-offs
