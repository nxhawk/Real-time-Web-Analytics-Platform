package config

import "time"

// maxEventsPerRequestLimit is the upper bound of the batch endpoint, fixed by the API
// contract in PLAN.md 5.2. A configuration file cannot raise it: the limit exists to bound
// the work one request can cost, and a value above it would be rejected by the handler
// anyway.
const maxEventsPerRequestLimit = 500

// Ingest holds the write-path settings. Defaults come from PHASES.md 2.4 and are re-tuned
// with real numbers in Level 3.
type Ingest struct {
	Sink            Sink       `mapstructure:"sink"`
	InsertMode      InsertMode `mapstructure:"insert_mode"`
	BatchSize       int        `mapstructure:"batch_size"`
	FlushIntervalMS int        `mapstructure:"flush_interval_ms"`
	BufferSize      int        `mapstructure:"buffer_size"`
	Workers         int        `mapstructure:"workers"`
	WALDir          string     `mapstructure:"wal_dir"`
	MaxEventsPerReq int        `mapstructure:"max_events_per_request"`
	RateLimitPerMin int        `mapstructure:"rate_limit_per_min"`
}

// FlushInterval is the maximum time a batch waits before being flushed.
func (i Ingest) FlushInterval() time.Duration {
	return time.Duration(i.FlushIntervalMS) * time.Millisecond
}

// validate implements section.
//
// The rule that ingest.sink=kafka needs brokers is not here: it spans two sections and lives
// in Config.Validate.
func (i Ingest) validate(p *problems) {
	if !i.Sink.Valid() {
		p.addf("ingest.sink (SINK) must be %s, got %q", oneOf(sinks), i.Sink)
	}
	if !i.InsertMode.Valid() {
		p.addf("ingest.insert_mode (INSERT_MODE) must be %s, got %q", oneOf(insertModes), i.InsertMode)
	}
	if i.BatchSize <= 0 {
		p.add("ingest.batch_size (BATCH_SIZE) must be greater than 0")
	}
	if i.FlushIntervalMS <= 0 {
		p.add("ingest.flush_interval_ms (FLUSH_INTERVAL_MS) must be greater than 0")
	}
	// A buffer smaller than a batch can never fill one, so the batch path would degrade to
	// the single-insert path without saying so.
	if i.BufferSize < i.BatchSize {
		p.add("ingest.buffer_size (BUFFER_SIZE) must be greater than or equal to ingest.batch_size")
	}
	if i.Workers <= 0 {
		p.add("ingest.workers (INGEST_WORKERS) must be greater than 0")
	}
	if i.MaxEventsPerReq <= 0 || i.MaxEventsPerReq > maxEventsPerRequestLimit {
		p.addf("ingest.max_events_per_request (MAX_EVENTS_PER_REQUEST) must be between 1 and %d",
			maxEventsPerRequestLimit)
	}
}
