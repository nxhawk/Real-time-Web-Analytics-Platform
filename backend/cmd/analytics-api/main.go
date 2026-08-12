// Command analytics-api is the read path: it answers dashboard queries from ClickHouse. It
// is CPU-heavy and scales independently of the ingest API (PLAN 2.3).
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

const serviceName = "analytics-api"

func main() {
	if err := run(context.Background()); err != nil {
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
	)

	// Level 1 wires the ClickHouse connection and the analytics repositories here.
	router := handler.NewAnalyticsRouter(cfg, log)

	server := httpx.NewServer(serviceName, cfg.HTTP.AnalyticsAddr, router, cfg, log)
	return server.Run(ctx)
}
