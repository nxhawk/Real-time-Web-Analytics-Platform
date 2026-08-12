# Pulse Analytics — Master Plan

> Real-time Web Analytics Platform (Google Analytics thu nhỏ)
> Go/Gin + ClickHouse + Kafka + Next.js
> Phiên bản tài liệu: 1.1 — 2026-08-12

---

## Bản đồ tài liệu

| File | Vai trò | Quan hệ với file này |
|---|---|---|
| [`README.md`](README.md) | Giới thiệu, quickstart, tổng quan API | Bản rút gọn tiếng Anh của tài liệu này |
| **`PLAN.md`** (file này) | **Đặc tả kỹ thuật** — kiến trúc, schema, DDL, query, contract, ADR | Nguồn sự thật về *thiết kế* |
| [`PHASES.md`](PHASES.md) | Giai đoạn triển khai: entry/exit, deliverable, rủi ro, số liệu chuẩn | Nguồn sự thật về *thứ tự và tiêu chí xong* |
| [`TODO.md`](TODO.md) | Checklist từng task, ước lượng, "Done khi" | Nguồn sự thật về *tiến độ* |
| [`DEPLOY-AWS.md`](DEPLOY-AWS.md) | Hạ tầng production: Vercel + EC2 + Terraform | **Thay thế** §17.4–17.5 của tài liệu này |
| [`CLAUDE.md`](CLAUDE.md) | Quy ước cho AI coding agent | Ràng buộc khi sinh code |

