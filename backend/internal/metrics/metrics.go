// Package metrics owns the Prometheus registry.
//
// Level 0 registered only build info plus the Go and process collectors — enough to prove
// the scrape path works end to end. Level 1 adds the write-path counters that validation
// feeds (ingest.go); the rest of PLAN.md 14.1 arrives with the code that can populate it,
// task by task. Nothing else in the codebase should create a registry.
//
// One file per concern:
//
//	metrics.go   the registry, the namespace, build info and the scrape handler
//	ingest.go    the write-path counters and the Observer that feeds them
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/nxhawk/pulse-analytics/backend/internal/version"
)

// Namespace prefixes every metric this project exposes.
const Namespace = "pulse"

// Registry is the single registry the whole process uses. It is deliberately not
// prometheus.DefaultRegisterer: an explicit registry keeps third-party libraries from
// silently adding metrics.
var Registry = prometheus.NewRegistry()

// buildInfo lets dashboards annotate a graph with the exact build that produced it.
var buildInfo = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "build_info",
		Help:      "Build metadata of the running binary. The value is always 1.",
	},
	[]string{"service", "tag", "commit", "go_version"},
)

func init() {
	Registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		buildInfo,
	)
}

// SetBuildInfo publishes the build metadata for the given service.
func SetBuildInfo(service string) {
	v := version.Get()
	buildInfo.WithLabelValues(service, v.Tag, v.Commit, v.GoVersion).Set(1)
}

// Handler serves the Prometheus exposition format for Registry.
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{
		Registry:          Registry,
		EnableOpenMetrics: true,
	})
}
