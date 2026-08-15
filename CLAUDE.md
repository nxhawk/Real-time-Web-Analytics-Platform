# CLAUDE.md

Guidance for Claude Code (and any AI coding agent) working in this repository.

---

## 0. Language rule — non-negotiable

**Everything written into the codebase must be in English.**

This applies to:

- Source code comments (Go, TypeScript, SQL, YAML, Dockerfile, shell)
- Identifiers: variable, function, type, package, table, and column names
- Log messages, error strings, and metric help texts
- API responses: error codes, messages, and field names
- Commit messages, branch names, pull request titles and bodies
- Documentation files, README sections, ADRs, and code-block annotations
- Test names and test failure messages
- UI copy in the dashboard (user-facing i18n strings live in locale files, not inline)

The planning documents `PLAN.md`, `PHASES.md`, `TODO.md`, and `DEPLOY-AWS.md` are written in
Vietnamese and stay that way — do not translate them unless explicitly asked.
`README.md` and this file are in English. When implementing a task
described in Vietnamese, **translate the intent into English code and English comments.**

```go
// GOOD: Flush the buffer when it is full or the flush interval elapses.
// BAD:  Đẩy buffer khi đầy hoặc hết thời gian flush.
```

---

## 1. Project context

**Pulse Analytics** — a self-hosted, real-time web analytics platform (a scaled-down Google
Analytics). Go/Gin ingest and query APIs, ClickHouse storage, Kafka event pipeline, Next.js
dashboard.

**Level 0 is complete.** The repository has a running backend skeleton — configuration,
logging, middleware, operational endpoints, Docker, Makefile and CI — but no event pipeline
yet. Build features by following the phase plan in `PHASES.md` and the level-by-level
checklist in `TODO.md`, and mark tasks `[x]` there as you finish them.

Module path: `github.com/nxhawk/pulse-analytics/backend`. Go is pinned to **1.26**.

### Source of truth, in priority order

1. `DEPLOY-AWS.md` — deployment only (Vercel + a single EC2 host). It supersedes
   `PLAN.md` §17.4–17.5 and `TODO.md` L6.4.
2. `PLAN.md` — the technical specification. Architecture, schemas, DDL, API contract, and
   design decisions live here. If code and `PLAN.md` disagree, `PLAN.md` wins unless the user
   says otherwise.
3. `PHASES.md` — delivery order, entry/exit criteria per phase, and **§2 "Bảng số liệu chuẩn"**,
   the canonical table for any number that appears in more than one document (tool versions,
   performance thresholds, API limits, seeder distributions). Change a shared number there
   first, then propagate.
4. `TODO.md` — the task list and acceptance criteria ("Done khi" = "Done when").
5. `docs/api/openapi.yaml` — the API contract once it exists. Frontend types are generated
   from it; never hand-edit generated types.

When a task's implementation reveals that a document is wrong, fix the document in the same
pull request. Never leave code that silently disagrees with the spec.

---

## 2. Repository structure

```
backend/
├── cmd/                    one directory per binary; main() only wires dependencies
│   ├── ingest-api/           write path: accepts events over HTTP           [L0 ✓]
│   ├── analytics-api/        read path: answers dashboard queries           [L0 ✓]
│   ├── consumer/             Kafka → ClickHouse sink                        [L4]
│   ├── migrate/              migration runner                               [L1]
│   └── seeder/               synthetic event generator                      [L3]
├── internal/               private to this module — the compiler enforces it
│   ├── config/               the ONLY package that reads the environment    [L0 ✓]
│   ├── logging/              slog setup: JSON in production, text locally   [L0 ✓]
│   ├── version/              build metadata injected via -ldflags           [L0 ✓]
│   ├── metrics/              the single Prometheus registry                 [L0 ✓]
│   ├── httpx/                middleware, error envelope, server, gin engine [L0 ✓]
│   ├── handler/              HTTP layer: decode → call service → encode     [L0 ✓]
│   ├── service/              business rules: validation, enrichment         [L1]
│   ├── repository/
│   │   ├── clickhouse/       connection, repos, queries/*.sql via go:embed  [L1]
│   │   └── postgres/         benchmark comparison only                      [L3]
│   ├── model/                domain types shared across layers              [L1]
│   ├── validate/             event validation rules                         [L1]
│   ├── buffer/               batch writer, backpressure, WAL fallback       [L3]
│   └── kafka/                producer, consumer, DLQ                        [L4]
├── pkg/                    importable from outside: geoip, uaparser wrappers
├── migrations/             numbered goose migrations, .up.sql / .down.sql
└── test/                   integration tests (testcontainers) + fixtures

deploy/                     runtime configuration: caddy, clickhouse, kafka, prometheus, grafana
infra/                      Terraform for the AWS production path
loadtest/                   k6 scripts + ClickHouse vs PostgreSQL benchmark
frontend/  sdk/js/          Next.js dashboard, pulse.js tracking snippet

docs/                    VitePress documentation site (bilingual) + the API contract
├── .vitepress/config/     shared.mts (base, search), en.mts, vi.mts (nav + sidebars)
├── guide/ reference/      English pages — root locale, served without a URL prefix
├── notes/ adr/            engineering notes (L2, L3, L6) and decision records
├── vi/                    Vietnamese mirror — same tree, same filenames
├── public/                static assets; openapi.yaml is copied here at build time
└── api/openapi.yaml       API CONTRACT — the single source of truth
```

### Rules for the documentation site

