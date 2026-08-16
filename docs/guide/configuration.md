# Configuration

Configuration lives in two places, and only these two:

```
backend/config/<APP_ENV>.config.yml   committed, reviewable defaults — the source of truth
.env plus the process environment      secrets and per-machine values
```

`backend/internal/config` is the only package allowed to touch `os.Getenv`. Everything else
receives a `*config.Config` through its constructor.

## How a value gets its value

`APP_ENV` selects the file. `APP_ENV=staging` loads `backend/config/staging.config.yml`;
unset means `development`.

Inside that file, anything written as `${VAR}` or `${VAR:-fallback}` is resolved from the
environment at startup:

```yaml
ingest:
  batch_size: "${BATCH_SIZE:-5000}"   # BATCH_SIZE if set, otherwise 5000
clickhouse:
  dsn: "${CLICKHOUSE_DSN}"            # required — no fallback
```

That second form is the point of the split. A placeholder with no fallback is **mandatory**:
if the variable is unset the process refuses to start and names it. `production.config.yml`
uses it for `CLICKHOUSE_DSN` and `CORS_ALLOWED_ORIGINS`, so a production deployment can never
quietly inherit a localhost default and look healthy while serving nothing.

```
invalid configuration: unset environment variables with no fallback:
CLICKHOUSE_DSN (required by clickhouse.dsn); CORS_ALLOWED_ORIGINS (required by http.cors_allowed_origins)
```

`.env` is loaded first, so it can supply those variables without exporting anything into your
shell. A real environment variable always wins over `.env`. A missing `.env` is normal — in a
container there is none — but a malformed one is a startup error.

::: tip Where the files are found
The loader walks up from the working directory looking for `config/`, which is why
`make run` works from `backend/` and a bare `go test ./...` works from anywhere in the tree.
`CONFIG_DIR` overrides it; the Docker image sets `CONFIG_DIR=/config` because the binary runs
from `/` with no source tree around it. `ENV_FILE` does the same for `.env`.
:::

## Precedence

Four layers decide a single value. Highest first:

| # | Source | Wins over |
|---|---|---|
| 1 | A real environment variable | everything below |
| 2 | The same variable in `.env` | the fallback and the literal |
| 3 | The fallback after `:-` in the YAML | the nothing that is left |
| 4 | A plain literal in the YAML | — it is absolute, see below |

In load order, the process does this:

```
loadDotEnv()           .env is read first — godotenv never overwrites a variable that is
                       already set, which is what makes layer 1 beat layer 2
      ▼
os.Getenv("APP_ENV")   read after .env, so .env can select the file too
      ▼
config/<APP_ENV>.config.yml
      ▼
expandEnvVars()        ${VAR} → os.LookupEnv; unset falls back to the text after :-;
                       unset with no :- is an error naming the variable
      ▼
Unmarshal → app.env must match the file name → Validate()
```

::: danger An environment variable only works if the YAML asks for it
This is the one behaviour that changed when configuration moved out of `env` struct tags.
A key written as a plain literal cannot be overridden at all:

```yaml
ingest:
  batch_size: 5000                       # BATCH_SIZE=100 does nothing
  batch_size: "${BATCH_SIZE:-5000}"      # BATCH_SIZE=100 works
```

If a knob should be tunable per deployment, it needs the placeholder — in **all four** files.
:::

### Required here, optional there

Layer 3 is what makes the split worth having: the same key can be mandatory in one
environment and defaulted in another, with no extra Go code.

```yaml
# development.config.yml — a fresh clone runs with no .env at all
dsn: "${CLICKHOUSE_DSN:-clickhouse://pulse:pulse@localhost:9000/analytics}"

# production.config.yml — the process refuses to start without it
dsn: "${CLICKHOUSE_DSN}"
```

### Where to set what

Two different questions, two different answers.

**A value that differs per environment but is not a secret** belongs in the corresponding
`*.config.yml`. That is what having four files buys: `log.level` is `debug` in staging and
`info` in production because the files say so, in a diff someone reviewed.

**A secret or a per-machine value** gets a `${VAR}` placeholder and arrives through the
environment:

| Environment | How the variable is supplied |
|---|---|
| Local development | `.env` at the repository root |
| Docker Compose | the `environment:` block in `docker-compose.yml` (`x-api-env`) |
| CI | `env:` in the workflow, or GitHub secrets |
| A server with systemd | `EnvironmentFile=/etc/pulse/pulse.env` |
| Plain Docker | `docker run --env-file /etc/pulse/pulse.env` |
| Kubernetes | `envFrom:` pointing at a Secret or ConfigMap |

