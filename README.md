# Pulse Analytics

> A self-hosted, real-time web analytics platform — a scaled-down Google Analytics.
> **Go/Gin + ClickHouse + Kafka + Next.js**

Pulse Analytics ingests events from websites and apps over HTTP, stores them in ClickHouse,
and serves a real-time dashboard: overview metrics, time series, top pages, device/country
breakdowns, funnels, and retention cohorts.

> **Status: Level 0 complete — the skeleton runs, the event pipeline does not exist yet.**
> Both binaries build and serve their operational routes (`/healthz`, `/readyz`, `/version`,
> `/metrics`), `make up` brings ClickHouse and the two services online, and CI lints, tests
> and builds images. Event ingestion, the ClickHouse schema and the dashboard arrive in
> Level 1. Progress is tracked in [`TODO.md`](TODO.md); the plan behind it is in
> [`PLAN.md`](PLAN.md) and [`PHASES.md`](PHASES.md).

---

## Document map

| Document | Role | Read it when |
|---|---|---|
| `README.md` (this file) | Overview, quick start, API summary | You are new to the repository |
| [`PLAN.md`](PLAN.md) | **Technical specification** — architecture, schema, DDL, queries, contract, ADRs | Before implementing anything |
| [`PHASES.md`](PHASES.md) | **Delivery phases** — order, entry/exit criteria, deliverables, risks | At the start of each phase and at review time |
| [`TODO.md`](TODO.md) | **Execution checklist** — task IDs, estimates, acceptance criteria | Day to day |
| [`DEPLOY-AWS.md`](DEPLOY-AWS.md) | Production infrastructure: Vercel + EC2 + Terraform | During the AWS phase (replaces L6.4) |
| [`CLAUDE.md`](CLAUDE.md) | Conventions for AI coding agents | When using an agent to generate code |

