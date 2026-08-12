package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/nxhawk/pulse-analytics/backend/internal/version"
)

// Prober reports whether one dependency is usable right now. ClickHouse implements it in
// Level 1 (task L1-13) and Kafka in Level 4; the health handler needs no change when they do.
type Prober interface {
	// Name identifies the dependency in the /readyz payload.
	Name() string
	// Check returns nil when the dependency is usable.
	Check(ctx context.Context) error
}

// probeTimeout bounds a single readiness check. Kubernetes-style probes give up quickly, so
// a slow dependency must not hold the response open.
const probeTimeout = 2 * time.Second

// HealthHandler serves the operational endpoints: liveness, readiness and build info.
type HealthHandler struct {
	probes []Prober
}

// NewHealthHandler builds the handler. With no probes, readiness only reports that the
// process is up — which is correct for a binary that has no dependencies yet.
func NewHealthHandler(probes ...Prober) *HealthHandler {
	return &HealthHandler{probes: probes}
}

// Register mounts the operational routes on the given router.
func (h *HealthHandler) Register(r gin.IRoutes) {
	r.GET("/healthz", h.Healthz)
	r.GET("/readyz", h.Readyz)
	r.GET("/version", h.Version)
}

// Healthz reports liveness: 200 for as long as the process is alive. It must never touch a
// dependency, otherwise an orchestrator would restart a healthy process when ClickHouse is
// briefly unavailable.
func (h *HealthHandler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// readyzResponse is the body of GET /readyz.
type readyzResponse struct {
	Status string            `json:"status"` // "ok" | "degraded"
	Checks map[string]string `json:"checks"` // dependency name -> "ok" or the error
}

// Readyz reports readiness: 200 when every dependency answers, 503 otherwise. A load
// balancer uses this to decide whether to send traffic here.
func (h *HealthHandler) Readyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), probeTimeout)
	defer cancel()

	resp := readyzResponse{Status: "ok", Checks: make(map[string]string, len(h.probes))}
	status := http.StatusOK

	for _, p := range h.probes {
		if err := p.Check(ctx); err != nil {
			resp.Checks[p.Name()] = err.Error()
			resp.Status = "degraded"
			status = http.StatusServiceUnavailable
			continue
		}
		resp.Checks[p.Name()] = "ok"
	}

	c.JSON(status, resp)
}

// Version returns the build metadata injected at link time.
func (h *HealthHandler) Version(c *gin.Context) {
	c.JSON(http.StatusOK, version.Get())
}
