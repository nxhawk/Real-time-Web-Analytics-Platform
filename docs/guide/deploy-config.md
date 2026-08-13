# Deploy configuration

<Badge type="tip" text="Level 0 / local stack" />

This page explains, line by line, what lives in [`deploy/`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/tree/main/deploy)
and what every block of [`docker-compose.yml`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/docker-compose.yml)
actually does — and, where a value is not obvious, why it was chosen and what breaks if you
change it.

[Configuration](./configuration) covers the variables the Go code reads. This page covers the
layer underneath: the containers, the mounts and the ClickHouse server files.

## What is in `deploy/`

`deploy/` holds the configuration that is **mounted into third-party containers**. It is not
application code and it is never compiled — the files land inside a container at a path where
that image already looks for them.

```
deploy/
├── caddy/                     .gitkeep   → Level 6: Caddyfile, TLS termination
├── clickhouse/
│   ├── config.d/pulse.xml     ✔ active   → server-level overrides
│   └── users.d/pulse.xml      ✔ active   → users, profiles, quotas
├── grafana/dashboards/        .gitkeep   → Level 6: provisioned dashboards
├── kafka/                     .gitkeep   → Level 4: broker + topic settings
├── prometheus/                .gitkeep   → Level 6: prometheus.yml, scrape jobs
└── scripts/                   .gitkeep   → deploy / backup / restore helpers
```

Only the two ClickHouse files are live today. The rest are `.gitkeep` placeholders that
reserve the layout so a later level adds a file instead of inventing a folder structure — the
same reason `docker-compose.yml` already names the ports Kafka and Grafana will take.

::: tip Why the split into `config.d/` and `users.d/`
The ClickHouse image reads `/etc/clickhouse-server/config.xml` and `users.xml`, then merges
every `*.xml` under `config.d/` and `users.d/` on top, in alphabetical order. Dropping a small
overlay file in is therefore safe across image upgrades; editing the base `config.xml` is not,
because the next image tag ships its own copy.
:::

## `docker-compose.yml`, block by block

The file describes the **development** stack: ClickHouse plus the two Go services. Kafka
arrives in Level 4, the Next.js dashboard in Level 1, Prometheus and Grafana in Level 6.

### Project name

```yaml
name: pulse
```

Sets the Compose project name explicitly instead of letting it default to the folder name.
Everything Compose creates is prefixed with it: the network is `pulse_default`, the volumes
are `pulse_clickhouse-data` and `pulse_clickhouse-logs`. Rename the checkout folder and your
data still comes back, because the volume name did not move.

### The three YAML anchors

Compose supports YAML anchors (`&name`) and merge keys (`<<: *name`). Keys starting with `x-`
are extension fields: Compose ignores them, so they are a legal place to park a fragment that
is reused later.

```yaml
x-api-common: &api-common     # runtime and security settings shared by both APIs
x-api-env:    &api-env        # environment variables shared by both APIs
x-build-args: &build-args     # build arguments shared by both images
```

The point is that `ingest-api` and `analytics-api` cannot drift apart. Harden one and you have
hardened both, because there is one definition, not two copies.

#### `x-api-common`

| Key | Value | What it does |
|---|---|---|
| `restart` | `unless-stopped` | Restart on crash and on Docker daemon start, but stay down after an explicit `docker compose stop` |
| `depends_on.clickhouse.condition` | `service_healthy` | Do not start the API until ClickHouse's healthcheck passes — not merely until its container exists |
| `security_opt` | `no-new-privileges:true` | Blocks `setuid` escalation inside the container |
| `read_only` | `true` | The container root filesystem is mounted read-only |
| `cap_drop` | `ALL` | Drops every Linux capability. A Go HTTP server on a port above 1024 needs none |
| `logging` | `json-file`, `10m` × 3 | Caps log growth at ~30 MB per container instead of filling the disk |

`depends_on: service_healthy` is what makes `make up` deterministic. Without it, both APIs
would race ClickHouse's ~20 s startup, fail their first connection and restart-loop until it
happened to win.

::: warning `read_only: true` and the write-ahead log
A read-only root filesystem means the process cannot write anywhere unless a volume or tmpfs
is mounted there. `WAL_DIR` defaults to `./data/wal` (Level 3), and there is no mount for it
in this file. When the WAL path is exercised, give the service either
`tmpfs: [/tmp]` plus `WAL_DIR=/tmp/wal`, or a named volume — otherwise the first fallback
write fails with `read-only file system`.
:::

