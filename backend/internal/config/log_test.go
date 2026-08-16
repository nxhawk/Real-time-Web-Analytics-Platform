package config_test

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nxhawk/pulse-analytics/backend/internal/config"
)

func TestSlogLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		level string
		want  slog.Level
	}{
		{level: "debug", want: slog.LevelDebug},
		{level: "info", want: slog.LevelInfo},
		{level: "warn", want: slog.LevelWarn},
		{level: "error", want: slog.LevelError},
		{level: "WARN", want: slog.LevelWarn},   // levels are matched case-insensitively
		{level: "", want: slog.LevelInfo},       // a zero Config still logs something
		{level: "chatty", want: slog.LevelInfo}, // unreachable through Load; guarded anyway
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, config.Log{Level: tt.level}.SlogLevel())
		})
	}
}
