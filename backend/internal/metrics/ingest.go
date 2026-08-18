package metrics

import "github.com/prometheus/client_golang/prometheus"

// The write-path counters validation feeds (PLAN.md 5.2 and 14.1).
//
// Every label on them is drawn from a closed set declared in internal/validate: a reason,
// a field name, a repair, a direction. None of them can take a value from a payload, which
// is the same rule that makes HTTP metrics use the route pattern instead of the raw path.
// site_id is the exception and is bounded by how many sites the deployment has.
var (
	eventsRejected = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "events_rejected_total",
			Help:      "Events refused by validation, by the reason returned to the client.",
		},
		[]string{"site", "reason"},
	)

	eventsFieldRepaired = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "events_field_repaired_total",
			Help:      "Event fields validation corrected instead of rejecting the event.",
		},
		[]string{"field", "repair"},
	)

	eventsClockSkew = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "events_clock_skew_total",
			Help:      "Event timestamps replaced with the server clock, by how the client clock was wrong.",
		},
		[]string{"direction"},
	)
)

func init() {
	Registry.MustRegister(eventsRejected, eventsFieldRepaired, eventsClockSkew)
}

// IngestObserver reports what validation did to the counters above.
//
// It satisfies validate.Observer without either package importing the other: the interface
// is declared where it is consumed, and both sides speak plain strings. That is what keeps
// the validation rules free of a Prometheus dependency and testable without a registry.
//
// It is stateless, so the zero value works and every method is safe for concurrent use —
// prometheus counters are.
type IngestObserver struct{}

// EventRejected records one event refused by validation.
func (IngestObserver) EventRejected(siteID, reason string) {
	eventsRejected.WithLabelValues(siteID, reason).Inc()
}

// FieldRepaired records one field corrected instead of the event being rejected.
func (IngestObserver) FieldRepaired(field, repair string) {
	eventsFieldRepaired.WithLabelValues(field, repair).Inc()
}

// ClockSkewed records one timestamp replaced with the server clock.
func (IngestObserver) ClockSkewed(direction string) {
	eventsClockSkew.WithLabelValues(direction).Inc()
}