#### `x-api-env`

```yaml
APP_ENV:              ${APP_ENV:-development}
LOG_LEVEL:            ${LOG_LEVEL:-info}
LOG_FORMAT:           ${LOG_FORMAT:-json}
CLICKHOUSE_DSN:       clickhouse://pulse:${CLICKHOUSE_PASSWORD:-pulse}@clickhouse:9000/analytics
CORS_ALLOWED_ORIGINS: ${CORS_ALLOWED_ORIGINS:-http://localhost:3000}
```

`${VAR:-default}` reads `VAR` from your shell or from the `.env` file next to
`docker-compose.yml`, falling back to the literal after `:-`. So the stack starts with no
`.env` at all, and `cp .env.example .env` only changes what you actually edited.

Two details worth internalising:

- The DSN host is **`clickhouse`**, the service name, resolved by Compose's internal DNS. From
  your own machine the same database is `localhost` — that is the single most common cause of
  `dial tcp: connection refused` when a command is run in the wrong place.
- Port **9000** is the native protocol. `8123` is HTTP, for `clickhouse-client` and ad-hoc
  debugging. Inserts always go over the native protocol.

Note what is *not* here: `BATCH_SIZE`, `INGEST_WORKERS`, `SINK` and the rest are absent, so
each service uses the defaults compiled into `internal/config`. Override them per environment
in `.env`, not by editing this file.

#### `x-build-args`

```yaml
COMMIT:  ${GIT_COMMIT:-dev}
VERSION: ${GIT_TAG:-dev}
```

The Makefile exports both:

```make
export GIT_COMMIT := $(shell git rev-parse --short HEAD)
export GIT_TAG    := $(shell git describe --tags --always --dirty)
```

They are passed to the Dockerfile as `ARG`s and stamped into the binary via
`-ldflags -X .../internal/version.Tag=...`, which is what `/healthz` reports back. Build the
image with plain `docker compose build` instead of `make up` and you get `dev` — the image
still works, it just cannot tell you which commit it is.

### Service: `clickhouse`

```yaml
image: clickhouse/clickhouse-server:26.3-alpine
```

Pinned to a minor version, not `latest`. ClickHouse changes defaults between releases —
`background_pool_size` semantics in the config file below are version-specific — so an
unpinned tag means today's stack and tomorrow's stack are different systems.

**Environment.** `CLICKHOUSE_DB`, `CLICKHOUSE_USER` and `CLICKHOUSE_PASSWORD` are read by the
image entrypoint on **first boot only**: it creates the database and the user, then writes
`/etc/clickhouse-server/users.d/default-user.xml`. Once `pulse_clickhouse-data` exists, changing
`CLICKHOUSE_PASSWORD` in `.env` does nothing until you `make nuke` or alter the user by hand.

**Ports.**

| Mapping | Purpose |
|---|---|
| `${CLICKHOUSE_HTTP_PORT:-8123}:8123` | HTTP interface — `clickhouse-client`, `curl .../ping`, debugging |
| `${CLICKHOUSE_NATIVE_PORT:-9000}:9000` | Native protocol — what the Go driver uses |

Both are made overridable so a machine that already runs something on 8123 or 9000 (a local
ClickHouse, or Portainer on 9000) does not force you to edit a tracked file.

**Volumes.**

```yaml
- clickhouse-data:/var/lib/clickhouse                                             # data
- clickhouse-logs:/var/log/clickhouse-server                                      # logs
- ./deploy/clickhouse/config.d/pulse.xml:/etc/clickhouse-server/config.d/pulse.xml:ro
- ./deploy/clickhouse/users.d/pulse.xml:/etc/clickhouse-server/users.d/pulse.xml:ro
```

The first two are named volumes, so `make down` keeps the data and only `make nuke` (`down -v`)
destroys it. The last two are read-only bind mounts of the files from `deploy/`.

::: warning Mount the file, not the directory
It is tempting to mount `./deploy/clickhouse/users.d` onto `/etc/clickhouse-server/users.d`.
Do not. The entrypoint needs to *write* `default-user.xml` into that directory, and a
read-only directory bind makes startup fail. Mounting a single file leaves the directory
writable. The `:ro` flag on the file is still right: nothing inside the container should ever
edit config you keep in git.
:::

