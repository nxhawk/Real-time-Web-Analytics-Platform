// Package config loads and validates all runtime configuration from the environment.
//
// This is the only package allowed to read environment variables. Everything else receives
// a *Config (or a narrower sub-struct) through its constructor, which keeps packages testable
// and makes the full set of knobs discoverable in one place.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// envFile is the optional local override file, loaded before the real environment is read.
const envFile = ".env"

// Environment names the deployment stage the process runs in.
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

// Config is the full configuration surface of every backend binary. Individual binaries use
// the subset they need; keeping one struct means `.env.example` can stay authoritative.
type Config struct {
	App        App
	HTTP       HTTP
	Log        Log
	ClickHouse ClickHouse
	Ingest     Ingest
	Kafka      Kafka
}

// App holds process-wide settings.
type App struct {
	Env             Environment   `env:"APP_ENV" envDefault:"development"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"30s"`
}

// HTTP holds the settings of both HTTP servers.
type HTTP struct {
	IngestAddr         string        `env:"HTTP_ADDR" envDefault:":8080"`
	AnalyticsAddr      string        `env:"ANALYTICS_ADDR" envDefault:":8081"`
	ReadHeaderTimeout  time.Duration `env:"HTTP_READ_HEADER_TIMEOUT" envDefault:"5s"`
	ReadTimeout        time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"15s"`
	WriteTimeout       time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"30s"`
	IdleTimeout        time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"60s"`
	MaxBodyBytes       int64         `env:"HTTP_MAX_BODY_BYTES" envDefault:"1048576"` // 1 MiB, see PLAN 5.2
	CORSAllowedOrigins []string      `env:"CORS_ALLOWED_ORIGINS" envSeparator:"," envDefault:"http://localhost:3000"`
}

// Log holds logging settings.
type Log struct {
	Level  string `env:"LOG_LEVEL" envDefault:"info"`  // debug | info | warn | error
	Format string `env:"LOG_FORMAT" envDefault:"json"` // json | text
}

// ClickHouse holds connection settings for the native protocol (port 9000).
type ClickHouse struct {
	DSN             string        `env:"CLICKHOUSE_DSN" envDefault:"clickhouse://pulse:pulse@localhost:9000/analytics"`
	MaxOpenConns    int           `env:"CLICKHOUSE_MAX_OPEN_CONNS" envDefault:"16"`
	MaxIdleConns    int           `env:"CLICKHOUSE_MAX_IDLE_CONNS" envDefault:"8"`
	ConnMaxLifetime time.Duration `env:"CLICKHOUSE_CONN_MAX_LIFETIME" envDefault:"10m"`
	DialTimeout     time.Duration `env:"CLICKHOUSE_DIAL_TIMEOUT" envDefault:"5s"`
	QueryTimeout    time.Duration `env:"CLICKHOUSE_QUERY_TIMEOUT" envDefault:"15s"`
}

// Ingest holds the write-path settings. Defaults come from PHASES.md 2.4 and are re-tuned
// with real numbers in Level 3.
type Ingest struct {
	Sink            Sink       `env:"SINK" envDefault:"direct"`
	InsertMode      InsertMode `env:"INSERT_MODE" envDefault:"batch"`
	BatchSize       int        `env:"BATCH_SIZE" envDefault:"5000"`
	FlushIntervalMS int        `env:"FLUSH_INTERVAL_MS" envDefault:"500"`
	BufferSize      int        `env:"BUFFER_SIZE" envDefault:"100000"`
	Workers         int        `env:"INGEST_WORKERS" envDefault:"4"`
	WALDir          string     `env:"WAL_DIR" envDefault:"./data/wal"`
	MaxEventsPerReq int        `env:"MAX_EVENTS_PER_REQUEST" envDefault:"500"`
	RateLimitPerMin int        `env:"INGEST_RATE_LIMIT_PER_MIN" envDefault:"1000"`
}

// FlushInterval is the maximum time a batch waits before being flushed.
func (i Ingest) FlushInterval() time.Duration {
	return time.Duration(i.FlushIntervalMS) * time.Millisecond
}

// Kafka holds the Level 4 pipeline settings. Empty Brokers means Kafka is not configured.
type Kafka struct {
	Brokers   []string `env:"KAFKA_BROKERS" envSeparator:","`
	TopicRaw  string   `env:"KAFKA_TOPIC_RAW" envDefault:"events.raw"`
	TopicDLQ  string   `env:"KAFKA_TOPIC_DLQ" envDefault:"events.dlq"`
	GroupID   string   `env:"KAFKA_GROUP_ID" envDefault:"clickhouse-sink"`
	BatchSize int      `env:"KAFKA_CONSUMER_BATCH_SIZE" envDefault:"10000"`
}

// Load reads the environment (after loading a local .env file when present), applies
// defaults, and validates the result. It never returns a partially valid Config.
func Load() (*Config, error) {
	// A missing .env is normal in containers and CI, so it is not an error. A malformed one
	// is: silently ignoring it would start the process with the wrong settings.
	if _, statErr := os.Stat(envFile); statErr == nil {
		if err := godotenv.Load(envFile); err != nil {
			return nil, fmt.Errorf("load %s: %w", envFile, err)
		}
	}

	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return nil, fmt.Errorf("parse environment: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	return &cfg, nil
}

// Validate reports every problem it can find, not just the first one, so a misconfigured
// deployment can be fixed in one pass.
func (c *Config) Validate() error {
	var problems []string

	switch c.App.Env {
	case EnvDevelopment, EnvStaging, EnvProduction, EnvTest:
	default:
		problems = append(problems, fmt.Sprintf(
			"APP_ENV must be one of development|staging|production|test, got %q", c.App.Env))
	}

	switch strings.ToLower(c.Log.Level) {
	case "debug", "info", "warn", "error":
	default:
		problems = append(problems, fmt.Sprintf(
			"LOG_LEVEL must be one of debug|info|warn|error, got %q", c.Log.Level))
	}

	switch c.Log.Format {
	case "json", "text":
	default:
		problems = append(problems, fmt.Sprintf("LOG_FORMAT must be json|text, got %q", c.Log.Format))
	}

	switch c.Ingest.InsertMode {
	case InsertModeBatch, InsertModeSingle:
	default:
		problems = append(problems, fmt.Sprintf("INSERT_MODE must be batch|single, got %q", c.Ingest.InsertMode))
	}

	switch c.Ingest.Sink {
	case SinkDirect:
	case SinkKafka:
		if len(c.Kafka.Brokers) == 0 {
			problems = append(problems, "SINK=kafka requires KAFKA_BROKERS to be set")
		}
	default:
		problems = append(problems, fmt.Sprintf("SINK must be direct|kafka, got %q", c.Ingest.Sink))
	}

	if c.HTTP.IngestAddr == "" {
		problems = append(problems, "HTTP_ADDR must not be empty")
	}
	if c.HTTP.AnalyticsAddr == "" {
		problems = append(problems, "ANALYTICS_ADDR must not be empty")
	}
	if c.HTTP.MaxBodyBytes <= 0 {
		problems = append(problems, "HTTP_MAX_BODY_BYTES must be greater than 0")
	}
	if c.ClickHouse.DSN == "" {
		problems = append(problems, "CLICKHOUSE_DSN must not be empty")
	}
	if c.Ingest.BatchSize <= 0 {
		problems = append(problems, "BATCH_SIZE must be greater than 0")
	}
	if c.Ingest.FlushIntervalMS <= 0 {
		problems = append(problems, "FLUSH_INTERVAL_MS must be greater than 0")
	}
	if c.Ingest.BufferSize < c.Ingest.BatchSize {
		problems = append(problems, "BUFFER_SIZE must be greater than or equal to BATCH_SIZE")
	}
	if c.Ingest.Workers <= 0 {
		problems = append(problems, "INGEST_WORKERS must be greater than 0")
	}
	if c.Ingest.MaxEventsPerReq <= 0 || c.Ingest.MaxEventsPerReq > 500 {
		problems = append(problems, "MAX_EVENTS_PER_REQUEST must be between 1 and 500")
	}
	if c.App.ShutdownTimeout <= 0 {
		problems = append(problems, "SHUTDOWN_TIMEOUT must be greater than 0")
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// IsProduction reports whether the process runs with production behaviour (release-mode
// router, stricter CORS, no debug endpoints).
func (c *Config) IsProduction() bool { return c.App.Env == EnvProduction }

// SlogLevel maps the configured level onto slog's level type.
func (c *Config) SlogLevel() slog.Level {
	switch strings.ToLower(c.Log.Level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
