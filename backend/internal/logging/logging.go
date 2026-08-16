// Package logging builds the structured logger used by every binary.
//
// Structured logs only: no fmt.Println, no log.Printf. The JSON handler is the default so
// that logs are queryable in production; the text handler exists for readable local output.
package logging

import (
	"io"
	"log/slog"
	"os"

	"github.com/nxhawk/pulse-analytics/backend/internal/config"
)

// New builds a logger for the given service and installs it as the slog default, so that
// libraries calling slog.Info end up in the same stream.
func New(cfg *config.Config, service string) *slog.Logger {
	return NewTo(os.Stdout, cfg, service)
}

// NewTo is New with an explicit destination. Tests use it to capture output.
func NewTo(w io.Writer, cfg *config.Config, service string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.Log.SlogLevel()}

	var handler slog.Handler
	if cfg.Log.Format == "text" {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}

	logger := slog.New(handler).With(
		slog.String("service", service),
		slog.String("env", string(cfg.App.Env)),
	)
	slog.SetDefault(logger)
	return logger
}
