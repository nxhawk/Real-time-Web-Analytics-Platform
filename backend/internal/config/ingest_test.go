package config_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nxhawk/pulse-analytics/backend/internal/config"
)

func TestFlushInterval(t *testing.T) {
	t.Parallel()

	cfg := valid()

	require.Equal(t, "500ms", cfg.Ingest.FlushInterval().String())
}

// TestSensitiveQueryParams covers the one knob whose misuse is silent. A query string pasted
// where a list of names belongs would match nothing and strip nothing, while looking to the
// operator exactly like a configured privacy control.
func TestSensitiveQueryParams(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		params []string
		want   string // the substring the error must contain; empty means valid
	}{
		{name: "unset", params: nil},
		{name: "a list of names", params: []string{"code", "ref", "state"}},
		{name: "a whole query string", params: []string{"?token=x"}, want: "not a query string"},
		{name: "a key=value pair", params: []string{"code=abc"}, want: "not a query string"},
		{name: "two names joined by an ampersand", params: []string{"code&ref"}, want: "not a query string"},
		{name: "a fragment", params: []string{"ref#top"}, want: "not a query string"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := valid()
			cfg.Ingest.SensitiveQueryParams = tc.params

			err := cfg.Validate()

			if tc.want == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestShippedFilesDeclareTheDenylistKey guards the rule in CLAUDE.md that a knob is added to
// all four configuration files or none: a key present in three of them is a setting that
// works everywhere except the one environment nobody tested.
func TestShippedFilesDeclareTheDenylistKey(t *testing.T) {
	// Not parallel: these mutate the process environment and the working directory.
	for _, appEnv := range []string{"development", "staging", "production", "test"} {
		t.Run(appEnv, func(t *testing.T) {
			// Resolved before isolate, which moves the working directory the relative path
			// would otherwise be taken from.
			dir, err := filepath.Abs(repoConfigDir)
			require.NoError(t, err)

			isolate(t)
			useConfigDir(t, appEnv, dir)
			// The placeholders the stricter files leave mandatory.
			t.Setenv("CLICKHOUSE_DSN", "clickhouse://ch:9000/analytics")
			t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example")
			t.Setenv("SENSITIVE_QUERY_PARAMS", "invite_code,ref")

			cfg, err := config.Load()

			require.NoError(t, err)
			assert.Equal(t, []string{"invite_code", "ref"}, cfg.Ingest.SensitiveQueryParams)
		})
	}
}