Thứ tự ưu tiên khi mâu thuẫn: **`DEPLOY-AWS.md` (phần deploy)** → **`PLAN.md`** →
**`PHASES.md`** → **`TODO.md`** → code.
Các con số xuất hiện ở nhiều file (version, ngưỡng hiệu năng, hạn mức API, phân phối dữ liệu
seeder) được chốt tại [`PHASES.md` §2 — Bảng số liệu chuẩn](PHASES.md#2-bảng-số-liệu-chuẩn);
sửa ở đó trước rồi mới lan sang các file khác.

---

## Mục lục

1. [Mục tiêu & phạm vi](#1-mục-tiêu--phạm-vi)
2. [Kiến trúc tổng thể](#2-kiến-trúc-tổng-thể)
3. [Tech stack & version](#3-tech-stack--version)
4. [Cấu trúc repository](#4-cấu-trúc-repository)
5. [Event schema & contract](#5-event-schema--contract)
6. [Thiết kế ClickHouse](#6-thiết-kế-clickhouse)
7. [Materialized View & pre-aggregation](#7-materialized-view--pre-aggregation)
8. [Analytics query cookbook](#8-analytics-query-cookbook)
9. [Backend design (Go/Gin)](#9-backend-design-gogin)
10. [Batch insert & backpressure](#10-batch-insert--backpressure)
11. [Kafka pipeline](#11-kafka-pipeline)
12. [API contract chi tiết](#12-api-contract-chi-tiết)
13. [Frontend design (Next.js)](#13-frontend-design-nextjs)
14. [Observability](#14-observability)
15. [Security, privacy & multi-tenant](#15-security-privacy--multi-tenant)
16. [Testing strategy](#16-testing-strategy)
17. [CI/CD](#17-cicd)
18. [Benchmark plan: PostgreSQL vs ClickHouse](#18-benchmark-plan-postgresql-vs-clickhouse)
19. [Roadmap theo Level](#19-roadmap-theo-level)
20. [ADR — Các quyết định kiến trúc](#20-adr--các-quyết-định-kiến-trúc)
21. [Rủi ro & cạm bẫy thường gặp](#21-rủi-ro--cạm-bẫy-thường-gặp)
22. [Definition of Done](#22-definition-of-done)

---

## 1. Mục tiêu & phạm vi

### 1.1 Mục tiêu sản phẩm

Xây một analytics platform tự host, nhận event từ website/app, lưu vào ClickHouse và hiển thị dashboard real-time: overview metrics, time series, top pages, breakdown theo device/country, funnel, retention.

### 1.2 Mục tiêu học tập (quan trọng hơn sản phẩm)

| Chủ đề | Cụ thể cần nắm sau project |
|---|---|
| ClickHouse storage | MergeTree, part/granule, `PARTITION BY`, `ORDER BY` vs primary key, sparse index, `index_granularity` |
| ClickHouse types | `LowCardinality`, `DateTime64`, `Decimal`, `Nullable` (và vì sao nên tránh), codec `ZSTD`/`Delta`/`DoubleDelta`/`Gorilla` |
| Skip index & projection | `bloom_filter`, `minmax`, `set`, `tokenbf_v1`, `PROJECTION` |
| Aggregation | `uniq` vs `uniqExact` vs `uniqCombined`, `quantile*`, `argMax`, `-State`/`-Merge`/`-If` combinators |
| MV & AggregatingMergeTree | Incremental MV, `SimpleAggregateFunction`, `AggregateFunction`, refreshable MV |
| Analytical SQL | `windowFunnel`, `retention`, `sequenceMatch`, `arrayJoin`, window functions |
| Write path | Vì sao insert từng row là anti-pattern, batch insert, `async_insert`, `Too many parts` |
| Go | Clean-ish layering, worker pool, channel buffer, graceful shutdown, context, generics nhẹ |
| Kafka | Producer/consumer, consumer group, partition, offset, at-least-once, retry, DLQ |
| DevOps | Docker Compose, GitHub Actions, image scan, migration tự động, blue-ish deploy |
| Observability | Prometheus metrics, Grafana, structured log, health/readiness probe |

### 1.3 In scope

- Event ingestion API (HTTP, batch + single)
- ClickHouse schema + migration + MV
- Analytics API (overview, timeseries, pages, devices, countries, funnel, retention, realtime)
- Dashboard Next.js
- Kafka pipeline (Level 4)
- Data generator + benchmark suite
- CI/CD với GitHub Actions + deploy Docker Compose lên **1 máy chủ duy nhất** — đường mặc định
  là EC2 `r7g.xlarge` + Vercel theo [`DEPLOY-AWS.md`](DEPLOY-AWS.md); đường thay thế là 1 VPS
  thường theo §17.4–17.5

### 1.4 Out of scope (giai đoạn này)

- Multi-region, replication ClickHouse (ReplicatedMergeTree/Keeper) — chỉ ghi chú
- Billing, user management phức tạp (chỉ API key theo site)
- Session replay, heatmap
- Mobile SDK native

---

## 2. Kiến trúc tổng thể

### 2.1 Phase 1 — Monolith đơn giản (Level 1–3)

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
                                 │
                        ┌────────┴─────────┐
                        │ Next.js Dashboard│
                        └──────────────────┘
```

### 2.2 Phase 2 — Event pipeline với Kafka (Level 4+)

```
┌──────────────┐
│  Web / SDK   │
└──────┬───────┘
       │
       ▼
┌───────────────┐   produce (async, acks=1)   ┌───────────────┐
│  Go Ingest    │────────────────────────────▶│     Kafka     │
│  API (gin)    │                             │ events.raw    │
│  - validate   │◀── 202 Accepted ngay        │ 6 partitions  │
│  - enrich     │                             │ retention 7d  │
└───────────────┘                             └───────┬───────┘
                                                      │
                              ┌───────────────────────┼──────────────┐
                              ▼                       ▼              ▼
                    ┌──────────────────┐    ┌──────────────────┐  ┌──────────┐
                    │  Go Consumer     │    │  Go Consumer     │  │  (future)│
                    │  group: ch-sink  │    │  group: alerting │  │  ML/ETL  │
                    │  batch 5k / 500ms│    └──────────────────┘  └──────────┘
                    └────────┬─────────┘             │
                             │                       ▼
                             │              ┌──────────────────┐
                             │              │ events.dlq       │
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

### 2.3 Nguyên tắc kiến trúc

1. **Ingest path không được phụ thuộc availability của ClickHouse.** ClickHouse chết → API vẫn trả 202 (Phase 2 nhờ Kafka; Phase 1 nhờ disk-backed buffer/WAL fallback).
2. **Write nhiều — read ít, nhưng read phải nhanh.** Mọi dashboard query phải < 300ms ở 100M events.
3. **Không dùng ORM.** Viết SQL trực tiếp để thấy rõ execution plan (`EXPLAIN`, `EXPLAIN PIPELINE`).
4. **Tách 2 binary từ Level 4**: `ingest-api` và `analytics-api` scale độc lập (ingest CPU-light I/O-heavy, analytics CPU-heavy).
5. **Idempotency**: mỗi event có `event_id` (UUIDv7) do client sinh → dedup được khi retry.

---

## 3. Tech stack & version

| Layer | Công nghệ | Version (8/2026) | Ghi chú |
|---|---|---|---|
| Language BE | Go | **1.27** | Có generic methods, JSON v2 engine — dùng `encoding/json/v2` cho ingest nếu ổn định |
| HTTP framework | Gin | v1.11+ | `gin-contrib/zap`, `gin-contrib/cors`, `gin-contrib/gzip` |
| DB | ClickHouse | **26.3 LTS** (hoặc 26.7 stable) | Chạy single node; ghi chú path lên Replicated |
| CH driver | `github.com/ClickHouse/clickhouse-go/v2` | v2.4x | Native protocol (port 9000), KHÔNG dùng HTTP cho insert |
| Migration | **`goose`** (đã chốt) | latest | Chọn goose thay `golang-migrate` vì hỗ trợ multi-statement ClickHouse tốt hơn |
| Kafka | Apache Kafka (KRaft, no ZK) | 4.x | Hoặc Redpanda cho dev nhẹ hơn |
| Kafka client Go | `github.com/twmb/franz-go` | latest | Nhanh, thuần Go, không cần cgo (khác confluent) |
| Config | `caarlos0/env` + `.env` | | Đơn giản hơn viper |
| Log | `log/slog` (stdlib) + JSON handler | | |
| Metrics | `prometheus/client_golang` | | |
| Tracing | OpenTelemetry Go SDK | | Optional Level 5 |
| Validation | `go-playground/validator/v10` | | |
| Test | stdlib + `testify` + `testcontainers-go` | | ClickHouse container thật cho integration test |
| Lint | `golangci-lint` v2 | | |
| Frontend | **Next.js 16.3** (App Router) + React 19 | | |
| Language FE | TypeScript 5.x strict | | |
| UI | TailwindCSS 4 + shadcn/ui | | |
| Chart | Recharts (đơn giản) hoặc Apache ECharts (nhiều điểm dữ liệu) | | ECharts cho time series > 5k điểm |
| Data fetching | TanStack Query v5 | | polling 10s cho realtime, hoặc SSE |
| Table | TanStack Table v8 | | |
| Date | `date-fns` + `date-fns-tz` | | |
| Test FE | Vitest + Testing Library + Playwright | | |
| Container | Docker + Docker Compose v2 | | |
| CI/CD | GitHub Actions | | GHCR cho image dev/CI; **ECR** cho production AWS ([`DEPLOY-AWS.md` §10](DEPLOY-AWS.md#10-cicd-github-actions--ecr--ssm)) |
| Reverse proxy | Caddy (tự động TLS) hoặc Nginx | | Caddy đỡ việc hơn nhiều |
| Load test | k6 (ingest) + custom Go generator (bulk seed) | | |
| Dashboard hạ tầng | Prometheus + Grafana | | |

---

## 4. Cấu trúc repository

Monorepo — dễ CI, dễ đồng bộ contract.

```
pulse-analytics/
├── README.md                     # giới thiệu + quickstart (tiếng Anh)
├── PLAN.md                       # tài liệu này — đặc tả kỹ thuật
├── PHASES.md                     # giai đoạn triển khai, entry/exit, số liệu chuẩn
├── TODO.md                       # checklist thực thi theo level
├── DEPLOY-AWS.md                 # hạ tầng production Vercel + EC2
├── CLAUDE.md                     # quy ước cho AI coding agent
├── CONTRIBUTING.md
├── LICENSE
├── Makefile                      # entrypoint mọi lệnh dev
├── .env.example
├── .editorconfig
├── .gitignore
├── .golangci.yml
├── docker-compose.yml            # dev: clickhouse + kafka + api + web
├── docker-compose.prod.yml
├── docker-compose.bench.yml      # thêm postgres cho benchmark
│
├── .github/
│   ├── workflows/
│   │   ├── ci-backend.yml
│   │   ├── ci-frontend.yml
│   │   ├── ci-migrations.yml
│   │   ├── security.yml          # trivy, gosec, govulncheck, npm audit
│   │   ├── cd-staging.yml
│   │   ├── cd-production.yml
│   │   └── benchmark.yml         # chạy tay (workflow_dispatch)
│   ├── dependabot.yml
│   ├── CODEOWNERS
│   └── pull_request_template.md
│
├── backend/
│   ├── go.mod / go.sum
│   ├── Dockerfile                # multi-stage, distroless
│   ├── cmd/
│   │   ├── ingest-api/main.go    # HTTP nhận event
│   │   ├── analytics-api/main.go # HTTP query
│   │   ├── consumer/main.go      # Kafka -> ClickHouse (Level 4)
│   │   ├── migrate/main.go       # chạy migration
│   │   └── seeder/main.go        # generate N triệu events
│   ├── internal/
│   │   ├── config/config.go
│   │   ├── httpx/                # middleware: reqid, logger, recover, cors, ratelimit, apikey
│   │   ├── handler/
│   │   │   ├── event_handler.go
│   │   │   ├── analytics_handler.go
│   │   │   └── health_handler.go
│   │   ├── service/
│   │   │   ├── event_service.go
│   │   │   ├── analytics_service.go
│   │   │   └── enrich.go         # UA parse, GeoIP, session stitching
│   │   ├── repository/
│   │   │   ├── clickhouse/
│   │   │   │   ├── conn.go
│   │   │   │   ├── event_repo.go
│   │   │   │   ├── analytics_repo.go
│   │   │   │   └── queries/      # *.sql nhúng bằng go:embed
│   │   │   └── postgres/         # chỉ cho benchmark
│   │   ├── buffer/               # batch writer, ring buffer, WAL fallback
│   │   ├── kafka/                # producer, consumer, dlq
│   │   ├── model/
│   │   │   ├── event.go
│   │   │   └── analytics.go
│   │   ├── metrics/metrics.go
│   │   └── validate/
│   ├── pkg/                      # thứ có thể public: geoip, uaparser wrapper
│   ├── migrations/
│   │   ├── 0001_create_database.up.sql
│   │   ├── 0002_events.up.sql
│   │   ├── 0003_mv_events_hourly.up.sql
│   │   ├── 0004_mv_daily_users.up.sql
│   │   ├── 0005_mv_page_stats_hourly.up.sql   # tách riêng: page cardinality cao
│   │   ├── 0006_mv_sessions.up.sql
│   │   ├── 0007_user_first_seen.up.sql
│   │   └── 0008_projections_ttl.up.sql
│   └── test/
│       ├── integration/
│       └── testdata/
│
├── frontend/
│   ├── package.json
│   ├── Dockerfile
│   ├── next.config.ts
│   ├── src/
│   │   ├── app/
│   │   │   ├── layout.tsx
│   │   │   ├── page.tsx                 # Overview
│   │   │   ├── realtime/page.tsx
│   │   │   ├── pages/page.tsx
│   │   │   ├── audience/page.tsx        # device / country / browser
│   │   │   ├── funnel/page.tsx
│   │   │   ├── retention/page.tsx
│   │   │   └── settings/page.tsx
│   │   ├── components/
│   │   │   ├── ui/                      # shadcn
│   │   │   ├── charts/
│   │   │   │   ├── TimeSeriesChart.tsx
│   │   │   │   ├── FunnelChart.tsx
│   │   │   │   ├── RetentionHeatmap.tsx
│   │   │   │   └── BarBreakdown.tsx
│   │   │   ├── StatCard.tsx
│   │   │   ├── DateRangePicker.tsx
│   │   │   └── FilterBar.tsx
│   │   ├── lib/
│   │   │   ├── api.ts                   # fetch wrapper + zod parse
│   │   │   ├── queries.ts               # TanStack Query hooks
│   │   │   └── format.ts
│   │   └── types/api.ts                 # sinh từ OpenAPI
│   └── e2e/                             # Playwright
│
├── sdk/
│   └── js/                       # pulse.js — tracking snippet (~2KB)
│       ├── src/index.ts
│       └── README.md
│
├── deploy/
│   ├── caddy/Caddyfile
│   ├── clickhouse/
│   │   ├── config.d/            # logger, listen_host, memory limits
│   │   └── users.d/             # profiles, quotas
│   ├── kafka/
│   ├── grafana/dashboards/
│   ├── prometheus/prometheus.yml
│   └── scripts/deploy.sh
│
├── infra/                        # Terraform cho đường AWS — xem DEPLOY-AWS.md §4
│   └── ...
│
├── loadtest/
│   ├── k6/ingest.js
│   ├── k6/query.js
│   └── bench/                    # so sánh CH vs PG
│       ├── queries_clickhouse.sql
│       ├── queries_postgres.sql
│       └── run_bench.go
│
└── docs/
    ├── api/openapi.yaml
    ├── adr/0001-....md
    ├── clickhouse-notes.md       # ghi chép học được (L2, >= 20 mục)
    ├── queries-ops.sql           # query "soi bảng" (L2)
    ├── benchmark-results.md      # kết quả CH vs PG (L3)
    └── runbook.md                # xử lý sự cố + RTO thực tế
```

> Task tạo cây thư mục này là `L0-04`; task đưa toàn bộ tài liệu vào repo là `L0-06`.

---

## 5. Event schema & contract

### 5.1 Payload từ client

```jsonc
// POST /api/v1/events  (single hoặc batch đều nhận)
{
  "site_id": "site_abc",             // bắt buộc, map với API key
  "events": [
    {
      "event_id": "0192f8a1-...",    // UUIDv7 client sinh — dùng để dedup
      "event": "page_view",          // bắt buộc, snake_case, <= 64 ký tự
      "user_id": "u_123",            // anonymous id nếu chưa login
      "session_id": "s_456",
      "timestamp": "2026-08-11T14:20:00.123Z", // ISO8601 UTC; server override nếu lệch > 24h
      "page": "/products/123",
      "referrer": "https://google.com/",
      "utm": { "source": "google", "medium": "cpc", "campaign": "summer" },
      "device": "desktop",           // desktop | mobile | tablet | bot | unknown
      "os": "macOS",
      "browser": "Chrome",
      "screen": "1920x1080",
      "country": "VN",               // server enrich từ IP nếu client bỏ trống
      "city": "Ho Chi Minh City",
      "revenue": 199000,             // chỉ với purchase
      "currency": "VND",
      "properties": { "product_id": "123", "category": "shoes" }
    }
  ]
}
```

### 5.2 Quy tắc validate

| Field | Rule | Hành vi khi sai |
|---|---|---|
| `site_id` | required, khớp API key | 401 |
| `event` | required, `^[a-z0-9_]{1,64}$` | reject event đó, không reject cả batch |
| `timestamp` | ISO8601; lệch server > 24h tương lai hoặc > 30 ngày quá khứ | override = `now()`, tăng counter `events_clock_skew_total` |
| `user_id` | <= 128 ký tự | truncate |
| `page` | <= 2048 ký tự, strip query string nhạy cảm (`token`, `email`, `password`) | sanitize |
| `properties` | JSON object, <= 8KB serialize | reject event |
| batch size | <= 500 events, body <= 1MB | 413 |

**Nguyên tắc partial success**: batch 100 event, 3 event sai → nhận 97, trả `202` kèm `rejected: [...]`. Không bao giờ để 1 event hỏng làm mất cả batch.

### 5.3 Enrichment phía server

1. IP → country/city bằng MaxMind GeoLite2 (`oschwald/geoip2-golang`), **không lưu IP thô** (chỉ lưu hash nếu cần).
2. User-Agent → device/os/browser/bot (`mileusna/useragent`), bỏ qua nếu client đã gửi.
3. `session_id` rỗng → sinh từ `hash(user_id + date + 30-min-window)`.
4. `ingested_at = now()` để đo end-to-end latency và phát hiện backlog.

---

## 6. Thiết kế ClickHouse

### 6.1 Bảng raw `events`

```sql
CREATE DATABASE IF NOT EXISTS analytics;

CREATE TABLE analytics.events
(
    -- time
    event_time    DateTime64(3, 'UTC')                       CODEC(Delta, ZSTD(1)),
    event_date    Date MATERIALIZED toDate(event_time),
    ingested_at   DateTime DEFAULT now()                     CODEC(Delta, ZSTD(1)),

    -- identity
    site_id       LowCardinality(String),
    event_id      UUID,
    event_name    LowCardinality(String),
    user_id       String                                     CODEC(ZSTD(1)),
    session_id    String                                     CODEC(ZSTD(1)),

    -- page context
    page          String                                     CODEC(ZSTD(3)),
    referrer      String                                     CODEC(ZSTD(3)),
    utm_source    LowCardinality(String),
    utm_medium    LowCardinality(String),
    utm_campaign  LowCardinality(String),

    -- audience
    country       LowCardinality(String),
    city          LowCardinality(String),
    device        LowCardinality(String),
    os            LowCardinality(String),
    browser       LowCardinality(String),

    -- commerce
    revenue       Decimal(18, 4) DEFAULT 0,
    currency      LowCardinality(String) DEFAULT 'VND',

    -- free-form
    properties    String                                     CODEC(ZSTD(3)),

    -- skip index
    INDEX idx_page     page     TYPE bloom_filter(0.01) GRANULARITY 4,
    INDEX idx_user     user_id  TYPE bloom_filter(0.01) GRANULARITY 4,
    INDEX idx_ingested ingested_at TYPE minmax          GRANULARITY 1
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_date)
ORDER BY (site_id, event_name, event_time)
TTL toDateTime(event_date) + INTERVAL 180 DAY DELETE,
    toDateTime(event_date) + INTERVAL 30 DAY  RECOMPRESS CODEC(ZSTD(9))
SETTINGS index_granularity = 8192;
```

#### Vì sao `ORDER BY (site_id, event_name, event_time)`?

- **Dashboard workload thực tế**: gần như mọi query đều có `WHERE site_id = ? AND event_date BETWEEN ? AND ?` và rất thường có `event_name = 'page_view'`. Đặt cột lọc-bằng, cardinality thấp lên trước → primary index cắt được nhiều granule nhất.
- `event_time` đứng cuối để dữ liệu trong mỗi (site, event) được sắp theo thời gian → nén `Delta + ZSTD` cực tốt và range scan theo thời gian là sequential.
- **Không đặt `user_id` vào ORDER BY**: cardinality cực cao, đặt sau `event_time` thì vô dụng cho việc skip, đặt trước thì phá vỡ locality theo thời gian. Thay vào đó dùng bloom filter skip index + **projection** riêng cho truy vấn theo user.

#### Projection cho funnel/retention (query theo user)

```sql
ALTER TABLE analytics.events ADD PROJECTION prj_by_user
(
    SELECT site_id, user_id, event_name, event_time, page, revenue
    ORDER BY (site_id, user_id, event_time)
);

ALTER TABLE analytics.events MATERIALIZE PROJECTION prj_by_user;
```

> Đánh đổi: tốn thêm ~40–60% dung lượng, insert chậm hơn ~15%. Đo trước/sau bằng `system.parts` và ghi vào `docs/benchmark-results.md`.

#### Những thứ cần quan sát sau khi tạo bảng

```sql
-- dung lượng & tỉ lệ nén từng cột
SELECT
    name,
    formatReadableSize(sum(data_compressed_bytes))   AS compressed,
    formatReadableSize(sum(data_uncompressed_bytes)) AS uncompressed,
    round(sum(data_uncompressed_bytes) / sum(data_compressed_bytes), 2) AS ratio
FROM system.parts_columns
WHERE table = 'events' AND active
GROUP BY name ORDER BY sum(data_compressed_bytes) DESC;

-- số part theo partition (canh chừng "Too many parts")
SELECT partition, count() AS parts, sum(rows) AS rows,
       formatReadableSize(sum(bytes_on_disk)) AS size
FROM system.parts WHERE table='events' AND active GROUP BY partition ORDER BY partition;

-- query nào chậm
SELECT query_duration_ms, read_rows, formatReadableSize(read_bytes) AS read, query
FROM system.query_log
WHERE type='QueryFinish' AND event_time > now() - INTERVAL 1 HOUR
ORDER BY query_duration_ms DESC LIMIT 20;
```

### 6.2 Bảng phụ

| Bảng | Engine | Mục đích |
|---|---|---|
| `events_hourly` | AggregatingMergeTree | Time series + overview theo giờ |
| `events_daily` | AggregatingMergeTree | DAU/WAU/MAU, retention nền |
| `page_stats_hourly` | AggregatingMergeTree | Top pages nhanh |
| `sessions` | AggregatingMergeTree | Session duration, bounce rate, entry/exit page |
| `user_first_seen` | AggregatingMergeTree (min) | Cohort cho retention |
| `events_dlq` | MergeTree | Event lỗi để replay |

---

## 7. Materialized View & pre-aggregation

### 7.1 Nguyên lý cần hiểu trước khi viết

- MV trong ClickHouse là **insert trigger**, không phải view lười. Nó chỉ thấy **block đang được insert**, không thấy dữ liệu cũ → tạo MV xong phải backfill thủ công.
- Với AggregatingMergeTree: cột nào cộng dồn được thì dùng `SimpleAggregateFunction(sum, T)`, cột nào cần thuật toán (uniq, quantile) thì dùng `AggregateFunction(...)` + `-State` khi ghi, `-Merge` khi đọc.
- Sai lầm kinh điển: khai báo `count UInt64` trong AggregatingMergeTree → merge sẽ **giữ 1 giá trị bất kỳ**, không cộng → số liệu sai âm thầm.

### 7.2 `events_hourly`

```sql
CREATE TABLE analytics.events_hourly
(
    site_id        LowCardinality(String),
    hour           DateTime,
    event_name     LowCardinality(String),
    country        LowCardinality(String),
    device         LowCardinality(String),
    events         SimpleAggregateFunction(sum, UInt64),
    revenue        SimpleAggregateFunction(sum, Decimal(38, 4)),
    users_state    AggregateFunction(uniq, String),
    sessions_state AggregateFunction(uniq, String)
)
ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(hour)
ORDER BY (site_id, hour, event_name, country, device);

CREATE MATERIALIZED VIEW analytics.mv_events_hourly TO analytics.events_hourly AS
SELECT
    site_id,
    toStartOfHour(event_time) AS hour,
    event_name,
    country,
    device,
    count()               AS events,
    sum(revenue)          AS revenue,
    uniqState(user_id)    AS users_state,
    uniqState(session_id) AS sessions_state
FROM analytics.events
GROUP BY site_id, hour, event_name, country, device;
```

Backfill sau khi tạo:

```sql
INSERT INTO analytics.events_hourly
SELECT site_id, toStartOfHour(event_time) AS hour, event_name, country, device,
       count(), sum(revenue), uniqState(user_id), uniqState(session_id)
FROM analytics.events
WHERE event_date < today()          -- tránh double-count phần MV đã bắt
GROUP BY site_id, hour, event_name, country, device;
```

Đọc:

```sql
SELECT
    hour,
    sum(events)              AS events,
    uniqMerge(users_state)   AS users,
    uniqMerge(sessions_state) AS sessions
FROM analytics.events_hourly
WHERE site_id = {site:String} AND hour >= {from:DateTime} AND hour < {to:DateTime}
GROUP BY hour
ORDER BY hour
SETTINGS max_execution_time = 10;
```

### 7.3 `events_daily` (DAU/WAU/MAU)

```sql
CREATE TABLE analytics.events_daily
(
    site_id     LowCardinality(String),
    day         Date,
    events      SimpleAggregateFunction(sum, UInt64),
    revenue     SimpleAggregateFunction(sum, Decimal(38, 4)),
    users_state AggregateFunction(uniq, String)
)
ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(day)
ORDER BY (site_id, day);

CREATE MATERIALIZED VIEW analytics.mv_events_daily TO analytics.events_daily AS
SELECT site_id, toDate(event_time) AS day, count(), sum(revenue), uniqState(user_id)
FROM analytics.events GROUP BY site_id, day;
```

WAU/MAU dùng cửa sổ trượt:

```sql
SELECT
    day,
    uniqMerge(users_state) AS dau,
    uniqMergeIf(users_state, 1) OVER (ORDER BY day ROWS BETWEEN 6 PRECEDING AND CURRENT ROW) AS _  -- KHÔNG dùng được
FROM analytics.events_daily ...
```

> `uniqMerge` không chạy được trong window function. Cách đúng: dùng `uniqMergeArray`/self-join theo khoảng, hoặc đơn giản là 3 query riêng cho DAU/WAU/MAU với 3 khoảng thời gian. Đây là một bài học hay — ghi vào `docs/clickhouse-notes.md`.

```sql
-- WAU đúng
SELECT uniqMerge(users_state) FROM analytics.events_daily
WHERE site_id = {site:String} AND day > today() - 7;
```

### 7.4 `sessions`

```sql
CREATE TABLE analytics.sessions
(
    site_id      LowCardinality(String),
    session_date Date,
    session_id   String,
    user_id      SimpleAggregateFunction(any, String),
    started_at   SimpleAggregateFunction(min, DateTime64(3)),
    ended_at     SimpleAggregateFunction(max, DateTime64(3)),
    pageviews    SimpleAggregateFunction(sum, UInt64),
    events       SimpleAggregateFunction(sum, UInt64),
    revenue      SimpleAggregateFunction(sum, Decimal(38,4)),
    entry_page   AggregateFunction(argMin, String, DateTime64(3)),
    exit_page    AggregateFunction(argMax, String, DateTime64(3)),
    country      SimpleAggregateFunction(any, LowCardinality(String)),
    device       SimpleAggregateFunction(any, LowCardinality(String))
)
ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(session_date)
ORDER BY (site_id, session_date, session_id);

CREATE MATERIALIZED VIEW analytics.mv_sessions TO analytics.sessions AS
SELECT
    site_id,
    toDate(event_time) AS session_date,
    session_id,
    any(user_id)                        AS user_id,
    min(event_time)                     AS started_at,
    max(event_time)                     AS ended_at,
    countIf(event_name = 'page_view')   AS pageviews,
    count()                             AS events,
    sum(revenue)                        AS revenue,
    argMinState(page, event_time)       AS entry_page,
    argMaxState(page, event_time)       AS exit_page,
    any(country)                        AS country,
    any(device)                         AS device
FROM analytics.events
GROUP BY site_id, session_date, session_id;
```

Bounce rate & avg session duration:

```sql
SELECT
    countIf(pv = 1) / count()                       AS bounce_rate,
    avg(dateDiff('second', started, ended))         AS avg_duration_sec,
    quantile(0.5)(dateDiff('second', started, ended)) AS median_duration_sec
FROM (
    SELECT session_id,
           sum(pageviews) AS pv,
           min(started_at) AS started,
           max(ended_at)   AS ended
    FROM analytics.sessions
    WHERE site_id = {site:String} AND session_date BETWEEN {from:Date} AND {to:Date}
    GROUP BY session_id
);
```

### 7.5 `user_first_seen` (cohort)

```sql
CREATE TABLE analytics.user_first_seen
(
    site_id    LowCardinality(String),
    user_id    String,
    first_date SimpleAggregateFunction(min, Date)
)
ENGINE = AggregatingMergeTree
ORDER BY (site_id, user_id);

CREATE MATERIALIZED VIEW analytics.mv_user_first_seen TO analytics.user_first_seen AS
SELECT site_id, user_id, min(toDate(event_time)) AS first_date
FROM analytics.events GROUP BY site_id, user_id;
```

---

## 8. Analytics query cookbook

> Tất cả query dùng **parameterized query** (`{name:Type}`) của ClickHouse — vừa an toàn SQL injection, vừa để driver bind kiểu đúng.

### 8.1 Overview

```sql
SELECT
    uniqMerge(users_state)    AS users,
    uniqMerge(sessions_state) AS sessions,
    sum(events)               AS events,
    sum(revenue)              AS revenue
FROM analytics.events_hourly
WHERE site_id = {site:String}
  AND hour >= {from:DateTime} AND hour < {to:DateTime};
```

So sánh kỳ trước (%) → chạy thêm 1 query với khoảng lùi, hoặc gộp bằng `-If`:

```sql
SELECT
    sumIf(events, hour >= {from:DateTime})                            AS cur_events,
    sumIf(events, hour <  {from:DateTime})                            AS prev_events,
    uniqMergeIf(users_state, hour >= {from:DateTime})                 AS cur_users,
    uniqMergeIf(users_state, hour <  {from:DateTime})                 AS prev_users
FROM analytics.events_hourly
WHERE site_id = {site:String} AND hour >= {prev_from:DateTime} AND hour < {to:DateTime};
```

### 8.2 Time series (auto granularity)

Chọn bucket theo độ dài khoảng: `<= 2 ngày → hour`, `<= 90 ngày → day`, còn lại `week`.

```sql
SELECT
    toStartOfInterval(hour, INTERVAL {step:UInt32} HOUR) AS ts,
    sum(events)             AS events,
    uniqMerge(users_state)  AS users
FROM analytics.events_hourly
WHERE site_id = {site:String} AND hour >= {from:DateTime} AND hour < {to:DateTime}
GROUP BY ts
ORDER BY ts
WITH FILL FROM {from:DateTime} TO {to:DateTime} STEP toIntervalHour({step:UInt32});
```

> `WITH FILL` là thứ rất đáng học: tự vá các bucket rỗng để chart không bị đứt đoạn — không cần generate_series như Postgres.

### 8.3 Top pages

```sql
SELECT
    page,
    sum(views)             AS views,
    uniqMerge(users_state) AS users
FROM analytics.page_stats_hourly
WHERE site_id = {site:String} AND hour >= {from:DateTime} AND hour < {to:DateTime}
GROUP BY page
ORDER BY views DESC
LIMIT {limit:UInt32} OFFSET {offset:UInt32};
```

### 8.4 Breakdown device / country / browser

```sql
SELECT
    {dim:Identifier}       AS name,       -- device | country | browser | os
    uniqMerge(users_state) AS users,
    sum(events)            AS events,
    round(100 * users / sum(users) OVER (), 2) AS pct
FROM analytics.events_hourly
WHERE site_id = {site:String} AND hour >= {from:DateTime} AND hour < {to:DateTime}
GROUP BY name
ORDER BY users DESC
LIMIT 20;
```

> `{dim:Identifier}` cho phép truyền tên cột an toàn. Vẫn phải whitelist ở Go — không tin tưởng client tuyệt đối.

### 8.5 Funnel — `windowFunnel`

```sql
WITH funnel AS (
    SELECT
        user_id,
        windowFunnel({window:UInt32})(
            event_time,
            event_name = {s1:String},
            event_name = {s2:String},
            event_name = {s3:String},
            event_name = {s4:String},
            event_name = {s5:String}
        ) AS level
    FROM analytics.events
    WHERE site_id = {site:String}
      AND event_date BETWEEN {from:Date} AND {to:Date}
      AND event_name IN ({s1:String},{s2:String},{s3:String},{s4:String},{s5:String})
    GROUP BY user_id
)
SELECT
    step,
    sum(level >= step) AS users
FROM funnel
ARRAY JOIN [1,2,3,4,5] AS step
GROUP BY step
ORDER BY step;
```

Điểm học: `windowFunnel(window_seconds)(timestamp, cond1, ..., condN)` trả về **số bước liên tiếp dài nhất** hoàn thành trong cửa sổ. Thêm mode `'strict_order'` nếu muốn không cho phép event lạ xen giữa.

### 8.6 Retention — cohort matrix

```sql
SELECT
    f.first_date                                   AS cohort_date,
    dateDiff('day', f.first_date, a.day)           AS day_offset,
    uniq(a.user_id)                                AS users
FROM (
    SELECT DISTINCT site_id, user_id, toDate(event_time) AS day
    FROM analytics.events
    WHERE site_id = {site:String} AND event_date BETWEEN {from:Date} AND {to:Date}
) AS a
INNER JOIN (
    SELECT site_id, user_id, min(first_date) AS first_date
    FROM analytics.user_first_seen
    WHERE site_id = {site:String}
    GROUP BY site_id, user_id
) AS f USING (site_id, user_id)
WHERE f.first_date BETWEEN {from:Date} AND {to:Date}
  AND dateDiff('day', f.first_date, a.day) BETWEEN 0 AND 30
GROUP BY cohort_date, day_offset
ORDER BY cohort_date, day_offset;
```

Bản dùng hàm `retention()` (gọn hơn, cố định số ngày):

```sql
SELECT
    sum(r[1]) AS d0, sum(r[2]) AS d1, sum(r[3]) AS d3, sum(r[4]) AS d7, sum(r[5]) AS d30
FROM (
    SELECT user_id,
        retention(
            event_date = {d0:Date},
            event_date = {d0:Date} + 1,
            event_date = {d0:Date} + 3,
            event_date = {d0:Date} + 7,
            event_date = {d0:Date} + 30
        ) AS r
    FROM analytics.events
    WHERE site_id = {site:String} AND event_date >= {d0:Date} AND event_date <= {d0:Date} + 30
    GROUP BY user_id
);
```

### 8.7 Realtime (30 phút gần nhất)

```sql
SELECT
    uniq(user_id) AS active_users,
    count()       AS events
FROM analytics.events
WHERE site_id = {site:String} AND event_time >= now() - INTERVAL 30 MINUTE;
```

> Query này chạy trên raw table nên phải nhanh: nhờ `ORDER BY (site_id, event_name, event_time)` + partition hiện tại nên chỉ đọc part mới nhất. Kiểm chứng bằng `EXPLAIN indexes = 1`.

### 8.8 Path analysis (bonus)

```sql
SELECT path, count() AS sessions
FROM (
    SELECT session_id,
           arrayStringConcat(arraySlice(groupArray(page), 1, 4), ' → ') AS path
    FROM (SELECT session_id, page, event_time FROM analytics.events
          WHERE site_id={site:String} AND event_name='page_view'
            AND event_date BETWEEN {from:Date} AND {to:Date}
          ORDER BY session_id, event_time)
    GROUP BY session_id
)
GROUP BY path ORDER BY sessions DESC LIMIT 20;
```

---

## 9. Backend design (Go/Gin)

### 9.1 Luồng

```
HTTP request
   │  middleware: requestID → logger → recover → cors → ratelimit → apikey → gzip
   ▼
Handler        (parse JSON, validate shape, trả HTTP status)
   ▼
Service        (business: enrich, sanitize, quyết định buffer/kafka)
   ▼
Repository     (SQL thuần, bind param, map row → struct)
   ▼
ClickHouse / Kafka
```

Quy tắc: handler **không** biết SQL; repository **không** biết HTTP; model dùng chung, không import ngược.

### 9.2 Ingest handler (rút gọn)

```go
func (h *EventHandler) Ingest(c *gin.Context) {
    var req model.IngestRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
        return
    }
    site := c.GetString(httpx.CtxSiteID) // từ middleware API key

    accepted, rejected := h.svc.Ingest(c.Request.Context(), site, req.Events)

    metrics.EventsAccepted.WithLabelValues(site).Add(float64(accepted))
    metrics.EventsRejected.WithLabelValues(site).Add(float64(len(rejected)))

    c.JSON(http.StatusAccepted, gin.H{
        "accepted": accepted,
        "rejected": rejected, // [{index, reason}]
    })
}
```

**Vì sao 202 chứ không 200/201**: server mới chỉ *nhận*, chưa *ghi xong*. Đây là hợp đồng quan trọng để ingest không bị chặn bởi ClickHouse.

### 9.3 Endpoint đặc biệt cho `sendBeacon`

`GET /api/v1/pixel.gif?e=<base64-json>` — trả GIF 1x1, dùng cho trường hợp JS bị chặn hoặc trang unload. Nhớ `Cache-Control: no-store`.

### 9.4 Graceful shutdown

```go
srv := &http.Server{Addr: cfg.Addr, Handler: r}
go srv.ListenAndServe()

ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
<-ctx.Done()

shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
_ = srv.Shutdown(shutdownCtx) // ngừng nhận request mới
writer.Close()                // FLUSH hết buffer còn lại — bắt buộc, nếu không mất data
```

---

## 10. Batch insert & backpressure

### 10.1 Vì sao không insert từng row

ClickHouse mỗi lần INSERT tạo ra **1 part mới** trên đĩa. Part phải được merge nền. Insert 10k row/s theo từng row = 10k part/s → merge không kịp → lỗi `Too many parts (300). Merges are processing significantly slower than inserts.` Ghi nhớ nguyên tắc:

> **Optimize for large sequential/batched writes, not frequent tiny mutations.**
> Mục tiêu: mỗi INSERT >= 10.000 row hoặc >= 1 giây dữ liệu, và <= 1 INSERT/giây/bảng cho mỗi client.

### 10.2 Thiết kế BatchWriter

```go
type BatchWriter struct {
    ch        chan model.Event   // buffered, size = 200_000
    batchSize int                // 5000
    flushEvery time.Duration     // 500ms
    workers   int                // 4
    conn      driver.Conn
    wg        sync.WaitGroup
}
```

Mỗi worker:

```
for {
  select {
  case e := <-ch:  buf = append(buf, e)
                   if len(buf) >= batchSize { flush() }
  case <-ticker.C: if len(buf) > 0 { flush() }
  case <-done:     flush(); return
  }
}
```

`flush()` dùng **column-oriented batch API** của clickhouse-go (nhanh hơn hẳn `Exec` từng dòng):

```go
batch, err := conn.PrepareBatch(ctx, "INSERT INTO analytics.events")
for _, e := range buf {
    if err := batch.Append(e.EventTime, e.SiteID, e.EventID, e.EventName, /* ... */); err != nil {
        return err
    }
}
err = batch.Send()
```

### 10.3 Backpressure — 3 mức

| Tình huống | Xử lý |
|---|---|
| Buffer < 70% | nhận bình thường, `202` |
| Buffer 70–95% | vẫn nhận nhưng bật metric cảnh báo, giảm flush interval xuống 200ms |
| Buffer > 95% | **drop có kiểm soát**: giữ lại event quan trọng (`purchase`, `signup`), drop `page_view`; trả `202` kèm `"sampled": true`. KHÔNG block HTTP handler |
| ClickHouse down | retry backoff 3 lần → ghi batch ra file WAL (`/var/lib/pulse/wal/*.ndjson`) → cron replay khi CH sống lại |

**Nguyên tắc**: không bao giờ để goroutine HTTP block trên `ch <- event`. Dùng:

```go
select {
case w.ch <- e:
    return true
default:
    metrics.EventsDropped.Inc()
    return false
}
```

### 10.4 So sánh với `async_insert` của ClickHouse

ClickHouse có sẵn cơ chế gộp phía server:

```sql
SET async_insert = 1, wait_for_async_insert = 0,
    async_insert_max_data_size = 10000000, async_insert_busy_timeout_ms = 1000;
```

**Bài tập bắt buộc**: benchmark 3 chiến lược và ghi kết quả:

| Chiến lược | Throughput (ev/s) | p99 latency API | Số part/phút | Ghi chú |
|---|---|---|---|---|
| 1 row / INSERT | | | | dự đoán: thảm hoạ |
| Batch 100 client-side | | | | |
| Batch 1.000 client-side | | | | |
| Batch 10.000 client-side | | | | |
| `async_insert` server-side | | | | mất kiểm soát retry |
| Kafka + consumer batch 10k | | | | |

---

## 11. Kafka pipeline

### 11.1 Topic design

| Topic | Partitions | Retention | Key | Ghi chú |
|---|---|---|---|---|
| `events.raw` | 6 | 7 ngày | `site_id\|session_id` | Key theo session để event cùng session vào cùng partition → giữ thứ tự cho funnel |
| `events.dlq` | 1 | 30 ngày | như trên | Kèm header `error`, `retry_count`, `original_topic` |
| `events.enriched` | 6 | 3 ngày | (Level 5) nếu tách bước enrich |

Producer config: `acks=1`, `linger.ms=50`, `batch.size=1MB`, `compression=zstd`, `max.in.flight=5`, `enable.idempotence=true`.

Consumer config: `group.id=clickhouse-sink`, `auto.offset.reset=earliest`, **commit thủ công sau khi ClickHouse ack**.

### 11.2 Consumer loop (at-least-once đúng cách)

```
poll(max 10.000 records / 500ms)
      │
      ▼
  decode + validate  ──lỗi──▶ produce vào events.dlq (không chặn)
      │
      ▼
  batch insert ClickHouse
      │
      ├── success ──▶ commit offset
      └── fail ──▶ retry 3 lần backoff (1s, 4s, 16s)
                    └── vẫn fail ──▶ DLQ + commit (không để kẹt partition)
```

**Sai lầm cần tránh**: commit offset trước khi insert → mất data khi crash. Ngược lại, không commit và retry vô hạn → consumer lag phình vô tận, chặn cả partition.

### 11.3 Dedup

Vì at-least-once, event có thể vào ClickHouse 2 lần. 3 lựa chọn — chọn (b) cho project này:

- (a) `ReplacingMergeTree(ingested_at)` với `ORDER BY (..., event_id)` — dedup cuối cùng nhưng phải `FINAL` khi query (chậm).
- (b) **Chấp nhận trùng ở raw, dedup ở tầng query** bằng `uniq(event_id)` cho các metric nhạy cảm. Đơn giản, nhanh.
- (c) Dedup ở consumer bằng Redis SETNX TTL 1h — thêm dependency.

### 11.4 Vì sao chèn Kafka vào giữa

- **Decoupling**: ClickHouse restart / OPTIMIZE nặng → ingest vẫn 202.
- **Replay**: đổi schema, thêm cột enrich → replay từ offset 0 để backfill.
- **Fan-out**: consumer thứ 2 cho alerting real-time, consumer thứ 3 cho ETL — không đụng consumer chính.
- **Buffer tự nhiên**: traffic spike 10x không giết ClickHouse.

> Ghi chú: ClickHouse có sẵn `Kafka` table engine + MV. **Vẫn nên tự viết consumer Go** vì mục tiêu là học Kafka; nhưng nên thử engine đó 1 lần để biết sự khác biệt và ghi vào docs.

---

## 12. API contract chi tiết

Base: `/api/v1`. Tất cả response JSON, `Content-Type: application/json; charset=utf-8`.

### 12.1 Ingest

| Method | Path | Auth | Mô tả |
|---|---|---|---|
| `POST` | `/events` | `X-API-Key` | Nhận 1..500 event |
| `GET` | `/pixel.gif` | query `k=` | Fallback pixel |

### 12.2 Analytics (đều yêu cầu `X-API-Key` hoặc session cookie của dashboard)

Query params dùng chung: `from` (ISO date/datetime), `to`, `tz` (mặc định `Asia/Ho_Chi_Minh`), `filter[device]`, `filter[country]`, `filter[page]`, `filter[event]`.

| Method | Path | Response chính |
|---|---|---|
| `GET` | `/analytics/overview` | `{users, sessions, events, pageviews, revenue, bounce_rate, avg_session_sec, delta:{...}}` |
| `GET` | `/analytics/timeseries?metric=users\|events\|sessions\|revenue&interval=hour\|day\|week` | `{series:[{ts, value}], interval}` |
| `GET` | `/analytics/pages?limit=&offset=&sort=views\|users` | `{items:[{page, views, users, avg_time_sec}], total}` |
| `GET` | `/analytics/devices` | `{items:[{name, users, events, pct}]}` |
| `GET` | `/analytics/countries` | như trên |
| `GET` | `/analytics/browsers` | như trên |
| `GET` | `/analytics/os` | như trên |
| `GET` | `/analytics/sources` | referrer + utm |
| `GET` | `/analytics/funnel?steps=page_view,product_view,add_to_cart,checkout,purchase&window=3600` | `{steps:[{name, users, conv_from_prev, conv_from_first}]}` |
| `GET` | `/analytics/retention?cohort=day&periods=30` | `{cohorts:[{date, size, values:[...]}]}` |
| `GET` | `/analytics/realtime` | `{active_users, events_last_5m, top_pages:[...], by_country:[...]}` |
| `GET` | `/analytics/events?limit=&cursor=` | Event stream thô, phân trang bằng cursor `(event_time, event_id)` |
| `GET` | `/analytics/export?format=csv` | Stream CSV, dùng `FORMAT CSVWithNames` của ClickHouse |

### 12.3 Ops

| Method | Path | Mô tả |
|---|---|---|
| `GET` | `/healthz` | Liveness — luôn 200 nếu process sống |
| `GET` | `/readyz` | Readiness — ping ClickHouse + Kafka, 503 nếu chưa sẵn sàng |
| `GET` | `/metrics` | Prometheus |
| `GET` | `/version` | commit sha, build time |

### 12.4 Chuẩn lỗi

```json
{ "error": { "code": "invalid_range", "message": "from must be before to", "details": {} }, "request_id": "01J..." }
```

Mã lỗi: `invalid_json`, `validation_failed`, `unauthorized`, `rate_limited`, `invalid_range`, `range_too_large`, `upstream_unavailable`, `internal`.

### 12.5 Giới hạn

- Range tối đa 400 ngày; `limit` tối đa 1000.
- Rate limit: ingest 1000 req/phút/API key; analytics 120 req/phút/IP.
- Query timeout server-side: `max_execution_time = 15`, `max_memory_usage = 4GB` set qua settings mỗi query.

### 12.6 OpenAPI

Viết tay `docs/api/openapi.yaml` (hoặc `swaggo/swag` annotation) → sinh type TypeScript bằng `openapi-typescript` cho frontend. **Contract là nguồn sự thật duy nhất.**

---

## 13. Frontend design (Next.js)

### 13.1 Cấu trúc trang

| Route | Nội dung |
|---|---|
| `/` | Overview: 4 stat card + time series + top pages + device/country |
| `/realtime` | Active users (poll 5s), event stream cuộn, bản đồ/top country |
| `/pages` | Bảng đầy đủ, sort, phân trang, filter |
| `/audience` | Device, browser, OS, country, source |
| `/funnel` | Funnel builder: chọn step bằng dropdown, chart bậc thang |
| `/retention` | Heatmap cohort |
| `/settings` | Site, API key, tracking snippet copy sẵn |

### 13.2 Layout Overview

```
┌────────────────────────────────────────────────────────────────┐
│  Pulse Analytics          [Site ▾]   [Last 7 days ▾]  [⟳ 10s]  │
├────────────────────────────────────────────────────────────────┤
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐            │
│ │ Users    │ │ Sessions │ │ Events   │ │ Revenue  │            │
│ │ 12,430   │ │ 15,231   │ │ 890K     │ │ 1.2B ₫   │            │
│ │ ▲ 12.4%  │ │ ▲ 8.1%   │ │ ▼ 2.0%   │ │ ▲ 31.0%  │            │
│ └──────────┘ └──────────┘ └──────────┘ └──────────┘            │
├────────────────────────────────────────────────────────────────┤
│  Events over time                       [users|events|revenue] │
│      ╭────╮                                                    │
│  ╭───╯    ╰────╮                                               │
│ ─╯             ╰──────                                         │
├──────────────────────────────┬─────────────────────────────────┤
│ Top Pages                    │ Devices                         │
│ /products      123,423       │ Desktop ████████████ 62%        │
│ /checkout       42,312       │ Mobile  ████████     35%        │
│ /home           31,222       │ Tablet  █             3%        │
├──────────────────────────────┴─────────────────────────────────┤
│ Top Countries          │ Traffic Sources                        │
└────────────────────────────────────────────────────────────────┘
```

### 13.3 Nguyên tắc kỹ thuật FE

- **Server Component** cho lần render đầu (SEO không cần nhưng TTFB tốt), **Client Component** cho phần có filter/polling.
- URL là state: `?from=2026-08-01&to=2026-08-11&device=mobile` → share link được, back/forward hoạt động.
- TanStack Query: `staleTime` 30s cho báo cáo, 3s cho realtime; `placeholderData: keepPreviousData` để đổi khoảng thời gian không nháy trắng.
- Zod parse mọi response — API đổi mà FE không đổi thì fail rõ ràng, không `undefined` âm thầm.
- Chart: giảm điểm dữ liệu ở server (bucket theo interval), không gửi 100k điểm về browser.
- Skeleton loading + empty state + error boundary cho từng widget (một widget lỗi không làm sập cả trang).
- Number format theo locale `vi-VN`; timezone hiển thị theo site setting, query gửi UTC.
- Dark mode.
- Accessibility: chart có bảng dữ liệu ẩn cho screen reader, màu đạt contrast AA.

### 13.4 Tracking SDK (`sdk/js`)

Snippet nhúng < 2KB gzip:

```html
<script defer src="https://cdn.example.com/pulse.js" data-site="site_abc"></script>
```

Tính năng: auto page_view (kể cả SPA route change qua `history.pushState` patch), `pulse('event_name', {props})`, batch 10 event / 3s, `navigator.sendBeacon` khi `visibilitychange`, tôn trọng `navigator.doNotTrack`, retry với localStorage queue.

---

## 14. Observability

### 14.1 Metrics Prometheus (bắt buộc có)

| Metric | Type | Label |
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
| `pulse_end_to_end_lag_seconds` | histogram | (ingested_at − event_time) |

### 14.2 Grafana dashboards

1. **Ingest health**: req/s, accept vs reject vs drop, buffer size, flush duration p50/p95/p99.
2. **ClickHouse**: parts count, merge queue (`system.merges`), memory, disk, `system.query_log` p99.
3. **Kafka**: consumer lag theo partition, throughput, DLQ rate.
4. **API**: RED metrics (Rate, Errors, Duration) theo endpoint.

### 14.3 Logging

`slog` JSON, mọi log có `request_id`, `site_id`, `trace_id`. **Không log payload event** (PII) ở mức info — chỉ log ở debug khi bật cờ.

### 14.4 Alert (Alertmanager hoặc chỉ Grafana alert)

- `pulse_events_dropped_total` tăng > 0 trong 5 phút → warning
- Consumer lag > 100k trong 10 phút → critical
- ClickHouse active parts > 300 ở 1 partition → warning
- p99 analytics API > 2s trong 10 phút → warning

---

## 15. Security, privacy & multi-tenant

| Chủ đề | Quyết định |
|---|---|
| Auth ingest | `X-API-Key` per site, hash lưu trong bảng `sites` (Postgres nhỏ hoặc file config ở giai đoạn đầu) |
| Auth dashboard | JWT (HS256) hoặc session cookie httpOnly + SameSite=Lax; đơn giản: 1 user admin cho MVP |
| CORS | Ingest: cho phép origin đã đăng ký của site. Dashboard API: chỉ origin dashboard |
| Rate limit | Token bucket in-memory (Level 1) → Redis (Level 4), theo API key + IP |
| SQL injection | 100% parameterized query; `Identifier` param phải whitelist |
| PII | Không lưu IP thô; `user_id` do client sinh, khuyến nghị hash. `page` strip các query param nhạy cảm |
| GDPR-ish | `DELETE FROM events WHERE user_id = ?` bằng `ALTER TABLE ... DELETE` (mutation) — nêu rõ đây là thao tác nặng |
| TTL | Raw 180 ngày, aggregate 3 năm |
| Secrets | GitHub Actions Secrets + `.env` không commit; production dùng file `.env` chmod 600 hoặc Docker secrets |
| Container | Chạy non-root, distroless, read-only rootfs, drop capabilities |
| Bot filtering | Loại UA bot khỏi metric mặc định nhưng vẫn lưu (`device='bot'`) |

---

## 16. Testing strategy

| Tầng | Công cụ | Nội dung |
|---|---|---|
| Unit | stdlib + testify | validate, enrich, sanitize page, session stitching, tính toán delta %, chọn interval |
| Repository (integration) | testcontainers-go + ClickHouse | migration chạy được, insert batch, mỗi query cookbook có ít nhất 1 test với dữ liệu cố định |
| MV correctness | integration | insert 10k event → so sánh kết quả từ `events` và từ `events_hourly` phải khớp tuyệt đối |
| Handler | httptest | status code, partial success, rate limit, auth |
| Consumer | testcontainers Kafka + CH | at-least-once, DLQ, retry, commit sau insert |
| Load | k6 | ingest 10k ev/s trong 10 phút, p99 < 50ms, drop = 0 |
| Query perf | Go bench + `system.query_log` | mọi endpoint < 300ms ở 100M events |
| FE unit | Vitest | format số, tính %, chọn interval |
| E2E | Playwright | seed data → mở dashboard → thấy đúng số → đổi date range → số đổi |
| Contract | schemathesis / dredd trên openapi.yaml | response khớp schema |

**Coverage mục tiêu**: backend `internal/service` + `internal/repository` >= 70%. Không chạy theo con số ở tầng handler.

**Golden test cho MV** là bài test quan trọng nhất của project — nó bắt được lỗi `SimpleAggregateFunction` dùng sai mà mắt thường không thấy.

---

## 17. CI/CD

### 17.1 Chiến lược nhánh

`main` (deploy prod, protected) ← PR ← `feat/*`, `fix/*`, `chore/*`. Conventional Commits. Tag `v*` → release.

### 17.2 `ci-backend.yml`

```yaml
name: CI Backend
on:
  pull_request: { paths: ['backend/**', '.github/workflows/ci-backend.yml'] }
  push: { branches: [main], paths: ['backend/**'] }

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.27', cache-dependency-path: backend/go.sum }
      - run: go mod verify
        working-directory: backend
      - uses: golangci/golangci-lint-action@v6
        with: { version: latest, working-directory: backend }
      - run: go vet ./... && gofmt -l . | tee /dev/stderr | (! read)
        working-directory: backend

  test:
    runs-on: ubuntu-latest
    services:
      clickhouse:
        image: clickhouse/clickhouse-server:26.3
        ports: ['9000:9000', '8123:8123']
        options: >-
          --health-cmd "wget -qO- http://localhost:8123/ping || exit 1"
          --health-interval 5s --health-timeout 3s --health-retries 20
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.27' }
      - run: go run ./cmd/migrate up
        working-directory: backend
        env: { CLICKHOUSE_DSN: 'clickhouse://localhost:9000/analytics' }
      - run: go test -race -coverprofile=coverage.out -covermode=atomic ./...
        working-directory: backend
        env: { CLICKHOUSE_DSN: 'clickhouse://localhost:9000/analytics' }
      - uses: codecov/codecov-action@v4   # hoặc chỉ in ra coverage

  build:
    needs: [lint, test]
    runs-on: ubuntu-latest
    permissions: { contents: read, packages: write }
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        with: { registry: ghcr.io, username: ${{ github.actor }}, password: ${{ secrets.GITHUB_TOKEN }} }
      - uses: docker/build-push-action@v6
        with:
          context: ./backend
          push: ${{ github.event_name == 'push' }}
          tags: |
            ghcr.io/${{ github.repository }}/backend:${{ github.sha }}
            ghcr.io/${{ github.repository }}/backend:latest
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

### 17.3 `security.yml`

- `govulncheck ./...`
- `gosec ./...`
- `trivy image` trên image vừa build (fail nếu HIGH/CRITICAL)
- `npm audit --audit-level=high` cho frontend
- `gitleaks` quét secret
- Chạy theo lịch hằng tuần + trên PR

### 17.4 `cd-production.yml`

> **Lưu ý:** §17.4 và §17.5 mô tả đường "1 VPS + SSH + docker compose".
> Đường triển khai **mặc định** của dự án là AWS (Vercel + EC2 + Terraform + ECR + SSM),
> đặc tả tại [`DEPLOY-AWS.md`](DEPLOY-AWS.md) — tài liệu đó **thay thế** hai mục này.
> Chọn một trong hai đường và đánh dấu đường còn lại là `[-]` trong `TODO.md`;
> so sánh hai đường ở [`PHASES.md` §10](PHASES.md#10-phase-l6--observability-security-cd-docs).

```
push tag v* / manual dispatch
        │
        ▼
  build & push image (immutable tag = sha)
        │
        ▼
  ssh vào VPS (appleboy/ssh-action, dùng deploy key)
        │
        ├─ docker compose -f docker-compose.prod.yml pull
        ├─ docker compose run --rm migrate up      # migration TRƯỚC khi đổi app
        ├─ docker compose up -d --no-deps api consumer web
        ├─ healthcheck loop: curl /readyz 30 lần, mỗi 2s
        └─ fail ──▶ docker compose rollback (giữ tag trước trong .env.PREV_TAG)
```

Quy tắc migration: **luôn tương thích ngược một bước** (add column trước, đọc/ghi sau, xoá ở release kế tiếp). ClickHouse `ALTER TABLE ADD COLUMN` là metadata-only nên rẻ; `DROP COLUMN`/`MODIFY COLUMN` mới nặng.

### 17.5 Môi trường

| Env | Hạ tầng | Data |
|---|---|---|
| `local` | docker compose, CH 1 node, Redpanda | seed 1M events |
| `ci` | services trong GitHub Actions | fixture nhỏ |
| `staging` | VPS 2vCPU/4GB · **đường AWS: Vercel preview + dùng chung EC2** | seed 10M |
| `prod` | VPS 4vCPU/16GB SSD · **đường AWS: EC2 `r7g.xlarge` 4vCPU/32GB + EBS gp3 500GB** | thật |

> Đường AWS không dựng máy staging riêng để tiết kiệm chi phí: preview của Vercel trỏ về cùng
> backend, phân biệt bằng CORS ([`DEPLOY-AWS.md` §12](DEPLOY-AWS.md#12-cors--authentication)).

---

## 18. Benchmark plan: PostgreSQL vs ClickHouse

### 18.1 Thiết lập công bằng

- Cùng máy, cùng dataset, cùng lượng RAM cấp phát.
- Postgres schema: bảng `events` tương đương + index `(event_name, event_time)`, `BRIN` trên `event_time`, và một biến thể có **partition theo tháng**. Chạy `VACUUM ANALYZE` trước khi đo.
- Đo 3 lần, lấy trung vị. Đo cả **cold** (drop cache: `echo 3 > /proc/sys/vm/drop_caches` + `SYSTEM DROP MARK CACHE; SYSTEM DROP UNCOMPRESSED CACHE`) và **warm**.
- Ghi cả **dung lượng đĩa** — đây thường là con số gây sốc nhất.

### 18.2 Dataset

| Mức | Số event | Mục đích |
|---|---|---|
| S | 1.000.000 | smoke, chạy trên laptop |
| M | 10.000.000 | thấy khác biệt rõ |
| L | 100.000.000 | thấy ClickHouse "bay" |
| XL | 500.000.000 | tuỳ chọn, cần đĩa lớn |

Generator (`cmd/seeder`): phân phối thực tế — Zipf cho page, **device 62/35/3**, giờ cao điểm
20–22h, session 1–15 event. Bảng phân phối đầy đủ (device, country, funnel, retention) chốt tại
[`PHASES.md` §2.5](PHASES.md#25-phân-phối-dữ-liệu-của-seeder) và được `L3-02`…`L3-05` hiện thực.

### 18.3 Bảng kết quả cần điền

| Query | PG (cold/warm) | PG partitioned | ClickHouse (cold/warm) | CH + MV | Tỉ lệ |
|---|---|---|---|---|---|
| `COUNT(*)` toàn bảng | | | | | |
| `COUNT` 7 ngày gần nhất | | | | | |
| `GROUP BY country` | | | | | |
| `GROUP BY toStartOfHour` 30 ngày | | | | | |
| `uniq(user_id)` 30 ngày | | | | | |
| Top 10 pages | | | | | |
| Funnel 5 bước | | | | | |
| Retention 30 ngày | | | | | |
| **Insert 10M rows** | | | | | |
| **Dung lượng đĩa** | | | | | |

### 18.4 Kết luận cần rút ra (viết vào `docs/benchmark-results.md`)

- Vì sao column store nhanh: chỉ đọc cột cần, nén tốt hơn nhiều lần, vectorized execution, sparse index thay B-tree.
- Vì sao ClickHouse chậm/kém ở: point lookup theo primary key, UPDATE/DELETE, JOIN lớn, transaction.
- Khi nào nên dùng cái nào — kết luận bằng bảng, không bằng cảm tính.

---

## 19. Roadmap theo Level

> Ước lượng theo giờ, giả định làm part-time. Tổng **~207 giờ** (232 task).
> Chi tiết entry/exit criteria, deliverable và rủi ro từng level: [`PHASES.md`](PHASES.md).
> Checklist từng task: [`TODO.md`](TODO.md).

| Level | Nội dung | Task | Ước lượng | Milestone / Demo được gì | Tag |
|---|---|---|---|---|---|
| **L0** | Init repo, Makefile, docker-compose (CH + api), CI lint/test, health endpoint | 25 | 12h | `make up` chạy được, CI xanh | — |
| **L1** | Event schema, migration `events`, ingest 1 row/INSERT, 5 endpoint analytics cơ bản, dashboard tối giản | 40 | 30h | Gửi event từ curl → thấy số trên trang web | `v0.1.0` |
| **L2** | Học sâu ClickHouse: ORDER BY thử nghiệm, LowCardinality, codec, TTL, skip index, projection, `EXPLAIN` | 24 | 25h | `docs/clickhouse-notes.md` có số liệu so sánh thật | — |
| **L3** | Batch writer + backpressure + WAL fallback, seeder 10M–100M, benchmark insert, benchmark query, so sánh PG | 32 | 35h | Bảng benchmark điền đầy đủ | `v0.3.0` |
| **L4** | Kafka: producer, consumer group, batch, retry, DLQ, tách 2 binary, consumer lag metric | 30 | 35h | Kill ClickHouse → ingest vẫn 202 → bật lại → data về đủ | `v0.4.0` |
| **L5** | MV (hourly/daily/page_stats/sessions/first_seen), funnel, retention, cohort, revenue, realtime, dashboard đầy đủ | 46 | 45h | Dashboard giống mock, mọi query < 300ms ở 100M | — |
| **L6** | Observability đầy đủ, security hardening, CD production, runbook, README + bài viết tổng kết | 35 | 25h | Deploy thật, có domain + TLS, có Grafana | `v1.0.0` |
| | **Tổng** | **232** | **~207h** | | |
| **AWS** | Terraform + EC2 + Vercel + ECR/SSM — **thay** L6.4 (`L6-20`→`L6-28`) | 32 | 14h | Hệ thống chạy trên domain thật, có backup đã diễn tập restore | — |
| | **Tổng khi đi đường AWS** | **255** | **~214h** | | |

L0–L3 xây kiến trúc Phase 1 (§2.1); L4–L6 chuyển sang Phase 2 (§2.2).

### Thứ tự ưu tiên nếu thiếu thời gian

1. L0 → L1 → L2 → L3 — đây là phần "đắt giá" nhất về kiến thức (storage + write path + benchmark).
2. L5.1 (MV) + L5.2 (funnel) — chứng minh hiểu AggregatingMergeTree và analytical SQL.
3. Kafka (L4) có thể lùi lại; nhưng nếu mục tiêu là event-driven thì đừng bỏ.
4. Benchmark PG vs CH có thể làm gọn ở mức 10M nếu không đủ đĩa — nhớ ghi rõ mức dữ liệu.

Không được rút gọn: golden test MV vs raw (`L5-03`), test "kill ClickHouse không mất event"
(`L3-17`, `L4-23`), diễn tập restore backup (`L6-26` / `AWS-28`).
Bảng đầy đủ: [`PHASES.md` §13](PHASES.md#13-đường-tắt-khi-thiếu-thời-gian).

---

## 20. ADR — Các quyết định kiến trúc

Mỗi ADR một file trong `docs/adr/`, format: Context → Decision → Consequences → Alternatives.

| # | Quyết định | Tóm tắt lý do |
|---|---|---|
| 0001 | Không dùng ORM, viết SQL thuần | Mục tiêu học ClickHouse; ORM che execution plan, không hỗ trợ tốt combinator/MV |
| 0002 | `ORDER BY (site_id, event_name, event_time)` | Khớp workload dashboard; user_id xử lý bằng projection |
| 0003 | Batch insert client-side thay vì `async_insert` | Kiểm soát retry, đo được, học được cơ chế |
| 0004 | Kafka đặt giữa ingest và ClickHouse từ L4 | Decoupling, replay, fan-out |
| 0005 | At-least-once + dedup ở tầng query | Đơn giản hơn exactly-once, đủ chính xác cho analytics |
| 0006 | AggregatingMergeTree + MV thay vì query raw | Dashboard phải < 300ms bất kể data lớn |
| 0007 | Monorepo | Đồng bộ contract FE/BE, CI đơn giản |
| 0008 | Docker Compose thay vì Kubernetes | 1 máy chủ, 1 người; k8s là chi phí vận hành không cần thiết. Máy chủ đó là EC2 `r7g.xlarge` theo [`DEPLOY-AWS.md`](DEPLOY-AWS.md) |
| 0009 | Single-node ClickHouse, chưa Replicated | Giảm phức tạp; ghi rõ đường nâng cấp (Keeper + ReplicatedMergeTree) |
| 0010 | Trả `202 Accepted` cho ingest | Ingest không được phụ thuộc độ sẵn sàng của storage |

---

## 21. Rủi ro & cạm bẫy thường gặp

| Cạm bẫy | Dấu hiệu | Cách tránh |
|---|---|---|
| `Too many parts` | Insert bắt đầu lỗi 252 | Batch >= 10k row, <= 1 insert/s/bảng, tăng `parts_to_throw_insert` chỉ là băng dán |
| MV dùng cột non-aggregate | Số liệu sai âm thầm sau merge | Dùng `SimpleAggregateFunction`/`AggregateFunction`; golden test so sánh raw vs MV |
| Quên backfill sau khi tạo MV | Dashboard thiếu dữ liệu cũ | Luôn `INSERT ... SELECT` backfill, chú ý biên thời gian để không double-count |
| `Nullable` khắp nơi | Chậm, tốn đĩa | Dùng default value thay `Nullable` |
| `SELECT *` trên bảng column store | Đọc toàn bộ cột, chậm gấp 10 | Luôn liệt kê cột |
| `FINAL` trong query nóng | Chậm khủng khiếp | Tránh ReplacingMergeTree ở hot path |
| Timezone lẫn lộn | Số liệu lệch 7 tiếng | Lưu UTC tuyệt đối, convert ở tầng query bằng `toTimeZone`, FE hiển thị theo tz site |
| Buffer in-memory mất khi restart | Mất event khi deploy | Flush trong graceful shutdown + WAL file |
| Commit Kafka offset trước insert | Mất data khi crash | Commit sau khi ClickHouse ack |
| Consumer retry vô hạn | Lag phình, chặn partition | Giới hạn retry → DLQ → commit |
| Cardinality nổ ở MV | `events_hourly` to gần bằng raw | Chỉ đưa cột cardinality thấp vào GROUP BY của MV; page tách bảng riêng |
| Dashboard query không giới hạn range | 1 request quét cả năm, OOM | Ép max range + `max_execution_time` + `max_memory_usage` |
| Bot traffic làm sai số | DAU phồng | Phân loại bot khi enrich, mặc định lọc |
| Đĩa đầy vì raw + projection | CH dừng ghi | TTL + monitor dung lượng + alert ở 75% |

---

## 22. Definition of Done

> Bản đối chiếu theo phase (tiêu chí nào thuộc trách nhiệm level nào) ở
> [`PHASES.md` §14](PHASES.md#14-definition-of-done-toàn-dự-án).
> Bản để tick ở [`TODO.md`](TODO.md#checklist-nghiệm-thu-cuối-copy-từ-plan-22).

Project được coi là hoàn thành khi:

- [ ] `git clone && make up` → toàn bộ hệ thống chạy trong < 5 phút trên máy sạch
- [ ] `make seed N=10000000` sinh được 10M event
- [ ] Dashboard hiển thị đúng 7 nhóm widget với dữ liệu thật
- [ ] Mọi endpoint analytics p95 < 300ms ở 100M events (có ảnh chụp `system.query_log` chứng minh)
- [ ] Ingest chịu được 10.000 event/s trong 10 phút, drop = 0, p99 API < 50ms
- [ ] Kill ClickHouse 5 phút → không mất event → bật lại thì số liệu khớp 100%
- [ ] Golden test MV vs raw khớp tuyệt đối
- [ ] CI xanh: lint, unit, integration, security scan, build image
- [ ] CD deploy được lên máy chủ production bằng 1 tag, có rollback và smoke test
- [ ] Grafana có 4 dashboard, alert hoạt động
- [ ] `docs/benchmark-results.md` điền đủ bảng PG vs CH kèm kết luận
- [ ] `docs/clickhouse-notes.md` >= 20 ghi chú rút ra từ thực nghiệm (không copy tài liệu)
- [ ] README có kiến trúc, quickstart, ảnh dashboard, và phần "những gì tôi học được"

---

*Tài liệu này là bản đặc tả thiết kế. Thứ tự triển khai và tiêu chí ra từng giai đoạn nằm ở
[`PHASES.md`](PHASES.md); checklist thực thi ở [`TODO.md`](TODO.md); hạ tầng production ở
[`DEPLOY-AWS.md`](DEPLOY-AWS.md).*
