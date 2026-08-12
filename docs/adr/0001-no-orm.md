# ADR-0001 — No ORM, hand-written SQL

**Status:** Accepted · **Date:** 2026-08-11

## Context

The backend is Go, and the Go ecosystem offers several ways to talk to a database: an ORM such
as GORM or Ent, a query builder such as squirrel, a code generator such as sqlc, or the driver
directly with SQL written by hand.

The primary goal of this project is learning ClickHouse deeply — MergeTree internals,
aggregate combinators, materialized views, skip indexes, projections. The secondary goal is a
platform where every dashboard query stays under 300 ms at 100 million events.

ClickHouse is also not the database most ORMs were built for. Its dialect includes
`-State` and `-Merge` combinators, `windowFunnel`, `retention`, `arrayJoin`,
`PROJECTION`, and per-query `SETTINGS` clauses. None of this maps onto an ORM's model of rows
and relations.

## Decision

**No ORM.** Queries are written by hand as SQL, stored as `.sql` files under
`internal/repository/clickhouse/queries/`, and embedded into the binary with `go:embed`. The
official driver `ClickHouse/clickhouse-go/v2` is used over the native protocol on port 9000.

All parameters are bound as ClickHouse named parameters (`{name:Type}`). Identifiers that
cannot be parameterised — a column name chosen by a dimension query parameter — go through an
explicit whitelist.

## Consequences

**Good**

- `EXPLAIN` and `EXPLAIN PIPELINE` work on exactly the query that runs in production. This is
  the whole point: an optimisation you cannot observe is one you cannot learn from.
- Combinators, `windowFunnel`, projections and per-query settings are all available without
  fighting an abstraction.
- A `.sql` file can be pasted straight into `clickhouse-client` and profiled.
- Query cost is visible at review time, in the diff.

**Bad**

- More boilerplate: scanning rows into structs is written by hand.
- No compile-time check that a query matches its struct. Mitigated by integration tests that
  run against a real ClickHouse container via testcontainers.
- Refactoring a column name means grepping the `queries/` directory rather than renaming a
  field.

**Neutral**

- Portability to another database is lost. This is not a cost here: the queries are
  ClickHouse-specific by design, and the PostgreSQL code in `repository/postgres/` exists
  purely for the Level 3 benchmark comparison.

## Alternatives considered

**GORM.** The most popular Go ORM, but ClickHouse support is a community driver, migrations
do not fit ClickHouse's DDL, and it actively hides the execution plan. Rejected on the primary
goal: it would prevent the learning the project exists for.

**sqlc.** Generates type-safe Go from SQL, which is genuinely attractive — SQL stays visible
and the scanning boilerplate disappears. Rejected because ClickHouse support is immature,
particularly for aggregate-state types, and the generation step would obscure the connection
between a `.sql` file and the code path that runs it. Worth revisiting if support matures.

**squirrel or another query builder.** Useful when queries are assembled dynamically. Here the
queries are known ahead of time and mostly static; a builder would add indirection without
removing the need to understand the SQL. Dynamic parts — an optional filter, a chosen
dimension — are handled with a whitelist and a small amount of string assembly, reviewed
carefully.

## Related

- [ClickHouse schema](/reference/clickhouse) — what the queries run against
- ADR-0006 — materialized views instead of querying raw data
- [`PLAN.md` §8](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/PLAN.md) —
  the query cookbook
