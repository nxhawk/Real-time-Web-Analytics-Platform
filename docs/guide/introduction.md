# Introduction

Pulse Analytics is a self-hosted web analytics platform: a scaled-down Google Analytics.
It accepts events from websites and apps over HTTP, stores them in ClickHouse, and serves a
real-time dashboard — overview metrics, time series, top pages, device and country
breakdowns, funnels, and retention cohorts.

## Two goals, in this order

**1. Learning.** The point of the project is to go deep on things that are hard to learn from
a tutorial: ClickHouse storage internals, high-throughput write paths in Go, Kafka consumer
semantics, and the operational side — metrics, alerts, deployment, backups that were actually
restored once.

**2. The product.** A working analytics platform that answers every dashboard query in under
300 ms at 100 million events, and keeps accepting traffic when ClickHouse is unavailable.

The order matters. When a shortcut would make the product better but teach nothing, the
project takes the long way — Level 1 deliberately inserts one row per request so that Level 3
has a baseline to beat.

## Architectural principles

These five are decided, not up for renegotiation per feature:

1. **The ingest path must not depend on ClickHouse availability.** Storage down still means
   `202 Accepted` — via Kafka from Level 4, via a disk-backed buffer before that.
2. **Write-heavy, read-light, but reads must be fast.** Every dashboard query stays under
   300 ms at 100M events. This is a hard requirement, not an aspiration.
3. **No ORM.** SQL is written by hand so that `EXPLAIN` and `EXPLAIN PIPELINE` stay
   meaningful. See [ADR-0001](/adr/0001-no-orm).
4. **Split binaries from Level 4.** `ingest-api` is I/O-heavy, `analytics-api` is CPU-heavy;
   they scale independently.
5. **Idempotency.** Every event carries a client-generated `event_id` (UUIDv7), so a retry
   can be de-duplicated at query time.

## What is in scope

- Event ingestion over HTTP, single and batched
- ClickHouse schema, migrations and materialized views
- Analytics API: overview, time series, pages, devices, countries, funnel, retention, realtime
- A Next.js dashboard
- A Kafka pipeline
- A data generator and a ClickHouse vs PostgreSQL benchmark suite
- CI/CD to a single production host

## What is not

- Multi-region and ClickHouse replication — documented as a future path, not built
- Complex user management or billing; authentication is an API key per site
- Session replay and heatmaps
- Native mobile SDKs

## How the documentation is organised

| Where | What it holds |
|---|---|
| This site | Narrative documentation: guides, reference, engineering notes, decisions |
| [`PLAN.md`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/PLAN.md) | The technical specification — architecture, DDL, query cookbook, contract. In Vietnamese |
| [`PHASES.md`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/PHASES.md) | Delivery phases: entry and exit criteria, deliverables, risks, canonical numbers |
| [`TODO.md`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/TODO.md) | The task checklist, 232 tasks across seven levels |
| [`DEPLOY-AWS.md`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/DEPLOY-AWS.md) | Production infrastructure: Vercel plus a single EC2 host, with Terraform |

The planning documents are written in Vietnamese by design; the code, this site's English
pages, and everything in the repository outside those four files are in English.

When two documents disagree, the order of precedence is `DEPLOY-AWS.md` for deployment, then
`PLAN.md`, then `PHASES.md`, then `TODO.md`, then the code.

## Next

- [Quick start](/guide/quick-start) — get it running in about five minutes
- [Architecture](/guide/architecture) — how the pieces fit together, in both phases
- [Project structure](/guide/project-structure) — where code goes and why
