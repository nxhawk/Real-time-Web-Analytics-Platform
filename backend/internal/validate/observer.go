package validate

// Field names the part of an event a repair touched.
//
// The set is closed because it becomes a Prometheus label. Passing the field name through
// from the payload would let a client open a metric series per request, which is the same
// mistake as labelling a metric with a raw URL path instead of a route pattern.
type Field string

// The fields validation can repair.
const (
	FieldEventID     Field = "event_id"
	FieldTimestamp   Field = "timestamp"
	FieldUserID      Field = "user_id"
	FieldSessionID   Field = "session_id"
	FieldPage        Field = "page"
	FieldReferrer    Field = "referrer"
	FieldUTMSource   Field = "utm_source"
	FieldUTMMedium   Field = "utm_medium"
	FieldUTMCampaign Field = "utm_campaign"
	FieldCountry     Field = "country"
	FieldCity        Field = "city"
	FieldDevice      Field = "device"
	FieldOS          Field = "os"
	FieldBrowser     Field = "browser"
	FieldCurrency    Field = "currency"
)

// Repair names what validation did to a field instead of rejecting the event. Like Field,
// the set is closed because it is a label.
type Repair string

// The repairs validation can apply.
const (
	// RepairTruncated means the value was longer than its column allows and was cut.
	RepairTruncated Repair = "truncated"
	// RepairStripped means part of the value was removed: a fragment, or a query parameter
	// carrying a credential.
	RepairStripped Repair = "stripped"
	// RepairDefaulted means the value was missing and the server supplied one.
	RepairDefaulted Repair = "defaulted"
	// RepairNormalised means the value was reshaped into the column's vocabulary, as an
	// unrecognised device class becomes "unknown".
	RepairNormalised Repair = "normalised"
)

// SkewDirection says which way a client clock was wrong.
type SkewDirection string

// The directions a clock can be wrong in.
const (
	SkewFuture SkewDirection = "future"
	SkewPast   SkewDirection = "past"
)

// Observer receives what validation did.
//
// It exists so that validation stays a pure function of its input — no package-level
// counters, no Prometheus import — while the numbers PLAN.md 5.2 and 14.1 require still get
// recorded. internal/metrics supplies the production implementation and cmd/ wires it in.
//
// The methods take plain strings rather than the types above so that neither package has to
// import the other: the values are bounded where they are produced, which is the only place
// that can guarantee it. Callers pass string(FieldPage) and so on.
//
// Implementations must be safe for concurrent use — the ingest handler validates from every
// request goroutine.
type Observer interface {
	// EventRejected reports one event refused by validation. Feeds
	// pulse_events_rejected_total{site,reason}.
	EventRejected(siteID, reason string)

	// FieldRepaired reports one field corrected instead of the event being rejected. Feeds
	// pulse_events_field_repaired_total{field,repair}.
	FieldRepaired(field, repair string)

	// ClockSkewed reports a timestamp replaced with the server clock. Feeds
	// pulse_events_clock_skew_total{direction}.
	ClockSkewed(direction string)
}

// nopObserver is the default, so that neither the Validator nor a caller needs a nil check
// on the hot path.
type nopObserver struct{}

func (nopObserver) EventRejected(_, _ string) {}
func (nopObserver) FieldRepaired(_, _ string) {}
func (nopObserver) ClockSkewed(_ string)      {}
