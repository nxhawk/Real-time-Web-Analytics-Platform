# Contributing

The full version lives in
[`CONTRIBUTING.md`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/CONTRIBUTING.md).
This page is the short form plus the parts specific to documentation.

## Language rule

**Everything written into the codebase is in English** — code, comments, identifiers, log
messages, error strings, commit messages, pull request bodies, tests.

The four planning documents (`PLAN.md`, `PHASES.md`, `TODO.md`, `DEPLOY-AWS.md`) are in
Vietnamese and stay that way. When you implement a task described in Vietnamese, translate the
*intent* into English code:

```go
// GOOD: Flush the buffer when it is full or the flush interval elapses.
// BAD:  Đẩy buffer khi đầy hoặc hết thời gian flush.
```

This documentation site is bilingual: English is the default, Vietnamese mirrors it.

## Branching and commits

`main` is protected — pull requests only, CI must pass, no force pushes.

Branches: `feat/<slug>`, `fix/<slug>`, `chore/<slug>`, `docs/<slug>`.

Commits follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(ingest): accept batches of up to 500 events
fix(clickhouse): map error 252 to a domain error
docs(guide): document the readiness prober
```

Reference the task id from `TODO.md` in the pull request body — `Closes L1-17` — so the
checklist and the git history stay in sync.

## Before opening a pull request

```bash
make check
```

That is gofmt, `go vet`, golangci-lint and race-enabled tests: exactly what CI runs. Add or
update a test for every behaviour change.

## Working on a task

1. Read the task in `TODO.md` and its phase in `PHASES.md` — entry criteria, exit criteria,
   deliverables.
2. Read the referenced `PLAN.md` section before writing code. The DDL, query shapes and API
   contract are already decided there.
3. Implement, respecting the [layering rules](/guide/project-structure#layering-rules).
4. Test. `make check` clean.
5. Tick the task `[x]` in `TODO.md`, and fix any document the implementation contradicted.

## Shared numbers

Values that appear in more than one document — tool versions, performance thresholds, API
limits, seeder distributions — are owned by `PHASES.md` §2. Change them there first, then
propagate to `PLAN.md`, `README.md`, `.env.example`, this site and the code. Never change one
in isolation.

## Documentation

The site is [VitePress](https://vitepress.dev). Content lives in `docs/`.

```bash
cd docs
npm ci
npm run dev      # http://localhost:5173, hot reload
npm run build    # production build; fails on dead links
```

### Where a page goes

| Directory | Content |
|---|---|
| `docs/guide/` | Narrative: how to do something, how something works |
| `docs/reference/` | Lookup material: endpoints, schemas, metric names |
| `docs/notes/` | Measurements and operational knowledge produced by a level |
| `docs/adr/` | One decision per file: context, decision, consequences, alternatives |
| `docs/vi/**` | The Vietnamese mirror — same tree, same filenames |

### Adding a page

1. Create the English page, for example `docs/guide/caching.md`.
2. Create the Vietnamese mirror at `docs/vi/guide/caching.md`.
3. Add both to the sidebars: `docs/.vitepress/config/en.mts` and `.../vi.mts`.
4. Run `npm run build` — a dead link fails the build.

A page that exists in one language but not the other is acceptable while a translation is
pending; add a short note at the top pointing at the other language. What is not acceptable is
a sidebar entry pointing at a file that does not exist — that breaks the build.

### Writing style

- Lead with the answer, then the explanation.
- Prefer a table over a bulleted list of pairs.
- Every code block should be runnable, or clearly marked as an excerpt.
- Mark features that do not exist yet with a badge: `<Badge type="warning" text="Level 3" />`.
- Link to `PLAN.md` for the specification rather than restating it — a restatement drifts.

### Deployment

Pushing to `main` with changes under `docs/` triggers `.github/workflows/docs.yml`, which
builds the site and publishes it to GitHub Pages. Pull requests build the site too, but do not
deploy — so a broken build is caught before merge.
