---
layout: home

hero:
  name: Pulse Analytics
  text: Real-time web analytics you host yourself
  tagline: >-
    Go, ClickHouse, Kafka and Next.js. Dashboard queries under 300 ms at 100M events, and an
    ingest path that keeps returning 202 even when storage is down.
  image:
    src: /logo.svg
    alt: Pulse Analytics
  actions:
    - theme: brand
      text: Get started
      link: /guide/quick-start
    - theme: alt
      text: Why this project
      link: /guide/introduction
    - theme: alt
      text: View on GitHub
      link: https://github.com/nxhawk/Real-time-Web-Analytics-Platform

features:
  - icon: 🚀
    title: Built for the write path
    details: >-
      Batched inserts over the native protocol, backpressure with priority dropping, and a
      disk-backed write-ahead log. Kill ClickHouse mid-flight and no event is lost.
    link: /reference/clickhouse
    linkText: How storage is designed
  - icon: ⚡
    title: Fast reads at 100M events
    details: >-
      AggregatingMergeTree materialized views, skip indexes and projections — chosen from
      measurements, not from blog posts. Every threshold is a hard requirement.
    link: /notes/benchmark-results
    linkText: Benchmark results
  - icon: 🔍
    title: Learning in the open
    details: >-
      Every design decision is an ADR, every ClickHouse experiment is written down with real
      numbers, and every level ends with a demo you can reproduce.
    link: /adr/
    linkText: Architecture decisions
  - icon: 🔒
    title: Privacy by construction
    details: >-
      Raw IP addresses are never stored — they are used for GeoIP enrichment and discarded.
      Sensitive query parameters are stripped before the page URL is written.
    link: /reference/event-schema
    linkText: Event schema
---

## Project status

**Level 0 is complete.** The backend skeleton runs: two binaries, configuration, structured
logging, middleware, operational endpoints, Docker Compose, and CI that lints, tests and
builds images. Event ingestion, the ClickHouse schema and the dashboard arrive in Level 1.

| Level | Scope | Status |
|---|---|---|
| **L0** | Bootstrap: conventions, skeleton, Docker, Makefile, CI | ✅ 22/25 |
| **L1** | MVP: ingest → query → dashboard | ⬜ |
| **L2** | ClickHouse deep dive: codecs, skip indexes, TTL, projections | ⬜ |
| **L3** | Batch insert, seeder, ClickHouse vs PostgreSQL benchmark | ⬜ |
| **L4** | Kafka pipeline: producer, consumer group, DLQ, split binaries | ⬜ |
| **L5** | Materialized views, funnel, retention, full dashboard | ⬜ |
| **L6** | Observability, security, CD, documentation | ⬜ |

The full roadmap with estimates is on the [roadmap page](/roadmap).

## What you can run today

```bash
git clone https://github.com/nxhawk/Real-time-Web-Analytics-Platform.git
cd Real-time-Web-Analytics-Platform
cp .env.example .env
make deps && make up

curl localhost:8080/healthz   # {"status":"ok"}
```

Full instructions on the [quick start](/guide/quick-start) page.
