package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/nxhawk/pulse-analytics/backend/internal/config"
)

// Server wraps http.Server with the timeouts and shutdown behaviour every binary needs.
type Server struct {
	httpServer      *http.Server
	log             *slog.Logger
	shutdownTimeout time.Duration
	name            string
}

// NewServer builds a Server for the given handler and listen address.
func NewServer(name, addr string, handler http.Handler, cfg *config.Config, log *slog.Logger) *Server {
	return &Server{
		name: name,
		log:  log,
		httpServer: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
			ReadTimeout:       cfg.HTTP.ReadTimeout,
			WriteTimeout:      cfg.HTTP.WriteTimeout,
			IdleTimeout:       cfg.HTTP.IdleTimeout,
		},
		shutdownTimeout: cfg.App.ShutdownTimeout,
	}
}

// Addr reports the configured listen address.
func (s *Server) Addr() string { return s.httpServer.Addr }

// Run serves until SIGINT or SIGTERM arrives, then drains in-flight requests within the
// shutdown timeout. It returns the first error that actually matters; a clean shutdown
// returns nil.
//
// hooks run after the HTTP listener is closed and before Run returns — that is where a
// buffer flush or a Kafka producer close belongs, so no accepted event is lost on deploy.
func (s *Server) Run(ctx context.Context, hooks ...func(context.Context) error) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		s.log.Info("http server listening",
			slog.String("server", s.name),
			slog.String("addr", s.httpServer.Addr))
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listen on %s: %w", s.httpServer.Addr, err)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.log.Info("shutdown signal received",
			slog.String("server", s.name),
			slog.Duration("timeout", s.shutdownTimeout))
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.shutdownTimeout)
	defer cancel()

	var shutdownErr error
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		shutdownErr = fmt.Errorf("graceful shutdown: %w", err)
		s.log.Error("graceful shutdown failed", slog.String("error", err.Error()))
	}

	// Hooks run even when the HTTP shutdown timed out: flushing buffered events matters more
	// than a few connections that refused to close.
	for _, hook := range hooks {
		if err := hook(shutdownCtx); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
			s.log.Error("shutdown hook failed", slog.String("error", err.Error()))
		}
	}

	s.log.Info("server stopped", slog.String("server", s.name))
	return shutdownErr
}
