package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nxhawk/pulse-analytics/backend/internal/config"
)

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
			name:    "unknown sink",
			mutate:  func(c *config.Config) { c.Ingest.Sink = "carrier pigeon" },
			wantErr: "SINK",
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
			name:    "empty ingest address",
			mutate:  func(c *config.Config) { c.HTTP.IngestAddr = "" },
			wantErr: "HTTP_ADDR",
		},
		{
			name:    "empty analytics address",
			mutate:  func(c *config.Config) { c.HTTP.AnalyticsAddr = "" },
			wantErr: "ANALYTICS_ADDR",
		},
		{
			name:    "zero body limit",
			mutate:  func(c *config.Config) { c.HTTP.MaxBodyBytes = 0 },
			wantErr: "HTTP_MAX_BODY_BYTES",
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
			name:    "zero flush interval",
			mutate:  func(c *config.Config) { c.Ingest.FlushIntervalMS = 0 },
			wantErr: "FLUSH_INTERVAL_MS",
		},
		{
			name:    "buffer smaller than batch",
			mutate:  func(c *config.Config) { c.Ingest.BufferSize = 10 },
			wantErr: "BUFFER_SIZE",
		},
		{
			name:    "no workers",
			mutate:  func(c *config.Config) { c.Ingest.Workers = 0 },
			wantErr: "INGEST_WORKERS",
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

// TestValidateNamesTheOffendingValue keeps the error messages useful: a reader who sees only
// the key learns nothing they did not already know from the file.
func TestValidateNamesTheOffendingValue(t *testing.T) {
	t.Parallel()

	cfg := valid()
	cfg.Ingest.InsertMode = "bulk"

	err := cfg.Validate()

	require.Error(t, err)
	require.Contains(t, err.Error(), `"bulk"`)
	require.Contains(t, err.Error(), "batch|single")
}
