// Command ingest-api is the write path: it accepts events over HTTP and hands them to the
// configured sink. It is I/O-heavy and scales independently of the analytics API (PLAN 2.3).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/nxhawk/pulse-analytics/backend/internal/config"
	"github.com/nxhawk/pulse-analytics/backend/internal/handler"
	"github.com/nxhawk/pulse-analytics/backend/internal/httpx"
	"github.com/nxhawk/pulse-analytics/backend/internal/logging"
	"github.com/nxhawk/pulse-analytics/backend/internal/metrics"
	"github.com/nxhawk/pulse-analytics/backend/internal/version"
)

const serviceName = "ingest-api"

func main() {
	if err := run(context.Background()); err != nil {
		// The logger may not exist yet when configuration fails, so report to stderr too.
		fmt.Fprintf(os.Stderr, "%s: %v\n", serviceName, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logging.New(cfg, serviceName)
	metrics.SetBuildInfo(serviceName)

	v := version.Get()
	log.Info("starting",
		slog.String("tag", v.Tag),
		slog.String("commit", v.Commit),
		slog.String("build_time", v.BuildTime),
		slog.String("go", v.GoVersion),
		slog.String("sink", string(cfg.Ingest.Sink)),
		slog.String("insert_mode", string(cfg.Ingest.InsertMode)),
	)

	// Dependencies are wired here and nowhere else. Level 1 opens the ClickHouse connection
	// (task L1-12) and passes it as a readiness probe; Level 3 adds the batch writer as a
	// shutdown hook so buffered events are flushed on deploy.
	router := handler.NewIngestRouter(cfg, log)

	server := httpx.NewServer(serviceName, cfg.HTTP.IngestAddr, router, cfg, log)
	return server.Run(ctx)
}