**`ulimits.nofile: 262144`.** ClickHouse keeps one file descriptor per part file, and a
MergeTree table under active inserts has many parts. The default of 1024 runs out quickly and
surfaces as `Too many open files` mid-benchmark rather than at startup.

**Healthcheck.**

```yaml
test: ["CMD-SHELL", "wget --no-verbose --tries=1 --spider http://127.0.0.1:8123/ping || exit 1"]
interval: 5s   timeout: 3s   retries: 20   start_period: 20s
```

`wget` because the Alpine image has BusyBox and no `curl`. `--spider` fetches headers only.
`start_period: 20s` is the grace window: failures during it do not count toward `retries`, so a
slow first boot (schema load, log rotation) is not mistaken for a broken server. Worst case the
service is declared unhealthy after `20 + 20 × 5 = 120 s`.

### Services: `ingest-api` and `analytics-api`

```yaml
ingest-api:
  <<: *api-common
  build:
    context: ./backend
    args:
      <<: *build-args
      SERVICE: ingest-api
  environment:
    <<: *api-env
    HTTP_ADDR: ":8080"
  ports:
    - "${INGEST_PORT:-8080}:8080"
```

One Dockerfile builds both images; `SERVICE` selects `./cmd/${SERVICE}` at the `go build` step.
That is why the two services differ by exactly three lines: the build arg, the listen-address
variable (`HTTP_ADDR` vs `ANALYTICS_ADDR`) and the published port.

The listen address is quoted — `:8080` unquoted is invalid YAML, since a colon starting a
scalar is parsed as a mapping.

::: tip Why the APIs declare no healthcheck
The runtime image is `gcr.io/distroless/static-debian12:nonroot`: a static binary, no shell,
no `wget`, no `curl`. There is nothing inside to run a probe with, and adding a shell purely to
run one would undo the reason for using distroless.

Readiness is therefore checked from **outside**, exactly as a load balancer does in production:
`make health` polls `http://localhost:8080/healthz` and `:8081/healthz` up to 30 times, one
second apart. If you want an in-container probe anyway, the usual pattern is to compile a
`/healthcheck` subcommand into the same binary and use `CMD ["/app", "healthcheck"]`.
:::

Combined with `read_only`, `cap_drop: ALL`, `no-new-privileges` and the image's
`USER nonroot` (uid 65532), a compromised handler gets a non-root process, no capabilities, no
writable filesystem and no way to escalate.

### `volumes`

```yaml
volumes:
  clickhouse-data:
  clickhouse-logs:
```

Declared with no options, so Docker manages them under `/var/lib/docker/volumes` as
`pulse_clickhouse-data` and `pulse_clickhouse-logs`. Inspect one with
`docker volume inspect pulse_clickhouse-data`.

## `deploy/clickhouse/config.d/pulse.xml`

Server-level overrides. The theme is: **stop ClickHouse from behaving as if it owns the
machine.** By default it sizes its caches and pools against total system RAM and every core,
which is right on a dedicated server and wrong on a laptop that is also running Go, Docker and
a browser. Production values live in `DEPLOY-AWS.md` §9.

| Setting | Value | Why |
|---|---|---|
| `logger.level` | `warning` | The default `trace`/`debug` writes hundreds of lines per query. Switch to `debug` while investigating, then switch back |
| `logger.console` | `true` | Send logs to stdout so `make logs` shows them |
| `logger.size` / `count` | `200M` / `3` | Log rotation ceiling inside the container |
| `max_server_memory_usage` | `4294967296` (4 GiB) | Hard ceiling for the whole server. Without it, ClickHouse assumes ~90 % of host RAM and the OOM killer eventually picks a victim |
| `mark_cache_size` | `536870912` (512 MiB) | Mark files locate granules inside a part; this cache is the one that genuinely pays for itself |
| `uncompressed_cache_size` | `0` | Off. It only helps repeated point lookups over the same small range; analytics scans just evict it |
| `background_pool_size` | `8` | Merge threads. Merges are the biggest background CPU consumer |
| `background_schedule_pool_size` | `8` | Periodic tasks (TTL, cleanup, replication) |
| `merge_tree.number_of_free_entries_in_pool_to_execute_mutation` | `4` | See below |
| `merge_tree.number_of_free_entries_in_pool_to_execute_optimize_entire_partition` | `4` | Same reason, for whole-partition `OPTIMIZE` |