::: warning Two `${VAR}` systems that look identical
`docker-compose.yml` has substitution of its own, and it is **not** the one described on this
page. Compose expands `${VAR}` before a container starts, reading the `.env` next to
`docker-compose.yml`. The Go loader expands `${VAR}` inside the container, reading that
container's environment.

So `CLICKHOUSE_PASSWORD` in `.env` travels in two hops: Compose reads it and puts
`CLICKHOUSE_DSN` into the container's environment, then `expandEnvVars` resolves the
placeholder in `production.config.yml` from there. A variable that Compose never forwards
does not reach the Go process, however correct it looks in `.env`.
:::

## Validation

Configuration is validated at startup, and `Validate()` collects **all** problems before
returning. Messages name the YAML key and the environment variable that feeds it:

```
invalid configuration in config/development.config.yml:
app.env (APP_ENV) must be one of development|staging|production|test, got "prod";
log.level (LOG_LEVEL) must be one of debug|info|warn|error, got "verbose";
ingest.batch_size (BATCH_SIZE) must be greater than 0
```

The process exits rather than starting in a half-configured state. `app.env` must also match
the file it was loaded from, which catches a file copied to the wrong name.

## The four files

| `APP_ENV` | File | What is different |
|---|---|---|
| `development` *(default)* | `development.config.yml` | Everything has a fallback: a fresh clone runs with no `.env` at all. `log.format` is `text` |
| `test` | `test.config.yml` | Small batches and short timeouts so tests do not wait for a 5000-row batch. Also what the CI smoke test boots |
| `staging` | `staging.config.yml` | `clickhouse.dsn` is required. `log.level` defaults to `debug`, CORS to `*` |
| `production` | `production.config.yml` | `clickhouse.dsn` **and** `cors_allowed_origins` are required. `log.format` is `json` |

## Application

| YAML key | Variable | Development default | Description |
|---|---|---|---|
| `app.env` | `APP_ENV` | `development` | Selects the file. Production and test put gin in release mode |
| `app.shutdown_timeout` | `SHUTDOWN_TIMEOUT` | `30s` | How long in-flight requests get to finish after `SIGTERM` |

## HTTP

| YAML key | Variable | Development default | Description |
|---|---|---|---|
| `http.ingest_addr` | `HTTP_ADDR` | `:8080` | Listen address of the ingest API |
| `http.analytics_addr` | `ANALYTICS_ADDR` | `:8081` | Listen address of the analytics API |
| `http.read_header_timeout` | `HTTP_READ_HEADER_TIMEOUT` | `5s` | Slowloris protection |
| `http.read_timeout` | `HTTP_READ_TIMEOUT` | `15s` | |
| `http.write_timeout` | `HTTP_WRITE_TIMEOUT` | `30s` | Raise this before adding CSV export streaming in Level 5 |
| `http.idle_timeout` | `HTTP_IDLE_TIMEOUT` | `60s` | |
| `http.max_body_bytes` | `HTTP_MAX_BODY_BYTES` | `1048576` | 1 MiB, matching the API contract |
| `http.cors_allowed_origins` | `CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | Comma-separated |

::: warning CORS edge cases
Empty means **no** cross-origin access — the middleware becomes a no-op and emits no CORS
headers rather than crashing. A single `*` allows every origin and automatically disables
credentials, because a browser rejects that combination anyway.
:::

## Logging

| YAML key | Variable | Development default | Description |
|---|---|---|---|
| `log.level` | `LOG_LEVEL` | `info` | `debug` · `info` · `warn` · `error` |
| `log.format` | `LOG_FORMAT` | `text` | `json` in staging and production, `text` for readable local output |

Every line carries `service`, `env` and `request_id`. Request logs use the route pattern, not
the raw path. `/healthz`, `/readyz` and `/metrics` are not logged — they are polled every few
seconds and would drown out real traffic.

## ClickHouse

| YAML key | Variable | Development default | Description |
|---|---|---|---|
| `clickhouse.dsn` | `CLICKHOUSE_DSN` | `clickhouse://pulse:pulse@localhost:9000/analytics` | **Native protocol, port 9000.** Never the HTTP port for inserts. Required in staging and production |
| — | `CLICKHOUSE_PASSWORD` | `pulse` | Used by Docker Compose when creating the user, not read by the Go code |
| `clickhouse.max_open_conns` | `CLICKHOUSE_MAX_OPEN_CONNS` | `16` | |
| `clickhouse.max_idle_conns` | `CLICKHOUSE_MAX_IDLE_CONNS` | `8` | |
| `clickhouse.conn_max_lifetime` | `CLICKHOUSE_CONN_MAX_LIFETIME` | `10m` | |
| `clickhouse.dial_timeout` | `CLICKHOUSE_DIAL_TIMEOUT` | `5s` | |
| `clickhouse.query_timeout` | `CLICKHOUSE_QUERY_TIMEOUT` | `15s` | Mirrors the server-side `max_execution_time` |

