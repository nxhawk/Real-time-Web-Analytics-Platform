package model

import "slices"

// RejectReason is the machine-readable cause of a rejected event.
//
// The set is closed for two reasons. It is returned to clients inside the 202 body, where
// an SDK switches on it to decide whether to fix the payload or drop the event (PLAN.md
// 5.2). And it is the reason label of pulse_events_rejected_total (PLAN.md 14.1), where an
// open set would let a payload create metric series.
type RejectReason string

// ReasonNone is the zero RejectReason and means the event passed. It is never serialised:
// an event with no reason is an accepted event and does not appear in `rejected`.
const ReasonNone RejectReason = ""

// The reasons an event can be refused. Each one names the single field at fault, so a
// client can act on it without parsing a message.
const (
	// ReasonMalformedEvent means the element was not a decodable JSON object. It is the
	// only reason that fires before any field is looked at.
	ReasonMalformedEvent RejectReason = "malformed_event"

	ReasonMissingEventName RejectReason = "missing_event_name"
	ReasonInvalidEventName RejectReason = "invalid_event_name"
	ReasonInvalidEventID   RejectReason = "invalid_event_id"
	ReasonInvalidTimestamp RejectReason = "invalid_timestamp"

	ReasonInvalidProperties  RejectReason = "invalid_properties"
	ReasonPropertiesTooLarge RejectReason = "properties_too_large"

	ReasonInvalidRevenue RejectReason = "invalid_revenue"
)

// rejectReasons is every reason that can reach a client, in contract order. ReasonNone is
// absent by design: it is the absence of a reason, not one of them.
var rejectReasons = []RejectReason{
	ReasonMalformedEvent,
	ReasonMissingEventName,
	ReasonInvalidEventName,
	ReasonInvalidEventID,
	ReasonInvalidTimestamp,
	ReasonInvalidProperties,
	ReasonPropertiesTooLarge,
	ReasonInvalidRevenue,
}

// Valid reports whether r is a reason this build can produce.
func (r RejectReason) Valid() bool { return slices.Contains(rejectReasons, r) }

// String makes RejectReason printable and gives metric code one conversion instead of
// many.
func (r RejectReason) String() string { return string(r) }

// RejectReasons returns the closed set of reasons, in contract order. The result is a copy
// so that a caller enumerating them — the OpenAPI generator, a metrics pre-registration —
// cannot alter the contract.
func RejectReasons() []RejectReason { return slices.Clone(rejectReasons) }