- **Every page exists in both languages.** Adding `docs/guide/x.md` means also adding
  `docs/vi/guide/x.md`, then registering both in `.vitepress/config/en.mts` and `vi.mts`.
  A sidebar entry pointing at a missing file breaks the build.
- English pages are the root locale (no `/en/` prefix); Vietnamese lives under `/vi/`.
- `npm run build` fails on a dead internal link. Run it before claiming a docs change works.
- Do not restate `PLAN.md` in the site — link to it. A restatement drifts out of date.
- Mark anything not built yet with `<Badge type="warning" text="Level 3" />`.
- The site deploys from `main` via `.github/workflows/docs.yml`; the `base` path in
  `shared.mts` must match the repository name or every asset 404s.

### Layering rules — these are not negotiable

Dependencies point one way: `cmd` → `handler` → `service` → `repository`. Never back.

| Layer | Responsibility | Must not |
|---|---|---|
| `cmd/` | Wire dependencies, start the server, handle shutdown | Contain business logic |
| `handler/` | Decode the request, call one service, encode the response | Talk to storage or build SQL |
| `service/` | Business rules: validation, enrichment, orchestration | Know it is being called over HTTP |
| `repository/` | Storage access, hand-written SQL | Contain business rules |
| `httpx/` | Transport plumbing reusable by any service | Know anything about analytics |
| `config/` | Read and validate the environment | Be bypassed by `os.Getenv` elsewhere |

Placing a new file: ask what it *does*, not what feature it belongs to. A funnel query goes in
`repository/clickhouse/`, the rule that a funnel has at most 8 steps goes in `service/`, and
the code that turns `?steps=a,b,c` into a slice goes in `handler/`.

### Extension points already in place

Use these instead of restructuring:

- **`handler.Prober`** — anything with `Name() string` and `Check(ctx) error` can be passed to
  `NewIngestRouter` / `NewAnalyticsRouter` and appears in `/readyz`. This is how ClickHouse
  (task L1-13) and Kafka (Level 4) get wired in.
- **`httpx.Server.Run(ctx, hooks...)`** — a shutdown hook runs after the listener closes and
  before the process exits. This is where the batch-writer flush belongs (task L3-12), so no
  accepted event is lost on deploy.
- **`config.Config`** — add a field with a `mapstructure` tag, add the key to all four files
  in `backend/config/`, extend `Validate()`, then document it in the configuration guide.
  All four, every time. A placeholder written `${VAR}` with no `:-fallback` is mandatory at
  startup; that is how production requires its secrets.
- **`metrics.Registry`** — register new collectors here. Never create a second registry and
  never use `prometheus.DefaultRegisterer`.
- **`handler.APIBasePath`** — mount new routes on the `/api/v1` group. A breaking API change
  means a second group, not an edit to this one.

### Conventions the linter enforces

`make check` runs gofmt, `go vet`, golangci-lint and the race-enabled tests — the same set as
CI. Beyond that:

- Errors wrap with `%w` and carry context: `fmt.Errorf("insert batch: %w", err)`.
- Log **or** return an error, never both.
- `context.Context` is the first parameter of every function that does I/O.
- Metric and log labels use the route pattern (`c.FullPath()`), never the raw path — a raw
  path explodes cardinality.
- Table-driven tests with `t.Parallel()` are the default style.
- No `fmt.Println`, no `log.Printf`, no event payloads in logs.

---

## 3. Security and privacy

- **Raw IP addresses are never stored.** Use them for GeoIP enrichment, then discard (store a
  hash only if a specific requirement demands it).
- Strip sensitive query parameters (`token`, `email`, `password`) from the `page` field before
  storage.
- Every ingest and analytics endpoint requires an API key; the API key determines `site_id`,
  and `site_id` is enforced in every query — no cross-tenant reads.
- Secrets come from the environment or the deployment secret store, never from source.
- Guard every analytics query with `max_execution_time` and `max_memory_usage` settings.

---

## 4. Performance targets

Treat these as hard requirements, not aspirations:

| Target | Value |
|---|---|
| Dashboard query latency | < 300 ms at 100M events |
| Ingest availability | `202` returned even when ClickHouse is unavailable |
| Insert path | Batched — `BATCH_SIZE` rows or `FLUSH_INTERVAL_MS`, whichever comes first |
| Ingest rate limit | 1000 req/min per API key |
| Analytics rate limit | 120 req/min per IP |

When a change could affect the write path or query latency, run `make bench` and record the
numbers in `docs/benchmark-results.md`.

---

## 5. Working on a task

1. Read the task in `TODO.md` and the phase it belongs to in `PHASES.md` (entry criteria,
   exit criteria, deliverables).
2. Read the referenced `PLAN.md` section before writing code. The DDL, query shapes and API
   contract are already decided there.
3. Implement, following the layering rules in section 2.
4. Add or update tests. `make check` must be clean.
5. Tick the task `[x]` in `TODO.md`, and fix any document the implementation contradicted.
6. Commit with Conventional Commits, referencing the task ID: `feat(ingest): accept batches
   of up to 500 events` with `Closes L1-17` in the pull request body.

Shared numbers — tool versions, performance thresholds, API limits, seeder distributions —
are owned by [`PHASES.md` §2](PHASES.md#2-bảng-số-liệu-chuẩn). Change them there first, then
propagate to `PLAN.md`, `README.md`, `backend/config/*.config.yml` and the code. Never change
one in isolation.
