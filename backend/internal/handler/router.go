// Package handler contains the HTTP layer: it decodes requests, calls a service, and encodes
// the result. Business rules belong in internal/service, storage access in
// internal/repository — a handler should stay thin enough to read in one screen.
package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/nxhawk/pulse-analytics/backend/internal/config"
	"github.com/nxhawk/pulse-analytics/backend/internal/httpx"
)

// APIBasePath is the version prefix shared by both services. Bumping the API version means
// mounting a second group here, never breaking this one.
const APIBasePath = "/api/v1"

// NewIngestRouter builds the router of the ingest API: the write path.
//
// Level 1 mounts POST /events and GET /pixel.gif on the returned v1 group (tasks L1-17,
// L1-20) behind the API key middleware (task L1-19).
func NewIngestRouter(cfg *config.Config, log *slog.Logger, probes ...Prober) *gin.Engine {
	engine := httpx.NewEngine(cfg, log)

	NewHealthHandler(probes...).Register(engine)

	v1 := engine.Group(APIBasePath)
	v1.Use(httpx.MaxBodySize(cfg.HTTP.MaxBodyBytes))
	_ = v1 // routes are mounted in Level 1

	return engine
}

// NewAnalyticsRouter builds the router of the analytics API: the read path.
//
// Level 1 mounts /analytics/overview, /timeseries, /pages, /devices and /countries on the
// returned group (tasks L1-25 to L1-28).
func NewAnalyticsRouter(cfg *config.Config, log *slog.Logger, probes ...Prober) *gin.Engine {
	engine := httpx.NewEngine(cfg, log)

	NewHealthHandler(probes...).Register(engine)

	v1 := engine.Group(APIBasePath)
	_ = v1 // routes are mounted in Level 1

	return engine
}
