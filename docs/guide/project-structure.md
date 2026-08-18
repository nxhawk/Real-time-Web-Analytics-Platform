# Project structure

A monorepo, so the API contract and the client that consumes it move together and one CI run
covers both. The Go side follows the conventional `cmd/` + `internal/` + `pkg/` split.

## The tree

```
pulse-analytics/
├── backend/                        # Go services — one module
│   ├── cmd/                        # one directory per binary; main() only wires dependencies
│   │   ├── ingest-api/             #   write path: accepts events over HTTP        [L0 ✓]
│   │   ├── analytics-api/          #   read path: answers dashboard queries        [L0 ✓]
│   │   ├── consumer/               #   Kafka → ClickHouse sink                     [L4]
│   │   ├── migrate/                #   migration runner                            [L1]
│   │   └── seeder/                 #   synthetic event generator                   [L3]
│   ├── internal/                   # private to this module — the compiler enforces it
│   │   ├── config/                 #   the ONLY package that reads the environment [L0 ✓]
│   │   ├── logging/                #   slog setup: JSON in production, text local  [L0 ✓]
│   │   ├── version/                #   build metadata injected via -ldflags        [L0 ✓]
│   │   ├── metrics/                #   the single Prometheus registry              [L0 ✓]
│   │   ├── httpx/                  #   middleware, error envelope, server, engine  [L0 ✓]
│   │   ├── handler/                #   HTTP layer: decode → service → encode       [L0 ✓]
│   │   ├── service/                #   business rules                              [L1]
│   │   ├── repository/
│   │   │   ├── clickhouse/         #     connection, repos, queries/*.sql embedded [L1]
│   │   │   └── postgres/           #     benchmark comparison only                 [L3]
│   │   ├── model/                  #   domain types shared across layers           [L1 ✓]
│   │   ├── validate/               #   event validation rules                      [L1 ✓]
│   │   ├── buffer/                 #   batch writer, backpressure, WAL fallback    [L3]
│   │   └── kafka/                  #   producer, consumer, DLQ                     [L4]
│   ├── config/                     # development|staging|production|test .config.yml
│   ├── pkg/                        # importable from outside: geoip, uaparser wrappers
│   ├── migrations/                 # numbered goose migrations, .up.sql / .down.sql
│   ├── test/                       # integration tests (testcontainers) + fixtures
│   ├── Dockerfile                  # multi-stage → distroless, one image per SERVICE arg
│   └── .golangci.yml               # lint configuration
│
├── frontend/                       # Next.js dashboard                             [L1]
├── sdk/js/                         # pulse.js tracking snippet, < 2 KB gzip        [L5]
│
├── deploy/                         # runtime configuration, not application code
│   ├── caddy/                      #   reverse proxy + automatic TLS
│   ├── clickhouse/config.d/        #   server settings: memory, logging, Prometheus
│   ├── clickhouse/users.d/         #   profiles and quotas: query guards, readonly user
│   ├── kafka/ prometheus/ grafana/ #   the rest of the stack                       [L4, L6]
│   └── scripts/                    #   deployment helpers
│
├── infra/                          # Terraform for the AWS production path         [AWS]
├── loadtest/                       # k6 scripts + ClickHouse vs PostgreSQL benchmark
├── docs/                           # this documentation site + the API contract
│
├── docker-compose.yml              # dev stack: ClickHouse + both APIs
├── docker-compose.prod.yml         # production stack                              [L6]
├── docker-compose.bench.yml        # adds PostgreSQL for the benchmark             [L3]
├── Makefile                        # every development command — run `make help`
└── .env.example                    # secrets and per-machine overrides only
```

## Layering rules

Dependencies point one way — `cmd` → `handler` → `service` → `repository` — and never back.

| Layer | Responsibility | Must not |
|---|---|---|
| `cmd/` | Wire dependencies, start the server, handle shutdown | Contain business logic |
| `handler/` | Decode the request, call one service, encode the response | Talk to storage or build SQL |
| `service/` | Business rules: validation, enrichment, orchestration | Know it is being called over HTTP |
| `repository/` | Storage access, hand-written SQL | Contain business rules |
| `httpx/` | Transport plumbing reusable by any service | Know anything about analytics |
| `config/` | Load and validate `backend/config/*.config.yml` | Be bypassed by `os.Getenv` elsewhere |

When placing a new file, ask what it *does*, not which feature it belongs to. A funnel query
goes in `repository/clickhouse/`; the rule that a funnel has at most eight steps goes in
`service/`; turning `?steps=a,b,c` into a slice goes in `handler/`.

## Extension points

Three seams exist so later levels are additive rather than invasive.

### `handler.Prober` — readiness

Anything that can answer "are you usable right now" can be added to `/readyz` without
touching the health handler:

```go
type Prober interface {
	Name() string
	Check(ctx context.Context) error
}
```

ClickHouse implements it in Level 1, Kafka in Level 4:

```go
router := handler.NewIngestRouter(cfg, log, clickhouseConn, kafkaProducer)
```

A failing probe turns `/readyz` into `503` with the reason per dependency, while `/healthz`
stays `200` — liveness must not depend on storage, or an orchestrator restarts a healthy
process every time ClickHouse blinks.

### `httpx.Server.Run(ctx, hooks...)` — shutdown

A shutdown hook runs after the HTTP listener closes and before the process exits. That is
where the batch-writer flush belongs, so no accepted event is lost on deploy:

```go
server := httpx.NewServer(serviceName, addr, router, cfg, log)
return server.Run(ctx, batchWriter.Close, kafkaProducer.Close)
```

Hooks run even when the HTTP shutdown timed out — flushing buffered events matters more than
a few connections that refused to close.

### `config.Config` — new settings

Adding a knob is four edits, every time: a field with a `mapstructure` tag, the key in all
four files under `backend/config/`, a check in `Validate()`, and a line on the configuration
page. `Validate()` reports *every* problem it finds rather than the first, so a misconfigured
deployment can be fixed in one pass.

## Conventions

`make check` runs gofmt, `go vet`, golangci-lint (21 linters) and race-enabled tests — the
same set as CI. Beyond what a linter can see:

- Errors wrap with `%w` and carry context: `fmt.Errorf("insert batch: %w", err)`.
- Log **or** return an error, never both.
- `context.Context` is the first parameter of every function that does I/O.
- Metric and log labels use the route pattern (`c.FullPath()`), never the raw path — a raw
  path explodes cardinality.
- Table-driven tests with `t.Parallel()` are the default style.
- No `fmt.Println`, no `log.Printf`, and never an event payload in a log line.