Inside Docker Compose the host is `clickhouse`; from your machine it is `localhost`.

## Ingest and the write path

| YAML key | Variable | Development default | Description |
|---|---|---|---|
| `ingest.sink` | `SINK` | `direct` | `direct` writes to ClickHouse; `kafka` produces to a topic (Level 4+) |
| `ingest.insert_mode` | `INSERT_MODE` | `batch` | `single` exists only to give the Level 3 benchmark a baseline |
| `ingest.batch_size` | `BATCH_SIZE` | `5000` | Rows per insert |
| `ingest.flush_interval_ms` | `FLUSH_INTERVAL_MS` | `500` | Maximum wait before a partial batch is flushed |
| `ingest.buffer_size` | `BUFFER_SIZE` | `100000` | Queue between the HTTP handler and the writer. Must be ≥ `batch_size` |
| `ingest.workers` | `INGEST_WORKERS` | `4` | Batch-writer workers |
| `ingest.wal_dir` | `WAL_DIR` | `./data/wal` | Where batches land when ClickHouse cannot be reached |
| `ingest.max_events_per_request` | `MAX_EVENTS_PER_REQUEST` | `500` | Capped at 500 by the API contract |
| `ingest.rate_limit_per_min` | `INGEST_RATE_LIMIT_PER_MIN` | `1000` | Per API key |

::: tip These defaults are provisional
`batch_size` and `flush_interval_ms` are re-tuned in Level 3 from measured throughput, part
number and p99 latency. When the measurement says something different, change the value in
`PHASES.md` §2.4 first, then propagate to `backend/config/*.config.yml` and this page.
:::

## Kafka

Unused until Level 4. Leave `KAFKA_BROKERS` empty until then — setting `sink: kafka` without
brokers is a startup error, on purpose.

| YAML key | Variable | Development default | Description |
|---|---|---|---|
| `kafka.brokers` | `KAFKA_BROKERS` | *(empty)* | Comma-separated broker list |
| `kafka.topic_raw` | `KAFKA_TOPIC_RAW` | `events.raw` | 6 partitions, 7-day retention, zstd |
| `kafka.topic_dlq` | `KAFKA_TOPIC_DLQ` | `events.dlq` | 1 partition, 30-day retention |
| `kafka.group_id` | `KAFKA_GROUP_ID` | `clickhouse-sink` | Consumer group |
| `kafka.batch_size` | `KAFKA_CONSUMER_BATCH_SIZE` | `10000` | Records per poll before an insert |

## Host ports for Docker Compose

Change these when a port is already taken: `INGEST_PORT`, `ANALYTICS_PORT`,
`CLICKHOUSE_HTTP_PORT`, `CLICKHOUSE_NATIVE_PORT`. They are read by `docker-compose.yml`, not
by the Go code, so they have no YAML counterpart.

## Secrets

Secrets come from the environment or the deployment secret store, never from source. The YAML
files are committed precisely because they contain no secrets — only defaults and
placeholders. In production `.env` is created on the host with mode `600` and is never
committed; `gitleaks` runs on every pull request from Level 6 onwards.

## Adding a new setting

`backend/internal/config` holds one file per configuration section — `app.go`, `http.go`,
`log.go`, `clickhouse.go`, `ingest.go`, `kafka.go` — so a setting is declared and validated in
the same place. `config.go` holds only the `Config` aggregate and the rules that span two
sections; `load.go` and `expand.go` hold the file lookup and the `${VAR}` resolution.

1. Add the field to the section file it belongs to, with a `mapstructure` tag.
2. Add the key to **all four** files in `backend/config/`, with a `${VAR:-fallback}`
   placeholder if it should be overridable. A key present in the struct but missing from a
   file decodes to the zero value — `TestShippedFilesLoad` is what catches that.
3. Add the check to that section's `validate` method — not to `Config.Validate`, which owns
   only cross-section rules. Every setting that can be wrong should say so at startup.
4. Document it on this page and, if it is one people will actually override, in
   `.env.example`.

A whole new section is the same shape: add `<name>.go` with the struct and its `validate`,
add the field to `Config`, and list it in `Config.sections`. Nothing else changes.
