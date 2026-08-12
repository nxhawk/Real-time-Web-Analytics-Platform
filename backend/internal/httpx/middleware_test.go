package httpx_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/nxhawk/pulse-analytics/backend/internal/config"
	"github.com/nxhawk/pulse-analytics/backend/internal/httpx"
)

func init() { gin.SetMode(gin.TestMode) }

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func bufferLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestRecoverTurnsPanicInto500(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	r := gin.New()
	r.Use(httpx.RequestID(), httpx.Recover(bufferLogger(&logs)))
	r.GET("/boom", func(*gin.Context) { panic("kaboom") })

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/boom", http.NoBody)
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)

	var body httpx.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, httpx.CodeInternal, body.Error.Code)
	require.NotEmpty(t, body.RequestID)

	require.Contains(t, logs.String(), "panic recovered")
	require.Contains(t, logs.String(), "kaboom")
}

func TestMaxBodySizeRejectsOversizedPayload(t *testing.T) {
	t.Parallel()

	const limit = 16

	r := gin.New()
	r.Use(httpx.RequestID(), httpx.Recover(discardLogger()), httpx.MaxBodySize(limit))
	r.POST("/events", func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			httpx.AbortWithError(c, http.StatusRequestEntityTooLarge,
				httpx.CodePayloadTooLarge, "request body is too large")
			return
		}
		c.Status(http.StatusAccepted)
	})

	t.Run("within the limit", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/events",
			strings.NewReader(`{"a":1}`))
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusAccepted, rec.Code)
	})

	t.Run("declared content length above the limit", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/events",
			strings.NewReader(strings.Repeat("x", limit*4)))
		r.ServeHTTP(rec, req)

		require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)

		var body httpx.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Equal(t, httpx.CodePayloadTooLarge, body.Error.Code)
	})
}

func TestLoggerSkipsOperationalPaths(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	r := gin.New()
	r.Use(httpx.RequestID(), httpx.Logger(bufferLogger(&logs), "/healthz"))
	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/other", func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", http.NoBody))
	require.Empty(t, logs.String(), "health checks must not be logged")

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/other", http.NoBody))
	require.Contains(t, logs.String(), `"route":"/other"`)
	require.Contains(t, logs.String(), `"request_id"`)
}

func TestRequestIDIsAvailableToHandlers(t *testing.T) {
	t.Parallel()

	var seen string

	r := gin.New()
	r.Use(httpx.RequestID())
	r.GET("/", func(c *gin.Context) {
		seen = httpx.RequestIDFrom(c)
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody))

	require.NotEmpty(t, seen)
	require.Equal(t, seen, rec.Header().Get(httpx.HeaderRequestID))
}

func TestCORSWithoutConfiguredOriginsIsANoop(t *testing.T) {
	t.Parallel()

	// An empty CORS_ALLOWED_ORIGINS must not panic the process at startup, and must simply
	// not emit CORS headers.
	r := gin.New()
	r.Use(httpx.CORS(&testCORSConfig))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	req.Header.Set("Origin", "https://evil.example")
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

// testCORSConfig has no allowed origins on purpose.
var testCORSConfig = config.Config{
	App:  config.App{Env: config.EnvTest},
	HTTP: config.HTTP{},
	Log:  config.Log{Level: "error", Format: "json"},
}
