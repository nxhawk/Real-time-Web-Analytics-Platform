package config

import (
	"slices"
	"strings"
)

// Environment names the deployment stage the process runs in. Its value also selects the
// configuration file: APP_ENV=staging loads config/staging.config.yml.
type Environment string

// The supported deployment stages.
const (
	EnvDevelopment Environment = "development"
	EnvStaging     Environment = "staging"
	EnvProduction  Environment = "production"
	EnvTest        Environment = "test"
)

// InsertMode selects how events reach ClickHouse. "single" exists only so that the naive
// path stays measurable against the batched path in the Level 3 benchmark.
type InsertMode string

// The supported insert modes.
const (
	InsertModeBatch  InsertMode = "batch"
	InsertModeSingle InsertMode = "single"
)

// Sink selects the transport between the ingest API and ClickHouse.
type Sink string

// The supported sinks.
const (
	SinkDirect Sink = "direct" // ingest API writes to ClickHouse itself (Levels 1-3)
	SinkKafka  Sink = "kafka"  // ingest API produces to Kafka, a consumer writes (Level 4+)
)

// The allowed values of every enumerated knob, in the order they appear in error messages.
//
// Each set is declared once and drives both the check and the message the check produces,
// so a new value can never be accepted by Valid while the error text still denies it.
var (
	environments = []Environment{EnvDevelopment, EnvStaging, EnvProduction, EnvTest}
	insertModes  = []InsertMode{InsertModeBatch, InsertModeSingle}
	sinks        = []Sink{SinkDirect, SinkKafka}

	// Log levels and formats stay plain strings: they are matched case-insensitively and
	// map onto slog's own vocabulary rather than onto a type of ours.
	logLevels  = []string{"debug", "info", "warn", "error"}
	logFormats = []string{"json", "text"}
)

// Valid reports whether e is a stage this build understands.
func (e Environment) Valid() bool { return slices.Contains(environments, e) }

// Valid reports whether m is an insert mode this build understands.
func (m InsertMode) Valid() bool { return slices.Contains(insertModes, m) }

// Valid reports whether s is a sink this build understands.
func (s Sink) Valid() bool { return slices.Contains(sinks, s) }

// oneOf renders an allowed set for an error message: "development|staging|production|test".
func oneOf[T ~string](values []T) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = string(v)
	}
	return strings.Join(parts, "|")
}
