# Quick start

## Prerequisites

| Tool | Version | Why |
|---|---|---|
| Docker + Compose v2 | current | ClickHouse and the services run in containers |
| Go | 1.26 | Building and running the backend |
| Node.js | 22+ | The dashboard (Level 1) and this documentation site |
| `make` | any | Every command goes through the Makefile |

::: tip Why Go 1.26 and not 1.27
1.27 is still a release candidate. The project pins the latest stable release. The pin lives
in `backend/go.mod`, `backend/Dockerfile` and `.github/workflows/ci-backend.yml`; change all
three together.
:::

## Run it

```bash
git clone https://github.com/nxhawk/Real-time-Web-Analytics-Platform.git
cd Real-time-Web-Analytics-Platform

# 1. Configuration. Every variable has a working default, so an unedited copy is fine.
cp .env.example .env

# 2. Resolve Go dependencies. First time only — this writes backend/go.sum.
make deps

# 3. Start ClickHouse and both APIs, then wait until they answer.
make up
```

`make up` builds the images, starts the stack and polls `/healthz` until both services
respond. On a clean machine expect two to three minutes, most of it pulling images.

## Verify

```bash
curl localhost:8080/healthz    # {"status":"ok"}
curl localhost:8080/readyz     # {"status":"ok","checks":{}}
curl localhost:8080/version    # tag, commit, build time, Go version
curl localhost:8080/metrics    # Prometheus exposition
curl localhost:8081/healthz    # analytics API, same operational routes
```

`checks` is empty because Level 0 has no dependencies wired into readiness yet. ClickHouse
joins it in Level 1, Kafka in Level 4 — see [project structure](/guide/project-structure) for
the extension point.

Open a ClickHouse shell:

```bash
make ch-cli
# then: SELECT version()
```

## Working on the backend

Run the API on your machine instead of in a container, against the containerised ClickHouse:

```bash
make down-app   # stop the API containers, keep ClickHouse running
make run        # go run ./cmd/ingest-api
```

Before opening a pull request:

```bash
make check      # fmt + vet + lint + race-enabled tests — the same set CI runs
```

## The commands you will actually use

`make help` prints all of them.

| Command | What it does |
|---|---|
| `make up` / `make down` | Start / stop the stack |
| `make nuke` | Stop and delete the volumes — all local data is lost |
| `make ps` / `make logs` | Service health / follow logs (`make logs S=ingest-api`) |
| `make health` | Poll `/healthz` on both APIs until they answer |
| `make build` | Build binaries into `backend/bin/` |
| `make test` | Unit tests with the race detector and a coverage summary |
| `make lint` / `make fmt` / `make vet` | golangci-lint / gofmt + goimports / go vet |
| `make check` | Everything CI runs |
| `make migrate-up` | Apply migrations <Badge type="warning" text="Level 1" /> |
| `make seed N=10000000` | Generate synthetic events <Badge type="warning" text="Level 3" /> |
| `make bench` | ClickHouse vs PostgreSQL benchmark <Badge type="warning" text="Level 3" /> |
| `make tools` | Install golangci-lint and goimports |

## Running this documentation site

```bash
cd docs
npm ci
npm run dev      # http://localhost:5173
npm run build    # static output in docs/.vitepress/dist
```

The build fails on a dead internal link, which is deliberate — see
[contributing](/guide/contributing#documentation).

## Troubleshooting

**Port already in use.** Override the host ports in `.env`: `INGEST_PORT`, `ANALYTICS_PORT`,
`CLICKHOUSE_HTTP_PORT`, `CLICKHOUSE_NATIVE_PORT`.

**`make up` hangs waiting for health.** Check `make logs S=clickhouse`. On a machine with
little free memory, lower `max_server_memory_usage` in
`deploy/clickhouse/config.d/pulse.xml`.

**`missing go.sum entry`.** Run `make deps` — the repository intentionally ships `go.mod`
without `go.sum`.

**Dependency downloads fail behind a proxy.** Set `GOPROXY` in your shell before `make deps`.
