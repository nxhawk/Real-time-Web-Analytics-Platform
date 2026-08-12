# ClickHouse schema

<Badge type="warning" text="Level 1 onwards" />

The authoritative DDL is
[`PLAN.md` §6–§7](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/PLAN.md)
and the migration files in `backend/migrations/`. This page explains *why* the schema looks
the way it does.

## Tables

| Table | Engine | Purpose | Level |
|---|---|---|---|
| `events` | MergeTree | Raw events, the source of truth | L1 |
| `events_hourly` | AggregatingMergeTree | Overview and time series | L5 |
| `events_daily` | AggregatingMergeTree | DAU / WAU / MAU | L5 |
| `page_stats_hourly` | AggregatingMergeTree | Top pages — separate because `page` has high cardinality | L5 |
| `sessions` | AggregatingMergeTree | Bounce rate, session duration, entry and exit page | L5 |
| `user_first_seen` | ReplacingMergeTree | Cohort assignment for retention | L5 |

## Sort order

The `ORDER BY` of a MergeTree table is the single most consequential choice in the schema: it
decides the sparse primary index, the compression ratio, and which queries can skip granules.

The starting proposal is `(site_id, event_name, event_time)` — it matches the dashboard
workload, which always filters by site, usually by event name, and always by time range.

That is a hypothesis, not a conclusion. Level 2 builds three tables over identical data with
three different sort orders, runs eight benchmark queries against each, and compares elapsed
time, `read_rows` and on-disk size. The winner becomes the migration and updates ADR-0002.
Queries by `user_id`, which no sort order here serves well, are handled with a projection —
if the measurement says the projection earns its cost.

## Types and codecs

Three rules, each with a measurement behind it in Level 2:

- **`LowCardinality(String)`** for `country`, `device`, `browser`, `event_name`. A dictionary
  per part instead of repeated strings: smaller on disk and faster to group by.
- **Never `Nullable`.** A nullable column stores a second bitmap and defeats several
  optimisations. A default value carries the same meaning at a lower cost.
- **Codecs matched to the data.** `Delta` or `DoubleDelta` before `ZSTD` for monotonically
  increasing timestamps, plain `ZSTD` for text. `ZSTD(1)` on hot data, `ZSTD(9)` after the
  recompression TTL.

## Materialized views

A materialized view in ClickHouse is an insert trigger, not a cached query: when a row lands
in `events`, the view's `SELECT` runs over just that block and its result is written into the
target table. That is why they are cheap, and also why two things bite:

**Aggregate state, not aggregate value.** The target table stores `-State` values which merge
correctly as parts merge, and queries read them back with `-Merge`. Putting a plain
non-aggregate column in an AggregatingMergeTree target produces numbers that are right at
first and quietly wrong after a merge. The golden test in Level 5 (task L5-03) exists
specifically to catch this: insert a fixed 50k events and compare every metric between raw
and view — they must match exactly.

**A view only sees inserts made after it exists.** Historical data needs an explicit
`INSERT ... SELECT` backfill, run month by month so it does not exhaust memory, with careful
attention to boundaries so nothing is counted twice.

**Cardinality explodes if you let it.** Only low-cardinality columns belong in a view's
`GROUP BY`. `page` has too many distinct values, which is why `page_stats_hourly` is a
separate table. If the views together exceed 15% of the raw table, the `GROUP BY` is wrong.

## Data lifecycle

| Age | What happens |
|---|---|
| 0–30 days | Hot, `ZSTD(1)` |
| 30 days | `TTL RECOMPRESS` to `ZSTD(9)` — slower to read, much smaller |
| 180 days | `TTL DELETE` |

An optional tiered storage policy can move cold parts to slower disks.

## Query guards

Every analytics query runs under `max_execution_time = 15` and `max_memory_usage = 4GB`,
plus a maximum range of 400 days at the API layer. These are set on the ClickHouse **user
profile** in `deploy/clickhouse/users.d/pulse.xml`, not in a per-query `SETTINGS` clause — a
clause is easy to forget, a profile applies even to a query typed by hand.

A `dashboard` user with `readonly = 2` exists so that from Level 6 the analytics API cannot
mutate data even if a handler has a bug.

## Traps worth knowing before you hit them

| Trap | Symptom | Avoidance |
|---|---|---|
| `Too many parts` | Inserts start failing with error 252 | Batch at least 10k rows, at most one insert per second per table. Raising `parts_to_throw_insert` is a band-aid |
| `FINAL` in a hot query | Enormous slowdown | Do not put ReplacingMergeTree on the hot path |
| `SELECT *` on a column store | Ten times slower than needed | Always list columns |
| Mixed timezones | Numbers off by seven hours | Store UTC absolutely, convert with `toTimeZone` at query time, display in the site's timezone |
| Unbounded range | One request scans a year and OOMs | Enforce the maximum range plus the profile guards |
| Disk full from raw plus projections | ClickHouse stops accepting writes | TTL, monitoring, alert at 75% |

## Learning by measurement

Level 2 is deliberately light on code and heavy on experiments. Everything learned goes into
[ClickHouse notes](/notes/clickhouse-notes) as *observation → number → explanation*, at least
twenty entries, none of them copied from the official documentation.

The exit criterion is being able to answer, with your own numbers: why this sort order, how
much `LowCardinality` saved, and whether the projection was worth it.
