# Knowledge base

<Badge type="tip" text="Background reading" />

The rest of this site documents *this* system: its schema, its API, its decisions. This section
is different — it explains the technologies underneath, from first principles, so the other
pages can stay short.

Read a page here when you want to understand **why** a tool behaves the way it does. Read
[Reference](/reference/api) when you want to know **what** this project does with it.

## Pages

| Page | What it covers |
|---|---|
| [ClickHouse explained](/knowledge/clickhouse) | Column stores, MergeTree, `ORDER BY` and the sparse index, skip indexes, projections, materialized views, TTL, codecs — plus when to use ClickHouse, and how it compares with PostgreSQL and Elasticsearch |

More pages land here as the project reaches the levels that need them — Kafka delivery
semantics and consumer groups in Level 4, Go concurrency patterns for the ingest path, and the
observability stack in Level 6.

## How these pages are written

- **Concepts before configuration.** If you cannot explain why a setting exists, the setting is
  cargo cult.
- **Every claim is checkable.** Numbers come with the dataset size next to them, and anything
  measured in this project links to [Notes](/notes/).
- **Comparisons are honest in both directions.** Every "X is better at this" is paired with a
  "and Y is better at that".
