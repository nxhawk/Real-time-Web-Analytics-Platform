// Package validate turns the events a client sends into the events this system may store.
//
// PLAN.md 5.2 asks for two different things under one word. Some faults cost the event: an
// event name that is not snake_case, properties that are not an object, a revenue that
// does not fit Decimal(18, 4). Other faults cost only the value: a user id longer than its
// column is truncated, a page carrying a password-reset token is stripped, a timestamp
// from a device with a wrong clock is replaced by the server's. The split is not
// arbitrary. Rejecting the second group throws away real traffic over circumstances the
// visitor did not choose; accepting the first group silently corrupts the numbers the
// dashboard reports.
//
// The pipeline is one ordered list of rules (rules.go). Each rule owns a few fields, writes
// them into a model.ValidatedEvent, and returns the reason it could not. That list is the
// extension point: a new rule is a function plus one line, and the test table iterates the
// same list, so a rule added without a test fails the suite rather than shipping unchecked.
//
// The package holds no mutable state. Repairs and rejections are reported through an
// Observer instead of a package-level counter, which is what lets the whole suite run with
// t.Parallel() and lets a caller measure validation without this package importing
// Prometheus.
//
// Nothing here knows about HTTP. A fault in the request as a whole comes back as a sentinel
// error and the handler decides which status code it deserves.
package validate
