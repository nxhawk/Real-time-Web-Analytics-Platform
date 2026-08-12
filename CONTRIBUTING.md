# Contributing

Thanks for working on Pulse Analytics. This file covers the rules that CI enforces and the
few conventions that CI cannot.

---

## 0. Language rule

**Everything written into the codebase must be in English** — code, comments, identifiers,
log messages, error strings, commit messages, and pull request bodies.

The planning documents (`PLAN.md`, `PHASES.md`, `TODO.md`, `DEPLOY-AWS.md`) are written in
Vietnamese by design and stay that way. When you implement a task described in Vietnamese,
translate the *intent* into English code. See [`CLAUDE.md`](CLAUDE.md) §0.

---

## 1. Development setup (5 minutes)

**Prerequisites:** Docker + Docker Compose v2, Go 1.26, Node.js 22+, `make`.

```bash
git clone https://github.com/nxhawk/pulse-analytics.git
cd pulse-analytics
cp .env.example .env
make up          # ClickHouse + ingest-api + analytics-api
make ps          # everything healthy?
curl localhost:8080/healthz
```

Running the API outside Docker, against the containerised ClickHouse:

```bash
make down-app    # stop the API containers, keep ClickHouse running
make run         # go run ./cmd/ingest-api
```

Useful targets — the full list is `make help`:

| Target | What it does |
|---|---|
| `make up` / `make down` | Start / stop the Compose stack |
| `make build` / `make run` | Build binaries into `backend/bin/` / run the ingest API |
| `make test` / `make test-int` | Unit tests / integration tests (testcontainers) |
| `make lint` / `make fmt` | golangci-lint / gofmt + goimports |
| `make migrate-up` / `make migrate-down` | Apply / roll back migrations |
| `make ch-cli` | Open a `clickhouse-client` shell |

---

## 2. Branching and commits

`main` is protected: pull requests only, CI must pass, no force pushes.

Branch names: `feat/<short-slug>`, `fix/<short-slug>`, `chore/<short-slug>`,
`docs/<short-slug>`.

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(ingest): accept batches of up to 500 events
fix(clickhouse): map error 252 to a domain error
chore(ci): cache Go modules
docs(plan): pin Go 1.26
```

Allowed types: `feat`, `fix`, `perf`, `refactor`, `test`, `docs`, `build`, `ci`, `chore`.
Scope is the package or area (`ingest`, `analytics`, `clickhouse`, `kafka`, `ci`, `deploy`).

Reference the task ID from [`TODO.md`](TODO.md) in the pull request body — for example
`Closes L0-13` — so the checklist and the git history stay in sync.

---

## 3. Before opening a pull request

```bash
make fmt
make lint
make test
```

All three must be clean. CI runs the same commands plus `go vet`, a `gofmt -l` check, and a
race-enabled test run.

Add or update a test for every behaviour change. Table-driven tests are the default style in
this repository.

---

## 4. Code conventions

- **Layering.** `cmd/` wires dependencies; `handler/` speaks HTTP; `service/` holds business
  rules; `repository/` speaks to storage. Dependencies point inward — a repository never
  imports a handler.
- **No ORM.** SQL is written by hand and lives in `internal/repository/clickhouse/queries/`,
  embedded with `go:embed`. This is a deliberate decision (ADR-0001) so that `EXPLAIN` stays
  meaningful.
- **Errors** wrap with `%w` and carry context: `fmt.Errorf("insert batch: %w", err)`. Never
  log and return the same error — pick one.
- **Logging** uses `log/slog` with the JSON handler. No `fmt.Println`, no `log.Printf`.
  Never log event payloads or anything that could contain personal data.
- **Context** is the first parameter of every function that does I/O.
- **Configuration** is read from the environment in `internal/config` only. No package reads
  `os.Getenv` on its own.

---

## 5. Documentation changes

When an implementation reveals that a document is wrong, fix the document in the same pull
request. Numbers that appear in more than one document (versions, performance thresholds, API
limits) are owned by [`PHASES.md` §2](PHASES.md#2-bảng-số-liệu-chuẩn) — change them there
first, then propagate.
