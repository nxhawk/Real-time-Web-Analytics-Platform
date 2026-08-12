package httpx

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/nxhawk/pulse-analytics/backend/internal/config"
)

// RequestID attaches a UUIDv7 to every request, echoes it in the response header, and makes
// it available to handlers and the logger. An inbound X-Request-ID is trusted and reused so
// that a trace survives a proxy hop.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if id == "" {
			if v7, err := uuid.NewV7(); err == nil {
				id = v7.String()
			} else {
				id = uuid.NewString()
			}
		}
		c.Set(ContextKeyRequestID, id)
		c.Writer.Header().Set(HeaderRequestID, id)
		c.Next()
	}
}

// Logger emits one structured line per request. Paths in skip are not logged: health and
// metrics endpoints are polled every few seconds and would drown out real traffic.
func Logger(log *slog.Logger, skip ...string) gin.HandlerFunc {
	skipped := make(map[string]struct{}, len(skip))
	for _, p := range skip {
		skipped[p] = struct{}{}
	}

	return func(c *gin.Context) {
		if _, ok := skipped[c.Request.URL.Path]; ok {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		// FullPath is the route pattern ("/api/v1/sites/:id"), not the raw path. Using the
		// pattern keeps metric and log cardinality bounded.
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}

		attrs := []any{
			slog.String("request_id", RequestIDFrom(c)),
			slog.String("method", c.Request.Method),
			slog.String("route", route),
			slog.Int("status", c.Writer.Status()),
			slog.Int("bytes", c.Writer.Size()),
			slog.Duration("duration", time.Since(start)),
		}
		if siteID := SiteIDFrom(c); siteID != "" {
			attrs = append(attrs, slog.String("site_id", siteID))
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("error", c.Errors.String()))
		}

		switch {
		case c.Writer.Status() >= http.StatusInternalServerError:
			log.Error("http request", attrs...)
		case c.Writer.Status() >= http.StatusBadRequest:
			log.Warn("http request", attrs...)
		default:
			log.Info("http request", attrs...)
		}
	}
}

// Recover turns a panic into a logged stack trace and a standard 500 envelope, so one bad
// request cannot take the process down.
func Recover(log *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
		log.Error("panic recovered",
			slog.String("request_id", RequestIDFrom(c)),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Any("panic", recovered),
			slog.String("stack", stackTrace()),
		)
		AbortWithError(c, http.StatusInternalServerError, CodeInternal, "internal server error")
	})
}

// CORS restricts browser access to the configured origins. Level 6 tightens this further:
// the ingest endpoint follows each site's registered origins (task L6-15).
//
// An empty CORS_ALLOWED_ORIGINS means "no cross-origin access", which is expressed by simply
// not emitting CORS headers. A single "*" allows every origin and therefore cannot be
// combined with credentials — the browser would reject that pair anyway.
func CORS(cfg *config.Config) gin.HandlerFunc {
	origins := nonEmpty(cfg.HTTP.CORSAllowedOrigins)
	if len(origins) == 0 {
		return func(c *gin.Context) { c.Next() }
	}

	allowAll := len(origins) == 1 && origins[0] == "*"

	conf := cors.Config{
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "X-API-Key", HeaderRequestID},
		ExposeHeaders:    []string{HeaderRequestID},
		AllowCredentials: !allowAll,
		MaxAge:           12 * time.Hour,
	}
	if allowAll {
		conf.AllowAllOrigins = true
	} else {
		conf.AllowOrigins = origins
	}

	return cors.New(conf)
}

// nonEmpty drops blank entries produced by a trailing comma in an environment variable.
func nonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// MaxBodySize rejects oversized payloads before they are read into memory.
func MaxBodySize(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > limit {
			AbortWithError(c, http.StatusRequestEntityTooLarge, CodePayloadTooLarge,
				"request body is too large")
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}
