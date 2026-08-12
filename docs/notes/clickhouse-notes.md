# ClickHouse notes

<Badge type="warning" text="Produced in Level 2" />

::: info Not written yet
Level 2 has not run. This page describes the format and the questions the notes must answer;
entries land here as the experiments are done.
:::

## Format

One entry per finding, always three parts:

> **Observation** — what was seen.
> **Number** — the measurement, with the dataset size next to it.
> **Explanation** — why ClickHouse behaves that way.

An entry without a number is an opinion. An entry without an explanation is trivia.

### Example of the shape

> **Observation.** Changing `country` from `String` to `LowCardinality(String)` shrank the
> column and sped up `GROUP BY country`.
>
> **Number.** At 10M events: column size 41 MB → 0.7 MB. `GROUP BY country` over 30 days:
> 180 ms → 62 ms, `read_rows` unchanged.
>
> **Explanation.** `LowCardinality` stores a dictionary per part plus an index of positions,
> so the same 200-odd country codes are written once instead of 10 million times. The grouping
> then happens over small integers instead of strings, which vectorises well.

## Questions the notes must answer

The level is not done until each of these has an answer backed by your own measurement:

1. **Why this `ORDER BY`?** Three tables over identical data with three sort orders, eight
   benchmark queries each, comparing elapsed time, `read_rows` and disk size.
2. **What does `index_granularity` change?** 8192 vs 4096 vs 16384: mark file size and point
   query speed.
3. **How much does `LowCardinality` save?** Per column, size and grouping speed.
4. **Which codec for which column?** `ZSTD(1)` vs `ZSTD(3)` vs `ZSTD(9)` vs `LZ4` on `page`
   and `properties`; `Delta+ZSTD` vs `DoubleDelta` on `event_time`.
5. **Why avoid `Nullable`?** `Nullable(String)` vs `String DEFAULT ''`, measured.
6. **Do skip indexes help?** `bloom_filter` on `user_id` and `page`, `tokenbf_v1` for
   substring search, `minmax` on `ingested_at` — `read_rows` before and after.
7. **Is the projection worth it?** Three numbers: disk increase, insert slowdown, query
   speedup.
8. **What does TTL recompression save?** Size before and after moving 30-day-old parts to
   `ZSTD(9)`.

## Measurement discipline

- **Minimum 10M events.** Conclusions from a small table are noise, and the shape of the
  answer changes with scale.
- **Note the dataset size beside every number.** "62 ms" means nothing on its own.
- **Cold and warm separately.** Drop the caches before a cold measurement:
  ```sql
  SYSTEM DROP MARK CACHE;
  SYSTEM DROP UNCOMPRESSED CACHE;
  ```
- **Three runs, take the median.** One run measures the machine's mood.
- **Read the plan, not just the clock.** `EXPLAIN indexes = 1` shows how many granules were
  skipped; `EXPLAIN PIPELINE` shows how many threads did the work. A query that got faster for
  a reason you cannot name will get slower again later.

## Useful introspection queries

These live in `docs/queries-ops.sql` once Level 2 starts:

```sql
-- Parts per table, size, row count
SELECT table, count() AS parts, formatReadableSize(sum(bytes_on_disk)) AS size, sum(rows)
FROM system.parts WHERE active AND database = 'analytics'
GROUP BY table ORDER BY sum(bytes_on_disk) DESC;

-- Compression ratio per column: the number that usually surprises people
SELECT column,
       formatReadableSize(sum(column_data_compressed_bytes))   AS compressed,
       formatReadableSize(sum(column_data_uncompressed_bytes)) AS uncompressed,
       round(sum(column_data_uncompressed_bytes) / sum(column_data_compressed_bytes), 2) AS ratio
FROM system.parts_columns WHERE active AND table = 'events'
GROUP BY column ORDER BY sum(column_data_compressed_bytes) DESC;

-- Slowest queries in the last hour, from the server's own log
SELECT query_duration_ms, read_rows, formatReadableSize(memory_usage), substring(query, 1, 120)
FROM system.query_log
WHERE type = 'QueryFinish' AND event_time > now() - INTERVAL 1 HOUR
ORDER BY query_duration_ms DESC LIMIT 20;
```