Precedence when documents disagree: **`DEPLOY-AWS.md` (deployment only)** → **`PLAN.md`** →
**`PHASES.md`** → **`TODO.md`** → code. Canonical values for anything that appears in more than
one document — versions, performance thresholds, API limits, seeder distributions — live in
[`PHASES.md` §2](PHASES.md#2-bảng-số-liệu-chuẩn); change them there first.

The planning documents (`PLAN.md`, `PHASES.md`, `TODO.md`, `DEPLOY-AWS.md`) are written in
Vietnamese by design; `README.md`, `CLAUDE.md`, and everything in the codebase are in English.
See [`CLAUDE.md`](CLAUDE.md).

---

## Table of contents

- [Document map](#document-map)
- [Why this project](#why-this-project)
- [Architecture](#architecture)
- [Tech stack](#tech-stack)
- [Repository layout](#repository-layout)
- [Quick start](#quick-start)
- [Configuration](#configuration)
- [API overview](#api-overview)
- [Event schema](#event-schema)
- [Roadmap](#roadmap)
- [Deployment](#deployment)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

---

## Why this project

Two goals, in order of importance:

1. **Learning.** Go deep on ClickHouse internals (MergeTree, materialized views, aggregate
   combinators, skip indexes, projections), high-throughput write paths in Go, Kafka
   consumer semantics, and production-grade observability and CI/CD.
2. **Product.** A working analytics platform that answers dashboard queries in
   **under 300 ms at 100M events**, and keeps accepting traffic even when ClickHouse is down.

### Architectural principles

1. **The ingest path must not depend on ClickHouse availability.** ClickHouse down → the API
   still returns `202` (via Kafka in Phase 2, via a disk-backed buffer in Phase 1).
2. **Write-heavy, read-light — but reads must be fast.** Every dashboard query must stay
   under 300 ms at 100M events.
3. **No ORM.** SQL is written by hand so that `EXPLAIN` / `EXPLAIN PIPELINE` stay meaningful.
4. **Split binaries from Level 4 on.** `ingest-api` (I/O-heavy) and `analytics-api`
   (CPU-heavy) scale independently.
5. **Idempotency.** Every event carries a client-generated `event_id` (UUIDv7), so retries
   can be de-duplicated.

---

## Architecture

### Phase 1 — Simple monolith (Levels 1–3)

```
┌──────────────┐
│  Web / SDK   │
└──────┬───────┘
       │ POST /api/v1/events   (batch, keepalive, sendBeacon)
       ▼
┌───────────────────────────────────────────┐
│              Go / Gin  (api)              │
│  ┌─────────────┐        ┌──────────────┐  │
│  │ Ingest HTTP │──chan─▶│ Batch Writer │  │
│  │  handler    │        │ (worker pool)│  │
│  └─────────────┘        └──────┬───────┘  │
│  ┌─────────────┐               │          │
│  │Analytics API│───────────────┼────────┐ │
│  └─────────────┘               │        │ │
└────────────────────────────────┼────────┼─┘
                                 ▼        ▼
                        ┌────────────────────┐
                        │     ClickHouse     │
                        │  events (raw)      │
                        │  events_hourly MV  │
                        │  daily_users MV    │
                        └────────────────────┘
                                 ▲
                        ┌────────┴─────────┐
                        │ Next.js Dashboard│
                        └──────────────────┘
```

### Phase 2 — Event pipeline with Kafka (Level 4+)

```
┌──────────────┐
│  Web / SDK   │
└──────┬───────┘
       ▼
┌───────────────┐   produce (async, acks=1)   ┌───────────────┐
│  Go Ingest    │────────────────────────────▶│     Kafka     │
│  API (gin)    │                             │ events.raw    │
│  - validate   │◀── 202 Accepted immediately │ 6 partitions  │
│  - enrich     │                             │ retention 7d  │
└───────────────┘                             └───────┬───────┘
                              ┌───────────────────────┼──────────────┐
                              ▼                       ▼              ▼
                    ┌──────────────────┐    ┌──────────────────┐  ┌──────────┐
                    │  Go Consumer     │    │  Go Consumer     │  │ (future) │
                    │  group: ch-sink  │    │  group: alerting │  │  ML/ETL  │
                    │  batch 5k / 500ms│    └────────┬─────────┘  └──────────┘
                    └────────┬─────────┘             ▼
                             │              ┌──────────────────┐
                             │              │   events.dlq     │
                             ▼              └──────────────────┘
                    ┌────────────────────┐
                    │     ClickHouse     │
                    └─────────┬──────────┘
                              ▼
                    ┌────────────────────┐
                    │  Go Analytics API  │──▶ Redis cache (optional)
                    └─────────┬──────────┘
                              ▼
                    ┌────────────────────┐
                    │ Next.js Dashboard  │
                    └────────────────────┘
```

---

## Tech stack

| Layer | Technology | Version | Notes |
|---|---|---|---|
| Backend language | Go | 1.26 | `log/slog`, generics, graceful shutdown |
| HTTP framework | Gin | v1.11+ | zap/cors/gzip contribs |
| Database | ClickHouse | 26.3 LTS | Single node; replication is a documented future path |
| ClickHouse driver | `ClickHouse/clickhouse-go/v2` | v2.4x | Native protocol (port 9000) — never HTTP for inserts |
| Migrations | goose | latest | Chosen over golang-migrate for multi-statement ClickHouse support |
| Streaming | Apache Kafka (KRaft) | 4.x | Redpanda is an acceptable lighter dev substitute |
| Kafka client | `twmb/franz-go` | latest | Pure Go, no cgo |
| Config | `caarlos0/env` + `.env` | | |
| Logging | `log/slog` + JSON handler | | Structured logs only |
| Metrics | `prometheus/client_golang` | | |
| Tracing | OpenTelemetry Go SDK | | Optional, Level 5 |
| Validation | `go-playground/validator/v10` | | |
| Testing | stdlib + `testify` + `testcontainers-go` | | Real ClickHouse container for integration tests |
| Linting | `golangci-lint` v2 | | |
| Frontend | Next.js 16.3 (App Router) + React 19 | | |
| Frontend language | TypeScript 5.x (strict) | | |
| UI | TailwindCSS 4 + shadcn/ui | | |
| Charts | Recharts / Apache ECharts | | ECharts for series above 5k points |
| Data fetching | TanStack Query v5 | | 10s polling on report pages, 5s on `/realtime` |
| Frontend tests | Vitest + Testing Library + Playwright | | |
| Containers | Docker + Docker Compose v2 | | |
| CI/CD | GitHub Actions | | GHCR for dev/CI images, ECR for the AWS production path |
| Reverse proxy | Caddy | | Automatic TLS |
| Load testing | k6 + custom Go seeder | | |
| Monitoring | Prometheus + Grafana | | |

---

## Repository layout

A monorepo, so the API contract and the client that consumes it move together and one CI run
covers both. The Go side follows the conventional `cmd/` + `internal/` split: `cmd/` holds one
tiny `main` per binary and nothing else, `internal/` holds everything the compiler must keep
private to this module, and `pkg/` holds the few pieces that could reasonably be imported by
someone else.

```
pulse-analytics/
├── backend/                        # Go services — one module
│   ├── cmd/                        # one directory per binary; main() only wires dependencies
│   │   ├── ingest-api/             #   write path: accepts events over HTTP
│   │   ├── analytics-api/          #   read path: answers dashboard queries
│   │   ├── consumer/               #   Kafka → ClickHouse sink              (Level 4)
│   │   ├── migrate/                #   migration runner                     (Level 1)
│   │   └── seeder/                 #   synthetic event generator            (Level 3)
│   ├── internal/                   # private to this module — the compiler enforces it
│   │   ├── config/                 #   the ONLY package that reads the environment
│   │   ├── logging/                #   slog setup (JSON in production, text locally)
│   │   ├── version/                #   build metadata injected via -ldflags
│   │   ├── metrics/                #   the single Prometheus registry
│   │   ├── httpx/                  #   transport plumbing: middleware, error envelope,
│   │   │                           #   graceful-shutdown server, base gin engine
│   │   ├── handler/                #   HTTP layer: decode → call a service → encode
│   │   ├── service/                #   business rules: validation, enrichment, orchestration
│   │   ├── repository/             #   storage access, no business logic
│   │   │   ├── clickhouse/         #     connection, repos, queries/*.sql via go:embed
│   │   │   └── postgres/           #     benchmark comparison only            (Level 3)
│   │   ├── model/                  #   domain types shared across layers
│   │   ├── validate/               #   event validation rules                (Level 1)
│   │   ├── buffer/                 #   batch writer, backpressure, WAL fallback (Level 3)
│   │   └── kafka/                  #   producer, consumer, DLQ               (Level 4)
│   ├── pkg/                        # importable from outside: geoip, uaparser wrappers
│   ├── migrations/                 # numbered goose migrations, .up.sql / .down.sql
│   ├── test/                       # integration tests (testcontainers) + fixtures
│   ├── Dockerfile                  # multi-stage → distroless, one image per SERVICE arg
│   └── .golangci.yml               # lint configuration
│
├── frontend/                       # Next.js dashboard                      (Level 1)
├── sdk/js/                         # pulse.js tracking snippet, < 2 KB gzip (Level 5)
│
├── deploy/                         # runtime configuration, not application code
│   ├── caddy/                      #   reverse proxy + automatic TLS
│   ├── clickhouse/config.d/        #   server settings: memory, logging, Prometheus
│   ├── clickhouse/users.d/         #   profiles and quotas: query guards, readonly user
│   ├── kafka/ prometheus/ grafana/ #   the rest of the stack                (Levels 4, 6)
│   └── scripts/                    #   deployment helpers
│
├── infra/                          # Terraform for the AWS production path  (AWS phase)
├── loadtest/                       # k6 scripts + ClickHouse vs PostgreSQL benchmark
│
├── docs/                           # documentation site (VitePress, EN + VI) + API contract
│   ├── .vitepress/config/          #   shared.mts (base, search), en.mts, vi.mts
│   ├── guide/ reference/           #   English pages — the root locale, no URL prefix
│   ├── notes/ adr/ roadmap.md      #   engineering notes and decision records
│   ├── vi/                         #   Vietnamese mirror — same tree, same filenames
│   ├── public/                     #   static assets served at the site root
│   └── api/openapi.yaml            #   API CONTRACT — the single source of truth
│
├── docker-compose.yml              # dev stack: ClickHouse + both APIs
├── docker-compose.prod.yml         # production stack                       (Level 6)
├── docker-compose.bench.yml        # adds PostgreSQL for the benchmark      (Level 3)
├── Makefile                        # every development command — run `make help`
├── .env.example                    # every configuration variable, documented
│
├── README.md                       # this file
├── PLAN.md                         # full technical specification
├── PHASES.md                       # phase-by-phase delivery plan and canonical numbers
├── TODO.md                         # execution checklist by level
├── DEPLOY-AWS.md                   # Vercel + EC2 deployment guide
├── CONTRIBUTING.md                 # setup, branching, commit and code conventions
└── CLAUDE.md                       # conventions for AI coding agents
```

### Why the layers are split this way

Dependencies point in one direction only — `cmd` → `handler` → `service` → `repository` — and
never back. A repository does not import a handler, and nothing outside `config` reads an
environment variable.

| Layer | Responsibility | Must not |
|---|---|---|
| `cmd/` | Wire dependencies, start the server, handle shutdown | Contain business logic |
| `handler/` | Decode the request, call one service, encode the response | Talk to storage or build SQL |
| `service/` | Business rules: validation, enrichment, orchestration | Know it is being called over HTTP |
| `repository/` | Storage access, hand-written SQL | Contain business rules |
| `httpx/` | Transport plumbing reusable by any service | Know anything about analytics |
| `config/` | Read and validate the environment | Be bypassed by `os.Getenv` elsewhere |

Two extension points are already in place so later levels are additive rather than invasive:
`handler.Prober` (anything with a `Check(ctx)` can be added to `/readyz` — ClickHouse in Level 1,
Kafka in Level 4), and `httpx.Server.Run(ctx, hooks...)` (a shutdown hook flushes the batch
writer in Level 3 so no accepted event is lost on deploy).

The full tree with every planned file is in [`PLAN.md` §4](PLAN.md#4-cấu-trúc-repository).

---

## Quick start

**Prerequisites:** Docker + Docker Compose v2, Go 1.26, Node.js 22+, `make`.

```bash
# 1. Configure
cp .env.example .env

# 2. Resolve Go dependencies (first time only — this writes backend/go.sum)
make deps

# 3. Start ClickHouse and both API services, then wait until they are healthy
make up

# 4. Apply database migrations                       (available from Level 1)
make migrate-up

# 5. Seed sample data, optional                      (available from Level 3)
make seed

# 6. Open the dashboard                              (available from Level 1)
open http://localhost:3000
```

Right now — with Level 0 complete — steps 1 to 3 work and give you two running services:

```bash
curl localhost:8080/healthz    # {"status":"ok"}
curl localhost:8080/readyz     # {"status":"ok","checks":{}}
curl localhost:8080/version    # tag, commit, build time, Go version
curl localhost:8080/metrics    # Prometheus exposition
curl localhost:8081/healthz    # the analytics API answers the same operational routes
```

Running the API on the host instead of in a container, against the containerised ClickHouse:

```bash
make down-app   # stop the API containers, keep ClickHouse
make run        # go run ./cmd/ingest-api
```

### Common Make targets

`make help` prints all of them. The ones used daily:

| Target | Description |
|---|---|
| `make deps` | Resolve and download Go dependencies (run once after cloning) |
| `make up` / `make down` | Start / stop the Docker Compose stack |
| `make health` | Poll `/healthz` on both APIs until they answer |
| `make logs` / `make ps` | Tail logs (`make logs S=ingest-api`) / list services |
| `make build` / `make run` | Build binaries into `backend/bin/` / run the ingest API |
| `make test` / `make test-int` | Unit tests with race detector / integration tests |
| `make lint` / `make fmt` / `make vet` | golangci-lint / gofmt + goimports / go vet |
| `make check` | Everything CI runs, in one command |
| `make migrate-up` / `make migrate-down` | Apply / roll back migrations |
| `make seed` | Generate synthetic events (`make seed N=10000000`) |
| `make bench` | Run the ClickHouse vs PostgreSQL benchmark suite |
| `make ch-cli` | Open a `clickhouse-client` shell |
| `make tools` | Install golangci-lint and goimports |
| `make clean` / `make nuke` | Remove build artifacts / also delete the data volumes |

### Sending your first event

> Available from Level 1, once the ingest endpoint and the `events` table exist.

```bash
curl -X POST http://localhost:8080/api/v1/events \
  -H "Content-Type: application/json" \
  -H "X-API-Key: dev_key" \
  -d '{
    "site_id": "site_abc",
    "events": [{
      "event_id": "0192f8a1-0000-7000-8000-000000000001",
      "event": "page_view",
      "user_id": "u_123",
      "session_id": "s_456",
      "page": "/products/123",
      "device": "desktop"
    }]
  }'
```

---

## Configuration

All configuration is read from environment variables (see `.env.example` for the full list).

| Variable | Default | Description |
|---|---|---|
| `APP_ENV` | `development` | `development` \| `staging` \| `production` |
| `HTTP_ADDR` | `:8080` | Listen address for the ingest API |
| `ANALYTICS_ADDR` | `:8081` | Listen address for the analytics API |
| `CLICKHOUSE_DSN` | — | Native-protocol DSN (port 9000) |
| `LOG_LEVEL` | `info` | slog level |
| `BATCH_SIZE` | `5000` | Rows per ClickHouse insert batch |
| `FLUSH_INTERVAL_MS` | `500` | Max time a batch waits before being flushed |
| `KAFKA_BROKERS` | — | Comma-separated broker list (Level 4+) |
| `INSERT_MODE` | `batch` | `batch` \| `single` — `single` exists only for benchmarking |

---

## API overview

Base path: `/api/v1`. All responses are JSON.

### Ingest

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/events` | `X-API-Key` | Accept 1–500 events, partial success |
| `GET` | `/pixel.gif` | `?k=` | No-JavaScript fallback pixel |

### Analytics

Shared query params: `from`, `to`, `tz` (default `Asia/Ho_Chi_Minh`; data is stored in UTC),
`filter[device]`, `filter[country]`, `filter[page]`, `filter[event]`.

| Method | Path | Returns |
|---|---|---|
| `GET` | `/analytics/overview` | users, sessions, events, pageviews, revenue, bounce rate, deltas |
| `GET` | `/analytics/timeseries` | `{series:[{ts, value}], interval}` |
| `GET` | `/analytics/pages` | Top pages with views, users, average time |
| `GET` | `/analytics/devices` \| `/countries` \| `/browsers` \| `/os` \| `/sources` | Breakdown tables |
| `GET` | `/analytics/funnel` | Step conversion via `windowFunnel` |
| `GET` | `/analytics/retention` | Cohort retention matrix |
| `GET` | `/analytics/realtime` | Active users, last-5-minute events, top pages |
| `GET` | `/analytics/events` | Raw event stream, cursor-paginated |
| `GET` | `/analytics/export?format=csv` | Streamed CSV export |

### Ops

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Liveness — 200 while the process is alive |
| `GET` | `/readyz` | Readiness — pings ClickHouse and Kafka, 503 otherwise |
| `GET` | `/metrics` | Prometheus exposition |
| `GET` | `/version` | Commit SHA and build time |

### Error format

```json
{
  "error": { "code": "invalid_range", "message": "from must be before to", "details": {} },
  "request_id": "01J..."
}
```

Error codes: `invalid_json`, `validation_failed`, `unauthorized`, `rate_limited`,
`invalid_range`, `range_too_large`, `upstream_unavailable`, `internal`.

### Limits

- Maximum date range: 400 days; maximum `limit`: 1000.
- Rate limits: ingest 1000 req/min per API key; analytics 120 req/min per IP.
- Server-side query guards: `max_execution_time = 15`, `max_memory_usage = 4GB`.

---

## Event schema

```jsonc
// POST /api/v1/events — accepts a single event or a batch
{
  "site_id": "site_abc",              // required, must match the API key
  "events": [
    {
      "event_id": "0192f8a1-...",     // UUIDv7, client-generated, used for de-duplication
      "event": "page_view",           // required, snake_case, <= 64 chars
      "user_id": "u_123",             // anonymous id when the user is not logged in
      "session_id": "s_456",
      "timestamp": "2026-08-11T14:20:00.123Z",
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

**Partial success:** in a batch of 100 events with 3 invalid ones, 97 are accepted and the
response is `202` with a `rejected: [...]` array. A single malformed event never discards
the whole batch.

**Server-side enrichment:** IP → country/city via MaxMind GeoLite2 (raw IPs are never
stored), User-Agent → device/OS/browser, session stitching via a 30-minute window, and an
`ingested_at` timestamp for end-to-end latency measurement.

See [`PLAN.md` §5.2](PLAN.md#52-quy-tắc-validate) for the complete validation rules.

---

## Roadmap

| Level | Scope | Tasks | Estimate | Tag |
|---|---|---|---|---|
| **L0** | Bootstrap: repo conventions, skeleton, Docker, Makefile, CI | 25 | 12h | — |
| **L1** | MVP: ingest → query → dashboard | 40 | 30h | `v0.1.0` |
| **L2** | ClickHouse deep dive: codecs, skip indexes, TTL, projections | 24 | 25h | — |
| **L3** | Batch insert, seeder, ClickHouse vs PostgreSQL benchmark | 32 | 35h | `v0.3.0` |
| **L4** | Kafka pipeline: producer, consumer group, DLQ, split binaries | 30 | 35h | `v0.4.0` |
| **L5** | Advanced analytics + full dashboard: MVs, funnel, retention, realtime | 46 | 45h | — |
| **L6** | Observability, security, CD, documentation | 35 | 25h | `v1.0.0` |
| | **Total** | **232** | **~207h** | |
| **AWS** | Production infrastructure — replaces L6.4 | 32 | 14h | — |
| | **Total on the AWS path** | **255** | **~214h** | |

Levels L0–L3 build the Phase 1 monolith; L4–L6 build the Phase 2 event pipeline.

Entry/exit criteria, deliverables, and per-phase risks are in [`PHASES.md`](PHASES.md).
Task-level detail, acceptance criteria, and progress live in [`TODO.md`](TODO.md).

---

## Deployment

Production target: **Next.js on Vercel** + **a single EC2 `r7g.xlarge`** in
`ap-southeast-1` running the whole backend stack under Docker Compose, provisioned with
Terraform, fronted by Caddy for automatic TLS, with EBS gp3 storage and DLM snapshots.

Full instructions — Terraform modules, cloud-init bootstrap, RAM allocation, ClickHouse and
Kafka tuning for a single host, CI/CD through ECR and SSM, backup/restore, cost monitoring,
and teardown — are in [`DEPLOY-AWS.md`](DEPLOY-AWS.md). Estimated running cost is ~$267/month
in `ap-southeast-1`; see [`DEPLOY-AWS.md` §14](DEPLOY-AWS.md#14-giám-sát-chi-phí).

This supersedes the "single VPS + docker compose" path described in
[`PLAN.md` §17.4–17.5](PLAN.md#174-cd-productionyml). Pick one of the two and mark the other as
intentionally skipped in `TODO.md` — see
[`PHASES.md` §11](PHASES.md#11-phase-aws--hạ-tầng-production).

---

## Documentation

### The documentation site

Narrative documentation lives in `docs/` as a [VitePress](https://vitepress.dev) site,
published to GitHub Pages at
**<https://nxhawk.github.io/Real-time-Web-Analytics-Platform/>**.

English is the default locale and sits at the root; Vietnamese mirrors the same tree under
`/vi/`. Every page exists in both languages, and the sidebar, search UI and footer are
translated too.

```bash
cd docs
npm ci
npm run dev      # http://localhost:5173, hot reload
npm run build    # static output; fails the build on a dead internal link
```

Pushing to `main` with changes under `docs/` deploys automatically via
`.github/workflows/docs.yml`. Pull requests build but do not deploy, so a broken link is
caught before merge.

| Document | Contents |
|---|---|
| [Documentation site](https://nxhawk.github.io/Real-time-Web-Analytics-Platform/) | Guides, reference, engineering notes and ADRs — English and Vietnamese |
| [`PLAN.md`](PLAN.md) | Full specification: architecture, schema, ClickHouse design, MVs, query cookbook, backend/frontend design, testing, CI/CD, benchmarks, ADRs |
| [`PHASES.md`](PHASES.md) | Phase-by-phase delivery plan: entry/exit criteria, deliverables, metrics to record, risks, traceability matrix, canonical numbers |
| [`TODO.md`](TODO.md) | Execution checklist by level, with estimates and acceptance criteria |
| [`DEPLOY-AWS.md`](DEPLOY-AWS.md) | Vercel + EC2 deployment guide |
| [`CLAUDE.md`](CLAUDE.md) | Working conventions for AI coding agents in this repository |
| `docs/api/openapi.yaml` | API contract — the single source of truth |
| `docs/adr/` | Architecture Decision Records (10 planned) |
| `docs/clickhouse-notes.md` | Experiment log from L2 — ≥ 20 observations with real numbers |
| `docs/benchmark-results.md` | ClickHouse vs PostgreSQL results from L3 |
| `docs/runbook.md` | Incident playbook and operational numbers (RTO, deploy time) |

---

## Contributing

- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/).
- `main` is protected: pull requests only, CI must pass, no force pushes.
- **All code, comments, identifiers, log messages, and documentation must be written in
  English.** See [`CLAUDE.md`](CLAUDE.md).
- Run `make lint && make test` before opening a pull request.

---

## License

MIT — see [`LICENSE`](LICENSE).
