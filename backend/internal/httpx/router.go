package httpx

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/nxhawk/pulse-analytics/backend/internal/config"
	"github.com/nxhawk/pulse-analytics/backend/internal/metrics"
)

// operationalPaths are polled constantly by probes and scrapers, so they are excluded from
// the request log.
var operationalPaths = []string{"/healthz", "/readyz", "/metrics"}

// NewEngine builds a gin engine with the middleware chain every service shares, in the order
// that matters: request id first (so everything downstream can log it), then recovery (so a
// panic in the logger's own chain is still caught), then logging.
//
// The returned engine has no routes yet apart from /metrics; callers mount their own.
func NewEngine(cfg *config.Config, log *slog.Logger) *gin.Engine {
	if cfg.IsProduction() || cfg.App.Env == config.EnvTest {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.RedirectTrailingSlash = true
	engine.HandleMethodNotAllowed = true

	// Trusting no proxy by default is the safe choice: the client IP is only used for GeoIP
	// enrichment and rate limiting, and a spoofable X-Forwarded-For would poison both.
	// Caddy sits in front in production and is configured explicitly in deploy/caddy.
	_ = engine.SetTrustedProxies(nil)

	engine.Use(
		RequestID(),
		Recover(log),
		Logger(log, operationalPaths...),
		CORS(cfg),
	)

	engine.NoRoute(NotFoundHandler())
	engine.NoMethod(MethodNotAllowedHandler())

	engine.GET("/metrics", gin.WrapH(metrics.Handler()))

	return engine
}
