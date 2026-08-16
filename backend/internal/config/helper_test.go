package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nxhawk/pulse-analytics/backend/internal/config"
)

// The test files mirror the package layout: load_test.go covers load.go and expand.go,
// validate_test.go covers Validate and the per-section rules, and the per-section files
// cover behaviour that lives next to a single struct. Shared fixtures live in this file so
// that none of them owns state the others depend on.

// repoConfigDir is backend/config relative to this package. `go test` runs with the package
// directory as the working directory, so this path is stable however the suite is invoked.
const repoConfigDir = "../../config"

// placeholderVars is every variable the shipped YAML files interpolate. Tests clear them so
// results do not depend on what the developer happens to have exported.
var placeholderVars = []string{
	"APP_ENV", "SHUTDOWN_TIMEOUT",
	"HTTP_ADDR", "ANALYTICS_ADDR", "HTTP_READ_HEADER_TIMEOUT", "HTTP_READ_TIMEOUT",
	"HTTP_WRITE_TIMEOUT", "HTTP_IDLE_TIMEOUT", "HTTP_MAX_BODY_BYTES", "CORS_ALLOWED_ORIGINS",
	"LOG_LEVEL", "LOG_FORMAT",
	"CLICKHOUSE_DSN", "CLICKHOUSE_MAX_OPEN_CONNS", "CLICKHOUSE_MAX_IDLE_CONNS",
	"CLICKHOUSE_CONN_MAX_LIFETIME", "CLICKHOUSE_DIAL_TIMEOUT", "CLICKHOUSE_QUERY_TIMEOUT",
	"SINK", "INSERT_MODE", "BATCH_SIZE", "FLUSH_INTERVAL_MS", "BUFFER_SIZE",
	"INGEST_WORKERS", "WAL_DIR", "MAX_EVENTS_PER_REQUEST", "INGEST_RATE_LIMIT_PER_MIN",
	"KAFKA_BROKERS", "KAFKA_TOPIC_RAW", "KAFKA_TOPIC_DLQ", "KAFKA_GROUP_ID",
	"KAFKA_CONSUMER_BATCH_SIZE",
	"CONFIG_DIR", "ENV_FILE",
}

// isolate gives the test a clean environment and an empty working directory, so neither an
// exported variable nor the repository's own .env can leak into the result. It returns
// nothing: callers point Load at a configuration directory with useConfigDir or useConfig.
func isolate(t *testing.T) {
	t.Helper()

	for _, name := range placeholderVars {
		if old, ok := os.LookupEnv(name); ok {
			t.Setenv(name, old) // registers the restore
			require.NoError(t, os.Unsetenv(name))
		}
	}
	t.Chdir(t.TempDir())
}

// useConfigDir points Load at an existing directory of configuration files.
func useConfigDir(t *testing.T, appEnv, dir string) {
	t.Helper()

	abs, err := filepath.Abs(dir)
	require.NoError(t, err)
	t.Setenv("CONFIG_DIR", abs)
	t.Setenv("APP_ENV", appEnv)
}

// useConfig writes body as config/<appEnv>.config.yml under the working directory, so that
// Load has to find it by walking up from the working directory — the path a developer
// running `make run` takes.
func useConfig(t *testing.T, appEnv, body string) {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)
	dir := filepath.Join(wd, "config")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, appEnv+".config.yml"), []byte(body), 0o600))

	t.Setenv("APP_ENV", appEnv)
}

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

// minimalYAML is the smallest file that passes validation, for tests about file lookup
// rather than about the contents of the shipped configuration.
func minimalYAML(appEnv string) string {
	return `
app:
  env: "` + appEnv + `"
  shutdown_timeout: "30s"
http:
  ingest_addr: ":8080"
  analytics_addr: ":8081"
  max_body_bytes: 1048576
log:
  level: "info"
  format: "json"
clickhouse:
  dsn: "clickhouse://localhost:9000/analytics"
ingest:
  sink: "direct"
  insert_mode: "batch"
  batch_size: 5000
  flush_interval_ms: 500
  buffer_size: 100000
  workers: 4
  max_events_per_request: 500
`
}
