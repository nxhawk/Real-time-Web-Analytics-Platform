# Configuration

Everything is read from the environment by `backend/internal/config`, which is the only
package allowed to touch `os.Getenv`. A local `.env` file is loaded first when present; real
environment variables always win over it.

Every variable has a default that works for local development, so an unedited
`cp .env.example .env` is a valid setup.

## Validation

Configuration is validated at startup, and `Validate()` collects **all** problems before
returning:

```
invalid configuration: APP_ENV must be one of development|staging|production|test, got "prod";
LOG_LEVEL must be one of debug|info|warn|error, got "verbose"; BATCH_SIZE must be greater than 0
```

The process exits rather than starting in a half-configured state.

## Application

| Variable | Default | Description |
|---|---|---|
| `APP_ENV` | `development` | `development` · `staging` · `production` · `test`. Production and test put gin in release mode |
| `SHUTDOWN_TIMEOUT` | `30s` | How long in-flight requests get to finish after `SIGTERM` |

## HTTP

| Variable | Default | Description |
|---|---|---|
| `HTTP_ADDR` | `:8080` | Listen address of the ingest API |
| `ANALYTICS_ADDR` | `:8081` | Listen address of the analytics API |
| `HTTP_READ_HEADER_TIMEOUT` | `5s` | Slowloris protection |
| `HTTP_READ_TIMEOUT` | `15s` | |
| `HTTP_WRITE_TIMEOUT` | `30s` | Raise this before adding CSV export streaming in Level 5 |
| `HTTP_IDLE_TIMEOUT` | `60s` | |
| `HTTP_MAX_BODY_BYTES` | `1048576` | 1 MiB, matching the API contract |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | Comma-separated |

::: warning CORS edge cases
Empty means **no** cross-origin access — the middleware becomes a no-op and emits no CORS
headers rather than crashing. A single `*` allows every origin and automatically disables
credentials, because a browser rejects that combination anyway.
:::

## Logging

| Variable | Default | Description |
|---|---|---|
| `LOG_LEVEL` | `info` | `debug` · `info` · `warn` · `error` |
| `LOG_FORMAT` | `json` | `json` in production, `text` for readable local output |

Every line carries `service`, `env` and `request_id`. Request logs use the route pattern, not
the raw path. `/healthz`, `/readyz` and `/metrics` are not logged — they are polled every few
seconds and would drown out real traffic.

## ClickHouse

| Variable | Default | Description |
|---|---|---|
| `CLICKHOUSE_DSN` | `clickhouse://pulse:pulse@localhost:9000/analytics` | **Native protocol, port 9000.** Never the HTTP port for inserts |
| `CLICKHOUSE_PASSWORD` | `pulse` | Used by Docker Compose when creating the user |
| `CLICKHOUSE_MAX_OPEN_CONNS` | `16` | |
| `CLICKHOUSE_MAX_IDLE_CONNS` | `8` | |
| `CLICKHOUSE_CONN_MAX_LIFETIME` | `10m` | |
| `CLICKHOUSE_DIAL_TIMEOUT` | `5s` | |
| `CLICKHOUSE_QUERY_TIMEOUT` | `15s` | Mirrors the server-side `max_execution_time` |

Inside Docker Compose the host is `clickhouse`; from your machine it is `localhost`.

## Ingest and the write path

| Variable | Default | Description |
|---|---|---|
| `SINK` | `direct` | `direct` writes to ClickHouse; `kafka` produces to a topic (Level 4+) |
| `INSERT_MODE` | `batch` | `single` exists only to give the Level 3 benchmark a baseline |
| `BATCH_SIZE` | `5000` | Rows per insert |
| `FLUSH_INTERVAL_MS` | `500` | Maximum wait before a partial batch is flushed |
| `BUFFER_SIZE` | `100000` | Queue between the HTTP handler and the writer. Must be ≥ `BATCH_SIZE` |
| `INGEST_WORKERS` | `4` | Batch-writer workers |
| `WAL_DIR` | `./data/wal` | Where batches land when ClickHouse cannot be reached |
| `MAX_EVENTS_PER_REQUEST` | `500` | Capped at 500 by the API contract |
| `INGEST_RATE_LIMIT_PER_MIN` | `1000` | Per API key |

::: tip These defaults are provisional
`BATCH_SIZE` and `FLUSH_INTERVAL_MS` are re-tuned in Level 3 from measured throughput, part
number and p99 latency. When the measurement says something different, change the value in
`PHASES.md` §2.4 first, then propagate to the code, `.env.example` and this page.
:::

## Kafka

Unused until Level 4. Leave `KAFKA_BROKERS` empty until then — setting `SINK=kafka` without
brokers is a startup error, on purpose.

| Variable | Default | Description |
|---|---|---|
| `KAFKA_BROKERS` | *(empty)* | Comma-separated broker list |
| `KAFKA_TOPIC_RAW` | `events.raw` | 6 partitions, 7-day retention, zstd |
| `KAFKA_TOPIC_DLQ` | `events.dlq` | 1 partition, 30-day retention |
| `KAFKA_GROUP_ID` | `clickhouse-sink` | Consumer group |
| `KAFKA_CONSUMER_BATCH_SIZE` | `10000` | Records per poll before an insert |

## Host ports for Docker Compose

Change these when a port is already taken: `INGEST_PORT`, `ANALYTICS_PORT`,
`CLICKHOUSE_HTTP_PORT`, `CLICKHOUSE_NATIVE_PORT`.

## Secrets

Secrets come from the environment or the deployment secret store, never from source. In
production `.env` is created on the host with mode `600` and is never committed; `gitleaks`
runs on every pull request from Level 6 onwards.

## Adding a new setting

1. Add the field to the right struct in `backend/internal/config/config.go`, with an `env`
   tag and an `envDefault`.
2. Add a check to `Validate()`. Every setting that can be wrong should say so at startup.
3. Document it in `.env.example` **and** on this page.
