# Architecture decisions

Eleven decisions shape this system. Each one is recorded as an ADR so that six months from now
the question is "was this still the right trade-off" rather than "why on earth is it like
this".

Format: **Context** → **Decision** → **Consequences** → **Alternatives considered**.

| # | Decision | Why | Status |
|---|---|---|---|
| [0001](/adr/0001-no-orm) | No ORM, hand-written SQL | The goal is learning ClickHouse; an ORM hides the execution plan and does not support combinators or materialized views | Accepted |
| 0002 | `ORDER BY (site_id, event_name, event_time)` | Matches the dashboard workload; `user_id` access is served by a projection | Provisional — Level 2 measures three alternatives |
| 0003 | Client-side batching instead of `async_insert` | Control over retries, measurable, and the mechanism is the thing being learned | Accepted |
| 0004 | Kafka between ingest and ClickHouse from Level 4 | Decoupling, replay, fan-out to future consumers | Accepted |
| 0005 | At-least-once plus de-duplication at query time | Simpler than exactly-once and accurate enough for analytics | Accepted |
| 0006 | AggregatingMergeTree views instead of querying raw | The 300 ms requirement cannot be met by scanning raw at 100M events | Accepted |
| 0007 | Monorepo | The API contract and its client stay in sync; one CI run covers both | Accepted |
| 0008 | Docker Compose instead of Kubernetes | One host, one person. Kubernetes is operational cost with no return here | Accepted |
| 0009 | Single-node ClickHouse, no replication yet | Less complexity now; the upgrade path via Keeper and ReplicatedMergeTree is documented | Accepted |
| 0010 | Return `202 Accepted` for ingest | Ingest must not depend on storage availability | Accepted |
| [0011](/adr/0011-hand-written-event-validation) | Hand-written validation, no `validator/v10` | Half the rules in PLAN §5.2 repair the value instead of rejecting the event, which a tag cannot express | Accepted |

## Provisional means provisional

ADR-0002 is marked provisional on purpose. The sort order is a hypothesis based on the query
shapes; Level 2 builds three tables with three different orders over identical data and
measures elapsed time, `read_rows` and disk size across eight queries. Whichever wins becomes
the migration, and this ADR is rewritten with the numbers attached.

A decision recorded with the measurement that produced it is worth ten decisions recorded with
a rationale.

## Writing a new one

One file per decision in `docs/adr/`, named `NNNN-short-slug.md`, plus its Vietnamese mirror
in `docs/vi/adr/`. Add both to the sidebars in `.vitepress/config/en.mts` and `vi.mts`, and to
the table above.

Keep it short. An ADR that takes twenty minutes to read will not be read. The value is in
recording what was *rejected* and why — that is the part nobody remembers.