::: warning The one non-obvious constraint
ClickHouse 26.x refuses to start if
`number_of_free_entries_in_pool_to_execute_mutation` (default **20**) is greater than
`background_pool_size × concurrency_ratio` — here `8 × 2 = 16`. Shrinking the pool without
lowering this value turns the container into a boot loop. If you raise `background_pool_size`
again, these two lines can go back to their defaults.
:::

**System log tables.** `query_log` and `metric_log` are kept, not disabled — Level 2 reads
`query_log` to attribute slow queries, and turning it off would remove the evidence. They are
bounded by TTL instead:

| Table | Flush | TTL |
|---|---|---|
| `system.query_log` | every 7.5 s | `event_date + INTERVAL 14 DAY DELETE` |
| `system.metric_log` | every 7.5 s, sampled every 1 s | `event_date + INTERVAL 7 DAY DELETE` |

`metric_log` samples every server metric once per second; without a TTL it is usually the
largest table on a development box within a week.

**Prometheus endpoint.**

```xml
<prometheus>
  <endpoint>/metrics</endpoint>
  <port>9363</port>
  <metrics>true</metrics> <events>true</events> <asynchronous_metrics>true</asynchronous_metrics>
</prometheus>
```

ClickHouse exposes its own metrics natively — no exporter sidecar. Level 6 points a scrape job
at `clickhouse:9363/metrics`.

::: tip Port 9363 is not published to the host
`docker-compose.yml` maps 8123 and 9000 only. Prometheus will reach it over the Compose
network, but to open it in a browser today, add `- "9363:9363"` to the `ports` list.
:::

**Removed features.**

```xml
<mysql_port remove="remove"/>
<postgresql_port remove="remove"/>
```

`remove="remove"` deletes the node inherited from the base `config.xml` rather than overriding
its value. ClickHouse can speak the MySQL and PostgreSQL wire protocols; this project uses
neither, so two listening sockets disappear from the attack surface.

## `deploy/clickhouse/users.d/pulse.xml`

Two profiles and two users. A **profile** is a named bundle of settings applied to every query
a user runs — including one you type by hand in `clickhouse-client`, which is exactly why the
guards live here instead of in a per-query `SETTINGS` clause that is easy to forget.

### Profile `pulse_app` — the application

| Setting | Value | Effect |
|---|---|---|
| `max_execution_time` | `15` | A query is killed after 15 s. Mirrors `CLICKHOUSE_QUERY_TIMEOUT` on the client, so both sides give up together |
| `max_memory_usage` | `4000000000` (~3.7 GiB) | Per-query ceiling; the query fails instead of the server dying |
| `max_bytes_before_external_group_by` | `2000000000` (~1.9 GiB) | Above this, `GROUP BY` spills to disk instead of failing. Slow, but it finishes |
| `max_concurrent_queries_for_user` | `32` | Reject query 33 immediately rather than queueing. Fail fast beats a queue nobody is watching |
| `network_compression_method` | `LZ4` | Compresses the insert stream on the wire. LZ4 over ZSTD: much cheaper CPU, and the network is not the bottleneck locally |
| `insert_distributed_sync` | `0` | Never block waiting for a distributed acknowledgement |
| `async_insert` | `0` | **Deliberately off** — Level 3 benchmarks client-side batching against server-side `async_insert`, and the comparison is only meaningful if the baseline has it disabled |
| `log_queries` | `1` | Write to `system.query_log`; the Level 2 analysis depends on it |

::: warning `max_memory_usage` sits very close to `max_server_memory_usage`
The per-query ceiling (~3.7 GiB) is nearly the whole-server ceiling (4 GiB). One heavy query
can consume the entire server budget and starve concurrent ones. It is a deliberate
development trade-off — you would rather one big query complete than fail — but on a shared or
production box set the per-query limit to a fraction of the server limit.
:::

### Profile `pulse_readonly` — the dashboard

Same time and memory limits, plus `readonly=2`. ClickHouse has three levels: `0` full access,
`1` read-only including settings, `2` read-only but allowed to change settings for the
session. `2` is what a dashboard needs — it can pass `max_threads` for one query, but `INSERT`,
`ALTER` and `DROP` are refused by the server, not by application code.

