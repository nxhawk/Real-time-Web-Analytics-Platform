package config

import (
	"log/slog"
	"slices"
	"strings"
)

// Log holds logging settings.
type Log struct {
	Level  string `mapstructure:"level"`  // debug | info | warn | error
	Format string `mapstructure:"format"` // json | text
}

// validate implements section.
func (l Log) validate(p *problems) {
	if !slices.Contains(logLevels, strings.ToLower(l.Level)) {
		p.addf("log.level (LOG_LEVEL) must be one of %s, got %q", oneOf(logLevels), l.Level)
	}
	if !slices.Contains(logFormats, l.Format) {
		p.addf("log.format (LOG_FORMAT) must be %s, got %q", oneOf(logFormats), l.Format)
	}
}

// SlogLevel maps the configured level onto slog's level type. An unrecognised level cannot
// reach here through Load — Validate rejects it first — so the default is only a guard for
// a hand-built Config in a test.
func (l Log) SlogLevel() slog.Level {
	switch strings.ToLower(l.Level) {
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
