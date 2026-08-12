package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nxhawk/pulse-analytics/backend/internal/config"
	"github.com/nxhawk/pulse-analytics/backend/internal/handler"
	"github.com/nxhawk/pulse-analytics/backend/internal/httpx"
)

// stubProbe is a Prober whose result the test controls.
type stubProbe struct {
	name string
	err  error
}

func (s stubProbe) Name() string                { return s.name }
func (s stubProbe) Check(context.Context) error { return s.err }

func testConfig() *config.Config {
	return &config.Config{
		App:  config.App{Env: config.EnvTest, ShutdownTimeout: 30_000_000_000},
		HTTP: config.HTTP{IngestAddr: ":0", AnalyticsAddr: ":0", MaxBodyBytes: 1 << 20},
		Log:  config.Log{Level: "error", Format: "json"},
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func do(t *testing.T, router http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), method, path, http.NoBody)
	router.ServeHTTP(rec, req)
	return rec
}

func TestHealthz(t *testing.T) {
	t.Parallel()

	router := handler.NewIngestRouter(testConfig(), testLogger())

	rec := do(t, router, http.MethodGet, "/healthz")

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

func TestHealthzIgnoresBrokenDependencies(t *testing.T) {
	t.Parallel()

	router := handler.NewIngestRouter(testConfig(), testLogger(),
		stubProbe{name: "clickhouse", err: errors.New("connection refused")})

	rec := do(t, router, http.MethodGet, "/healthz")

	// Liveness must not depend on storage, otherwise an orchestrator restarts a healthy
	// process every time ClickHouse blinks.
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestReadyzWithoutProbes(t *testing.T) {
	t.Parallel()

	router := handler.NewIngestRouter(testConfig(), testLogger())

	rec := do(t, router, http.MethodGet, "/readyz")

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestReadyzHealthyProbe(t *testing.T) {
	t.Parallel()

	router := handler.NewIngestRouter(testConfig(), testLogger(),
		stubProbe{name: "clickhouse"})

	rec := do(t, router, http.MethodGet, "/readyz")

	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "ok", body.Status)
	require.Equal(t, "ok", body.Checks["clickhouse"])
}

func TestReadyzFailingProbe(t *testing.T) {
	t.Parallel()

	router := handler.NewIngestRouter(testConfig(), testLogger(),
		stubProbe{name: "clickhouse", err: errors.New("connection refused")},
		stubProbe{name: "kafka"})

	rec := do(t, router, http.MethodGet, "/readyz")

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "degraded", body.Status)
	require.Contains(t, body.Checks["clickhouse"], "connection refused")
	require.Equal(t, "ok", body.Checks["kafka"])
}

func TestVersion(t *testing.T) {
	t.Parallel()

	router := handler.NewIngestRouter(testConfig(), testLogger())

	rec := do(t, router, http.MethodGet, "/version")

	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Contains(t, body, "commit")
	require.Contains(t, body, "build_time")
	require.NotEmpty(t, body["go_version"])
}

func TestMetricsEndpoint(t *testing.T) {
	t.Parallel()

	router := handler.NewAnalyticsRouter(testConfig(), testLogger())

	rec := do(t, router, http.MethodGet, "/metrics")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "go_goroutines")
}

func TestRequestIDIsEchoed(t *testing.T) {
	t.Parallel()

	router := handler.NewIngestRouter(testConfig(), testLogger())

	rec := do(t, router, http.MethodGet, "/healthz")
	generated := rec.Header().Get(httpx.HeaderRequestID)
	require.NotEmpty(t, generated)

	// An inbound id is reused so a trace survives a proxy hop.
	rec2 := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", http.NoBody)
	req.Header.Set(httpx.HeaderRequestID, "01JABCDEF")
	router.ServeHTTP(rec2, req)

	require.Equal(t, "01JABCDEF", rec2.Header().Get(httpx.HeaderRequestID))
}

func TestUnknownRouteUsesErrorEnvelope(t *testing.T) {
	t.Parallel()

	router := handler.NewIngestRouter(testConfig(), testLogger())

	rec := do(t, router, http.MethodGet, "/nope")

	require.Equal(t, http.StatusNotFound, rec.Code)

	var body httpx.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, httpx.CodeNotFound, body.Error.Code)
	require.NotEmpty(t, body.RequestID)
}

func TestMethodNotAllowedUsesErrorEnvelope(t *testing.T) {
	t.Parallel()

	router := handler.NewIngestRouter(testConfig(), testLogger())

	rec := do(t, router, http.MethodPost, "/healthz")

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)

	var body httpx.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, httpx.CodeNotFound, body.Error.Code)
}
