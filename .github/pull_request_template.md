# Summary

<!-- What changes and why. One or two sentences. -->

**Task:** <!-- e.g. Closes L0-13 — see TODO.md -->

## Type of change

- [ ] `feat` — new behaviour
- [ ] `fix` — bug fix
- [ ] `perf` — performance
- [ ] `refactor` — no behaviour change
- [ ] `docs` / `ci` / `chore`

## How this was verified

<!-- Commands run, numbers measured, screenshots. "It compiles" is not verification. -->

```
make lint && make test
```

## Checklist

- [ ] `make fmt && make lint && make test` pass locally
- [ ] Tests added or updated for the behaviour that changed
- [ ] Everything written in English (code, comments, log messages, this PR)
- [ ] Documentation updated if the change contradicts `PLAN.md`, `PHASES.md`, or `TODO.md`
- [ ] Shared numbers (versions, thresholds, limits) changed in `PHASES.md` §2 first
- [ ] No secrets, no raw IP addresses, no event payloads in logs
- [ ] Migration (if any) is backward compatible by one step

## Performance impact

<!-- Required when touching the write path or an analytics query.
     Run `make bench` and record the numbers in docs/benchmark-results.md. -->

- [ ] Not applicable
