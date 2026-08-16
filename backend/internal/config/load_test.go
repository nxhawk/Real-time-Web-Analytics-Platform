package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nxhawk/pulse-analytics/backend/internal/config"
)

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
