package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nxhawk/pulse-analytics/backend/internal/config"
)

// valid returns a Config that passes validation, so each test can break exactly one field.
func valid() config.Config {
	return config.Config{
		App: config.App{Env: config.EnvDevelopment, ShutdownTimeout: 30_000_000_000},
		HTTP: config.HTTP{
			IngestAddr:    ":8080",
			AnalyticsAddr: ":8081",
			MaxBodyBytes:  1 << 20,
		},
		Log:        config.Log{Level: "info", Format: "json"},
		ClickHouse: config.ClickHouse{DSN: "clickhouse://localhost:9000/analytics"},
		Ingest: config.Ingest{
			Sink:            config.SinkDirect,
			InsertMode:      config.InsertModeBatch,
			BatchSize:       5000,
			FlushIntervalMS: 500,
			BufferSize:      100000,
			Workers:         4,
			MaxEventsPerReq: 500,
		},
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*config.Config)
		wantErr string
	}{
		{
			name:   "valid configuration",
			mutate: func(*config.Config) {},
		},
		{
			name:    "unknown environment",
			mutate:  func(c *config.Config) { c.App.Env = "prod" },
			wantErr: "APP_ENV",
		},
		{
			name:    "unknown log level",
			mutate:  func(c *config.Config) { c.Log.Level = "verbose" },
			wantErr: "LOG_LEVEL",
		},
		{
			name:    "unknown log format",
			mutate:  func(c *config.Config) { c.Log.Format = "logfmt" },
			wantErr: "LOG_FORMAT",
		},
		{
			name:    "unknown insert mode",
			mutate:  func(c *config.Config) { c.Ingest.InsertMode = "bulk" },
			wantErr: "INSERT_MODE",
		},
		{
			name:    "kafka sink without brokers",
			mutate:  func(c *config.Config) { c.Ingest.Sink = config.SinkKafka },
			wantErr: "KAFKA_BROKERS",
		},
		{
			name: "kafka sink with brokers",
			mutate: func(c *config.Config) {
				c.Ingest.Sink = config.SinkKafka
				c.Kafka.Brokers = []string{"localhost:9092"}
			},
		},
		{
			name:    "empty clickhouse dsn",
			mutate:  func(c *config.Config) { c.ClickHouse.DSN = "" },
			wantErr: "CLICKHOUSE_DSN",
		},
		{
			name:    "zero batch size",
			mutate:  func(c *config.Config) { c.Ingest.BatchSize = 0 },
			wantErr: "BATCH_SIZE",
		},
		{
			name:    "buffer smaller than batch",
			mutate:  func(c *config.Config) { c.Ingest.BufferSize = 10 },
			wantErr: "BUFFER_SIZE",
		},
		{
			name:    "batch limit above the API contract",
			mutate:  func(c *config.Config) { c.Ingest.MaxEventsPerReq = 501 },
			wantErr: "MAX_EVENTS_PER_REQUEST",
		},
		{
			name:    "non-positive shutdown timeout",
			mutate:  func(c *config.Config) { c.App.ShutdownTimeout = 0 },
			wantErr: "SHUTDOWN_TIMEOUT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := valid()
			tt.mutate(&cfg)
			err := cfg.Validate()

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidateReportsEveryProblemAtOnce(t *testing.T) {
	t.Parallel()

	cfg := valid()
	cfg.App.Env = "nope"
	cfg.Log.Level = "nope"
	cfg.Ingest.BatchSize = 0

	err := cfg.Validate()

	require.Error(t, err)
	require.Contains(t, err.Error(), "APP_ENV")
	require.Contains(t, err.Error(), "LOG_LEVEL")
	require.Contains(t, err.Error(), "BATCH_SIZE")
}

func TestLoadAppliesDefaults(t *testing.T) {
	// Not parallel: it mutates the process environment.
	t.Setenv("APP_ENV", "test")
	t.Chdir(t.TempDir()) // no .env file in an empty directory

	cfg, err := config.Load()

	require.NoError(t, err)
	require.Equal(t, config.EnvTest, cfg.App.Env)
	require.Equal(t, ":8080", cfg.HTTP.IngestAddr)
	require.Equal(t, ":8081", cfg.HTTP.AnalyticsAddr)
	require.Equal(t, 5000, cfg.Ingest.BatchSize)
	require.Equal(t, 500, cfg.Ingest.FlushIntervalMS)
	require.Equal(t, config.InsertModeBatch, cfg.Ingest.InsertMode)
	require.Equal(t, config.SinkDirect, cfg.Ingest.Sink)
	require.False(t, cfg.IsProduction())
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "banana")
	t.Chdir(t.TempDir())

	_, err := config.Load()

	require.Error(t, err)
	require.Contains(t, err.Error(), "APP_ENV")
}

func TestFlushInterval(t *testing.T) {
	t.Parallel()

	cfg := valid()

	require.Equal(t, "500ms", cfg.Ingest.FlushInterval().String())
}
