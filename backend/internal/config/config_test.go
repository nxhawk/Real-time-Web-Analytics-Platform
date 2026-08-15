package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nxhawk/pulse-analytics/backend/internal/config"
)

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

// TestShippedFilesLoad parses every file in backend/config, which turns a YAML typo or a
// renamed struct field into a failing unit test rather than a crash on deploy.
func TestShippedFilesLoad(t *testing.T) {
	// Not parallel: these mutate the process environment and the working directory.
	tests := []struct {
		appEnv string
		// secrets are the placeholders the file deliberately leaves mandatory.
		secrets map[string]string
		wantEnv config.Environment
	}{
		{appEnv: "development", wantEnv: config.EnvDevelopment},
		{appEnv: "test", wantEnv: config.EnvTest},
		{
			appEnv:  "staging",
			secrets: map[string]string{"CLICKHOUSE_DSN": "clickhouse://ch:9000/analytics"},
			wantEnv: config.EnvStaging,
		},
		{
			appEnv: "production",
			secrets: map[string]string{
				"CLICKHOUSE_DSN":       "clickhouse://ch:9000/analytics",
				"CORS_ALLOWED_ORIGINS": "https://app.example",
			},
			wantEnv: config.EnvProduction,
		},
	}

	for _, tt := range tests {
		t.Run(tt.appEnv, func(t *testing.T) {
			dir, err := filepath.Abs(repoConfigDir)
			require.NoError(t, err)

			isolate(t)
			useConfigDir(t, tt.appEnv, dir)
			for name, value := range tt.secrets {
				t.Setenv(name, value)
			}

			cfg, err := config.Load()

			require.NoError(t, err)
			require.Equal(t, tt.wantEnv, cfg.App.Env)
			require.Equal(t, tt.appEnv == "production", cfg.IsProduction())
		})
	}
}

// TestProductionRequiresItsSecrets is the guarantee production.config.yml relies on: a
// placeholder with no fallback stops the process instead of defaulting to something that
// looks healthy and serves nothing.
func TestProductionRequiresItsSecrets(t *testing.T) {
	dir, err := filepath.Abs(repoConfigDir)
	require.NoError(t, err)

	isolate(t)
	useConfigDir(t, "production", dir)

	_, err = config.Load()

	require.Error(t, err)
	require.Contains(t, err.Error(), "CLICKHOUSE_DSN")
	require.Contains(t, err.Error(), "CORS_ALLOWED_ORIGINS")
}

// TestDevelopmentDefaultsNeedNoEnvironment is the other half: an unedited checkout with no
// .env at all still produces a usable configuration.
func TestDevelopmentDefaultsNeedNoEnvironment(t *testing.T) {
	dir, err := filepath.Abs(repoConfigDir)
	require.NoError(t, err)

	isolate(t)
	useConfigDir(t, "development", dir)

	cfg, err := config.Load()

	require.NoError(t, err)
	require.Equal(t, "30s", cfg.App.ShutdownTimeout.String())
	require.Equal(t, ":8080", cfg.HTTP.IngestAddr)
	require.Equal(t, ":8081", cfg.HTTP.AnalyticsAddr)
	require.Equal(t, int64(1<<20), cfg.HTTP.MaxBodyBytes)
	require.Equal(t, []string{"http://localhost:3000"}, cfg.HTTP.CORSAllowedOrigins)
	require.Equal(t, "15s", cfg.ClickHouse.QueryTimeout.String())
	require.Equal(t, 5000, cfg.Ingest.BatchSize)
	require.Equal(t, 500, cfg.Ingest.FlushIntervalMS)
	require.Equal(t, config.InsertModeBatch, cfg.Ingest.InsertMode)
	require.Equal(t, config.SinkDirect, cfg.Ingest.Sink)
	require.Empty(t, cfg.Kafka.Brokers)
}

// TestEnvironmentOverridesYAML covers the ${VAR} half of the contract, including the
// comma-separated list and the duration decoding.
func TestEnvironmentOverridesYAML(t *testing.T) {
	dir, err := filepath.Abs(repoConfigDir)
	require.NoError(t, err)

	isolate(t)
	useConfigDir(t, "development", dir)
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("SHUTDOWN_TIMEOUT", "5s")
	t.Setenv("BATCH_SIZE", "250")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://a.example,https://b.example")

	cfg, err := config.Load()

	require.NoError(t, err)
	require.Equal(t, ":9090", cfg.HTTP.IngestAddr)
	require.Equal(t, "debug", cfg.Log.Level)
	require.Equal(t, "5s", cfg.App.ShutdownTimeout.String())
	require.Equal(t, 250, cfg.Ingest.BatchSize)
	require.Equal(t, []string{"https://a.example", "https://b.example"}, cfg.HTTP.CORSAllowedOrigins)
}

// TestLoadFindsConfigByWalkingUp is the developer path: `make run` starts the binary from
// backend/, one level below the directory holding config/.
func TestLoadFindsConfigByWalkingUp(t *testing.T) {
	isolate(t)
	useConfig(t, "development", minimalYAML("development"))

	wd, err := os.Getwd()
	require.NoError(t, err)
	nested := filepath.Join(wd, "cmd", "ingest-api")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	t.Chdir(nested)

	cfg, err := config.Load()

	require.NoError(t, err)
	require.Equal(t, config.EnvDevelopment, cfg.App.Env)
}

func TestLoadReportsAMissingFile(t *testing.T) {
	isolate(t)
	t.Setenv("APP_ENV", "banana")

	_, err := config.Load()

	require.Error(t, err)
	require.Contains(t, err.Error(), "banana.config.yml")
}

// TestLoadRejectsMismatchedAppEnv catches a file copied to the wrong name.
func TestLoadRejectsMismatchedAppEnv(t *testing.T) {
	isolate(t)
	useConfig(t, "staging", minimalYAML("production"))

	_, err := config.Load()

	require.Error(t, err)
	require.Contains(t, err.Error(), "app.env")
}

func TestFlushInterval(t *testing.T) {
	t.Parallel()

	cfg := valid()

	require.Equal(t, "500ms", cfg.Ingest.FlushInterval().String())
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
