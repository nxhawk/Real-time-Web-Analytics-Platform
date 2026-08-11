# CLAUDE.md

Guidance for Claude Code (and any AI coding agent) working in this repository.

---

## 0. Language rule — non-negotiable

**Everything written into the codebase must be in English.**

This applies to:

- Source code comments (Go, TypeScript, SQL, YAML, Dockerfile, shell)
- Identifiers: variable, function, type, package, table, and column names
- Log messages, error strings, and metric help texts
- API responses: error codes, messages, and field names
- Commit messages, branch names, pull request titles and bodies
- Documentation files, README sections, ADRs, and code-block annotations
- Test names and test failure messages
- UI copy in the dashboard (user-facing i18n strings live in locale files, not inline)

The planning documents `PLAN.md`, `TODO.md`, and `DEPLOY-AWS.md` are written in Vietnamese
and stay that way — do not translate them unless explicitly asked. When implementing a task
described in Vietnamese, **translate the intent into English code and English comments.**

```go
// GOOD: Flush the buffer when it is full or the flush interval elapses.
// BAD:  Đẩy buffer khi đầy hoặc hết thời gian flush.
```

---

## 1. Project context

**Pulse Analytics** — a self-hosted, real-time web analytics platform (a scaled-down Google
Analytics). Go/Gin ingest and query APIs, ClickHouse storage, Kafka event pipeline, Next.js
dashboard.

The repository is currently in the **design phase**: it contains specification documents but
no implementation yet. Build features by following the level-by-level checklist in `TODO.md`.

### Source of truth, in priority order

1. `PLAN.md` — the technical specification. Architecture, schemas, DDL, API contract, and
   design decisions live here. If code and `PLAN.md` disagree, `PLAN.md` wins unless the user
   says otherwise.
2. `TODO.md` — the execution order and acceptance criteria ("Done khi" = "Done when").
3. `DEPLOY-AWS.md` — the deployment target (Vercel + a single EC2 host), which supersedes the
   "VPS + docker compose" plan in `PLAN.md` §17.
4. `docs/api/openapi.yaml` — the API contract once it exists. Frontend types are generated
   from it; never hand-edit generated types.

---

## 2. Security and privacy

- **Raw IP addresses are never stored.** Use them for GeoIP enrichment, then discard (store a
  hash only if a specific requirement demands it).
- Strip sensitive query parameters (`token`, `email`, `password`) from the `page` field before
  storage.
- Every ingest and analytics endpoint requires an API key; the API key determines `site_id`,
  and `site_id` is enforced in every query — no cross-tenant reads.
- Secrets come from the environment or the deployment secret store, never from source.
- Guard every analytics query with `max_execution_time` and `max_memory_usage` settings.

---

## 3. Performance targets

Treat these as hard requirements, not aspirations:

| Target | Value |
|---|---|
| Dashboard query latency | < 300 ms at 100M events |
| Ingest availability | `202` returned even when ClickHouse is unavailable |
| Insert path | Batched — `BATCH_SIZE` rows or `FLUSH_INTERVAL_MS`, whichever comes first |
| Ingest rate limit | 1000 req/min per API key |
| Analytics rate limit | 120 req/min per IP |

When a change could affect the write path or query latency, run `make bench` and record the
numbers in `docs/benchmark-results.md`.

---