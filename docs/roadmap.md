# Roadmap

Seven levels, 232 tasks, roughly 207 hours part-time. Each level ends with something
demonstrable — not "the code is written" but "here is the thing working".

| Level | Scope | Tasks | Estimate | Tag | Status |
|---|---|---|---|---|---|
| **L0** | Bootstrap: conventions, skeleton, Docker, Makefile, CI | 25 | 12h | — | ✅ 22/25 |
| **L1** | MVP: ingest → query → dashboard | 40 | 30h | `v0.1.0` | ⬜ |
| **L2** | ClickHouse deep dive: codecs, skip indexes, TTL, projections | 24 | 25h | — | ⬜ |
| **L3** | Batch insert, seeder, ClickHouse vs PostgreSQL benchmark | 32 | 35h | `v0.3.0` | ⬜ |
| **L4** | Kafka pipeline: producer, consumer group, DLQ, split binaries | 30 | 35h | `v0.4.0` | ⬜ |
| **L5** | Materialized views, funnel, retention, realtime, full dashboard | 46 | 45h | — | ⬜ |
| **L6** | Observability, security, CD, documentation | 35 | 25h | `v1.0.0` | ⬜ |
| | **Total** | **232** | **~207h** | | |
| **AWS** | Production infrastructure — replaces L6.4 | 32 | 14h | — | ⬜ |
| | **On the AWS path** | **255** | **~214h** | | |

L0–L3 build the Phase 1 monolith; L4–L6 move to the Phase 2 event pipeline. See
[architecture](/guide/architecture).

## What each level proves

**L0 — foundations.** `make up` works on a clean machine, CI is green, both binaries serve
their operational routes. No product behaviour yet, by design.

**L1 — the first complete path.** `curl` an event, see the number change on a web page. The
insert is deliberately naive, one row per request, so Level 3 has a baseline to beat.

**L2 — understanding, not features.** Almost no code. Three tables with three sort orders,
codec comparisons, skip indexes, projections, TTL. The deliverable is
[twenty measured notes](/notes/clickhouse-notes), and the exit criterion is being able to
answer "why this sort order" with your own numbers.

**L3 — the write path grows up.** Batch writer with backpressure and a write-ahead log, a
seeder that produces realistic data at 100M scale, and the
[benchmark](/notes/benchmark-results) the whole project builds toward. Exit test: kill
ClickHouse mid-load and lose nothing.

**L4 — event-driven.** Kafka between ingest and storage, a consumer that commits offsets only
after ClickHouse acknowledges, a dead-letter queue, and three chaos tests. Demo: kill
ClickHouse while under load, bring it back, watch the counts match exactly.

**L5 — the product.** Five materialized views, funnel, retention, realtime, revenue, the full
dashboard and the tracking SDK. The golden test comparing view output against raw is the most
important test in the project.

**L6 — operable.** Metrics, four dashboards, four alerts, security hardening, automated
deployment with rollback, and a backup that has actually been restored once.

## Order of dependencies

```
L0 ──▶ L1 ──┬──▶ L2 ──┐
            │         ├──▶ L3 ──▶ L4 ──▶ L5 ──▶ L6 ──▶ AWS
            └─────────┘
```

One documented exception: **L2 needs at least 10M events to experiment on.** Building the
seeder (tasks L3-01 to L3-07) before or alongside L2 is the sanctioned way to get them.

## If time runs short

Priority order, most valuable first:

1. **L0 → L1 → L2 → L3.** Storage internals, the write path, and the benchmark. Dropping
   these removes the reason the project exists.
2. **L5.1 and L5.2** — materialized views and the funnel. This is where AggregatingMergeTree
   and analytical SQL are demonstrated.
3. **L4** — Kafka. Deferrable if the goal is not event-driven architecture, but the goal *is*
   event-driven architecture.
4. **L6.1 and L6.5** — metrics and documentation. Cheap, and they raise the project's value
   noticeably.
5. Everything else can wait.

Acceptable reductions: benchmark at 10M instead of 100M if disk is short (note the size next
to every number), Redpanda instead of Kafka in development, skip OpenTelemetry, skip tiered
storage.

**Not reducible:** the golden test comparing materialized views against raw, the "kill
ClickHouse and lose nothing" tests, and the backup restore rehearsal. Those three are where
the difference between a demo and a system lives.

## Definition of done

The project is complete when all thirteen hold:

1. `git clone && make up` works in under five minutes on a clean machine
2. `make seed N=10000000` succeeds in under five minutes
3. The dashboard shows seven widget groups with real data
4. Every analytics endpoint is p95 under 300 ms at 100M events, with `system.query_log` as
   evidence
5. Ingest sustains 10,000 events/s for ten minutes with zero drops and p99 under 50 ms
6. Killing ClickHouse for five minutes loses nothing; the counts match exactly afterwards
7. The golden test matches materialized views against raw exactly
8. CI is green: lint, unit, integration, security scan, image build
9. Deployment happens from one tag, with rollback and a smoke test
10. Grafana has four dashboards and working alerts
11. `benchmark-results.md` is complete with conclusions
12. `clickhouse-notes.md` has at least twenty measured entries
13. The README has architecture, quick start, screenshots, and what was learned

Task-level detail is in
[`TODO.md`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/TODO.md);
entry and exit criteria per level are in
[`PHASES.md`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/PHASES.md).