### Users

| User | Profile | Quota | Password | Purpose |
|---|---|---|---|---|
| `pulse` | `pulse_app` | `default` | from `CLICKHOUSE_PASSWORD`, via the entrypoint's `default-user.xml` | Both Go services: read and write |
| `dashboard` | `pulse_readonly` | `dashboard` | empty in git — **unusable until provisioned** | Level 6: the analytics API's read path |

`access_management=0` on `pulse` means it cannot create users or grant privileges. The
application account should not be able to rewrite its own permissions.

The empty `<password></password>` on `dashboard` is intentional, not an oversight: an empty
password in ClickHouse means *no valid credential*, so the user exists but cannot log in until
a real one is injected at deploy time. Committing a placeholder that happens to work is how
placeholders reach production.

`<networks><ip>::/0</ip></networks>` allows connections from anywhere — acceptable because the
only route to port 9000 is the Compose network plus whatever your host firewall permits. In
production the security group is the boundary (`DEPLOY-AWS.md`).

### Quota `dashboard`

```xml
<interval>
  <duration>3600</duration>       <!-- rolling 1-hour window -->
  <queries>10000</queries>        <!-- max queries -->
  <errors>500</errors>            <!-- max failures -->
  <execution_time>1800</execution_time>  <!-- max 30 CPU-minutes -->
</interval>
```

A quota limits consumption over time, where a profile limits a single query. It is the guard
against a dashboard that polls in a loop after a frontend bug — 10 000 queries per hour is
generous for a human and cheap to exceed for a `setInterval` with a missing cleanup.

`pulse` uses the built-in `default` quota, which is unlimited.

## How it fits together

```
.env / shell env
      │  ${VAR:-default} substitution
      ▼
docker-compose.yml ──build args──▶ backend/Dockerfile ──▶ distroless image
      │                                                        │
      │  bind mount :ro                                        │ env vars
      ▼                                                        ▼
deploy/clickhouse/config.d/pulse.xml   ┐              internal/config
deploy/clickhouse/users.d/pulse.xml    ├─ merged over  ─┐      │
image entrypoint → users.d/default-user.xml            │      │  clickhouse://pulse@clickhouse:9000
                                        base config.xml ┘      ▼
                                                          ClickHouse
```

Two configuration systems that never touch each other: environment variables configure the Go
processes, XML files configure the ClickHouse server. `CLICKHOUSE_PASSWORD` is the only value
that appears on both sides, which is why it is a single variable rather than two.

## Everyday operations

| Task | Command |
|---|---|
| Start everything and wait until healthy | `make up` |
| Stop, keep data | `make down` |
| Stop and delete all local data | `make nuke` |
| Follow one service's logs | `make logs S=clickhouse` |
| Check readiness from the host | `make health` |
| SQL shell inside the container | `make ch-cli` |
| Apply an XML change | `docker compose restart clickhouse` |

After editing either XML file, verify that ClickHouse actually took it — a typo makes the
server ignore the file or refuse to start:

```sql
SELECT name, value FROM system.settings WHERE name = 'max_execution_time';
SELECT name, profile FROM system.users;
SELECT * FROM system.quota_usage;
```

::: warning If ClickHouse restarts in a loop
Read `make logs S=clickhouse` first. The two most common causes are malformed XML in a file
under `deploy/` (the server refuses to start) and the mutation-pool constraint described
above. A directory bind over `users.d/` produces a third: the entrypoint cannot write
`default-user.xml`.
:::

## Known gaps

- **`make bench` references `docker-compose.bench.yml`**, which does not exist yet. That
  overlay arrives with the Level 3 ClickHouse-vs-PostgreSQL comparison.
- **`WAL_DIR` has no mount** while `read_only: true` is in effect — see the warning above.
- **The `dashboard` user has no password**, by design. Provision it before Level 6 or the
  analytics API cannot use the read-only path.
- **Port 9363 is not published**, so ClickHouse's Prometheus endpoint is reachable only from
  inside the Compose network today.

## See also

- [Configuration](./configuration) — every environment variable the Go code reads
- [Deployment](./deployment) — the AWS production topology
- [ClickHouse schema](../reference/clickhouse) — tables, codecs, partitioning
- [Runbook](../notes/runbook) — what to do when something is on fire
