// Package model holds the domain types shared by the handler, service and repository
// layers.
//
// It carries no behaviour beyond what a type needs to describe itself. The rules that
// decide whether an Event may become a ValidatedEvent live in internal/validate, and the
// SQL that stores one lives in internal/repository. Because this package imports neither,
// every layer can import it without inverting the dependency direction that CLAUDE.md
// section 2 fixes.
package model
