# ADR-0011 — Hand-written validation pipeline, not `go-playground/validator`

**Status:** Accepted · **Date:** 2026-08-18

## Context

`PLAN.md` §3 listed `go-playground/validator/v10` as the validation library. Writing the
rules in §5.2 revealed that only some of them are validation in that library's sense.

Read the table again and it splits in two:

| Rule | What happens when it is violated |
|---|---|
| `event` is not `^[a-z0-9_]{1,64}$` | **Reject** that event |
| `properties` is not an object, or over 8 KB | **Reject** that event |
| `revenue` does not fit `Decimal(18, 4)` | **Reject** that event |
| `user_id` is over 128 characters | **Truncate** it |
| `page` carries a `token` parameter | **Strip** the parameter |
| `timestamp` is 40 hours in the future | **Replace** it with `now()` |
| `device` is `"smart_fridge"` | **Normalise** it to `unknown` |

Only the first group is a predicate. The second group *repairs the value and keeps the
event*, which is a deliberate product decision: a device with a wrong clock is common, and
its traffic is real. Tag-based validation cannot express a repair — a validator returns a
verdict, it does not rewrite the struct it was handed.

A third constraint: partial success. A batch of 100 with 3 bad events must accept 97 and
report 3 *by index*. That requires each element to be decoded on its own, which is a
property of how the request is parsed rather than of any validator.

## Decision

**Hand-written rules, no validation library.**

`internal/validate` is an ordered list of rules. Each rule owns a few fields, writes them
into a `model.ValidatedEvent`, and returns either `ReasonNone` or the reason it could not:

```go
var eventRules = []eventRule{
    {"event_name", ruleEventName},
    {"event_id",   ruleEventID},
    {"timestamp",  ruleTimestamp},
    // ...
}
```

Three consequences fall out of that shape:

- **`model.Event` → `model.ValidatedEvent`.** Only this package produces the second type, so
  an unchecked event cannot reach the repository. The type is the guarantee, not a
  convention.
- **`IngestRequest.Events` is `[]json.RawMessage`.** One malformed element costs one index.
- **Repairs are reported through an `Observer`,** not counted in a package-level variable, so
  the package holds no state and `internal/metrics` supplies the Prometheus implementation
  without either package importing the other.

`PLAN.md` §3 has been corrected in the same change. The resulting structure, and what to do
when you need to add a rule, are in the [validation pipeline guide](/guide/validation).

## Consequences

**Good**

- Repairs and rejections live side by side and read as the same kind of thing, which is what
  they are.
- No reflection on the hot path. The ingest target is 10,000 events per second, and a
  tag-based validator walks the struct with `reflect` for every one of them.
- A new rule is a function plus one line. The test table iterates the same list and fails if
  a rule has no case, so the registry cannot grow a step nobody exercised.
- Every bound is a field of `validate.Limits`, so a test shrinks one instead of constructing
  an 8 KB string, and a future per-plan limit is a value rather than a rewrite.

**Bad**

- More code than a struct tag. Roughly 250 lines of rules against perhaps 20 lines of tags —
  though the tags would have covered maybe half the table.
- The rules are ours to keep correct. A library would have had its edge cases found by other
  people first.
- Two types where a library would have used one, so a field added to the payload has to be
  carried through both.

**Neutral**

- `go-playground/validator/v10` is still an indirect dependency through Gin's binding. This
  decision is about the ingest path, not about removing it from the module graph.

## Alternatives considered

**`go-playground/validator/v10` for the reject-only rules, hand code for the rest.** Follows
`PLAN.md` §3 literally. Rejected because it splits one table across two mechanisms: a reader
asking "what happens to an over-long `user_id`?" would have to know which half of the rules
to look in, and the reason codes would come from two different places.

**A struct tag per rule, with custom validators for everything.** Custom validators can be
registered for the reject rules, but a validator that truncates its input is a validator
lying about what it is. It would also make the reason returned to the client a formatted
message rather than a closed set, and that set is a Prometheus label.

**`ozzo-validation` or another builder-style library.** Closer to the right shape — rules are
values, not tags — but it still models a rule as a predicate, so the repairs would have had
to sit outside it anyway. The remaining benefit was a nicer way to write eight `if`
statements.
