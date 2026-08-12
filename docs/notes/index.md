# Engineering notes

Documents produced *by* the work rather than *before* it. Each one is an output of a specific
level and is empty until that level runs.

The rule for all three: **a number without a measurement behind it does not go in.** Copying a
claim from the official documentation is not a note; measuring it on your own data is.

| Note | Produced by | Contents |
|---|---|---|
| [ClickHouse notes](/notes/clickhouse-notes) | Level 2 | At least 20 experiments — observation, number, explanation |
| [Benchmark results](/notes/benchmark-results) | Level 3 | ClickHouse vs PostgreSQL across four dataset sizes |
| [Runbook](/notes/runbook) | Level 6 | What to do at 3am when something breaks |

## Why they are part of the deliverable

Level 2 produces almost no code. Its entire output is `clickhouse-notes.md` — which makes the
note the deliverable, not a byproduct. A level whose measurements were never written down
cannot be marked complete, because six weeks later nobody remembers whether the projection was
worth it.

The same applies to the runbook. It is written while building the thing, not during the
incident.
